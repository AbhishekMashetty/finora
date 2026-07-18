package domain

import (
	"context"
	"time"
)

// Category is a user-owned label used to classify transactions (e.g.
// "Groceries", "Salary"). Kept intentionally minimal: create + list only.
type Category struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Name      string          `json:"name"`
	Type      TransactionType `json:"type"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// CategoryRepository persists and queries Category documents, always scoped
// to a specific owning user.
type CategoryRepository interface {
	Create(ctx context.Context, category *Category) error
	ListByUser(ctx context.Context, userID string) ([]Category, error)
}

// CategoryService is the business-logic surface handlers depend on.
type CategoryService interface {
	Create(ctx context.Context, userID string, in CreateCategoryInput) (*Category, error)
	List(ctx context.Context, userID string) ([]Category, error)
}

// CreateCategoryInput carries validated fields for creating a Category.
type CreateCategoryInput struct {
	Name string
	Type TransactionType
}
