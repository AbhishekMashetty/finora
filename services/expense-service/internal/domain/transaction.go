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
// (ignoring pagination), so callers can compute total pages. Page/PageSize
// carry the *resolved* values the service layer actually applied (after
// defaulting/capping) — not necessarily what the caller originally
// requested — so a handler echoing them back in the response reports what
// really happened, not an unvalidated echo of the raw query params.
type TransactionPage struct {
	Transactions []Transaction
	Page         int
	PageSize     int
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
	Import(ctx context.Context, userID, accountID string, rows []ImportRow) (ImportResult, error)
}

// ImportRow is one CSV data row, still raw strings — column parsing
// (encoding/csv, header aliasing) is the handler's job, same as JSON
// binding is for Create/Update; turning those raw strings into a valid
// Transaction (date formats, amount signs, income-vs-expense inference) is
// business logic, so it happens in the service, not the handler.
//
// A CSV carries its amount one of two shapes, never both interpreted at
// once (Amount wins if both happen to be present): a single signed
// "Amount" column (sign gives the direction), or a separate "Debit"/
// "Credit" pair — the standard bank-statement shape, where both columns
// hold unsigned magnitudes and only one is populated per row. Capital
// One's own real CSV export uses exactly the latter shape (Debit for
// purchases, Credit for payments), which is what makes this a real
// column shape to support, not a hypothetical one — a naive "Debit is
// just another alias for Amount" treatment silently mislabeled every
// purchase as income, since Debit values are always positive.
type ImportRow struct {
	Date        string
	Description string
	Amount      string
	Debit       string
	Credit      string
	// Type is optional: an explicit "income"/"expense" override. Most
	// credit card statement exports don't have this column at all — when
	// empty, the type is inferred from whichever amount shape is present:
	// Amount's sign (negative = expense, positive = income), or which of
	// Debit/Credit is populated (Debit = expense, Credit = income).
	Type string
}

// ImportRowError explains why one CSV row was skipped. Row is 1-indexed
// against the uploaded file's own lines, with the header counted as row 1
// (so the first data row is row 2) — matching what a user sees if they
// open the CSV in a spreadsheet, not a 0-indexed slice position.
type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportResult summarizes a CSV import. Errors is capped (see
// service.maxImportErrors) so a garbage file with thousands of bad rows
// doesn't blow up the response — Skipped still reports the true total.
type ImportResult struct {
	Imported int              `json:"imported"`
	Skipped  int              `json:"skipped"`
	Errors   []ImportRowError `json:"errors"`
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
