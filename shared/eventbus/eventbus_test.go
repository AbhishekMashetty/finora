//go:build integration

package eventbus_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/finora/shared/eventbus"
	"github.com/finora/shared/natstest"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBus_PublishAndSubscribe_RoundTrip(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url, discardLogger())
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
	bus, err := eventbus.Connect(url, discardLogger())
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
	bus, err := eventbus.Connect(url, discardLogger())
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
	bus, err := eventbus.Connect(url, discardLogger())
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

// TestBus_Subscribe_RetryAppliesBackoffNotInstantRedelivery is the actual
// regression test for F09's core bug: nats.go's Msg.Nak() "does not adhere
// to AckWait or Backoff configured on the consumer and triggers instant
// redelivery" (its own doc comment) — so before this fix, a handler
// failure hot-retried as fast as NATS could redeliver, turning a partial
// outage into a self-inflicted retry storm. Subscribe must apply the delay
// itself via NakWithDelay.
func TestBus_Subscribe_RetryAppliesBackoffNotInstantRedelivery(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url, discardLogger())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	if err := bus.EnsureStream(ctx, "TEST_STREAM", []string{"test.>"}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}

	var mu sync.Mutex
	var deliveries []time.Time
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = bus.Subscribe(subCtx, "TEST_STREAM", "backoff-consumer", "test.event", func(_ context.Context, _ string, _ []byte) error {
			mu.Lock()
			deliveries = append(deliveries, time.Now())
			n := len(deliveries)
			mu.Unlock()
			if n == 1 {
				return errors.New("first attempt fails on purpose, and is retryable")
			}
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)

	if err := bus.Publish(ctx, "test.event", []byte("payload"), ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	deadline := time.After(8 * time.Second)
	for {
		mu.Lock()
		n := len(deliveries)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for the retried delivery")
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	gap := deliveries[1].Sub(deliveries[0])
	mu.Unlock()
	// The first backoff step is 1s; allow generous scheduling slack, but
	// this must be clearly non-instant — a bare Nak() would redeliver
	// within milliseconds.
	if gap < 700*time.Millisecond {
		t.Errorf("redelivery gap = %v, want at least ~1s — backoff doesn't seem to be applied "+
			"(did this regress to instant Nak() redelivery?)", gap)
	}
}

// TestBus_Subscribe_TerminalErrorDeadLettersAndDoesNotRetry proves the
// other half of F09: a handler error wrapping eventbus.ErrTerminal is
// never redelivered, and is published to "finora.dlq.<durable>" instead of
// just disappearing after a log line — the exact gap the fleet's original
// ad hoc "log and ack" malformed-event handling had.
func TestBus_Subscribe_TerminalErrorDeadLettersAndDoesNotRetry(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url, discardLogger())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	// Two subject patterns on one stream: "test.>" for the normal event
	// this test publishes, "finora.dlq.>" to capture where Subscribe
	// actually publishes dead letters (deadLetterSubjectPrefix is a fixed
	// "finora.dlq." regardless of what stream/subject the original event
	// used).
	if err := bus.EnsureStream(ctx, "TEST_STREAM", []string{"test.>", "finora.dlq.>"}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}

	const durableName = "terminal-consumer"

	var mu sync.Mutex
	var deliveryCount int
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = bus.Subscribe(subCtx, "TEST_STREAM", durableName, "test.event", func(_ context.Context, _ string, _ []byte) error {
			mu.Lock()
			deliveryCount++
			mu.Unlock()
			return fmt.Errorf("%w: this payload is permanently bad", eventbus.ErrTerminal)
		})
	}()

	dlqReceived := make(chan []byte, 1)
	dlqCtx, dlqCancel := context.WithCancel(context.Background())
	defer dlqCancel()
	go func() {
		_ = bus.Subscribe(dlqCtx, "TEST_STREAM", "dlq-watcher", "finora.dlq."+durableName, func(_ context.Context, _ string, data []byte) error {
			dlqReceived <- data
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)

	if err := bus.Publish(ctx, "test.event", []byte("bad payload"), ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-dlqReceived:
		var dl struct {
			OriginalSubject string `json:"original_subject"`
			Durable         string `json:"durable"`
			Error           string `json:"error"`
		}
		if err := json.Unmarshal(got, &dl); err != nil {
			t.Fatalf("decode dead letter: %v", err)
		}
		if dl.OriginalSubject != "test.event" || dl.Durable != durableName {
			t.Errorf("dead letter = %+v, want OriginalSubject=test.event Durable=%s", dl, durableName)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the dead-lettered message")
	}

	// Give an (incorrect) retry a moment to happen, then confirm it never did.
	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	got := deliveryCount
	mu.Unlock()
	if got != 1 {
		t.Errorf("deliveryCount = %d, want exactly 1 — a terminal error must never be retried", got)
	}
}

// TestBus_Subscribe_HandlesMessagesConcurrently proves handlerConcurrency
// actually fans out: the underlying NATS client delivers to Consume's
// callback one message at a time, so without Subscribe's own worker pool,
// a slow handler would pin this consumer's throughput to
// 1/handler_latency regardless of how many messages are available.
func TestBus_Subscribe_HandlesMessagesConcurrently(t *testing.T) {
	url := natstest.StartURL(t)
	bus, err := eventbus.Connect(url, discardLogger())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	if err := bus.EnsureStream(ctx, "TEST_STREAM", []string{"test.>"}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}

	const messageCount = 10
	var inFlight, maxInFlight int32
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = bus.Subscribe(subCtx, "TEST_STREAM", "concurrency-consumer", "test.event", func(_ context.Context, _ string, _ []byte) error {
			n := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&maxInFlight)
				if n <= old || atomic.CompareAndSwapInt32(&maxInFlight, old, n) {
					break
				}
			}
			time.Sleep(300 * time.Millisecond) // hold the slot long enough for others to overlap
			atomic.AddInt32(&inFlight, -1)
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)

	for i := 0; i < messageCount; i++ {
		if err := bus.Publish(ctx, "test.event", []byte("payload"), ""); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	time.Sleep(2 * time.Second)
	if got := atomic.LoadInt32(&maxInFlight); got < 2 {
		t.Errorf("maxInFlight = %d, want at least 2 — messages should be handled concurrently, not one at a time", got)
	}
}
