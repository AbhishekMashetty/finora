package domain

import (
	"context"
	"time"
)

// CategorySummary is one budget's computed budget-vs-actual figures for the
// requested range.
type CategorySummary struct {
	Category  string  `json:"category"`
	Period    Period  `json:"period"`
	Budgeted  float64 `json:"budgeted"`
	Actual    float64 `json:"actual"`
	Remaining float64 `json:"remaining"`
}

// ReportSummary is the real, computed budget-vs-actual report: one
// CategorySummary per the caller's budgets, plus totals across all of them.
// Built on read from this service's own budgets collection plus a REST call
// to expense-service for actual spend — never persisted.
type ReportSummary struct {
	From          string            `json:"from"`
	To            string            `json:"to"`
	Categories    []CategorySummary `json:"categories"`
	TotalBudgeted float64           `json:"total_budgeted"`
	TotalActual   float64           `json:"total_actual"`
}

// ExpenseClient is the outbound interface report_service depends on to fetch
// a user's actual spend in a category from expense-service. Defined here in
// domain (not in internal/client) so report_service depends only on this
// interface, never the concrete HTTP implementation (Dependency Inversion).
//
// If categoryName has no match among the user's expense-service categories,
// implementations return (0, nil) — the user simply hasn't logged anything
// under that name yet, which is not an error condition.
type ExpenseClient interface {
	SumExpensesByCategory(ctx context.Context, userID, categoryName string, from, to time.Time) (float64, error)
}

// ReportService is the business-logic surface the reports handler depends on.
//
// Phase 7 note: Summary is a pure read again — it used to also trigger an
// overspend notification as a side effect (a documented, deliberate
// trade-off from before this fleet had an event bus), which read-endpoint
// side effect has now been removed entirely. Overspend detection moved to
// overspendService, triggered by a real finora.transaction.created event
// instead of a page load — see internal/service/overspend_service.go.
type ReportService interface {
	Summary(ctx context.Context, userID string, from, to time.Time) (*ReportSummary, error)
}
