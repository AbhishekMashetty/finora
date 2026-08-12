//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/repository"
	"github.com/finora/shared/mongotest"
	"go.mongodb.org/mongo-driver/bson"
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

	t.Run("AggregateByCategory groups by category via a real Mongo aggregation pipeline", func(t *testing.T) {
		userID := "aggregate-user"
		groceries, rent := strPtr("cat-groceries"), strPtr("cat-rent")
		from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)

		mustCreate(t, repo, ctx, &domain.Transaction{
			UserID: userID, AccountID: "acc", CategoryID: groceries, Type: domain.TransactionTypeExpense,
			Amount: 40, Currency: "USD", Date: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
		})
		mustCreate(t, repo, ctx, &domain.Transaction{
			UserID: userID, AccountID: "acc", CategoryID: groceries, Type: domain.TransactionTypeExpense,
			Amount: 25, Currency: "USD", Date: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		})
		mustCreate(t, repo, ctx, &domain.Transaction{
			UserID: userID, AccountID: "acc", CategoryID: rent, Type: domain.TransactionTypeExpense,
			Amount: 1200, Currency: "USD", Date: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		})
		// Out of range, wrong type, no category, and another user's data —
		// none of these should appear in the result.
		mustCreate(t, repo, ctx, &domain.Transaction{
			UserID: userID, AccountID: "acc", CategoryID: groceries, Type: domain.TransactionTypeExpense,
			Amount: 999, Currency: "USD", Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		})
		mustCreate(t, repo, ctx, &domain.Transaction{
			UserID: userID, AccountID: "acc", CategoryID: groceries, Type: domain.TransactionTypeIncome,
			Amount: 999, Currency: "USD", Date: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		})
		mustCreate(t, repo, ctx, &domain.Transaction{
			UserID: userID, AccountID: "acc", Type: domain.TransactionTypeExpense,
			Amount: 999, Currency: "USD", Date: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		})
		mustCreate(t, repo, ctx, &domain.Transaction{
			UserID: "someone-else", AccountID: "acc", CategoryID: groceries, Type: domain.TransactionTypeExpense,
			Amount: 999, Currency: "USD", Date: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		})

		totals, err := repo.AggregateByCategory(ctx, userID, domain.TransactionTypeExpense, from, to)
		if err != nil {
			t.Fatalf("AggregateByCategory: %v", err)
		}

		byCategory := make(map[string]domain.CategoryTotal, len(totals))
		for _, ct := range totals {
			byCategory[ct.CategoryID] = ct
		}
		if len(totals) != 2 {
			t.Fatalf("len(totals) = %d, want 2 (got %+v)", len(totals), totals)
		}
		if g := byCategory[*groceries]; g.Total != 65 || g.Count != 2 {
			t.Errorf("groceries = %+v, want Total=65 Count=2", g)
		}
		if r := byCategory[*rent]; r.Total != 1200 || r.Count != 1 {
			t.Errorf("rent = %+v, want Total=1200 Count=1", r)
		}
	})

	t.Run("EnsureIndexes created idx_user_date, idx_user_account_date, and the sparse idx_user_category_date", func(t *testing.T) {
		cursor, err := db.Collection("transactions").Indexes().List(ctx)
		if err != nil {
			t.Fatalf("Indexes().List: %v", err)
		}
		defer cursor.Close(ctx)

		var haveUserDate, haveUserAccountDate, haveSparseCategoryDate bool
		for cursor.Next(ctx) {
			var idx struct {
				Name   string `bson:"name"`
				Sparse bool   `bson:"sparse"`
			}
			if err := cursor.Decode(&idx); err != nil {
				t.Fatalf("decode index: %v", err)
			}
			switch idx.Name {
			case "idx_user_date":
				haveUserDate = true
			case "idx_user_account_date":
				haveUserAccountDate = true
			case "idx_user_category_date":
				haveSparseCategoryDate = true
				if !idx.Sparse {
					t.Error("idx_user_category_date index exists but is not sparse, per architecture/database-design.md")
				}
			}
		}
		if !haveUserDate {
			t.Error("expected idx_user_date on transactions — EnsureIndexes did not create it")
		}
		if !haveUserAccountDate {
			t.Error("expected idx_user_account_date on transactions — EnsureIndexes did not create it")
		}
		if !haveSparseCategoryDate {
			t.Error("expected sparse idx_user_category_date on transactions — EnsureIndexes did not create it")
		}

		// The Phase-6 indexes this replaced (date ascending, wrong compound
		// shape for the sort) must actually be gone, not just superseded —
		// EnsureIndexes explicitly drops them by name (see mongo.go) so a
		// database that's run the old code doesn't end up carrying both the
		// old, sort-unfriendly index and the new one forever.
		cursor2, err := db.Collection("transactions").Indexes().List(ctx)
		if err != nil {
			t.Fatalf("Indexes().List (second pass): %v", err)
		}
		defer cursor2.Close(ctx)
		for cursor2.Next(ctx) {
			var idx struct {
				Name string `bson:"name"`
			}
			if err := cursor2.Decode(&idx); err != nil {
				t.Fatalf("decode index: %v", err)
			}
			if idx.Name == "user_id_1_account_id_1_date_1" || idx.Name == "user_id_1_category_id_1" {
				t.Errorf("old index %q should have been dropped by EnsureIndexes, but it's still present", idx.Name)
			}
		}
	})

	t.Run("the default (no account filter) list query is served by an index, with no blocking in-memory SORT stage", func(t *testing.T) {
		// Regression test for the exact bug this file's index changes fix:
		// ListByUser always sorts {date: -1}, and when the caller doesn't
		// filter by account_id (the default dashboard view), the old
		// {user_id, account_id, date} compound index couldn't serve that
		// sort from the index — Mongo fell back to a blocking in-memory
		// SORT over the user's entire matched set on every page. Assert
		// directly against the query planner rather than against timing,
		// so this fails deterministically the moment the index stops
		// matching the query shape, regardless of how much test data exists.
		var explainResult bson.M
		explainCmd := bson.D{
			{Key: "explain", Value: bson.D{
				{Key: "find", Value: "transactions"},
				{Key: "filter", Value: bson.D{{Key: "user_id", Value: "pagination-user"}}},
				{Key: "sort", Value: bson.D{{Key: "date", Value: -1}}},
			}},
			{Key: "verbosity", Value: "executionStats"},
		}
		if err := db.RunCommand(ctx, explainCmd).Decode(&explainResult); err != nil {
			t.Fatalf("explain: %v", err)
		}

		queryPlanner, ok := explainResult["queryPlanner"].(bson.M)
		if !ok {
			t.Fatalf("explain result has no queryPlanner map: %+v", explainResult)
		}
		winningPlan, ok := queryPlanner["winningPlan"].(bson.M)
		if !ok {
			t.Fatalf("explain result has no winningPlan map: %+v", queryPlanner)
		}

		stages := planStageNames(winningPlan)
		for _, s := range stages {
			if s == "SORT" {
				t.Errorf("winning plan uses a blocking in-memory SORT stage (stages seen: %v) — "+
					"idx_user_date should let Mongo serve {user_id, sort:date} directly from the index", stages)
			}
		}
		hasIXSCAN := false
		for _, s := range stages {
			if s == "IXSCAN" {
				hasIXSCAN = true
			}
		}
		if !hasIXSCAN {
			t.Errorf("winning plan does not use an index scan at all (stages seen: %v)", stages)
		}
	})
}

// planStageNames walks a MongoDB explain() winningPlan tree (a "stage"
// field plus an optional single "inputStage" or an "inputStages" array —
// the two shapes explain() actually produces, depending on the stage type)
// and returns every stage name found, so a test can assert on the plan's
// shape without hardcoding the whole tree structure.
func planStageNames(plan bson.M) []string {
	var names []string
	if s, ok := plan["stage"].(string); ok {
		names = append(names, s)
	}
	if input, ok := plan["inputStage"].(bson.M); ok {
		names = append(names, planStageNames(input)...)
	}
	if inputs, ok := plan["inputStages"].(bson.A); ok {
		for _, in := range inputs {
			if m, ok := in.(bson.M); ok {
				names = append(names, planStageNames(m)...)
			}
		}
	}
	return names
}

func mustCreate(t *testing.T, repo *repository.TransactionRepository, ctx context.Context, tx *domain.Transaction) {
	t.Helper()
	if err := repo.Create(ctx, tx); err != nil {
		t.Fatalf("Create: %v", err)
	}
}
