package service

import (
	"context"
	"strings"
	"time"

	"github.com/finora/expense-service/internal/domain"
)

// categoryService implements domain.CategoryService against a
// domain.CategoryRepository.
type categoryService struct {
	repo domain.CategoryRepository
}

// NewCategoryService builds a CategoryService backed by repo.
func NewCategoryService(repo domain.CategoryRepository) domain.CategoryService {
	return &categoryService{repo: repo}
}

var _ domain.CategoryService = (*categoryService)(nil)

func (s *categoryService) Create(ctx context.Context, userID string, in domain.CreateCategoryInput) (*domain.Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, NewValidationError("name", "name is required")
	}
	if !domain.ValidTransactionType(in.Type) {
		return nil, NewValidationError("type", "type must be one of: income, expense")
	}

	now := time.Now().UTC()
	category := &domain.Category{
		UserID:    userID,
		Name:      name,
		Type:      in.Type,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *categoryService) List(ctx context.Context, userID string) ([]domain.Category, error) {
	return s.repo.ListByUser(ctx, userID)
}
