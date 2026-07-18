package service

import (
	"context"
	"strings"
	"time"

	"github.com/finora/expense-service/internal/domain"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// transactionService implements domain.TransactionService. It also depends on
// an AccountRepository (still just a domain interface) so it can verify the
// referenced account is owned by the same caller before attaching a
// transaction to it — the account_id in the request body is otherwise an
// unchecked cross-tenant reference.
type transactionService struct {
	repo        domain.TransactionRepository
	accountRepo domain.AccountRepository
}

// NewTransactionService builds a TransactionService backed by repo, using
// accountRepo only to verify account ownership on create/update.
func NewTransactionService(repo domain.TransactionRepository, accountRepo domain.AccountRepository) domain.TransactionService {
	return &transactionService{repo: repo, accountRepo: accountRepo}
}

var _ domain.TransactionService = (*transactionService)(nil)

func (s *transactionService) validateCommon(accountID string, txType domain.TransactionType, amount float64, currency string, date time.Time) error {
	if strings.TrimSpace(accountID) == "" {
		return NewValidationError("account_id", "account_id is required")
	}
	if !domain.ValidTransactionType(txType) {
		return NewValidationError("type", "type must be one of: income, expense")
	}
	if amount <= 0 {
		return NewValidationError("amount", "amount must be greater than zero")
	}
	if strings.TrimSpace(currency) == "" {
		return NewValidationError("currency", "currency is required")
	}
	if date.IsZero() {
		return NewValidationError("date", "date is required")
	}
	return nil
}

func (s *transactionService) Create(ctx context.Context, userID string, in domain.CreateTransactionInput) (*domain.Transaction, error) {
	if err := s.validateCommon(in.AccountID, in.Type, in.Amount, in.Currency, in.Date); err != nil {
		return nil, err
	}
	if _, err := s.accountRepo.GetByIDForUser(ctx, in.AccountID, userID); err != nil {
		if err == domain.ErrNotFound {
			return nil, NewValidationError("account_id", "account does not exist")
		}
		return nil, err
	}

	now := time.Now().UTC()
	tx := &domain.Transaction{
		UserID:     userID,
		AccountID:  in.AccountID,
		CategoryID: in.CategoryID,
		Type:       in.Type,
		Amount:     in.Amount,
		Currency:   in.Currency,
		Date:       in.Date,
		Note:       in.Note,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *transactionService) List(ctx context.Context, userID string, in domain.ListTransactionsInput) (domain.TransactionPage, error) {
	page := in.Page
	if page < 1 {
		page = defaultPage
	}
	pageSize := in.PageSize
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	filter := domain.TransactionFilter{
		AccountID: in.AccountID,
		Category:  in.Category,
		From:      in.From,
		To:        in.To,
		Page:      page,
		PageSize:  pageSize,
	}
	return s.repo.ListByUser(ctx, userID, filter)
}

func (s *transactionService) Get(ctx context.Context, userID, id string) (*domain.Transaction, error) {
	return s.repo.GetByIDForUser(ctx, id, userID)
}

func (s *transactionService) Update(ctx context.Context, userID, id string, in domain.UpdateTransactionInput) (*domain.Transaction, error) {
	tx, err := s.repo.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if err := s.validateCommon(in.AccountID, in.Type, in.Amount, in.Currency, in.Date); err != nil {
		return nil, err
	}
	if in.AccountID != tx.AccountID {
		if _, err := s.accountRepo.GetByIDForUser(ctx, in.AccountID, userID); err != nil {
			if err == domain.ErrNotFound {
				return nil, NewValidationError("account_id", "account does not exist")
			}
			return nil, err
		}
	}

	tx.AccountID = in.AccountID
	tx.CategoryID = in.CategoryID
	tx.Type = in.Type
	tx.Amount = in.Amount
	tx.Currency = in.Currency
	tx.Date = in.Date
	tx.Note = in.Note
	tx.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *transactionService) Delete(ctx context.Context, userID, id string) error {
	return s.repo.DeleteByIDForUser(ctx, id, userID)
}
