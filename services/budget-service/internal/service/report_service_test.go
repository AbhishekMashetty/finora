package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/finora/budget-service/internal/domain"
)

// fakeExpenseClient is a hand-written fake of domain.ExpenseClient so
// report_service can be tested without any live HTTP call to
// expense-service, per CLAUDE.md §7 (no live Mongo/HTTP in unit tests).
type fakeExpenseClient struct {
	// byCategory maps a category name (case-sensitive, tests use exact
	// matches) to the actual-spend figure that category should report.
	// Categories absent from this map return (0, nil), mirroring the real
	// client's "no matching expense-service category" contract.
	byCategory map[string]float64
	err        error
}

func (f *fakeExpenseClient) SumExpensesByCategory(ctx context.Context, userID, categoryName string, from, to time.Time) (float64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.byCategory[categoryName], nil
}

// discardLogger is a *slog.Logger that writes nowhere, for tests that don't
// care about log output but need a non-nil logger (used by
// overspend_service_test.go, in this same package).
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestReportService_Summary(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("computes budgeted/actual/remaining per budget and totals", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 500, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}
		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "rent", Amount: 1000, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}
		// A different user's budget must never leak into user-1's report.
		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-2", domain.CreateBudgetInput{Category: "groceries", Amount: 9999, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{
			"groceries": 320,
			"rent":      1000,
		}}

		svc := NewReportService(budgetRepo, expenseClient)
		summary, err := svc.Summary(ctx, "user-1", from, to)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(summary.Categories) != 2 {
			t.Fatalf("len(Categories) = %d, want 2 (got %+v)", len(summary.Categories), summary.Categories)
		}

		byCategory := make(map[string]domain.CategorySummary, len(summary.Categories))
		for _, cs := range summary.Categories {
			byCategory[cs.Category] = cs
		}

		groceries, ok := byCategory["groceries"]
		if !ok {
			t.Fatalf("expected a groceries entry, got %+v", summary.Categories)
		}
		if groceries.Budgeted != 500 || groceries.Actual != 320 || groceries.Remaining != 180 {
			t.Errorf("groceries = %+v, want Budgeted=500 Actual=320 Remaining=180", groceries)
		}

		rent, ok := byCategory["rent"]
		if !ok {
			t.Fatalf("expected a rent entry, got %+v", summary.Categories)
		}
		if rent.Budgeted != 1000 || rent.Actual != 1000 || rent.Remaining != 0 {
			t.Errorf("rent = %+v, want Budgeted=1000 Actual=1000 Remaining=0", rent)
		}

		if summary.TotalBudgeted != 1500 {
			t.Errorf("TotalBudgeted = %v, want 1500", summary.TotalBudgeted)
		}
		if summary.TotalActual != 1320 {
			t.Errorf("TotalActual = %v, want 1320", summary.TotalActual)
		}
		if summary.From != from.Format(time.RFC3339) || summary.To != to.Format(time.RFC3339) {
			t.Errorf("From/To = %q/%q, want %q/%q", summary.From, summary.To, from.Format(time.RFC3339), to.Format(time.RFC3339))
		}
	})

	t.Run("budget category with no expense-service match reports actual=0, not an error", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "hobbies", Amount: 200, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		// Empty byCategory map -- every lookup misses, exactly like a user
		// who has never logged a transaction under this category name.
		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{}}

		svc := NewReportService(budgetRepo, expenseClient)
		summary, err := svc.Summary(ctx, "user-1", from, to)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(summary.Categories) != 1 {
			t.Fatalf("len(Categories) = %d, want 1", len(summary.Categories))
		}
		if summary.Categories[0].Actual != 0 {
			t.Errorf("Actual = %v, want 0", summary.Categories[0].Actual)
		}
		if summary.Categories[0].Remaining != 200 {
			t.Errorf("Remaining = %v, want 200", summary.Categories[0].Remaining)
		}
	})

	t.Run("no budgets means an empty report, not an error", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{}}

		svc := NewReportService(budgetRepo, expenseClient)
		summary, err := svc.Summary(context.Background(), "user-with-no-budgets", from, to)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(summary.Categories) != 0 {
			t.Errorf("len(Categories) = %d, want 0", len(summary.Categories))
		}
		if summary.TotalBudgeted != 0 || summary.TotalActual != 0 {
			t.Errorf("totals = %v/%v, want 0/0", summary.TotalBudgeted, summary.TotalActual)
		}
	})

	t.Run("expense-service error propagates instead of being swallowed", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 500, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		wantErr := errors.New("expense-service unreachable")
		expenseClient := &fakeExpenseClient{err: wantErr}

		svc := NewReportService(budgetRepo, expenseClient)
		if _, err := svc.Summary(ctx, "user-1", from, to); !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
	})
}
