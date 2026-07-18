//go:build integration

// Real-Mongo integration tests (CLAUDE.md §7's Phase 6 seam — see
// shared/mongotest's doc comment for why this is separate from the fast,
// fake-backed unit tests in account_handler_test.go/account_service_test.go).
// Excluded from the default `go test ./...` run by the `integration` build
// tag; run explicitly via `go test -tags=integration ./...` (see the root
// Makefile's `test-integration` target, and .github/workflows/ci.yml for
// how CI runs this tier).
package repository_test

import (
	"context"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/repository"
	"github.com/finora/shared/mongotest"
)

func TestAccountRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("expense_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewAccountRepository(db)
	ctx := context.Background()

	t.Run("create then get round-trips through real Mongo", func(t *testing.T) {
		acc := &domain.Account{
			UserID:   "user-1",
			Name:     "Chase Checking",
			Type:     domain.AccountTypeChecking,
			Currency: "USD",
		}
		if err := repo.Create(ctx, acc); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if acc.ID == "" {
			t.Fatal("Create did not populate an ID")
		}

		got, err := repo.GetByIDForUser(ctx, acc.ID, "user-1")
		if err != nil {
			t.Fatalf("GetByIDForUser: %v", err)
		}
		if got.Name != "Chase Checking" || got.Currency != "USD" || got.Type != domain.AccountTypeChecking {
			t.Errorf("got %+v, want the account just created", got)
		}
	})

	t.Run("ownership is enforced by the real Mongo query, not just application code", func(t *testing.T) {
		acc := &domain.Account{UserID: "owner", Name: "Savings", Type: domain.AccountTypeSavings, Currency: "USD"}
		if err := repo.Create(ctx, acc); err != nil {
			t.Fatalf("Create: %v", err)
		}

		if _, err := repo.GetByIDForUser(ctx, acc.ID, "someone-else"); err != domain.ErrNotFound {
			t.Errorf("cross-user GetByIDForUser: err = %v, want ErrNotFound", err)
		}

		list, err := repo.ListByUser(ctx, "someone-else")
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		for _, a := range list {
			if a.ID == acc.ID {
				t.Errorf("ListByUser(someone-else) leaked owner's account %s", acc.ID)
			}
		}
	})

	t.Run("update persists to the real document", func(t *testing.T) {
		acc := &domain.Account{UserID: "user-2", Name: "Original", Type: domain.AccountTypeCash, Currency: "USD"}
		if err := repo.Create(ctx, acc); err != nil {
			t.Fatalf("Create: %v", err)
		}

		newBalance := 250.75
		acc.Name = "Renamed"
		acc.Balance = newBalance
		if err := repo.Update(ctx, acc); err != nil {
			t.Fatalf("Update: %v", err)
		}

		got, err := repo.GetByIDForUser(ctx, acc.ID, "user-2")
		if err != nil {
			t.Fatalf("GetByIDForUser after update: %v", err)
		}
		if got.Name != "Renamed" || got.Balance != newBalance {
			t.Errorf("got %+v, want Name=Renamed Balance=%v", got, newBalance)
		}
	})

	t.Run("delete removes the document; a cross-user delete is a no-op ErrNotFound", func(t *testing.T) {
		acc := &domain.Account{UserID: "user-3", Name: "To delete", Type: domain.AccountTypeChecking, Currency: "USD"}
		if err := repo.Create(ctx, acc); err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := repo.DeleteByIDForUser(ctx, acc.ID, "someone-else"); err != domain.ErrNotFound {
			t.Errorf("cross-user delete: err = %v, want ErrNotFound", err)
		}
		if _, err := repo.GetByIDForUser(ctx, acc.ID, "user-3"); err != nil {
			t.Fatalf("account should still exist after a cross-user delete attempt, got: %v", err)
		}

		if err := repo.DeleteByIDForUser(ctx, acc.ID, "user-3"); err != nil {
			t.Fatalf("DeleteByIDForUser: %v", err)
		}
		if _, err := repo.GetByIDForUser(ctx, acc.ID, "user-3"); err != domain.ErrNotFound {
			t.Errorf("after delete: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("EnsureIndexes actually created the documented user_id index", func(t *testing.T) {
		cursor, err := db.Collection("accounts").Indexes().List(ctx)
		if err != nil {
			t.Fatalf("Indexes().List: %v", err)
		}
		defer cursor.Close(ctx)

		var found bool
		for cursor.Next(ctx) {
			var idx struct {
				Name string `bson:"name"`
			}
			if err := cursor.Decode(&idx); err != nil {
				t.Fatalf("decode index: %v", err)
			}
			if idx.Name == "user_id_1" {
				found = true
			}
		}
		if !found {
			t.Error("expected an index named user_id_1 on accounts, per architecture/database-design.md — EnsureIndexes did not create it")
		}
	})
}
