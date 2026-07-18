package domain

import (
	"context"
	"time"
)

// TransactionType enumerates whether a transaction increases or decreases funds.
type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "income"
	TransactionTypeExpense TransactionType = "expense"
)

// ValidTransactionType reports whether t is a recognized transaction type.
func ValidTransactionType(t TransactionType) bool {
	switch t {
	case TransactionTypeIncome, TransactionTypeExpense:
		return true
	default:
		return false
	}
}

// Transaction is a single user-owned income/expense entry against an account.
type Transaction struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	AccountID  string          `json:"account_id"`
	CategoryID *string         `json:"category_id,omitempty"`
	Type       TransactionType `json:"type"`
	Amount     float64         `json:"amount"`
	Currency   string          `json:"currency"`
	Date       time.Time       `json:"date"`
	Note       string          `json:"note,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// TransactionFilter narrows a ListByUser query. Zero values mean "no filter".
type TransactionFilter struct {
	AccountID string
	Category  string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}

// TransactionPage is one page of transactions plus the total matching count
// (ignoring pagination), so callers can compute total pages.
type TransactionPage struct {
	Transactions []Transaction
	Total        int64
}

// TransactionRepository persists and queries Transaction documents, always
// scoped to a specific owning user.
type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	ListByUser(ctx context.Context, userID string, filter TransactionFilter) (TransactionPage, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*Transaction, error)
	Update(ctx context.Context, tx *Transaction) error
	DeleteByIDForUser(ctx context.Context, id, userID string) error
}

// TransactionService is the business-logic surface handlers depend on.
type TransactionService interface {
	Create(ctx context.Context, userID string, in CreateTransactionInput) (*Transaction, error)
	List(ctx context.Context, userID string, in ListTransactionsInput) (TransactionPage, error)
	Get(ctx context.Context, userID, id string) (*Transaction, error)
	Update(ctx context.Context, userID, id string, in UpdateTransactionInput) (*Transaction, error)
	Delete(ctx context.Context, userID, id string) error
}

// CreateTransactionInput carries validated fields for creating a Transaction.
type CreateTransactionInput struct {
	AccountID  string
	CategoryID *string
	Type       TransactionType
	Amount     float64
	Currency   string
	Date       time.Time
	Note       string
}

// UpdateTransactionInput carries validated fields for updating a Transaction.
type UpdateTransactionInput struct {
	AccountID  string
	CategoryID *string
	Type       TransactionType
	Amount     float64
	Currency   string
	Date       time.Time
	Note       string
}

// ListTransactionsInput carries validated/normalized query params for listing.
type ListTransactionsInput struct {
	AccountID string
	Category  string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}
