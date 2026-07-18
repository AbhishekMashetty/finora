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

// fakeNotificationClient is a hand-written fake of domain.NotificationClient,
// mirroring fakeExpenseClient above, so the Phase 4 overspend-notification
// trigger can be tested without any live HTTP call to notification-service.
type fakeNotificationClient struct {
	calls []notifyCall
	err   error
}

type notifyCall struct {
	userID, title, message string
}

func (f *fakeNotificationClient) Notify(ctx context.Context, userID, title, message string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, notifyCall{userID: userID, title: title, message: message})
	return nil
}

// discardLogger is a *slog.Logger that writes nowhere, for tests that don't
// care about log output but need a non-nil logger passed to NewReportService.
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

		svc := NewReportService(budgetRepo, expenseClient, &fakeNotificationClient{}, discardLogger())
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

		svc := NewReportService(budgetRepo, expenseClient, &fakeNotificationClient{}, discardLogger())
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

		svc := NewReportService(budgetRepo, expenseClient, &fakeNotificationClient{}, discardLogger())
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

		svc := NewReportService(budgetRepo, expenseClient, &fakeNotificationClient{}, discardLogger())
		if _, err := svc.Summary(ctx, "user-1", from, to); !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
	})
}

// TestReportService_Summary_OverspendNotification covers the Phase 4
// overspend-notification trigger and its dedup rule (see
// notifyIfOverspent's doc comment in report_service.go): a newly-over-budget
// category notifies exactly once per reporting period, an under-budget
// category never notifies, and a later period past the stored
// LastNotifiedAt re-notifies if still over budget.
func TestReportService_Summary_OverspendNotification(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("a newly-over-budget category triggers exactly one notification", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 150}}
		notifier := &fakeNotificationClient{}

		svc := NewReportService(budgetRepo, expenseClient, notifier, discardLogger())
		summary, err := svc.Summary(ctx, "user-1", from, to)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.Categories[0].Remaining != -50 {
			t.Fatalf("Remaining = %v, want -50", summary.Categories[0].Remaining)
		}

		if len(notifier.calls) != 1 {
			t.Fatalf("notifier.calls = %d, want 1", len(notifier.calls))
		}
		if notifier.calls[0].userID != "user-1" {
			t.Errorf("notify userID = %q, want %q", notifier.calls[0].userID, "user-1")
		}
		if notifier.calls[0].title != "Budget exceeded" {
			t.Errorf("notify title = %q, want %q", notifier.calls[0].title, "Budget exceeded")
		}
	})

	t.Run("a second Summary call for the same period does not re-notify (dedup)", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 150}}
		notifier := &fakeNotificationClient{}
		svc := NewReportService(budgetRepo, expenseClient, notifier, discardLogger())

		if _, err := svc.Summary(ctx, "user-1", from, to); err != nil {
			t.Fatalf("first Summary call failed: %v", err)
		}
		if _, err := svc.Summary(ctx, "user-1", from, to); err != nil {
			t.Fatalf("second Summary call failed: %v", err)
		}

		if len(notifier.calls) != 1 {
			t.Fatalf("notifier.calls = %d after two Summary calls for the same period, want 1 (dedup failed)", len(notifier.calls))
		}
	})

	t.Run("a Summary call for a later period re-triggers if still over budget", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 150}}
		notifier := &fakeNotificationClient{}
		svc := NewReportService(budgetRepo, expenseClient, notifier, discardLogger())

		if _, err := svc.Summary(ctx, "user-1", from, to); err != nil {
			t.Fatalf("first Summary call failed: %v", err)
		}
		if len(notifier.calls) != 1 {
			t.Fatalf("notifier.calls after first period = %d, want 1", len(notifier.calls))
		}

		// A later period: LastNotifiedAt (set to time.Now() during the first
		// call, i.e. "today" in the test run) is before nextFrom, which is
		// far enough in the future to always be past it.
		nextFrom := time.Now().UTC().Add(24 * time.Hour)
		nextTo := nextFrom.Add(30 * 24 * time.Hour)
		if _, err := svc.Summary(ctx, "user-1", nextFrom, nextTo); err != nil {
			t.Fatalf("second-period Summary call failed: %v", err)
		}

		if len(notifier.calls) != 2 {
			t.Fatalf("notifier.calls after a later period = %d, want 2 (should re-notify)", len(notifier.calls))
		}
	})

	t.Run("notifying for a recent period does not suppress a later, older-period notification", func(t *testing.T) {
		// Regression for a real bug caught in review: dedup used to compare
		// LastNotifiedAt (wall-clock notify time) against the queried
		// `from` as a time ordering, which wrongly suppressed a legitimate
		// first notification for an older period viewed *after* a more
		// recent one already notified. Dedup now keys on exact-period-match
		// (LastNotifiedAt stores the period's `from`, not real time), so
		// notifying for January must not suppress a never-before-notified
		// March that's queried afterward, even though "now" (when January
		// notified) is after March's `from`.
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 150}}
		notifier := &fakeNotificationClient{}
		svc := NewReportService(budgetRepo, expenseClient, notifier, discardLogger())

		// Notify for a "current" period first (e.g. viewed today).
		currentFrom := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		currentTo := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
		if _, err := svc.Summary(ctx, "user-1", currentFrom, currentTo); err != nil {
			t.Fatalf("current-period Summary call failed: %v", err)
		}
		if len(notifier.calls) != 1 {
			t.Fatalf("notifier.calls after current period = %d, want 1", len(notifier.calls))
		}

		// Now view an *older*, never-before-notified period — must still
		// notify, even though it chronologically precedes the period just
		// notified for.
		olderFrom := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		olderTo := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
		if _, err := svc.Summary(ctx, "user-1", olderFrom, olderTo); err != nil {
			t.Fatalf("older-period Summary call failed: %v", err)
		}
		if len(notifier.calls) != 2 {
			t.Fatalf("notifier.calls after older, never-notified period = %d, want 2 (bug: older-period notification was suppressed)", len(notifier.calls))
		}
	})

	t.Run("an under-budget category never triggers a notification", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 500, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 100}}
		notifier := &fakeNotificationClient{}
		svc := NewReportService(budgetRepo, expenseClient, notifier, discardLogger())

		if _, err := svc.Summary(ctx, "user-1", from, to); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(notifier.calls) != 0 {
			t.Fatalf("notifier.calls = %d, want 0 for an under-budget category", len(notifier.calls))
		}
	})
}
