package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/finora/budget-service/internal/domain"
)

// fakeEventPublisher is a hand-written fake of domain.EventPublisher
// (Phase 7), recording every published budget.overspent event so tests can
// assert on it without a real outbox/NATS.
type fakeEventPublisher struct {
	published []domain.BudgetOverspentEvent
	err       error
}

func (f *fakeEventPublisher) PublishBudgetOverspent(_ context.Context, event domain.BudgetOverspentEvent) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, event)
	return nil
}

// TestOverspendService_HandleTransactionCreated covers the Phase 7
// overspend-notification trigger and its dedup rule — the same rule
// report_service.go's now-removed notifyIfOverspent used, just evaluated
// here (triggered by an event, not a page load) and publishing instead of
// calling notification-service over REST. See overspend_service.go's own
// doc comment for why this only ever evaluates the CURRENT period, not an
// arbitrary caller-chosen one.
func TestOverspendService_HandleTransactionCreated(t *testing.T) {
	t.Run("a newly-over-budget category publishes exactly one event", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 150}}
		pub := &fakeEventPublisher{}
		svc := NewOverspendService(budgetRepo, expenseClient, pub, discardLogger())

		now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		if err := svc.HandleTransactionCreated(ctx, "user-1", now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(pub.published) != 1 {
			t.Fatalf("published = %d, want 1", len(pub.published))
		}
		event := pub.published[0]
		if event.UserID != "user-1" || event.Category != "groceries" || event.Budgeted != 100 || event.Actual != 150 {
			t.Errorf("published event = %+v, want UserID=user-1 Category=groceries Budgeted=100 Actual=150", event)
		}
	})

	t.Run("a second event for the same period does not re-publish (dedup)", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 150}}
		pub := &fakeEventPublisher{}
		svc := NewOverspendService(budgetRepo, expenseClient, pub, discardLogger())

		now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		later := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC) // same month -> same currentPeriodStart

		if err := svc.HandleTransactionCreated(ctx, "user-1", now); err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		if err := svc.HandleTransactionCreated(ctx, "user-1", later); err != nil {
			t.Fatalf("second call failed: %v", err)
		}

		if len(pub.published) != 1 {
			t.Fatalf("published = %d after two events in the same period, want 1 (dedup failed)", len(pub.published))
		}
	})

	t.Run("an event in a later period re-triggers if still over budget", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 150}}
		pub := &fakeEventPublisher{}
		svc := NewOverspendService(budgetRepo, expenseClient, pub, discardLogger())

		january := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		february := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)

		if err := svc.HandleTransactionCreated(ctx, "user-1", january); err != nil {
			t.Fatalf("january call failed: %v", err)
		}
		if len(pub.published) != 1 {
			t.Fatalf("published after january = %d, want 1", len(pub.published))
		}

		if err := svc.HandleTransactionCreated(ctx, "user-1", february); err != nil {
			t.Fatalf("february call failed: %v", err)
		}
		if len(pub.published) != 2 {
			t.Fatalf("published after february = %d, want 2 (a new period should re-trigger)", len(pub.published))
		}
	})

	t.Run("an under-budget category never publishes", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()

		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 500, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{"groceries": 100}}
		pub := &fakeEventPublisher{}
		svc := NewOverspendService(budgetRepo, expenseClient, pub, discardLogger())

		if err := svc.HandleTransactionCreated(ctx, "user-1", time.Now().UTC()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("published = %d, want 0 for an under-budget category", len(pub.published))
		}
	})

	t.Run("a per-period expense-client error is logged and does not prevent checking the user's budgets in other periods", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()
		now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC) // a Wednesday

		// Two budgets on DIFFERENT periods, so HandleTransactionCreated makes
		// two separate SumExpensesByCategory calls (one per distinct period
		// start) rather than one shared call — see overspend_service.go's
		// doc comment on why isolation is now per-period, not per-budget.
		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "flaky", Amount: 100, Period: domain.PeriodMonthly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}
		if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: "groceries", Amount: 100, Period: domain.PeriodWeekly}); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		monthlyStart := currentPeriodStart(domain.PeriodMonthly, now)
		expenseClient := &fakeExpenseClient{
			errForFrom:    &monthlyStart,
			errForFromErr: errors.New("expense-service unreachable"),
			byCategory:    map[string]float64{"groceries": 150},
		}
		pub := &fakeEventPublisher{}
		svc := NewOverspendService(budgetRepo, expenseClient, pub, discardLogger())

		if err := svc.HandleTransactionCreated(ctx, "user-1", now); err != nil {
			t.Fatalf("expected HandleTransactionCreated to return nil (per-period errors are logged, not propagated), got: %v", err)
		}
		if len(pub.published) != 1 {
			t.Fatalf("published = %d, want 1 (the weekly budget's period should still be checked despite the monthly period's expense-client error)", len(pub.published))
		}
		if pub.published[0].Category != "groceries" {
			t.Errorf("published category = %q, want groceries", pub.published[0].Category)
		}
	})

	t.Run("budgets sharing the same period make exactly one expense-client call, not one per budget", func(t *testing.T) {
		budgetRepo := newFakeBudgetRepository()
		ctx := context.Background()
		now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)

		for _, cat := range []string{"groceries", "rent", "utilities"} {
			if _, err := NewBudgetService(budgetRepo).Create(ctx, "user-1", domain.CreateBudgetInput{Category: cat, Amount: 100, Period: domain.PeriodMonthly}); err != nil {
				t.Fatalf("setup create failed: %v", err)
			}
		}

		expenseClient := &fakeExpenseClient{byCategory: map[string]float64{}}
		pub := &fakeEventPublisher{}
		svc := NewOverspendService(budgetRepo, expenseClient, pub, discardLogger())

		if err := svc.HandleTransactionCreated(ctx, "user-1", now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(expenseClient.calls) != 1 {
			t.Fatalf("expenseClient.calls = %d, want 1 for three budgets sharing one period — this is the whole point of F01's grouping", len(expenseClient.calls))
		}
	})

	t.Run("expense-client error propagates for List but not per-category lookups", func(t *testing.T) {
		budgetRepo := &erroringListBudgetRepository{err: errors.New("mongo unreachable")}
		expenseClient := &fakeExpenseClient{}
		pub := &fakeEventPublisher{}
		svc := NewOverspendService(budgetRepo, expenseClient, pub, discardLogger())

		if err := svc.HandleTransactionCreated(context.Background(), "user-1", time.Now().UTC()); err == nil {
			t.Fatal("expected an error when the budget repository itself fails")
		}
	})
}

func TestCurrentPeriodStart(t *testing.T) {
	tests := []struct {
		name   string
		period domain.Period
		now    time.Time
		want   time.Time
	}{
		{
			name:   "monthly returns the 1st of the current month",
			period: domain.PeriodMonthly,
			now:    time.Date(2026, 3, 17, 14, 30, 0, 0, time.UTC),
			want:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "yearly returns January 1st of the current year",
			period: domain.PeriodYearly,
			now:    time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC),
			want:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "weekly returns the most recent Monday",
			period: domain.PeriodWeekly,
			now:    time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC), // a Wednesday
			want:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),  // the preceding Monday
		},
		{
			name:   "weekly on a Monday returns the same day",
			period: domain.PeriodWeekly,
			now:    time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC), // already a Monday
			want:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "weekly on a Sunday returns the Monday six days earlier",
			period: domain.PeriodWeekly,
			now:    time.Date(2026, 3, 22, 8, 0, 0, 0, time.UTC), // a Sunday
			want:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := currentPeriodStart(tt.period, tt.now)
			if !got.Equal(tt.want) {
				t.Errorf("currentPeriodStart(%v, %v) = %v, want %v", tt.period, tt.now, got, tt.want)
			}
		})
	}
}

// erroringListBudgetRepository fails ListByUser unconditionally, to prove a
// repository-level failure (as opposed to a per-category expense-client
// failure) genuinely propagates.
type erroringListBudgetRepository struct {
	err error
}

func (f *erroringListBudgetRepository) Create(context.Context, *domain.Budget) error { return nil }
func (f *erroringListBudgetRepository) ListByUser(context.Context, string) ([]domain.Budget, error) {
	return nil, f.err
}
func (f *erroringListBudgetRepository) GetByIDForUser(context.Context, string, string) (*domain.Budget, error) {
	return nil, domain.ErrNotFound
}
func (f *erroringListBudgetRepository) Update(context.Context, *domain.Budget) error { return nil }
func (f *erroringListBudgetRepository) DeleteByIDForUser(context.Context, string, string) error {
	return nil
}

var _ domain.BudgetRepository = (*erroringListBudgetRepository)(nil)
