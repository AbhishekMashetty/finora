package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/finora/expense-service/internal/domain"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100

	// maxAmount is a defense-in-depth ceiling against float64 garbage/overflow
	// in downstream aggregations (e.g. budget-service's report summation) —
	// it is not meant to model any real-world spending limit.
	maxAmount = 1_000_000_000_000
)

// transactionService implements domain.TransactionService. It also depends on
// an AccountRepository and a CategoryRepository (still just domain
// interfaces) so it can verify the referenced account and category are owned
// by the same caller before attaching a transaction to them — account_id and
// category_id in the request body are otherwise unchecked cross-tenant
// references. eventPublisher (Phase 7) publishes a transaction.created event
// after every successful create, for budget-service's overspend detection —
// see publishCreated's doc comment for why a publish failure never fails the
// create itself.
type transactionService struct {
	repo           domain.TransactionRepository
	accountRepo    domain.AccountRepository
	categoryRepo   domain.CategoryRepository
	eventPublisher domain.EventPublisher
	log            *slog.Logger
}

// NewTransactionService builds a TransactionService backed by repo, using
// accountRepo to verify account ownership, categoryRepo to verify category
// ownership on create/update, and eventPublisher to publish a
// transaction.created event after every successful create (Phase 7). log
// records only a failed event publish — never anything that changes
// Create/Import's own result.
func NewTransactionService(repo domain.TransactionRepository, accountRepo domain.AccountRepository, categoryRepo domain.CategoryRepository, eventPublisher domain.EventPublisher, log *slog.Logger) domain.TransactionService {
	return &transactionService{repo: repo, accountRepo: accountRepo, categoryRepo: categoryRepo, eventPublisher: eventPublisher, log: log}
}

// publishCreated enqueues a transaction.created event for tx, logging (not
// returning) any failure. Deliberate: by the time this is called, tx is
// already durably persisted in MongoDB — the create itself already
// succeeded. Failing the whole Create/Import call at this point would be
// actively misleading: a caller that sees an error and retries would
// create a SECOND, duplicate transaction, since the first one already
// exists. A lost event is a reliability gap (see shared/outbox's own doc
// comment on the narrow, accepted crash-window race this shares the same
// character as), not a reason to make a successful write look like a
// failure.
func (s *transactionService) publishCreated(ctx context.Context, tx *domain.Transaction) {
	if err := s.eventPublisher.PublishTransactionCreated(ctx, tx); err != nil {
		s.log.Error("failed to enqueue transaction.created event",
			slog.String("transaction_id", tx.ID),
			slog.String("user_id", tx.UserID),
			slog.String("error", err.Error()),
		)
	}
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
	if amount > maxAmount {
		return NewValidationError("amount", "amount exceeds the maximum allowed value")
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
	if in.CategoryID != nil && strings.TrimSpace(*in.CategoryID) != "" {
		if _, err := s.categoryRepo.GetByIDForUser(ctx, *in.CategoryID, userID); err != nil {
			if err == domain.ErrNotFound {
				return nil, NewValidationError("category_id", "category does not exist")
			}
			return nil, err
		}
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
	s.publishCreated(ctx, tx)
	return tx, nil
}

func (s *transactionService) List(ctx context.Context, userID string, in domain.ListTransactionsInput) (domain.TransactionPage, error) {
	if in.From != nil && in.To != nil && in.To.Before(*in.From) {
		return domain.TransactionPage{}, NewValidationError("to", "to must not be before from")
	}

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
	result, err := s.repo.ListByUser(ctx, userID, filter)
	if err != nil {
		return domain.TransactionPage{}, err
	}
	// The repository doesn't know about resolved-vs-requested pagination —
	// stamp the values the service actually applied so the handler echoes
	// back reality, not a possibly-zero raw query param (a real bug this
	// fixes: previously the handler echoed in.Page directly, which was 0
	// whenever the caller omitted ?page=, and page_size wasn't returned at
	// all).
	result.Page = page
	result.PageSize = pageSize
	return result, nil
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
	if in.CategoryID != nil && strings.TrimSpace(*in.CategoryID) != "" {
		if _, err := s.categoryRepo.GetByIDForUser(ctx, *in.CategoryID, userID); err != nil {
			if err == domain.ErrNotFound {
				return nil, NewValidationError("category_id", "category does not exist")
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
