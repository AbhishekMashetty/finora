//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/finora/shared/mongotest"
	"github.com/finora/user-service/internal/domain"
	"github.com/finora/user-service/internal/repository"
)

func TestSettingsRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("user_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewSettingsRepository(db)
	ctx := context.Background()

	t.Run("FindByUserID on a user with no saved settings returns ErrNotFound", func(t *testing.T) {
		// The service layer (user_service.go's GetSettings) is what turns
		// this into sane USD/UTC defaults for the caller — the repository
		// itself must not paper over "nothing saved yet", or that
		// service-layer fallback logic couldn't distinguish it from a real
		// error.
		if _, err := repo.FindByUserID(ctx, "brand-new-user"); err != domain.ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("upsert creates on first call, replaces on second — a real Mongo upsert, not two code paths", func(t *testing.T) {
		userID := "user-1"

		if err := repo.Upsert(ctx, &domain.Settings{UserID: userID, Currency: "USD", Timezone: "UTC", UpdatedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("Upsert (create): %v", err)
		}
		got, err := repo.FindByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByUserID after first upsert: %v", err)
		}
		if got.Currency != "USD" || got.Timezone != "UTC" {
			t.Errorf("got %+v after create-upsert, want USD/UTC", got)
		}

		if err := repo.Upsert(ctx, &domain.Settings{UserID: userID, Currency: "EUR", Timezone: "Europe/London", UpdatedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("Upsert (replace): %v", err)
		}
		got, err = repo.FindByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByUserID after second upsert: %v", err)
		}
		if got.Currency != "EUR" || got.Timezone != "Europe/London" {
			t.Errorf("got %+v after replace-upsert, want EUR/Europe/London", got)
		}
	})

	t.Run("the real unique index on user_id prevents two settings documents for one user", func(t *testing.T) {
		// Upsert always replaces (never inserts a second doc for the same
		// user_id), so this proves the index by trying a raw duplicate
		// insert directly against the collection — bypassing the
		// repository's own upsert semantics on purpose, to test the index
		// itself rather than the code that happens to avoid triggering it.
		userID := "duplicate-check-user"
		if err := repo.Upsert(ctx, &domain.Settings{UserID: userID, Currency: "USD", Timezone: "UTC"}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		_, err := db.Collection("settings").InsertOne(ctx, domain.Settings{UserID: userID, Currency: "GBP", Timezone: "Europe/London"})
		if err == nil {
			t.Error("expected a duplicate-key error inserting a second settings document for the same user_id, got nil")
		}
	})
}
