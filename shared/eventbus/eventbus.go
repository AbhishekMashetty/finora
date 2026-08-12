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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// defaultMaxDeliver bounds how many times JetStream will redeliver a
	// message this fleet's consumers never manage to Ack — a genuinely
	// unprocessable message (a bug, a malformed payload) must not retry
	// forever and silently monopolize the consumer.
	defaultMaxDeliver = 10

	// defaultAckWait must exceed the slowest realistic handler call —
	// otherwise JetStream treats a still-in-progress message as timed out
	// and redelivers it while the first attempt is still running.
	defaultAckWait = 30 * time.Second

	// defaultMaxAckPending bounds how many delivered-but-unacked messages
	// JetStream will let this consumer hold at once — a bulkhead against
	// one durable consumer's backlog consuming unbounded server-side
	// tracking state, and the ceiling handlerConcurrency below stays under.
	defaultMaxAckPending = 256

	// handlerConcurrency bounds how many messages this process's Subscribe
	// loop hands to Handler at once. The underlying NATS client delivers
	// messages to Consume's callback serially (one JetStream subscription
	// = one dispatch goroutine), so without this a slow handler pins
	// throughput to 1/handler_latency regardless of MaxAckPending; this
	// fans each delivered message out to its own goroutine, bounded by a
	// semaphore, so handler latency and consumer throughput are no longer
	// the same number.
	handlerConcurrency = 32
)

// defaultBackoff is the redelivery delay schedule for a Handler that
// returns a retryable error, indexed by delivery attempt (1st failed
// attempt uses index 0, growing more patient on each subsequent failure).
// Deliberately NOT the consumer's AckWait/BackOff config: nats.go's
// Msg.Nak() "does not adhere to AckWait or Backoff configured on the
// consumer and triggers instant redelivery" (see its own doc comment) —
// only NakWithDelay(d) actually applies a delay, which is why Subscribe
// computes d itself rather than relying on ConsumerConfig.BackOff to do
// it. Without this, a handler failing because a dependency is down
// hot-retries every message at whatever rate NATS can redeliver it,
// turning a partial outage into a self-inflicted retry storm against the
// exact dependency that's already struggling.
var defaultBackoff = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	15 * time.Second,
	30 * time.Second,
	1 * time.Minute,
	5 * time.Minute,
}

func backoffForAttempt(numDelivered uint64) time.Duration {
	idx := int(numDelivered) - 1 // NumDelivered is 1 on the first delivery
	if idx < 0 {
		idx = 0
	}
	if idx >= len(defaultBackoff) {
		idx = len(defaultBackoff) - 1
	}
	return defaultBackoff[idx]
}

// ErrTerminal marks a Handler error as permanent: retrying it, no matter
// how many times or how long the wait, cannot succeed (a payload that will
// never parse, a business rule that will never become true). Wrap it with
// fmt.Errorf("%w: ...", eventbus.ErrTerminal) or return it directly.
// Subscribe treats a terminal error, and a retryable error that has
// exhausted defaultMaxDeliver attempts, the same way: Term() the message
// (never redelivered again, regardless of MaxDeliver) and publish it to a
// dead-letter subject so the failure is an observable event, not a
// message that silently stops existing.
var ErrTerminal = errors.New("eventbus: terminal handler error, do not retry")

// deadLetterSubjectPrefix namespaces dead-lettered messages under the same
// "finora.>" wildcard every service's EnsureStream call already subscribes
// its stream to, so no new stream or subject registration is needed for
// dead letters to land durably alongside everything else.
const deadLetterSubjectPrefix = "finora.dlq."

// deadLetter is the payload published to a dead-letter subject: enough to
// diagnose and, if the underlying bug is fixed, manually replay the
// original event.
type deadLetter struct {
	OriginalSubject string `json:"original_subject"`
	Durable         string `json:"durable"`
	NumDelivered    uint64 `json:"num_delivered"`
	Error           string `json:"error"`
	Payload         []byte `json:"payload"` // json.Marshal base64-encodes []byte automatically
}

// Bus wraps a NATS connection and its JetStream context — the one type
// every publisher/consumer in this fleet is built from.
type Bus struct {
	conn *nats.Conn
	js   jetstream.JetStream
	log  *slog.Logger
}

// Connect dials url and wraps it as a Bus, logging through log — currently
// only used for a failed dead-letter publish inside Subscribe (see
// deadLetter), everything else in this package returns errors rather than
// logging them itself. A bounded connect timeout plus unlimited automatic
// reconnects (nats.go's own background reconnect logic, not a loop this
// package writes) means a transient NATS outage after startup self-heals;
// a NATS that's unreachable at startup fails fast here, matching
// mongox.Connect's same fail-fast-on-bad-URI philosophy — every service
// that publishes or consumes events needs NATS to do its job, so a
// slow/failed initial connect should surface immediately, not hang.
func Connect(url string, log *slog.Logger) (*Bus, error) {
	conn, err := nats.Connect(url, nats.Timeout(10*time.Second), nats.MaxReconnects(-1))
	if err != nil {
		return nil, fmt.Errorf("eventbus: connect: %w", err)
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("eventbus: jetstream: %w", err)
	}
	return &Bus{conn: conn, js: js, log: log}, nil
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
// Retryable handler errors are redelivered with growing backoff (see
// defaultBackoff), not instantly — a NATS/nats.go quirk means Subscribe
// must apply this delay itself via NakWithDelay rather than leaning on
// ConsumerConfig.BackOff (see that var's doc comment). A handler error
// wrapping ErrTerminal, or a retryable error on the delivery attempt that
// exhausts defaultMaxDeliver, is treated as permanent: the message is
// Term()'d (never redelivered again) and published to a dead-letter
// subject ("finora.dlq.<durableName>") so the failure is a durable,
// observable event instead of one that silently stops existing — see
// architecture/api-contracts.md's Async Events section for how to consume
// a dead-letter subject.
//
// Up to handlerConcurrency messages are handled concurrently: the
// underlying NATS client delivers to Consume's callback one at a time, so
// without this, one slow handler call would pin this consumer's whole
// throughput to 1/handler_latency regardless of how many messages NATS is
// willing to have in flight (MaxAckPending).
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
		MaxDeliver:    defaultMaxDeliver,
		AckWait:       defaultAckWait,
		MaxAckPending: defaultMaxAckPending,
	})
	if err != nil {
		return fmt.Errorf("eventbus: consumer %s: %w", durableName, err)
	}

	sem := make(chan struct{}, handlerConcurrency)
	var wg sync.WaitGroup

	consumeCtx, err := cons.Consume(func(msg jetstream.Msg) {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			b.handleOne(ctx, durableName, handler, msg)
		}()
	})
	if err != nil {
		return fmt.Errorf("eventbus: consume %s: %w", durableName, err)
	}

	<-ctx.Done()
	// Stop pulling new messages, then wait for every in-flight handler
	// goroutine to finish its Ack/Nak/Term before returning — the same
	// "drain before you stop" instinct as shared/server.Run's shutdown
	// sequence, so a service shutdown doesn't abandon a message mid-handle.
	consumeCtx.Stop()
	wg.Wait()
	return nil
}

// handleOne runs handler for a single delivered message and resolves it:
// Ack on success; on failure, Term()+dead-letter if the error is
// ErrTerminal or delivery attempts are exhausted, otherwise
// NakWithDelay(backoffForAttempt(...)) so JetStream redelivers later
// rather than instantly (see Subscribe's doc comment for why Nak() alone
// doesn't give this delay).
func (b *Bus) handleOne(ctx context.Context, durableName string, handler Handler, msg jetstream.Msg) {
	err := handler(ctx, msg.Subject(), msg.Data())
	if err == nil {
		_ = msg.Ack()
		return
	}

	numDelivered := uint64(1)
	if md, mdErr := msg.Metadata(); mdErr == nil {
		numDelivered = md.NumDelivered
	}

	if errors.Is(err, ErrTerminal) || numDelivered >= defaultMaxDeliver {
		_ = msg.Term()
		b.deadLetter(ctx, durableName, msg, numDelivered, err)
		return
	}

	_ = msg.NakWithDelay(backoffForAttempt(numDelivered))
}

// deadLetter best-effort publishes msg to its dead-letter subject. A
// failure here is logged, never propagated — the original message has
// already been Term()'d by the caller, so there's no primary operation
// left to fail; losing a dead-letter entry means losing debuggability for
// this one failure, not losing the failure's terminal disposition.
func (b *Bus) deadLetter(ctx context.Context, durableName string, msg jetstream.Msg, numDelivered uint64, cause error) {
	dl := deadLetter{
		OriginalSubject: msg.Subject(),
		Durable:         durableName,
		NumDelivered:    numDelivered,
		Error:           cause.Error(),
		Payload:         msg.Data(),
	}
	payload, err := json.Marshal(dl)
	if err != nil {
		b.log.Error("eventbus: failed to marshal dead letter", slog.String("error", err.Error()))
		return
	}
	if err := b.Publish(ctx, deadLetterSubjectPrefix+durableName, payload, ""); err != nil {
		b.log.Error("eventbus: failed to publish dead letter",
			slog.String("durable", durableName),
			slog.String("original_subject", dl.OriginalSubject),
			slog.String("error", err.Error()),
		)
	}
}
