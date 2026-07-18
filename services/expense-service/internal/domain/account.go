// Package domain contains plain data types and repository/service interfaces
// for expense-service. It must never import gin, the mongo driver, or any
// other framework — that keeps the business rules testable in isolation and
// keeps the dependency direction pointing inward (handlers/repositories
// depend on domain, not the other way around).
package domain

import (
	"context"
	"errors"
	"time"
)

// AccountType enumerates the kinds of financial accounts a user can track.
type AccountType string

const (
	AccountTypeChecking AccountType = "checking"
	AccountTypeSavings  AccountType = "savings"
	AccountTypeCredit   AccountType = "credit"
	AccountTypeCash     AccountType = "cash"
)

// ValidAccountType reports whether t is one of the recognized account types.
func ValidAccountType(t AccountType) bool {
	switch t {
	case AccountTypeChecking, AccountTypeSavings, AccountTypeCredit, AccountTypeCash:
		return true
	default:
		return false
	}
}

// Account is a user-owned financial account (bank account, credit card, cash wallet, ...).
type Account struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Name      string      `json:"name"`
	Type      AccountType `json:"type"`
	Currency  string      `json:"currency"`
	Balance   float64     `json:"balance"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// ErrNotFound is returned by repositories when a resource doesn't exist, or
// exists but belongs to a different user. Callers must never distinguish
// between those two cases in the response (see ownership rule in
// architecture/api-contracts.md) — both map to 404 NOT_FOUND.
var ErrNotFound = errors.New("resource not found")

// AccountRepository persists and queries Account documents, always scoped to
// a specific owning user.
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	ListByUser(ctx context.Context, userID string) ([]Account, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*Account, error)
	Update(ctx context.Context, account *Account) error
	DeleteByIDForUser(ctx context.Context, id, userID string) error
}

// AccountService is the business-logic surface handlers depend on.
type AccountService interface {
	Create(ctx context.Context, userID string, in CreateAccountInput) (*Account, error)
	List(ctx context.Context, userID string) ([]Account, error)
	Get(ctx context.Context, userID, id string) (*Account, error)
	Update(ctx context.Context, userID, id string, in UpdateAccountInput) (*Account, error)
	Delete(ctx context.Context, userID, id string) error
}

// CreateAccountInput carries validated fields for creating an Account.
type CreateAccountInput struct {
	Name     string
	Type     AccountType
	Currency string
}

// UpdateAccountInput carries validated fields for updating an Account.
type UpdateAccountInput struct {
	Name     string
	Type     AccountType
	Currency string
	Balance  *float64
}
