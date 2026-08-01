//go:build integration

package eventbus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/finora/shared/eventbus"
	"github.com/finora/shared/natstest"
)

func TestBus_PublishAndSubscribe_RoundTrip(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	if err := bus.EnsureStream(ctx, "TEST_STREAM", []string{"test.>"}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}

	received := make(chan string, 1)
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = bus.Subscribe(subCtx, "TEST_STREAM", "test-consumer", "test.event", func(_ context.Context, subject string, data []byte) error {
			received <- subject + ":" + string(data)
			return nil
		})
	}()

	// Give the consumer a moment to actually attach before publishing —
	// this test asserts basic delivery, not the durability-across-restart
	// behavior (see TestBus_DurableConsumer_ResumesAfterRestart below).
	time.Sleep(300 * time.Millisecond)

	if err := bus.Publish(ctx, "test.event", []byte("hello"), ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if got != "test.event:hello" {
			t.Errorf("got %q, want %q", got, "test.event:hello")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for delivery")
	}
}

// TestBus_DurableConsumer_ResumesAfterRestart is the whole point of using
// JetStream over core NATS pub/sub: a message published while nobody was
// subscribed must still be delivered once a durable consumer with the same
// name attaches later. Core NATS would simply drop it.
func TestBus_DurableConsumer_ResumesAfterRestart(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	if err := bus.EnsureStream(ctx, "TEST_STREAM", []string{"test.>"}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}

	// Publish BEFORE any consumer exists.
	if err := bus.Publish(ctx, "test.event", []byte("published while nobody was listening"), ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	received := make(chan string, 1)
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = bus.Subscribe(subCtx, "TEST_STREAM", "late-consumer", "test.event", func(_ context.Context, _ string, data []byte) error {
			received <- string(data)
			return nil
		})
	}()

	select {
	case got := <-received:
		if got != "published while nobody was listening" {
			t.Errorf("got %q, want the message published before subscribing", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out — a durable JetStream consumer should still receive a message published before it existed")
	}
}

// TestBus_Publish_DedupByMsgID guards the exact behavior
// resolveImportAmount-adjacent producers rely on: retrying a publish with
// the same msgID must not result in the consumer seeing it twice.
func TestBus_Publish_DedupByMsgID(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	if err := bus.EnsureStream(ctx, "TEST_STREAM", []string{"test.>"}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}

	var mu sync.Mutex
	var count int
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = bus.Subscribe(subCtx, "TEST_STREAM", "dedup-consumer", "test.event", func(_ context.Context, _ string, _ []byte) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)

	for i := 0; i < 2; i++ {
		if err := bus.Publish(ctx, "test.event", []byte("retry"), "same-msg-id"); err != nil {
			t.Fatalf("publish attempt %d: %v", i, err)
		}
	}

	time.Sleep(1 * time.Second)
	mu.Lock()
	got := count
	mu.Unlock()
	if got != 1 {
		t.Errorf("expected exactly 1 delivery for two publishes with the same msgID, got %d", got)
	}
}

func TestChecker_ReportsConnectionState(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	checker := eventbus.Checker{Bus: bus}
	if checker.Name() != "nats" {
		t.Errorf("expected Name() = \"nats\", got %q", checker.Name())
	}
	if err := checker.Check(context.Background()); err != nil {
		t.Errorf("expected a healthy connection to report no error, got %v", err)
	}

	bus.Close()
	if err := checker.Check(context.Background()); err == nil {
		t.Error("expected an error after Close(), got nil")
	}
}
