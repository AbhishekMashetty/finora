// Package domain contains plain data types and repository/service interfaces
// for budget-service. It must never import gin, the mongo driver, or any
// other framework — that keeps the business rules testable in isolation and
// keeps the dependency direction pointing inward (handlers/repositories
// depend on domain, not the other way around).
package domain

import (
	"context"
	"errors"
	"time"
)

// Period enumerates the cadence a budget resets on.
type Period string

const (
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
	PeriodYearly  Period = "yearly"
)

// ValidPeriod reports whether p is one of the recognized budget periods.
func ValidPeriod(p Period) bool {
	switch p {
	case PeriodWeekly, PeriodMonthly, PeriodYearly:
		return true
	default:
		return false
	}
}

// Budget is a user-owned spending limit for a category over a period.
type Budget struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Category  string    `json:"category"`
	Amount    float64   `json:"amount"`
	Period    Period    `json:"period"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// LastNotifiedAt is internal bookkeeping for the Phase 4 overspend
	// notification trigger (see internal/service/report_service.go's
	// notifyIfOverspent). Despite the name, it stores the `from` boundary of
	// the report *period* that was last notified for, not a wall-clock
	// timestamp — dedup is keyed on exact-period-match (not time ordering),
	// specifically so notifying for a recent period never suppresses a
	// legitimate first notification for an older, previously-unviewed
	// period. Deliberately json:"-" — it's not something a frontend should
	// see or set, only something reports' dedup logic reads/writes via the
	// repository. nil means "never notified for any period".
	LastNotifiedAt *time.Time `json:"-"`
}

// ErrNotFound is returned by repositories when a resource doesn't exist, or
// exists but belongs to a different user. Callers must never distinguish
// between those two cases in the response (see ownership rule in
// architecture/api-contracts.md) — both map to 404 NOT_FOUND.
var ErrNotFound = errors.New("resource not found")

// ErrValidation is the sentinel wrapped by service-layer input validation
// failures, so handlers can errors.Is against it regardless of which
// service produced the error.
var ErrValidation = errors.New("validation error")

// BudgetRepository persists and queries Budget documents, always scoped to a
// specific owning user.
type BudgetRepository interface {
	Create(ctx context.Context, budget *Budget) error
	ListByUser(ctx context.Context, userID string) ([]Budget, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*Budget, error)
	Update(ctx context.Context, budget *Budget) error
	DeleteByIDForUser(ctx context.Context, id, userID string) error
}

// BudgetService is the business-logic surface handlers depend on.
type BudgetService interface {
	Create(ctx context.Context, userID string, in CreateBudgetInput) (*Budget, error)
	List(ctx context.Context, userID string) ([]Budget, error)
	Get(ctx context.Context, userID, id string) (*Budget, error)
	Update(ctx context.Context, userID, id string, in UpdateBudgetInput) (*Budget, error)
	Delete(ctx context.Context, userID, id string) error
}

// CreateBudgetInput carries validated fields for creating a Budget.
type CreateBudgetInput struct {
	Category string
	Amount   float64
	Period   Period
}

// UpdateBudgetInput carries validated fields for updating a Budget.
type UpdateBudgetInput struct {
	Category string
	Amount   float64
	Period   Period
}
