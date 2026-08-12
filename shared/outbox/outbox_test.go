//go:build integration

package outbox_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/finora/shared/eventbus"
	"github.com/finora/shared/mongotest"
	"github.com/finora/shared/natstest"
	"github.com/finora/shared/outbox"
	"go.mongodb.org/mongo-driver/bson"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakePublisher is a hand-written stand-in for outbox.Publisher, so
// Store/Relay's own Mongo-facing behavior (querying unpublished events,
// marking them published) can be tested without a real NATS server —
// that's shared/eventbus's own job (see eventbus_test.go), not this
// package's.
type fakePublisher struct {
	mu        sync.Mutex
	published []publishedCall
	failNext  int // number of upcoming calls to fail before succeeding
}

type publishedCall struct {
	subject string
	data    string
	msgID   string
}

func (f *fakePublisher) Publish(_ context.Context, subject string, data []byte, msgID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext > 0 {
		f.failNext--
		return errors.New("simulated publish failure")
	}
	f.published = append(f.published, publishedCall{subject, string(data), msgID})
	return nil
}

func (f *fakePublisher) calls() []publishedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]publishedCall, len(f.published))
	copy(out, f.published)
	return out
}

var _ outbox.Publisher = (*fakePublisher)(nil)

func TestStore_EnqueueAndEnsureIndexes(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("outbox_test")
	store := outbox.NewStore(db)

	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	if err := store.Enqueue(ctx, "test.subject", []byte("payload"), "msg-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// A second EnsureIndexes call must be idempotent (services call this at
	// every startup, not just the first).
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes (second call): %v", err)
	}
}

func TestRelay_PublishesUnpublishedEventsAndMarksThemPublished(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("outbox_test")
	store := outbox.NewStore(db)
	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	if err := store.Enqueue(ctx, "transaction.created", []byte(`{"amount":10}`), "evt-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := store.Enqueue(ctx, "budget.overspent", []byte(`{"budget_id":"b1"}`), "evt-2"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	pub := &fakePublisher{}
	relay := outbox.NewRelay(store, pub, discardLogger())
	relay.RelayOnce(ctx)

	calls := pub.calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 published events, got %d: %+v", len(calls), calls)
	}

	// A second pass must NOT re-publish anything already marked published —
	// this is what proves the store update actually took effect, not just
	// that Publish was called once.
	relay.RelayOnce(ctx)
	if len(pub.calls()) != 2 {
		t.Fatalf("expected still exactly 2 published events after a second relay pass (nothing left unpublished), got %d", len(pub.calls()))
	}
}

func TestRelay_FailedPublishStaysClaimedUntilItsLockExpiresThenGetsRetried(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("outbox_test")
	store := outbox.NewStore(db)
	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	if err := store.Enqueue(ctx, "transaction.created", []byte("payload"), "evt-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	pub := &fakePublisher{failNext: 1}
	relay := outbox.NewRelay(store, pub, discardLogger())

	relay.RelayOnce(ctx) // fails — publish errors, the event stays claimed but unpublished
	if len(pub.calls()) != 0 {
		t.Fatalf("expected 0 successful publishes after the simulated failure, got %d", len(pub.calls()))
	}

	// Immediately retrying must NOT re-publish: a failed publish deliberately
	// leaves the claim's lock in place (see Relay.publishOne) — a fixed
	// backoff instead of hot-retrying a dependency that just failed, every
	// poll interval, which is exactly the retry-storm pattern F09 also fixes
	// on the consumer side.
	relay.RelayOnce(ctx)
	if len(pub.calls()) != 0 {
		t.Fatalf("expected the event to stay claimed (not retried) immediately after a failed publish, got %d publishes", len(pub.calls()))
	}

	// Force the claim's lock to look expired the way real wall-clock time
	// passing (relayLockDuration) would, rather than sleeping 30s in a test.
	if _, err := db.Collection("outbox_events").UpdateMany(ctx,
		bson.M{}, bson.M{"$set": bson.M{"locked_until": time.Now().Add(-time.Minute)}}); err != nil {
		t.Fatalf("force-expire the claim's lock: %v", err)
	}

	relay.RelayOnce(ctx) // the lock has "expired" — this pass reclaims and retries the event
	calls := pub.calls()
	if len(calls) != 1 {
		t.Fatalf("expected the event to be reclaimed and published once its lock expired, got %d publishes", len(calls))
	}
}

// TestRelay_ConcurrentInstancesDoNotDoublePublish is the actual regression
// test for F04: two Relay instances (standing in for two replicas of the
// same service) racing RelayOnce against the same Store and the same
// backlog of events must still publish each event exactly once. This only
// proves anything against a real MongoDB — the guarantee comes entirely
// from FindOneAndUpdate's document-level atomicity, which a fake can't
// exercise.
func TestRelay_ConcurrentInstancesDoNotDoublePublish(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("outbox_test")
	store := outbox.NewStore(db)
	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	const eventCount = 20
	for i := 0; i < eventCount; i++ {
		if err := store.Enqueue(ctx, "test.subject", []byte("payload"), ""); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	pub := &fakePublisher{}
	relayA := outbox.NewRelay(store, pub, discardLogger())
	relayB := outbox.NewRelay(store, pub, discardLogger())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); relayA.RelayOnce(ctx) }()
	go func() { defer wg.Done(); relayB.RelayOnce(ctx) }()
	wg.Wait()

	calls := pub.calls()
	if len(calls) != eventCount {
		t.Fatalf("expected exactly %d publishes across both relay instances racing on the same %d events, got %d — "+
			"more than %d means the claim step failed to prevent a double publish",
			eventCount, eventCount, len(calls), eventCount)
	}
}

// TestOutbox_EndToEnd_WithRealNATS proves shared/outbox and shared/eventbus
// actually compose correctly together — Store.Enqueue -> Relay -> a real
// eventbus.Bus -> a real NATS consumer receiving the event — not just that
// each package works in isolation against a fake.
func TestOutbox_EndToEnd_WithRealNATS(t *testing.T) {
	mongoClient := mongotest.StartClient(t)
	db := mongoClient.Database("outbox_e2e_test")
	store := outbox.NewStore(db)
	ctx := context.Background()
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	natsURL := natstest.StartURL(t)
	bus, err := eventbus.Connect(natsURL, discardLogger())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer bus.Close()
	if err := bus.EnsureStream(ctx, "E2E_STREAM", []string{"e2e.>"}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}

	received := make(chan string, 1)
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = bus.Subscribe(subCtx, "E2E_STREAM", "e2e-consumer", "e2e.event", func(_ context.Context, _ string, data []byte) error {
			received <- string(data)
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)

	if err := store.Enqueue(ctx, "e2e.event", []byte("real end-to-end payload"), "e2e-msg-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	relay := outbox.NewRelay(store, bus, discardLogger())
	relay.RelayOnce(ctx)

	select {
	case got := <-received:
		if got != "real end-to-end payload" {
			t.Errorf("got %q, want %q", got, "real end-to-end payload")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the outbox-relayed event to reach the NATS consumer")
	}
}
