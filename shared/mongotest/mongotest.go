//go:build integration

// Package mongotest starts a real, throwaway MongoDB instance (via
// testcontainers-go) for Phase 6 integration tests — CLAUDE.md §7
// explicitly deferred "real-Mongo integration tests" to this phase (before
// then, every unit test ran against an in-memory fake repository, never a
// live database). This package is that seam, shared once instead of
// duplicated per service.
//
// Test-only: nothing outside a _test.go file imports this package, so
// testcontainers-go's dependency tree never ends up linked into any
// service's compiled production binary — it only adds checksums to
// go.sum, not code to any binary that actually ships.
//
// pinned at testcontainers-go v0.32.0 / its mongodb module, deliberately
// not "latest": v0.34.0+ requires Go 1.22, and this repo is pinned to Go
// 1.21.3 everywhere (see docs/local-development.md's macOS linker note,
// which is specific to this exact version) — upgrading the toolchain
// project-wide is a much bigger, separate decision than "add integration
// tests," so v0.32.0 (the newest version still compatible with 1.21.3) was
// chosen instead of forcing that upgrade as a side effect.
package mongotest

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StartClient boots a real mongo:7 container — the exact image
// docker-compose.yml runs in production, not a stand-in — connects to it,
// and returns a ready *mongo.Client. The container and client are both
// torn down automatically via t.Cleanup, so callers never manage their own
// lifecycle. Every call gets its own fresh container (not a container
// shared across a package's tests): slower than a shared TestMain
// container, but each test is fully isolated with no risk of one test's
// leftover data affecting another's assertions, and no shared-fixture
// bookkeeping to get wrong — a deliberate simplicity-over-speed trade-off
// for a test tier that's already opt-in and separate from the fast default
// `go test` run (build-tagged `integration`, see any *_integration_test.go
// file for the tag).
func StartClient(t *testing.T) *mongo.Client {
	t.Helper()
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("mongotest: failed to start mongo:7 container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("mongotest: failed to terminate mongo container: %v", err)
		}
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("mongotest: failed to get connection string: %v", err)
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("mongotest: failed to connect: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Logf("mongotest: failed to disconnect: %v", err)
		}
	})

	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("mongotest: failed to ping: %v", err)
	}

	return client
}
