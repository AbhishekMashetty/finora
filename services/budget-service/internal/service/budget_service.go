// Package service implements domain business logic. It depends only on
// domain interfaces (never on mongo-driver or gin directly), so it can be
// unit-tested with hand-written fakes standing in for the repositories.
package service

import (
	"context"

	"github.com/finora/budget-service/internal/domain"
)

// budgetService implements domain.BudgetService.
type budgetService struct {
	repo domain.BudgetRepository
}

// NewBudgetService builds a BudgetService backed by repo.
func NewBudgetService(repo domain.BudgetRepository) domain.BudgetService {
	return &budgetService{repo: repo}
}

func (s *budgetService) Create(ctx context.Context, userID string, in domain.CreateBudgetInput) (*domain.Budget, error) {
	if in.Category == "" {
		return nil, wrapValidation("category is required")
	}
	if in.Amount <= 0 {
		return nil, wrapValidation("amount must be greater than zero")
	}
	if !domain.ValidPeriod(in.Period) {
		return nil, wrapValidation("period must be one of: weekly, monthly, yearly")
	}

	budget := &domain.Budget{
		UserID:   userID,
		Category: in.Category,
		Amount:   in.Amount,
		Period:   in.Period,
	}
	if err := s.repo.Create(ctx, budget); err != nil {
		return nil, err
	}
	return budget, nil
}

func (s *budgetService) List(ctx context.Context, userID string) ([]domain.Budget, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *budgetService) Get(ctx context.Context, userID, id string) (*domain.Budget, error) {
	return s.repo.GetByIDForUser(ctx, id, userID)
}

func (s *budgetService) Update(ctx context.Context, userID, id string, in domain.UpdateBudgetInput) (*domain.Budget, error) {
	if in.Category == "" {
		return nil, wrapValidation("category is required")
	}
	if in.Amount <= 0 {
		return nil, wrapValidation("amount must be greater than zero")
	}
	if !domain.ValidPeriod(in.Period) {
		return nil, wrapValidation("period must be one of: weekly, monthly, yearly")
	}

	existing, err := s.repo.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	existing.Category = in.Category
	existing.Amount = in.Amount
	existing.Period = in.Period

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *budgetService) Delete(ctx context.Context, userID, id string) error {
	return s.repo.DeleteByIDForUser(ctx, id, userID)
}

// validationError wraps domain.ErrValidation with a human-readable message
// so errors.Is(err, domain.ErrValidation) still works while the handler can
// surface err.Error() as the message.
type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }
func (e *validationError) Unwrap() error { return domain.ErrValidation }

func wrapValidation(msg string) error {
	return &validationError{msg: msg}
}
