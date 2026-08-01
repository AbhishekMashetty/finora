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

func TestRelay_FailedPublishLeavesEventQueuedForNextPoll(t *testing.T) {
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

	relay.RelayOnce(ctx) // fails — publish errors, event stays unpublished
	if len(pub.calls()) != 0 {
		t.Fatalf("expected 0 successful publishes after the simulated failure, got %d", len(pub.calls()))
	}

	relay.RelayOnce(ctx) // succeeds this time — proves the failed event was retried, not dropped
	calls := pub.calls()
	if len(calls) != 1 {
		t.Fatalf("expected the event to be retried and published on the next pass, got %d publishes", len(calls))
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
	bus, err := eventbus.Connect(natsURL)
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
