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

// NotificationClient is the outbound interface report_service depends on to
// create an in-app notification for a user when Summary finds a budget over
// spend. Defined here in domain (not internal/client) so report_service
// depends only on this interface, never the concrete HTTP implementation
// (Dependency Inversion) — this keeps report_service unit-testable with a
// fake, per CLAUDE.md §7.
//
// This is a deliberate, documented trade-off: Finora has no event bus yet
// (Phase 7 introduces one), so a read endpoint (GET /reports/summary) is the
// only place that currently computes "is this budget over spent," and it
// triggers the notification as a side effect of being read rather than a
// real domain event. See architecture/api-contracts.md's
// budget-service -> notification-service subsection.
type NotificationClient interface {
	Notify(ctx context.Context, userID, title, message string) error
}

// ReportService is the business-logic surface the reports handler depends on.
type ReportService interface {
	Summary(ctx context.Context, userID string, from, to time.Time) (*ReportSummary, error)
}
