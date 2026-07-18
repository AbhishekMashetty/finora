package domain

import (
	"context"
	"time"
)

// Goal is a user-owned savings target. Full CRUD as of Phase 3: CurrentAmount
// is manually updated by the user via PUT (progress logging) — goals do NOT
// get the cross-service expense-linkage treatment that budgets/reports do,
// per the roadmap's distinction between "savings goals" and reports' "cross-
// service aggregation" clause.
type Goal struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Name          string    `json:"name"`
	TargetAmount  float64   `json:"target_amount"`
	TargetDate    time.Time `json:"target_date"`
	CurrentAmount float64   `json:"current_amount"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GoalRepository persists and queries Goal documents, always scoped to a
// specific owning user.
type GoalRepository interface {
	Create(ctx context.Context, goal *Goal) error
	ListByUser(ctx context.Context, userID string) ([]Goal, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*Goal, error)
	Update(ctx context.Context, goal *Goal) error
	DeleteByIDForUser(ctx context.Context, id, userID string) error
}

// GoalService is the business-logic surface handlers depend on.
type GoalService interface {
	Create(ctx context.Context, userID string, in CreateGoalInput) (*Goal, error)
	List(ctx context.Context, userID string) ([]Goal, error)
	Get(ctx context.Context, userID, id string) (*Goal, error)
	Update(ctx context.Context, userID, id string, in UpdateGoalInput) (*Goal, error)
	Delete(ctx context.Context, userID, id string) error
}

// CreateGoalInput carries validated fields for creating a Goal.
type CreateGoalInput struct {
	Name         string
	TargetAmount float64
	TargetDate   time.Time
}

// UpdateGoalInput carries validated fields for updating a Goal, including
// manual progress logging via CurrentAmount.
type UpdateGoalInput struct {
	Name          string
	TargetAmount  float64
	TargetDate    time.Time
	CurrentAmount float64
}
