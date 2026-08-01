package service

import (
	"context"
	"time"

	"github.com/finora/budget-service/internal/domain"
)

// reportService implements domain.ReportService: a real, computed
// budget-vs-actual report built from this service's own budgets plus a
// cross-service REST call to expense-service (via domain.ExpenseClient) for
// actual spend per category. Depends only on domain interfaces, never a
// concrete repository or HTTP client type, so it's unit-testable with fakes.
//
// Phase 7 note: through Phase 6, Summary also triggered a best-effort
// overspend notification as a side effect of being read — a documented
// trade-off accepted only because this fleet had no event bus yet. That
// side effect has been removed entirely: Summary is a pure read again, per
// normal REST/GET semantics. Overspend detection is now triggered by a real
// finora.transaction.created event instead of a page load — see
// overspend_service.go, which independently recomputes the same
// budgeted/actual/remaining figures for exactly this purpose and publishes
// finora.budget.overspent when appropriate.
type reportService struct {
	budgetRepo    domain.BudgetRepository
	expenseClient domain.ExpenseClient
}

// NewReportService builds a ReportService backed by budgetRepo (for the
// caller's budgets) and expenseClient (for actual spend per category).
func NewReportService(budgetRepo domain.BudgetRepository, expenseClient domain.ExpenseClient) domain.ReportService {
	return &reportService{budgetRepo: budgetRepo, expenseClient: expenseClient}
}

func (s *reportService) Summary(ctx context.Context, userID string, from, to time.Time) (*domain.ReportSummary, error) {
	budgets, err := s.budgetRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary := &domain.ReportSummary{
		From:       from.Format(time.RFC3339),
		To:         to.Format(time.RFC3339),
		Categories: make([]domain.CategorySummary, 0, len(budgets)),
	}

	for _, b := range budgets {
		// A budget whose category name has no match in expense-service's
		// categories returns actual=0, not an error — the user simply
		// hasn't logged anything under that name yet (see
		// domain.ExpenseClient's doc comment).
		actual, err := s.expenseClient.SumExpensesByCategory(ctx, userID, b.Category, from, to)
		if err != nil {
			return nil, err
		}
		remaining := b.Amount - actual

		summary.Categories = append(summary.Categories, domain.CategorySummary{
			Category:  b.Category,
			Period:    b.Period,
			Budgeted:  b.Amount,
			Actual:    actual,
			Remaining: remaining,
		})
		summary.TotalBudgeted += b.Amount
		summary.TotalActual += actual
	}

	return summary, nil
}
