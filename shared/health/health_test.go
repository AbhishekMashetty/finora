package health

import (
	"context"
	"testing"
)

func TestGate_StartsHealthy(t *testing.T) {
	g := &Gate{}
	if err := g.Check(context.Background()); err != nil {
		t.Fatalf("fresh gate should be healthy, got error: %v", err)
	}
}

func TestGate_MarkNotReadyFailsSubsequentChecks(t *testing.T) {
	g := &Gate{}
	g.MarkNotReady()
	if err := g.Check(context.Background()); err == nil {
		t.Fatal("expected an error after MarkNotReady, got nil")
	}
}

func TestGate_MarkNotReadyIsIdempotent(t *testing.T) {
	g := &Gate{}
	g.MarkNotReady()
	g.MarkNotReady()
	if err := g.Check(context.Background()); err == nil {
		t.Fatal("expected an error after calling MarkNotReady twice, got nil")
	}
}

func TestGate_Name(t *testing.T) {
	g := &Gate{}
	if got := g.Name(); got != "shutdown" {
		t.Fatalf("Name() = %q, want %q", got, "shutdown")
	}
}
