//go:build integration

// Real-Mongo integration tests (CLAUDE.md §7's Phase 6 seam). Excluded from
// the default `go test ./...` run; run via `go test -tags=integration
// ./...` (see the root Makefile's `test-integration` target).
package repository_test

import (
	"context"
	"testing"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/budget-service/internal/repository"
	"github.com/finora/shared/mongotest"
)

func TestBudgetRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("budget_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewMongoBudgetRepository(db)
	ctx := context.Background()

	t.Run("create then get round-trips through real Mongo", func(t *testing.T) {
		b := &domain.Budget{UserID: "user-1", Category: "groceries", Amount: 500, Period: domain.PeriodMonthly}
		if err := repo.Create(ctx, b); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if b.ID == "" {
			t.Fatal("Create did not populate an ID")
		}

		got, err := repo.GetByIDForUser(ctx, b.ID, "user-1")
		if err != nil {
			t.Fatalf("GetByIDForUser: %v", err)
		}
		if got.Category != "groceries" || got.Amount != 500 || got.Period != domain.PeriodMonthly {
			t.Errorf("got %+v, want the budget just created", got)
		}
	})

	t.Run("ownership is enforced by the real Mongo query", func(t *testing.T) {
		b := &domain.Budget{UserID: "owner", Category: "rent", Amount: 2000, Period: domain.PeriodMonthly}
		if err := repo.Create(ctx, b); err != nil {
			t.Fatalf("Create: %v", err)
		}

		if _, err := repo.GetByIDForUser(ctx, b.ID, "someone-else"); err != domain.ErrNotFound {
			t.Errorf("cross-user GetByIDForUser: err = %v, want ErrNotFound", err)
		}
		list, err := repo.ListByUser(ctx, "someone-else")
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		for _, x := range list {
			if x.ID == b.ID {
				t.Errorf("ListByUser(someone-else) leaked owner's budget %s", b.ID)
			}
		}
	})

	t.Run("update persists, including the Phase 4 LastNotifiedAt overspend-dedup field", func(t *testing.T) {
		b := &domain.Budget{UserID: "user-2", Category: "dining", Amount: 200, Period: domain.PeriodWeekly}
		if err := repo.Create(ctx, b); err != nil {
			t.Fatalf("Create: %v", err)
		}

		b.Amount = 250
		if err := repo.Update(ctx, b); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := repo.GetByIDForUser(ctx, b.ID, "user-2")
		if err != nil {
			t.Fatalf("GetByIDForUser after update: %v", err)
		}
		if got.Amount != 250 {
			t.Errorf("Amount = %v after update, want 250", got.Amount)
		}
		if got.LastNotifiedAt != nil {
			t.Errorf("LastNotifiedAt = %v, want nil for a budget that's never triggered a notification", got.LastNotifiedAt)
		}
	})

	t.Run("delete removes the document; a cross-user delete is a no-op ErrNotFound", func(t *testing.T) {
		b := &domain.Budget{UserID: "user-3", Category: "entertainment", Amount: 60, Period: domain.PeriodMonthly}
		if err := repo.Create(ctx, b); err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := repo.DeleteByIDForUser(ctx, b.ID, "someone-else"); err != domain.ErrNotFound {
			t.Errorf("cross-user delete: err = %v, want ErrNotFound", err)
		}
		if err := repo.DeleteByIDForUser(ctx, b.ID, "user-3"); err != nil {
			t.Fatalf("DeleteByIDForUser: %v", err)
		}
		if _, err := repo.GetByIDForUser(ctx, b.ID, "user-3"); err != domain.ErrNotFound {
			t.Errorf("after delete: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("EnsureIndexes created the documented indexes", func(t *testing.T) {
		assertIndexExists(t, ctx, db, "budgets", "user_id_1")
		assertIndexExists(t, ctx, db, "budgets", "user_id_1_period_1")
	})
}
