//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/finora/shared/mongotest"
	"github.com/finora/user-service/internal/domain"
	"github.com/finora/user-service/internal/repository"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestRefreshTokenRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("user_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewRefreshTokenRepository(db)
	ctx := context.Background()

	t.Run("create then find by jti hash round-trips through real Mongo", func(t *testing.T) {
		rt := &domain.RefreshToken{
			ID:        uuid.NewString(),
			UserID:    "user-1",
			JTIHash:   "hash-abc123",
			ExpiresAt: time.Now().Add(168 * time.Hour).UTC(),
			CreatedAt: time.Now().UTC(),
		}
		if err := repo.Create(ctx, rt); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := repo.FindByJTIHash(ctx, "hash-abc123")
		if err != nil {
			t.Fatalf("FindByJTIHash: %v", err)
		}
		if got.UserID != "user-1" || got.Revoked {
			t.Errorf("FindByJTIHash = %+v, want a fresh, non-revoked token for user-1", got)
		}
	})

	t.Run("the real unique index on jti_hash rejects a duplicate", func(t *testing.T) {
		hash := "hash-duplicate"
		first := &domain.RefreshToken{ID: uuid.NewString(), UserID: "user-2", JTIHash: hash, ExpiresAt: time.Now().Add(time.Hour)}
		if err := repo.Create(ctx, first); err != nil {
			t.Fatalf("Create (first): %v", err)
		}

		second := &domain.RefreshToken{ID: uuid.NewString(), UserID: "user-2", JTIHash: hash, ExpiresAt: time.Now().Add(time.Hour)}
		if err := repo.Create(ctx, second); !mongo.IsDuplicateKeyError(err) {
			t.Errorf("Create (duplicate jti_hash): err = %v, want a real Mongo duplicate-key error", err)
		}
	})

	t.Run("Revoke marks a single token revoked and is a real update against Mongo", func(t *testing.T) {
		rt := &domain.RefreshToken{ID: uuid.NewString(), UserID: "user-3", JTIHash: "hash-revoke-me", ExpiresAt: time.Now().Add(time.Hour)}
		if err := repo.Create(ctx, rt); err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := repo.Revoke(ctx, rt.ID); err != nil {
			t.Fatalf("Revoke: %v", err)
		}

		got, err := repo.FindByJTIHash(ctx, "hash-revoke-me")
		if err != nil {
			t.Fatalf("FindByJTIHash after revoke: %v", err)
		}
		if !got.Revoked {
			t.Error("token should be Revoked=true after Revoke, but wasn't")
		}
	})

	t.Run("RevokeAllForUser containment: revokes every non-revoked token for that user, and no one else's", func(t *testing.T) {
		victim := "victim-user"
		bystander := "bystander-user"

		// Two live sessions for the victim (simulating token theft — the
		// attacker replayed a rotated-away token, triggering full
		// containment), plus one already-revoked token that must stay
		// revoked (not an error to "re-revoke"), plus a bystander's token
		// that must be completely untouched.
		session1 := &domain.RefreshToken{ID: uuid.NewString(), UserID: victim, JTIHash: "victim-session-1", ExpiresAt: time.Now().Add(time.Hour)}
		session2 := &domain.RefreshToken{ID: uuid.NewString(), UserID: victim, JTIHash: "victim-session-2", ExpiresAt: time.Now().Add(time.Hour)}
		alreadyRevoked := &domain.RefreshToken{ID: uuid.NewString(), UserID: victim, JTIHash: "victim-already-revoked", ExpiresAt: time.Now().Add(time.Hour), Revoked: true}
		bystanderToken := &domain.RefreshToken{ID: uuid.NewString(), UserID: bystander, JTIHash: "bystander-session", ExpiresAt: time.Now().Add(time.Hour)}
		for _, rt := range []*domain.RefreshToken{session1, session2, alreadyRevoked, bystanderToken} {
			if err := repo.Create(ctx, rt); err != nil {
				t.Fatalf("Create(%s): %v", rt.JTIHash, err)
			}
		}

		if err := repo.RevokeAllForUser(ctx, victim); err != nil {
			t.Fatalf("RevokeAllForUser: %v", err)
		}

		for _, hash := range []string{"victim-session-1", "victim-session-2", "victim-already-revoked"} {
			got, err := repo.FindByJTIHash(ctx, hash)
			if err != nil {
				t.Fatalf("FindByJTIHash(%s): %v", hash, err)
			}
			if !got.Revoked {
				t.Errorf("%s should be Revoked=true after RevokeAllForUser(%s)", hash, victim)
			}
		}

		bystanderGot, err := repo.FindByJTIHash(ctx, "bystander-session")
		if err != nil {
			t.Fatalf("FindByJTIHash(bystander-session): %v", err)
		}
		if bystanderGot.Revoked {
			t.Error("RevokeAllForUser(victim) must never revoke a different user's token, but it revoked the bystander's")
		}
	})

	t.Run("FindByJTIHash on an unknown hash returns ErrNotFound", func(t *testing.T) {
		if _, err := repo.FindByJTIHash(ctx, "no-such-hash"); err != domain.ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
