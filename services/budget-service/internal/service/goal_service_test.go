package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/finora/budget-service/internal/domain"
)

// fakeGoalRepository is a hand-written fake of domain.GoalRepository so the
// service layer can be tested without any real Mongo.
type fakeGoalRepository struct {
	goals  map[string]domain.Goal
	nextID int
}

func newFakeGoalRepository() *fakeGoalRepository {
	return &fakeGoalRepository{goals: make(map[string]domain.Goal)}
}

func (f *fakeGoalRepository) Create(ctx context.Context, goal *domain.Goal) error {
	f.nextID++
	id := "goal-" + strconv.Itoa(f.nextID)
	goal.ID = id
	f.goals[id] = *goal
	return nil
}

func (f *fakeGoalRepository) ListByUser(ctx context.Context, userID string) ([]domain.Goal, error) {
	out := make([]domain.Goal, 0)
	for _, g := range f.goals {
		if g.UserID == userID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (f *fakeGoalRepository) GetByIDForUser(ctx context.Context, id, userID string) (*domain.Goal, error) {
	g, ok := f.goals[id]
	if !ok || g.UserID != userID {
		return nil, domain.ErrNotFound
	}
	cp := g
	return &cp, nil
}

func (f *fakeGoalRepository) Update(ctx context.Context, goal *domain.Goal) error {
	existing, ok := f.goals[goal.ID]
	if !ok || existing.UserID != goal.UserID {
		return domain.ErrNotFound
	}
	f.goals[goal.ID] = *goal
	return nil
}

func (f *fakeGoalRepository) DeleteByIDForUser(ctx context.Context, id, userID string) error {
	existing, ok := f.goals[id]
	if !ok || existing.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.goals, id)
	return nil
}

func TestGoalService_Create(t *testing.T) {
	future := time.Now().UTC().Add(30 * 24 * time.Hour)
	past := time.Now().UTC().Add(-24 * time.Hour)

	tests := []struct {
		name    string
		in      domain.CreateGoalInput
		wantErr error
	}{
		{
			name: "valid goal is created",
			in:   domain.CreateGoalInput{Name: "Emergency fund", TargetAmount: 5000, TargetDate: future},
		},
		{
			name:    "empty name is rejected",
			in:      domain.CreateGoalInput{Name: "", TargetAmount: 5000, TargetDate: future},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "zero target amount is rejected",
			in:      domain.CreateGoalInput{Name: "Emergency fund", TargetAmount: 0, TargetDate: future},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "past target date is rejected",
			in:      domain.CreateGoalInput{Name: "Emergency fund", TargetAmount: 5000, TargetDate: past},
			wantErr: domain.ErrValidation,
		},
		{
			name:    "target amount above the ceiling is rejected",
			in:      domain.CreateGoalInput{Name: "Emergency fund", TargetAmount: 1e13, TargetDate: future},
			wantErr: domain.ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeGoalRepository()
			svc := NewGoalService(repo)

			goal, err := svc.Create(context.Background(), "user-1", tt.in)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if goal.ID == "" {
				t.Error("expected goal to have an ID assigned")
			}
			if goal.CurrentAmount != 0 {
				t.Errorf("CurrentAmount = %v, want 0 (Phase-3 stub, no progress tracking yet)", goal.CurrentAmount)
			}
		})
	}
}

func TestGoalService_List(t *testing.T) {
	repo := newFakeGoalRepository()
	svc := NewGoalService(repo)
	ctx := context.Background()
	future := time.Now().UTC().Add(30 * 24 * time.Hour)

	if _, err := svc.Create(ctx, "user-1", domain.CreateGoalInput{Name: "Trip", TargetAmount: 2000, TargetDate: future}); err != nil {
		t.Fatalf("setup create failed: %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", domain.CreateGoalInput{Name: "Trip", TargetAmount: 2000, TargetDate: future}); err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	goals, err := svc.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(goals) != 1 {
		t.Fatalf("len(goals) = %d, want 1", len(goals))
	}
	if goals[0].UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", goals[0].UserID, "user-1")
	}
}

func TestGoalService_Get_CrossUserIsNotFound(t *testing.T) {
	repo := newFakeGoalRepository()
	svc := NewGoalService(repo)
	ctx := context.Background()
	future := time.Now().UTC().Add(30 * 24 * time.Hour)

	created, err := svc.Create(ctx, "owner", domain.CreateGoalInput{Name: "Trip", TargetAmount: 2000, TargetDate: future})
	if err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	if _, err := svc.Get(ctx, "owner", created.ID); err != nil {
		t.Fatalf("owner Get failed: %v", err)
	}

	_, err = svc.Get(ctx, "someone-else", created.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
	}
}

func TestGoalService_Update(t *testing.T) {
	repo := newFakeGoalRepository()
	svc := NewGoalService(repo)
	ctx := context.Background()
	future := time.Now().UTC().Add(30 * 24 * time.Hour)
	past := time.Now().UTC().Add(-24 * time.Hour)

	created, err := svc.Create(ctx, "owner", domain.CreateGoalInput{Name: "Trip", TargetAmount: 2000, TargetDate: future})
	if err != nil {
		t.Fatalf("setup create failed: %v", err)
	}

	t.Run("owner can update including progress", func(t *testing.T) {
		updated, err := svc.Update(ctx, "owner", created.ID, domain.UpdateGoalInput{
			Name: "Trip2", TargetAmount: 2500, TargetDate: future, CurrentAmount: 500,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Name != "Trip2" || updated.TargetAmount != 2500 || updated.CurrentAmount != 500 {
			t.Errorf("update did not apply: %+v", updated)
		}
	})

	t.Run("current_amount of exactly zero is allowed (progress reset)", func(t *testing.T) {
		updated, err := svc.Update(ctx, "owner", created.ID, domain.UpdateGoalInput{
			Name: "Trip2", TargetAmount: 2500, TargetDate: future, CurrentAmount: 0,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.CurrentAmount != 0 {
			t.Errorf("CurrentAmount = %v, want 0", updated.CurrentAmount)
		}
	})

	t.Run("cross-user update is not found", func(t *testing.T) {
		_, err := svc.Update(ctx, "someone-else", created.ID, domain.UpdateGoalInput{Name: "x", TargetAmount: 1, TargetDate: future})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, domain.ErrNotFound)
		}
	})

	t.Run("empty name is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, "owner", created.ID, domain.UpdateGoalInput{Name: "", TargetAmount: 2500, TargetDate: future})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want %v", err, domain.ErrValidation)
		}
	})

	t.Run("past target date is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, "owner", created.ID, domain.UpdateGoalInput{Name: "Trip2", TargetAmount: 2500, TargetDate: past})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want %v", err, domain.ErrValidation)
		}
	})

	t.Run("negative current_amount is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, "owner", created.ID, domain.UpdateGoalInput{Name: "Trip2", TargetAmount: 2500, TargetDate: future, CurrentAmount: -1})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want %v", err, domain.ErrValidation)
		}
	})

	t.Run("target amount above the ceiling is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, "owner", created.ID, domain.UpdateGoalInput{Name: "Trip2", TargetAmount: 1e13, TargetDate: future})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want %v", err, domain.ErrValidation)
		}
	})

	t.Run("current_amount above the ceiling is rejected", func(t *testing.T) {
		_, err := svc.Update(ctx, "owner", created.ID, domain.UpdateGoalInput{Name: "Trip2", TargetAmount: 2500, TargetDate: future, CurrentAmount: 1e13})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("err = %v, want %v", err, domain.ErrValidation)
		}
	})

	t.Run("re-sending an already-past target_date unchanged is allowed", func(t *testing.T) {
		// A goal whose due date has since passed (seeded directly via the repo,
		// since Create rejects past dates) must still be updatable — PUT is a
		// full-replace, and it's also how progress-logging (CurrentAmount)
		// works, so resending the goal's own unchanged past date must not
		// permanently lock it out of every future update once it's overdue.
		overdue := domain.Goal{UserID: "owner", Name: "Overdue trip", TargetAmount: 1000, TargetDate: past, CurrentAmount: 200}
		if err := repo.Create(ctx, &overdue); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		updated, err := svc.Update(ctx, "owner", overdue.ID, domain.UpdateGoalInput{
			Name: overdue.Name, TargetAmount: overdue.TargetAmount, TargetDate: past, CurrentAmount: 1000,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.CurrentAmount != 1000 {
			t.Errorf("CurrentAmount = %v, want 1000", updated.CurrentAmount)
		}
	})
}

func TestGoalService_Delete(t *testing.T) {
	repo := newFakeGoalRepository()
	svc := NewGoalService(repo)
	ctx := context.Background()
	future := time.Now().UTC().Add(30 * 24 * time.Hour)

	created, err := svc.Create(ctx, "owner", domain.CreateGoalInput{Name: "Trip", TargetAmount: 2000, TargetDate: future})
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
			t.Fatalf("expected deleted goal to be gone, err = %v", err)
		}
	})
}
