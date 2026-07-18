package service

import (
	"context"
	"time"

	"github.com/finora/budget-service/internal/domain"
)

// goalService implements domain.GoalService.
type goalService struct {
	repo domain.GoalRepository
}

// NewGoalService builds a GoalService backed by repo.
func NewGoalService(repo domain.GoalRepository) domain.GoalService {
	return &goalService{repo: repo}
}

func (s *goalService) Create(ctx context.Context, userID string, in domain.CreateGoalInput) (*domain.Goal, error) {
	if in.Name == "" {
		return nil, wrapValidation("name is required")
	}
	if in.TargetAmount <= 0 {
		return nil, wrapValidation("target_amount must be greater than zero")
	}
	if in.TargetDate.IsZero() {
		return nil, wrapValidation("target_date is required")
	}
	if in.TargetDate.Before(time.Now().UTC()) {
		return nil, wrapValidation("target_date must be in the future")
	}

	goal := &domain.Goal{
		UserID:       userID,
		Name:         in.Name,
		TargetAmount: in.TargetAmount,
		TargetDate:   in.TargetDate,
	}
	if err := s.repo.Create(ctx, goal); err != nil {
		return nil, err
	}
	return goal, nil
}

func (s *goalService) List(ctx context.Context, userID string) ([]domain.Goal, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *goalService) Get(ctx context.Context, userID, id string) (*domain.Goal, error) {
	return s.repo.GetByIDForUser(ctx, id, userID)
}

func (s *goalService) Update(ctx context.Context, userID, id string, in domain.UpdateGoalInput) (*domain.Goal, error) {
	if in.Name == "" {
		return nil, wrapValidation("name is required")
	}
	if in.TargetAmount <= 0 {
		return nil, wrapValidation("target_amount must be greater than zero")
	}
	if in.TargetDate.IsZero() {
		return nil, wrapValidation("target_date is required")
	}
	if in.CurrentAmount < 0 {
		return nil, wrapValidation("current_amount must not be negative")
	}

	existing, err := s.repo.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	// Only reject a past target_date when the caller is actually changing it.
	// PUT is a full-replace, and it's also how progress-logging (CurrentAmount)
	// happens — re-sending an existing goal's now-past target_date (because
	// the caller only meant to log progress) must not permanently lock the
	// goal out of every future update once its due date passes.
	if !in.TargetDate.Equal(existing.TargetDate) && in.TargetDate.Before(time.Now().UTC()) {
		return nil, wrapValidation("target_date must be in the future")
	}

	existing.Name = in.Name
	existing.TargetAmount = in.TargetAmount
	existing.TargetDate = in.TargetDate
	existing.CurrentAmount = in.CurrentAmount

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *goalService) Delete(ctx context.Context, userID, id string) error {
	return s.repo.DeleteByIDForUser(ctx, id, userID)
}
