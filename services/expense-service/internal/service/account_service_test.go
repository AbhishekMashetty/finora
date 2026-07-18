package service_test

import (
	"context"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/service"
)

func TestAccountService_Create(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		input   domain.CreateAccountInput
		wantErr bool
	}{
		{
			name:   "valid checking account",
			userID: "user-1",
			input: domain.CreateAccountInput{
				Name:     "Primary Checking",
				Type:     domain.AccountTypeChecking,
				Currency: "USD",
			},
			wantErr: false,
		},
		{
			name:   "missing name",
			userID: "user-1",
			input: domain.CreateAccountInput{
				Name:     "",
				Type:     domain.AccountTypeChecking,
				Currency: "USD",
			},
			wantErr: true,
		},
		{
			name:   "invalid type",
			userID: "user-1",
			input: domain.CreateAccountInput{
				Name:     "Weird",
				Type:     domain.AccountType("crypto"),
				Currency: "USD",
			},
			wantErr: true,
		},
		{
			name:   "missing currency",
			userID: "user-1",
			input: domain.CreateAccountInput{
				Name: "No Currency",
				Type: domain.AccountTypeCash,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeAccountRepository()
			svc := service.NewAccountService(repo)

			account, err := svc.Create(context.Background(), tt.userID, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if _, ok := service.AsValidationError(err); !ok {
					t.Fatalf("expected ValidationError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if account.ID == "" {
				t.Fatalf("expected account to have an ID")
			}
			if account.UserID != tt.userID {
				t.Fatalf("expected UserID %q, got %q", tt.userID, account.UserID)
			}
			if account.Balance != 0 {
				t.Fatalf("expected new account balance to default to 0, got %v", account.Balance)
			}
		})
	}
}

func TestAccountService_List_ScopedToUser(t *testing.T) {
	repo := newFakeAccountRepository()
	svc := service.NewAccountService(repo)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "user-1", domain.CreateAccountInput{Name: "A1", Type: domain.AccountTypeCash, Currency: "USD"}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := svc.Create(ctx, "user-1", domain.CreateAccountInput{Name: "A2", Type: domain.AccountTypeSavings, Currency: "USD"}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", domain.CreateAccountInput{Name: "B1", Type: domain.AccountTypeChecking, Currency: "EUR"}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	accounts, err := svc.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts for user-1, got %d", len(accounts))
	}
	for _, a := range accounts {
		if a.UserID != "user-1" {
			t.Fatalf("leaked account belonging to %q into user-1's list", a.UserID)
		}
	}
}

func TestAccountService_CrossUserAccess_ReturnsNotFound(t *testing.T) {
	repo := newFakeAccountRepository()
	svc := service.NewAccountService(repo)
	ctx := context.Background()

	owned, err := svc.Create(ctx, "owner", domain.CreateAccountInput{Name: "Mine", Type: domain.AccountTypeCash, Currency: "USD"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("get", func(t *testing.T) {
		_, err := svc.Get(ctx, "intruder", owned.ID)
		if err != domain.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		_, err := svc.Update(ctx, "intruder", owned.ID, domain.UpdateAccountInput{Name: "Hijacked", Type: domain.AccountTypeCash, Currency: "USD"})
		if err != domain.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := svc.Delete(ctx, "intruder", owned.ID)
		if err != domain.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	// Sanity: the owner can still access it after the failed intruder attempts.
	if _, err := svc.Get(ctx, "owner", owned.ID); err != nil {
		t.Fatalf("owner should still be able to access their own account: %v", err)
	}
}

func TestAccountService_Update(t *testing.T) {
	repo := newFakeAccountRepository()
	svc := service.NewAccountService(repo)
	ctx := context.Background()

	account, err := svc.Create(ctx, "user-1", domain.CreateAccountInput{Name: "Old Name", Type: domain.AccountTypeCash, Currency: "USD"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	newBalance := 250.50
	updated, err := svc.Update(ctx, "user-1", account.ID, domain.UpdateAccountInput{
		Name:     "New Name",
		Type:     domain.AccountTypeSavings,
		Currency: "EUR",
		Balance:  &newBalance,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" || updated.Type != domain.AccountTypeSavings || updated.Currency != "EUR" || updated.Balance != newBalance {
		t.Fatalf("update did not apply expected fields: %+v", updated)
	}
}

func TestAccountService_Delete_NonexistentAccount(t *testing.T) {
	repo := newFakeAccountRepository()
	svc := service.NewAccountService(repo)

	err := svc.Delete(context.Background(), "user-1", "does-not-exist")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
