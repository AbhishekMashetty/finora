// Package outbox implements the transactional-outbox pattern: a service
// records an event to its own MongoDB in the same call path as the write
// that triggered it, and a separate background Relay publishes queued
// events to NATS (via shared/eventbus) on its own schedule, retrying until
// it succeeds. This exists so "the write succeeded but the event never got
// published" (e.g. NATS was briefly unreachable) doesn't lose the event —
// it just sits queued until the Relay's next poll succeeds.
//
// Known, deliberate limitation: this is NOT a true atomic outbox. A real
// transactional outbox writes the domain row and the outbox row in one
// ACID transaction, so a crash between the two writes is impossible by
// construction. MongoDB multi-document transactions require a replica
// set, and this project's Mongo containers run standalone (see
// docker-compose.yml's mongo-* services — no --replSet flag) — converting
// every service's MongoDB deployment topology to support transactions is a
// real, separate infrastructure change, not something to fold silently
// into "add an outbox." Enqueue is therefore a second, ordinary insert
// immediately after the triggering write, not wrapped in a transaction
// with it: a crash in the narrow window between the two writes could lose
// an event. This is accepted and documented, the same way this codebase
// already documents other narrow, low-consequence races rather than
// engineering around them (see budget-service's report_service.go doc
// comment on its own read-then-write notify race) — the Relay still
// protects against the much more common failure mode (NATS itself being
// temporarily unreachable), which is the actual reliability problem Phase 7
// set out to solve.
package outbox

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collectionName = "outbox_events"

const (
	// relayBatchSize bounds how many events one RelayOnce pass claims, so a
	// backlog that built up during an outage is drained in bounded chunks
	// across successive polls instead of one pass holding a single
	// unbounded cursor open — which used to fail outright once Mongo's
	// idle-cursor timeout (10 minutes) was exceeded partway through a large
	// backlog, restarting from the top on the next tick and never making
	// forward progress.
	relayBatchSize = 500

	// relayConcurrency bounds how many claimed events are published to NATS
	// concurrently within one RelayOnce pass. Claiming is cheap, local Mongo
	// round trips; publishing is a network call to NATS, which is where
	// concurrency actually buys throughput, so only that phase is
	// parallelized.
	relayConcurrency = 32

	// relayLockDuration is how long a claimed-but-not-yet-published event
	// stays reserved by the claiming Relay instance before another
	// instance (or this one, on a later pass) is allowed to reclaim it —
	// long enough to cover a normal publish attempt, short enough that a
	// crashed relay doesn't strand an event for long.
	relayLockDuration = 30 * time.Second
)

// Event is one queued (or already-published) domain event. LockedUntil and
// LockedBy implement the claim Relay.claim uses so that running more than
// one replica of a service never causes two replicas to publish the same
// event outside JetStream's short dedup window — see Relay's doc comment.
type Event struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Subject     string             `bson:"subject"`
	MsgID       string             `bson:"msg_id,omitempty"`
	Payload     []byte             `bson:"payload"`
	CreatedAt   time.Time          `bson:"created_at"`
	PublishedAt *time.Time         `bson:"published_at"`
	LockedUntil *time.Time         `bson:"locked_until,omitempty"`
	LockedBy    string             `bson:"locked_by,omitempty"`
}

// Store persists queued events in a service's own MongoDB — always
// finora_<service>'s own database, never shared across services, per
// CLAUDE.md §2's "one MongoDB per service" rule; outbox_events is just
// another collection in that same database.
type Store struct {
	col *mongo.Collection
}

// NewStore builds a Store backed by db's outbox_events collection.
func NewStore(db *mongo.Database) *Store {
	return &Store{col: db.Collection(collectionName)}
}

// EnsureIndexes creates the indexes the Relay's claim query and reaping
// need. Call this once at service startup alongside the service's other
// repository.EnsureIndexes calls.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	// Phase 7 shipped a single plain {published_at: 1} index for the
	// Relay's old bson.M{"published_at": nil} find. The Relay now claims
	// work via FindOneAndUpdate sorted by created_at (see Relay.claim) and
	// reaps published rows via a TTL index that also keys on published_at
	// — a different shape MongoDB won't let coexist with the old index
	// under the same key pattern. Drop the old one first (a fresh database
	// never had it, so DropOne failing is expected there and ignored) so a
	// database that already ran the old code doesn't end up erroring on
	// the TTL index below.
	_, _ = s.col.Indexes().DropOne(ctx, "published_at_1")

	// Unpublished-only partial index backing Relay.claim's query
	// (FindOneAndUpdate filtered on published_at: nil, sorted by
	// created_at). Because the partial filter excludes every published
	// row, this index's size is bounded by outstanding backlog, not by the
	// total number of events ever published, so it stays small permanently
	// instead of growing forever — unlike outbox_events itself would,
	// without the TTL index below.
	if _, err := s.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().
			SetName("outbox_unpublished_by_created_at").
			SetPartialFilterExpression(bson.M{"published_at": bson.M{"$eq": nil}}),
	}); err != nil {
		return err
	}

	// TTL index: once an event is published, published_at holds a real
	// timestamp and MongoDB reaps the document 72h after it — generous
	// post-mortem/debugging headroom, but bounded, unlike the unbounded
	// growth outbox_events had before this change (nothing else in this
	// codebase ever deletes from it). MongoDB's TTL mechanism only expires
	// documents whose indexed field holds an actual date, so unpublished
	// documents (published_at: nil) are never touched by it.
	if _, err := s.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "published_at", Value: 1}},
		Options: options.Index().SetName("outbox_ttl").SetExpireAfterSeconds(72 * 3600),
	}); err != nil {
		return err
	}

	return nil
}

// Enqueue records subject/payload as a new, unpublished event. msgID (may
// be "") is carried through to the eventual NATS publish for JetStream
// dedup — see eventbus.Bus.Publish's doc comment.
func (s *Store) Enqueue(ctx context.Context, subject string, payload []byte, msgID string) error {
	_, err := s.col.InsertOne(ctx, Event{
		Subject:     subject,
		MsgID:       msgID,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
		PublishedAt: nil,
	})
	return err
}

// Publisher is the one method Relay needs from eventbus.Bus — kept as a
// narrow interface (Dependency Inversion, same pattern this codebase
// already uses for domain.NotificationClient/domain.ExpenseClient) so
// Relay is unit-testable against a fake, not a real NATS connection.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte, msgID string) error
}

// Relay polls Store for unpublished events and publishes them via
// Publisher, marking each published_at only after a successful publish —
// so a publish failure (NATS down) leaves the event queued for the next
// poll instead of being lost.
//
// Safe to run one Relay per replica of a service, not just one per fleet:
// each RelayOnce pass first claims a batch of events via an atomic
// per-document FindOneAndUpdate (see claim), stamping the winning
// replica's instance identity and a short-lived lock onto each one, so two
// replicas polling concurrently can't both pick up and publish the same
// event outside JetStream's ~2-minute Msg-Id dedup window (which is the
// only thing that protected against double-publishing before this claim
// step existed). A replica that crashes mid-batch simply lets its claims
// expire after relayLockDuration; whichever replica polls next reclaims
// them.
type Relay struct {
	store     *Store
	publisher Publisher
	log       *slog.Logger
	instance  string
}

// NewRelay builds a Relay. log records publish failures only — a failure
// is expected to be transient and self-heal on a later poll, so it's never
// fatal to the relay loop itself. instance (used as the claim lock's
// locked_by value, purely for operator visibility when inspecting
// outbox_events during an incident) defaults to os.Hostname(), which is a
// pod's name in Kubernetes.
func NewRelay(store *Store, publisher Publisher, log *slog.Logger) *Relay {
	instance, err := os.Hostname()
	if err != nil || instance == "" {
		instance = "unknown"
	}
	return &Relay{store: store, publisher: publisher, log: log, instance: instance}
}

// Run polls for and publishes unpublished events every interval, blocking
// until ctx is cancelled. Intended to run in its own goroutine, started
// once per service alongside server.Run in cmd/server/main.go.
func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.RelayOnce(ctx)
		}
	}
}

// RelayOnce runs a single claim-and-publish pass immediately: it claims up
// to relayBatchSize unpublished events (oldest first) and publishes them
// with up to relayConcurrency in flight at once, then returns once every
// claimed event has been attempted. Run calls this on every tick; tests
// call it directly for deterministic timing instead of waiting on a
// ticker.
func (r *Relay) RelayOnce(ctx context.Context) {
	events := r.claim(ctx, relayBatchSize)
	if len(events) == 0 {
		return
	}

	sem := make(chan struct{}, relayConcurrency)
	var wg sync.WaitGroup
	for _, ev := range events {
		ev := ev
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r.publishOne(ctx, ev)
		}()
	}
	wg.Wait()
}

// claim atomically reserves up to limit unpublished events that are not
// currently locked by another Relay instance (or whose lock has expired),
// oldest first, and returns them. Each reservation is its own
// FindOneAndUpdate — the atomic primitive that makes the claim safe under
// concurrent replicas — so claiming is `limit` sequential, local round
// trips to this service's own Mongo, not a network call to NATS; the
// concurrency that actually matters for throughput happens afterward, in
// RelayOnce's publish phase.
func (r *Relay) claim(ctx context.Context, limit int) []Event {
	now := time.Now().UTC()
	lockedUntil := now.Add(relayLockDuration)

	claimed := make([]Event, 0, limit)
	for i := 0; i < limit; i++ {
		var ev Event
		err := r.store.col.FindOneAndUpdate(ctx,
			bson.M{
				"published_at": nil,
				"$or": bson.A{
					bson.M{"locked_until": nil},
					bson.M{"locked_until": bson.M{"$lt": now}},
				},
			},
			bson.M{"$set": bson.M{"locked_until": lockedUntil, "locked_by": r.instance}},
			options.FindOneAndUpdate().
				SetSort(bson.D{{Key: "created_at", Value: 1}}).
				SetReturnDocument(options.After),
		).Decode(&ev)
		if err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				r.log.Error("outbox: failed to claim event", slog.String("error", err.Error()))
			}
			break
		}
		claimed = append(claimed, ev)
	}
	return claimed
}

// publishOne publishes a single claimed event and, on success, marks it
// published and clears its claim. A failed publish deliberately leaves the
// claim in place rather than clearing it immediately: the event simply
// stays reserved by this instance until the lock expires
// (relayLockDuration), at which point it (or another replica) reclaims it
// on a later pass. That gives a failing publish a fixed backoff instead of
// hot-retrying every poll interval against a dependency that's actually
// down — the same "don't hammer a known-bad dependency" instinct as F09's
// consumer backoff.
func (r *Relay) publishOne(ctx context.Context, ev Event) {
	if err := r.publisher.Publish(ctx, ev.Subject, ev.Payload, ev.MsgID); err != nil {
		r.log.Error("outbox: failed to publish event, will retry once its claim expires",
			slog.String("subject", ev.Subject),
			slog.String("error", err.Error()),
		)
		return
	}

	now := time.Now().UTC()
	update := bson.M{
		"$set":   bson.M{"published_at": now},
		"$unset": bson.M{"locked_until": "", "locked_by": ""},
	}
	if _, err := r.store.col.UpdateByID(ctx, ev.ID, update); err != nil {
		r.log.Error("outbox: published but failed to mark published_at — will be republished once its claim expires",
			slog.String("subject", ev.Subject),
			slog.String("error", err.Error()),
		)
	}
}
