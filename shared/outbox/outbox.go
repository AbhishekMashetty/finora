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
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const collectionName = "outbox_events"

// Event is one queued (or already-published) domain event.
type Event struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Subject     string             `bson:"subject"`
	MsgID       string             `bson:"msg_id,omitempty"`
	Payload     []byte             `bson:"payload"`
	CreatedAt   time.Time          `bson:"created_at"`
	PublishedAt *time.Time         `bson:"published_at"`
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

// EnsureIndexes creates the index the Relay's poll query needs. Call this
// once at service startup alongside the service's other
// repository.EnsureIndexes calls.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "published_at", Value: 1}},
	})
	return err
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
type Relay struct {
	store     *Store
	publisher Publisher
	log       *slog.Logger
}

// NewRelay builds a Relay. log records publish failures only — a failure
// is expected to be transient and self-heal on the next poll, so it's
// never fatal to the relay loop itself.
func NewRelay(store *Store, publisher Publisher, log *slog.Logger) *Relay {
	return &Relay{store: store, publisher: publisher, log: log}
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

// RelayOnce runs a single poll-and-publish pass immediately. Run calls
// this on every tick; tests call it directly for deterministic timing
// instead of waiting on a ticker.
func (r *Relay) RelayOnce(ctx context.Context) {
	cursor, err := r.store.col.Find(ctx, bson.M{"published_at": nil})
	if err != nil {
		r.log.Error("outbox: failed to query unpublished events", slog.String("error", err.Error()))
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var ev Event
		if err := cursor.Decode(&ev); err != nil {
			r.log.Error("outbox: failed to decode event", slog.String("error", err.Error()))
			continue
		}

		if err := r.publisher.Publish(ctx, ev.Subject, ev.Payload, ev.MsgID); err != nil {
			r.log.Error("outbox: failed to publish event, will retry next poll",
				slog.String("subject", ev.Subject),
				slog.String("error", err.Error()),
			)
			continue
		}

		now := time.Now().UTC()
		if _, err := r.store.col.UpdateByID(ctx, ev.ID, bson.M{"$set": bson.M{"published_at": now}}); err != nil {
			r.log.Error("outbox: published but failed to mark published_at — will be republished next poll",
				slog.String("subject", ev.Subject),
				slog.String("error", err.Error()),
			)
		}
	}
}
