//go:build integration

// Real-Mongo integration tests (CLAUDE.md §7's Phase 6 seam). Excluded from
// the default `go test ./...` run; run via `go test -tags=integration
// ./...` (see the root Makefile's `test-integration` target).
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/finora/shared/mongotest"
	"github.com/finora/user-service/internal/domain"
	"github.com/finora/user-service/internal/repository"
	"github.com/google/uuid"
)

func TestUserRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("user_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	t.Run("create then find round-trips through real Mongo", func(t *testing.T) {
		u := &domain.User{
			ID:           uuid.NewString(),
			Email:        "alice@example.com",
			PasswordHash: "bcrypt-hash-placeholder",
			Name:         "Alice",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}

		byEmail, err := repo.FindByEmail(ctx, "alice@example.com")
		if err != nil {
			t.Fatalf("FindByEmail: %v", err)
		}
		if byEmail.ID != u.ID || byEmail.Name != "Alice" {
			t.Errorf("FindByEmail = %+v, want the user just created", byEmail)
		}

		byID, err := repo.FindByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if byID.Email != "alice@example.com" {
			t.Errorf("FindByID = %+v, want email alice@example.com", byID)
		}
	})

	t.Run("the real unique index on email rejects a duplicate registration, not just application logic", func(t *testing.T) {
		email := "duplicate@example.com"
		first := &domain.User{ID: uuid.NewString(), Email: email, Name: "First"}
		if err := repo.Create(ctx, first); err != nil {
			t.Fatalf("Create (first): %v", err)
		}

		second := &domain.User{ID: uuid.NewString(), Email: email, Name: "Second"}
		err := repo.Create(ctx, second)
		if err != domain.ErrDuplicateEmail {
			t.Fatalf("Create (duplicate email): err = %v, want ErrDuplicateEmail — this must come from a real unique index, not application-level checking, so a race between two concurrent registrations can't both succeed", err)
		}
	})

	t.Run("update persists to the real document", func(t *testing.T) {
		u := &domain.User{ID: uuid.NewString(), Email: "bob@example.com", Name: "Bob"}
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}

		u.Name = "Bobby"
		if err := repo.Update(ctx, u); err != nil {
			t.Fatalf("Update: %v", err)
		}

		got, err := repo.FindByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("FindByID after update: %v", err)
		}
		if got.Name != "Bobby" {
			t.Errorf("Name = %q after update, want Bobby", got.Name)
		}
	})

	t.Run("FindByID/FindByEmail on a nonexistent user returns ErrNotFound", func(t *testing.T) {
		if _, err := repo.FindByID(ctx, uuid.NewString()); err != domain.ErrNotFound {
			t.Errorf("FindByID (nonexistent): err = %v, want ErrNotFound", err)
		}
		if _, err := repo.FindByEmail(ctx, "nobody@example.com"); err != domain.ErrNotFound {
			t.Errorf("FindByEmail (nonexistent): err = %v, want ErrNotFound", err)
		}
	})

	t.Run("EnsureIndexes created the documented indexes across all three collections", func(t *testing.T) {
		assertIndexExists(t, ctx, db, "users", "email_1")
		assertIndexExists(t, ctx, db, "refresh_tokens", "jti_hash_1")
		// The user_id and expires_at (TTL) indexes on refresh_tokens were
		// documented in architecture/database-design.md since Phase 0 but
		// never actually implemented — found while writing this test suite,
		// fixed in EnsureIndexes in the same pass. See that function's
		// updated doc comment for why user_id specifically matters
		// (RevokeAllForUser's token-theft containment path).
		assertIndexExists(t, ctx, db, "refresh_tokens", "user_id_1")
		assertTTLIndexExists(t, ctx, db, "refresh_tokens", "expires_at_1")
		assertIndexExists(t, ctx, db, "settings", "user_id_1")
	})
}
