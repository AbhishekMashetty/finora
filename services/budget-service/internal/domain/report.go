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

// ExpenseSummary is one category's actual expense total for a requested
// [from, to] range, keyed by category NAME — not expense-service's
// internal category id — because both report_service and overspendService
// only ever need to match it against a budget's own Category field, which
// is a name too.
type ExpenseSummary struct {
	Category string
	Actual   float64
}

// ExpenseClient is the outbound interface report_service and
// overspendService depend on to fetch a user's actual spend, per category,
// from expense-service. Defined here in domain (not in internal/client) so
// callers depend only on this interface, never the concrete HTTP
// implementation (Dependency Inversion).
//
// SumExpensesByCategory returns one entry per category the caller has any
// EXPENSE-type transactions in for [from, to] — never one entry per
// budget, and never one call per budget. That's the whole point: the
// interface used to return a single category's sum per call
// (SumExpensesByCategory(ctx, userID, categoryName, from, to) (float64,
// error)), which meant a caller with B budgets made B calls, each walking
// its own paginated transaction list client-side — B*(1+P) internal HTTP
// requests to expense-service for one user action (see
// infrastructure/scale-readiness-review.html finding F01). This shape
// costs exactly one call, independent of budget count: implementations
// fetch every category's total in a single aggregation query and let the
// caller match categories by name locally.
//
// A budget whose category has no entry in the returned slice hasn't had
// anything logged under that name in range yet — the caller's job is to
// treat that as actual=0, not an error. Category name matching against a
// budget's own Category field should be done case-insensitively (the same
// contract the old per-category lookup honored via strings.EqualFold),
// since a budget's category name and expense-service's category name are
// independently user-typed strings.
type ExpenseClient interface {
	SumExpensesByCategory(ctx context.Context, userID string, from, to time.Time) ([]ExpenseSummary, error)
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
