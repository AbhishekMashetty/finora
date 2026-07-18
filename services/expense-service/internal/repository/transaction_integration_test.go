//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/repository"
	"github.com/finora/shared/mongotest"
)

func strPtr(s string) *string { return &s }

func TestTransactionRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("expense_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewTransactionRepository(db)
	ctx := context.Background()

	t.Run("create then get round-trips through real Mongo", func(t *testing.T) {
		tx := &domain.Transaction{
			UserID:     "user-1",
			AccountID:  "acc-1",
			CategoryID: strPtr("cat-groceries"),
			Type:       domain.TransactionTypeExpense,
			Amount:     42.50,
			Currency:   "USD",
			Date:       time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			Note:       "Whole Foods",
		}
		if err := repo.Create(ctx, tx); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if tx.ID == "" {
			t.Fatal("Create did not populate an ID")
		}

		got, err := repo.GetByIDForUser(ctx, tx.ID, "user-1")
		if err != nil {
			t.Fatalf("GetByIDForUser: %v", err)
		}
		if got.Amount != 42.50 || got.Note != "Whole Foods" || *got.CategoryID != "cat-groceries" {
			t.Errorf("got %+v, want the transaction just created", got)
		}
	})

	t.Run("pagination and filtering are enforced by the real Mongo query", func(t *testing.T) {
		userID := "pagination-user"
		accountA, accountB := "account-a", "account-b"
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		// 5 on account A (Jan 1-5), 3 on account B (Jan 10-12) — enough to
		// exercise both filtering (by account) and pagination (page size 2).
		for i := 0; i < 5; i++ {
			mustCreate(t, repo, ctx, &domain.Transaction{
				UserID: userID, AccountID: accountA, Type: domain.TransactionTypeExpense,
				Amount: float64(10 + i), Currency: "USD", Date: base.AddDate(0, 0, i),
			})
		}
		for i := 0; i < 3; i++ {
			mustCreate(t, repo, ctx, &domain.Transaction{
				UserID: userID, AccountID: accountB, Type: domain.TransactionTypeExpense,
				Amount: float64(100 + i), Currency: "USD", Date: base.AddDate(0, 0, 10+i),
			})
		}

		// Filter by account: only account A's 5 should match, regardless of
		// what other accounts have.
		filtered, err := repo.ListByUser(ctx, userID, domain.TransactionFilter{AccountID: accountA, Page: 1, PageSize: 100})
		if err != nil {
			t.Fatalf("ListByUser (account filter): %v", err)
		}
		if filtered.Total != 5 {
			t.Errorf("account-filtered Total = %d, want 5", filtered.Total)
		}
		for _, tx := range filtered.Transactions {
			if tx.AccountID != accountA {
				t.Errorf("account filter leaked a transaction from %s", tx.AccountID)
			}
		}

		// Pagination: page size 2 over account A's 5 transactions -> 3 pages,
		// last page has 1, and Total reflects the full match count, not the
		// page size.
		page1, err := repo.ListByUser(ctx, userID, domain.TransactionFilter{AccountID: accountA, Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("ListByUser (page 1): %v", err)
		}
		if len(page1.Transactions) != 2 || page1.Total != 5 {
			t.Errorf("page 1 = %d results (total %d), want 2 results (total 5)", len(page1.Transactions), page1.Total)
		}
		page3, err := repo.ListByUser(ctx, userID, domain.TransactionFilter{AccountID: accountA, Page: 3, PageSize: 2})
		if err != nil {
			t.Fatalf("ListByUser (page 3): %v", err)
		}
		if len(page3.Transactions) != 1 {
			t.Errorf("page 3 = %d results, want 1 (the remainder)", len(page3.Transactions))
		}

		// Date range filter: from Jan 10 onward should only match account B's
		// 3 transactions.
		from := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
		byDate, err := repo.ListByUser(ctx, userID, domain.TransactionFilter{From: &from, Page: 1, PageSize: 100})
		if err != nil {
			t.Fatalf("ListByUser (date filter): %v", err)
		}
		if byDate.Total != 3 {
			t.Errorf("date-filtered Total = %d, want 3", byDate.Total)
		}
	})

	t.Run("ownership is enforced by the real Mongo query", func(t *testing.T) {
		tx := &domain.Transaction{
			UserID: "owner", AccountID: "acc", Type: domain.TransactionTypeExpense,
			Amount: 5, Currency: "USD", Date: time.Now().UTC(),
		}
		if err := repo.Create(ctx, tx); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := repo.GetByIDForUser(ctx, tx.ID, "someone-else"); err != domain.ErrNotFound {
			t.Errorf("cross-user GetByIDForUser: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("EnsureIndexes created the compound (user_id, account_id, date) index and the sparse (user_id, category_id) index", func(t *testing.T) {
		cursor, err := db.Collection("transactions").Indexes().List(ctx)
		if err != nil {
			t.Fatalf("Indexes().List: %v", err)
		}
		defer cursor.Close(ctx)

		var haveCompound, haveSparseCategory bool
		for cursor.Next(ctx) {
			var idx struct {
				Name   string `bson:"name"`
				Sparse bool   `bson:"sparse"`
			}
			if err := cursor.Decode(&idx); err != nil {
				t.Fatalf("decode index: %v", err)
			}
			switch idx.Name {
			case "user_id_1_account_id_1_date_1":
				haveCompound = true
			case "user_id_1_category_id_1":
				haveSparseCategory = true
				if !idx.Sparse {
					t.Error("user_id_1_category_id_1 index exists but is not sparse, per architecture/database-design.md")
				}
			}
		}
		if !haveCompound {
			t.Error("expected compound index user_id_1_account_id_1_date_1 on transactions — EnsureIndexes did not create it")
		}
		if !haveSparseCategory {
			t.Error("expected sparse compound index user_id_1_category_id_1 on transactions — EnsureIndexes did not create it")
		}
	})
}

func mustCreate(t *testing.T, repo *repository.TransactionRepository, ctx context.Context, tx *domain.Transaction) {
	t.Helper()
	if err := repo.Create(ctx, tx); err != nil {
		t.Fatalf("Create: %v", err)
	}
}
