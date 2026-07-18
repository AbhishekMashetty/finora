package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/finora/budget-service/internal/domain"
)

// fakeBudgetRepository is a hand-written fake of domain.BudgetRepository so
// the service layer can be tested without any real Mongo.
type fakeBudgetRepository struct {
	budgets map[string]domain.Budget // keyed by id
	nextID  int
}

func newFakeBudgetRepository() *fakeBudgetRepository {
	return &fakeBudgetRepository{budgets: make(map[string]domain.Budget)}
}

func (f *fakeBudgetRepository) Create(ctx context.Context, budget *domain.Budget) error {
	f.nextID++
	id := "budget-" + strconv.Itoa(f.nextID)
	budget.ID = id
	f.budgets[id] = *budget
	return nil
}

func (f *fakeBudgetRepository) ListByUser(ctx context.Context, userID string) ([]domain.Budget, error) {
	out := make([]domain.Budget, 0)
	for _, b := range f.budgets {
		if b.UserID == userID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBudgetRepository) GetByIDForUser(ctx context.Context, id, userID string) (*domain.Budget, error) {
	b, ok := f.budgets[id]
	if !ok || b.UserID != userID {
		return nil, domain.ErrNotFound
	}
	cp := b
	return &cp, nil
}

func (f *fakeBudgetRepository) Update(ctx context.Context, budget *domain.Budget) error {
	existing, ok := f.budgets[budget.ID]
	if !ok || existing.UserID != budget.UserID {
		return domain.ErrNotFound
	}
	f.budgets[budget.ID] = *budget
	return nil
}

func (f *fakeBudgetRepository) DeleteByIDForUser(ctx context.Context, id, userID string) error {
	existing, ok := f.budgets[id]
	if !ok || existing.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.budgets, id)
	return nil
}

func TestBudgetService_Create(t *testing.T) {
	tests := []struct {
		name    string
		in      domain.CreateBudgetInput
		wantErr error
	}{
		{
			name: "valid budget is created",
			in:   domain.CreateBudgetInput{Category: "groceries", Amount: 500, Period: domain.PeriodMonthly},
		},
		{
			name:    "empty category is rejected",
			in:      domain.CreateBudgetInput{Category: "", Amount: 500, Period: domain.PeriodMonthly},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "zero amount is rejected",
			in:      domain.CreateBudgetInput{Category: "groceries", Amount: 0, Period: domain.PeriodMonthly},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "negative amount is rejected",
			in:      domain.CreateBudgetInput{Category: "groceries", Amount: -10, Period: domain.PeriodMonthly},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "invalid period is rejected",
			in:      domain.CreateBudgetInput{Category: "groceries", Amount: 500, Period: "daily"},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "amount above the ceiling is rejected",
			in:      domain.CreateBudgetInput{Category: "groceries", Amount: 1e13, Period: domain.PeriodMonthly},
			wantErr: domain.ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeBudgetRepository()
			svc := NewBudgetService(repo)

			budget, err := svc.Create(context.Background(), "user-1", tt.in)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if budget.ID == "" {
				t.Error("expected budget to have an ID assigned")
			}
			if budget.UserID != "user-1" {
				t.Errorf("UserID = %q, want %q", budget.UserID, "user-1")
			}
			if budget.Category != tt.in.Category {
				t.Errorf("Category = %q, want %q", budget.Category, tt.in.Category)
			}
		})
	}
}

func TestBudgetService_List(t *testing.T) {
	repo := newFakeBudgetRepository()
	svc := NewBudgetService(repo)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "user-1", domain.CreateBudgetInput{Category: "rent", Amount: 1000, Period: domain.PeriodMonthly}); err != nil {
		t.Fatalf("setup create failed: %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", domain.CreateBudgetInput{Category: "rent", Amount: 900, Period: domain.PeriodMonthly}); err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	budgets, err := svc.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(budgets) != 1 {
		t.Fatalf("len(budgets) = %d, want 1", len(budgets))
	}
	if budgets[0].UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", budgets[0].UserID, "user-1")
	}
}

func TestBudgetService_Get_CrossUserIsNotFound(t *testing.T) {
	repo := newFakeBudgetRepository()
	svc := NewBudgetService(repo)
	ctx := context.Background()

	created, err := svc.Create(ctx, "owner", domain.CreateBudgetInput{Category: "fun", Amount: 100, Period: domain.PeriodWeekly})
	if err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	// The owner can fetch it.
	if _, err := svc.Get(ctx, "owner", created.ID); err != nil {
		t.Fatalf("owner Get failed: %v", err)
	}

	// A different user gets 404-equivalent ErrNotFound, never a 403-equivalent.
	_, err = svc.Get(ctx, "someone-else", created.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
	}
}

func TestBudgetService_Update(t *testing.T) {
	repo := newFakeBudgetRepository()
	svc := NewBudgetService(repo)
	ctx := context.Background()

	created, err := svc.Create(ctx, "owner", domain.CreateBudgetInput{Category: "fun", Amount: 100, Period: domain.PeriodWeekly})
	if err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	t.Run("owner can update", func(t *testing.T) {
		updated, err := svc.Update(ctx, "owner", created.ID, domain.UpdateBudgetInput{Category: "fun2", Amount: 200, Period: domain.PeriodMonthly})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Category != "fun2" || updated.Amount != 200 || updated.Period != domain.PeriodMonthly {
			t.Errorf("update did not apply: %+v", updated)
		}
	})

	t.Run("cross-user update is not found", func(t *testing.T) {
		_, err := svc.Update(ctx, "someone-else", created.ID, domain.UpdateBudgetInput{Category: "x", Amount: 1, Period: domain.PeriodWeekly})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
		}
	})

	t.Run("invalid input is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, "owner", created.ID, domain.UpdateBudgetInput{Category: "", Amount: 200, Period: domain.PeriodMonthly})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want %v", err, domain.ErrValidation)
		}
	})

	t.Run("amount above the ceiling is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, "owner", created.ID, domain.UpdateBudgetInput{Category: "fun", Amount: 1e13, Period: domain.PeriodMonthly})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want %v", err, domain.ErrValidation)
		}
	})
}

func TestBudgetService_Delete(t *testing.T) {
	repo := newFakeBudgetRepository()
	svc := NewBudgetService(repo)
	ctx := context.Background()

	created, err := svc.Create(ctx, "owner", domain.CreateBudgetInput{Category: "fun", Amount: 100, Period: domain.PeriodWeekly})
	if err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	t.Run("cross-user delete is not found", func(t *testing.T) {
		if err := svc.Delete(ctx, "someone-else", created.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
		}
	})

	t.Run("owner can delete", func(t *testing.T) {
		if err := svc.Delete(ctx, "owner", created.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := svc.Get(ctx, "owner", created.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected deleted budget to be gone, err = %v", err)
		}
	})
}
