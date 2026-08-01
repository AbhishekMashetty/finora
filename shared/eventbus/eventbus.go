// Package eventbus wraps NATS JetStream, giving every Finora service one
// consistent way to publish and consume domain events (Phase 7 — see
// CLAUDE.md §2 and architecture/development-roadmap.md's Phase 7 entry).
//
// JetStream, not core NATS pub/sub, is deliberate: core NATS is
// fire-and-forget with no persistence — a subscriber that's down
// (deploying, restarting, crashed) simply never sees anything published
// while it was unavailable. JetStream adds message persistence (a stream
// retains events so a (re)started consumer can catch up) and durable
// consumers (server-side tracking of exactly where each named consumer
// left off, so a process restart resumes instead of replaying everything
// or losing its place) — both are exactly what the roadmap's "keep
// delivery reliable" goal requires, and neither exists in core NATS.
package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Bus wraps a NATS connection and its JetStream context — the one type
// every publisher/consumer in this fleet is built from.
type Bus struct {
	conn *nats.Conn
	js   jetstream.JetStream
}

// Connect dials url and wraps it as a Bus. A bounded connect timeout plus
// unlimited automatic reconnects (nats.go's own background reconnect
// logic, not a loop this package writes) means a transient NATS outage
// after startup self-heals; a NATS that's unreachable at startup fails
// fast here, matching mongox.Connect's same fail-fast-on-bad-URI
// philosophy — every service that publishes or consumes events needs NATS
// to do its job, so a slow/failed initial connect should surface
// immediately, not hang.
func Connect(url string) (*Bus, error) {
	conn, err := nats.Connect(url, nats.Timeout(10*time.Second), nats.MaxReconnects(-1))
	if err != nil {
		return nil, fmt.Errorf("eventbus: connect: %w", err)
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("eventbus: jetstream: %w", err)
	}
	return &Bus{conn: conn, js: js}, nil
}

// Close drains and closes the underlying NATS connection.
func (b *Bus) Close() {
	b.conn.Close()
}

// Checker adapts a *Bus into a health.Checker (shared/health), so every
// service that depends on NATS reports it on /ready — exactly like
// mongox.Checker already does for MongoDB.
type Checker struct {
	Bus *Bus
}

func (c Checker) Name() string { return "nats" }

func (c Checker) Check(_ context.Context) error {
	if !c.Bus.conn.IsConnected() {
		return fmt.Errorf("nats: not connected (status: %s)", c.Bus.conn.Status())
	}
	return nil
}

// EnsureStream creates (or, if it already exists, updates in place) a
// JetStream stream capturing subjects. Idempotent and safe to call from
// every service that touches this stream at its own startup — a stream is
// shared infrastructure (e.g. expense-service, the producer, and
// budget-service, a consumer, both depend on the same stream existing)
// rather than something only one designated owner provisions once.
func (b *Bus) EnsureStream(ctx context.Context, name string, subjects []string) error {
	_, err := b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      name,
		Subjects:  subjects,
		Retention: jetstream.LimitsPolicy,
		MaxAge:    30 * 24 * time.Hour, // generous headroom for a consumer that's been down a while; not an archival/compliance policy
	})
	if err != nil {
		return fmt.Errorf("eventbus: ensure stream %s: %w", name, err)
	}
	return nil
}

// Publish sends data on subject. msgID, if non-empty, is used as
// JetStream's de-duplication key: publishing the same msgID twice within
// the server's dedup window (2 minutes by default) is collapsed into a
// single stored message. This is what lets a producer safely retry a
// publish (e.g. after a network blip left it unsure whether the first
// attempt landed) without risking a consumer seeing the same logical event
// twice — with zero application-side idempotency bookkeeping. Pass "" for
// events where a duplicate is harmless or the producer never retries.
func (b *Bus) Publish(ctx context.Context, subject string, data []byte, msgID string) error {
	msg := &nats.Msg{Subject: subject, Data: data}
	var opts []jetstream.PublishOpt
	if msgID != "" {
		opts = append(opts, jetstream.WithMsgID(msgID))
	}
	if _, err := b.js.PublishMsg(ctx, msg, opts...); err != nil {
		return fmt.Errorf("eventbus: publish %s: %w", subject, err)
	}
	return nil
}

// Handler processes one delivered message. Returning a non-nil error means
// "retry later" — the message is Nak'd and JetStream redelivers it later
// (subject to the consumer's own MaxDeliver bound); returning nil acks it,
// and JetStream never redelivers an acked message to this durable
// consumer.
type Handler func(ctx context.Context, subject string, data []byte) error

// Subscribe creates (or resumes) a durable JetStream consumer named
// durableName on the stream streamName, filtered to filterSubject, and
// runs handler for every message it delivers — blocking until ctx is
// cancelled. "Durable" is the property that matters: if this process
// restarts, the SAME durable name resumes exactly where JetStream last
// recorded it left off, rather than starting over (missing nothing
// published while the process was down) or replaying everything from
// scratch (double-processing what it already handled) — this durability is
// what makes Phase 7's delivery guarantee real, not just a
// same-process convenience.
//
// Intended to be started once per subject a service consumes, each in its
// own goroutine at startup — see cmd/server/main.go in budget-service and
// notification-service.
func (b *Bus) Subscribe(ctx context.Context, streamName, durableName, filterSubject string, handler Handler) error {
	stream, err := b.js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("eventbus: stream %s: %w", streamName, err)
	}
	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       durableName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: filterSubject,
		// A bounded retry count, not infinite: a genuinely unprocessable
		// message (a bug, a malformed payload) must not retry forever and
		// silently monopolize the consumer — same defense-in-depth
		// instinct as this codebase's other explicit ceilings
		// (transaction-service's maxAmount, the CSV importer's
		// maxImportRows).
		MaxDeliver: 10,
	})
	if err != nil {
		return fmt.Errorf("eventbus: consumer %s: %w", durableName, err)
	}

	consumeCtx, err := cons.Consume(func(msg jetstream.Msg) {
		if err := handler(ctx, msg.Subject(), msg.Data()); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("eventbus: consume %s: %w", durableName, err)
	}
	defer consumeCtx.Stop()

	<-ctx.Done()
	return nil
}
