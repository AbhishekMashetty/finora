package service

import (
	"context"
	"strings"
	"time"

	"github.com/finora/expense-service/internal/domain"
)

// accountService implements domain.AccountService against a
// domain.AccountRepository, so the constructor's concrete backing store
// (Mongo in production, a fake in tests) is invisible to this layer.
type accountService struct {
	repo domain.AccountRepository
}

// NewAccountService builds an AccountService backed by repo.
func NewAccountService(repo domain.AccountRepository) domain.AccountService {
	return &accountService{repo: repo}
}

var _ domain.AccountService = (*accountService)(nil)

func (s *accountService) Create(ctx context.Context, userID string, in domain.CreateAccountInput) (*domain.Account, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, NewValidationError("name", "name is required")
	}
	if !domain.ValidAccountType(in.Type) {
		return nil, NewValidationError("type", "type must be one of: checking, savings, credit, cash")
	}
	currency := strings.TrimSpace(in.Currency)
	if currency == "" {
		return nil, NewValidationError("currency", "currency is required")
	}

	now := time.Now().UTC()
	account := &domain.Account{
		UserID:    userID,
		Name:      name,
		Type:      in.Type,
		Currency:  currency,
		Balance:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *accountService) List(ctx context.Context, userID string) ([]domain.Account, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *accountService) Get(ctx context.Context, userID, id string) (*domain.Account, error) {
	return s.repo.GetByIDForUser(ctx, id, userID)
}

func (s *accountService) Update(ctx context.Context, userID, id string, in domain.UpdateAccountInput) (*domain.Account, error) {
	account, err := s.repo.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, NewValidationError("name", "name is required")
	}
	if !domain.ValidAccountType(in.Type) {
		return nil, NewValidationError("type", "type must be one of: checking, savings, credit, cash")
	}
	currency := strings.TrimSpace(in.Currency)
	if currency == "" {
		return nil, NewValidationError("currency", "currency is required")
	}

	account.Name = name
	account.Type = in.Type
	account.Currency = currency
	if in.Balance != nil {
		account.Balance = *in.Balance
	}
	account.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *accountService) Delete(ctx context.Context, userID, id string) error {
	return s.repo.DeleteByIDForUser(ctx, id, userID)
}
