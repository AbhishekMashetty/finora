//go:build integration

// Package natstest starts a real, throwaway NATS server (via
// testcontainers-go's generic container support — there's no dedicated
// "modules/nats" package at the pinned testcontainers-go v0.32.0, unlike
// mongodb's, so this uses the same underlying testcontainers.Container API
// that module would wrap) for Phase 7 integration tests, mirroring
// shared/mongotest's shape exactly: real infrastructure, gated behind the
// `integration` build tag, one fresh container per test.
//
// Test-only: nothing outside a _test.go file imports this package, so
// nothing here adds to any service's compiled production binary.
package natstest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartURL boots a real nats:2-alpine container with JetStream enabled
// (-js — every event this fleet publishes goes through JetStream, never
// core NATS, so a test against core-NATS-only would not exercise the real
// delivery guarantees) and returns its connection URL. The container is
// torn down automatically via t.Cleanup.
func StartURL(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "nats:2-alpine",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("natstest: failed to start nats:2-alpine container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("natstest: failed to terminate nats container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("natstest: failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "4222/tcp")
	if err != nil {
		t.Fatalf("natstest: failed to get mapped port: %v", err)
	}

	// The wait strategy already confirmed the "Server is ready" log line,
	// but give the mapped port a moment more under load (CI runners) before
	// the first real connect attempt — matches the same small allowance
	// mongotest's own Ping-after-connect check effectively gives Mongo.
	time.Sleep(200 * time.Millisecond)

	return fmt.Sprintf("nats://%s:%s", host, port.Port())
}
