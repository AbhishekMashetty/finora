//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/budget-service/internal/repository"
	"github.com/finora/shared/mongotest"
)

func TestGoalRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("budget_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewMongoGoalRepository(db)
	ctx := context.Background()

	future := time.Now().UTC().Add(90 * 24 * time.Hour)

	t.Run("create then get round-trips through real Mongo", func(t *testing.T) {
		g := &domain.Goal{UserID: "user-1", Name: "Emergency Fund", TargetAmount: 10000, TargetDate: future}
		if err := repo.Create(ctx, g); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if g.ID == "" {
			t.Fatal("Create did not populate an ID")
		}

		got, err := repo.GetByIDForUser(ctx, g.ID, "user-1")
		if err != nil {
			t.Fatalf("GetByIDForUser: %v", err)
		}
		if got.Name != "Emergency Fund" || got.TargetAmount != 10000 {
			t.Errorf("got %+v, want the goal just created", got)
		}
	})

	t.Run("ownership is enforced by the real Mongo query", func(t *testing.T) {
		g := &domain.Goal{UserID: "owner", Name: "Private Goal", TargetAmount: 500, TargetDate: future}
		if err := repo.Create(ctx, g); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := repo.GetByIDForUser(ctx, g.ID, "someone-else"); err != domain.ErrNotFound {
			t.Errorf("cross-user GetByIDForUser: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("update persists manual progress-logging (current_amount), including exactly zero", func(t *testing.T) {
		g := &domain.Goal{UserID: "user-2", Name: "Trip", TargetAmount: 2000, TargetDate: future}
		if err := repo.Create(ctx, g); err != nil {
			t.Fatalf("Create: %v", err)
		}

		g.CurrentAmount = 500
		if err := repo.Update(ctx, g); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := repo.GetByIDForUser(ctx, g.ID, "user-2")
		if err != nil {
			t.Fatalf("GetByIDForUser: %v", err)
		}
		if got.CurrentAmount != 500 {
			t.Errorf("CurrentAmount = %v, want 500", got.CurrentAmount)
		}

		// Regression-shaped: a zero-value CurrentAmount (resetting progress)
		// must actually persist as 0, not be mistaken for "field omitted".
		g.CurrentAmount = 0
		if err := repo.Update(ctx, g); err != nil {
			t.Fatalf("Update (reset to zero): %v", err)
		}
		got, err = repo.GetByIDForUser(ctx, g.ID, "user-2")
		if err != nil {
			t.Fatalf("GetByIDForUser after reset: %v", err)
		}
		if got.CurrentAmount != 0 {
			t.Errorf("CurrentAmount = %v after resetting to 0, want 0", got.CurrentAmount)
		}
	})

	t.Run("delete removes the document; a cross-user delete is a no-op ErrNotFound", func(t *testing.T) {
		g := &domain.Goal{UserID: "user-3", Name: "To delete", TargetAmount: 100, TargetDate: future}
		if err := repo.Create(ctx, g); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := repo.DeleteByIDForUser(ctx, g.ID, "someone-else"); err != domain.ErrNotFound {
			t.Errorf("cross-user delete: err = %v, want ErrNotFound", err)
		}
		if err := repo.DeleteByIDForUser(ctx, g.ID, "user-3"); err != nil {
			t.Fatalf("DeleteByIDForUser: %v", err)
		}
		if _, err := repo.GetByIDForUser(ctx, g.ID, "user-3"); err != domain.ErrNotFound {
			t.Errorf("after delete: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("EnsureIndexes created the documented index on the real 'goals' collection (not 'savings_goals')", func(t *testing.T) {
		assertIndexExists(t, ctx, db, "goals", "user_id_1")
	})
}
