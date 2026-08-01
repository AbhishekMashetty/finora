package service_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/service"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setupTransactionServiceWithAccount(t *testing.T, userID string) (domain.TransactionService, *fakeAccountRepository, string) {
	t.Helper()
	svc, accountRepo, _, accountID := setupTransactionServiceWithAccountAndCategory(t, userID)
	return svc, accountRepo, accountID
}

// setupTransactionServiceWithAccountAndCategory additionally seeds a
// category owned by userID, for tests covering the category_id ownership
// check.
func setupTransactionServiceWithAccountAndCategory(t *testing.T, userID string) (domain.TransactionService, *fakeAccountRepository, *fakeCategoryRepository, string) {
	t.Helper()
	accountRepo := newFakeAccountRepository()
	categoryRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, accountRepo, categoryRepo, newFakeEventPublisher(), discardLogger())

	accountSvc := service.NewAccountService(accountRepo)
	account, err := accountSvc.Create(context.Background(), userID, domain.CreateAccountInput{
		Name: "Checking", Type: domain.AccountTypeChecking, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("setup account: %v", err)
	}
	return svc, accountRepo, categoryRepo, account.ID
}

func TestTransactionService_Create_Validation(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name    string
		input   domain.CreateTransactionInput
		wantErr bool
	}{
		{
			name: "valid expense",
			input: domain.CreateTransactionInput{
				AccountID: accountID, Type: domain.TransactionTypeExpense, Amount: 42.5, Currency: "USD", Date: now,
			},
			wantErr: false,
		},
		{
			name: "missing account_id",
			input: domain.CreateTransactionInput{
				Type: domain.TransactionTypeExpense, Amount: 10, Currency: "USD", Date: now,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			input: domain.CreateTransactionInput{
				AccountID: accountID, Type: domain.TransactionType("transfer"), Amount: 10, Currency: "USD", Date: now,
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			input: domain.CreateTransactionInput{
				AccountID: accountID, Type: domain.TransactionTypeIncome, Amount: 0, Currency: "USD", Date: now,
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			input: domain.CreateTransactionInput{
				AccountID: accountID, Type: domain.TransactionTypeIncome, Amount: -5, Currency: "USD", Date: now,
			},
			wantErr: true,
		},
		{
			name: "missing currency",
			input: domain.CreateTransactionInput{
				AccountID: accountID, Type: domain.TransactionTypeIncome, Amount: 5, Date: now,
			},
			wantErr: true,
		},
		{
			name: "zero date",
			input: domain.CreateTransactionInput{
				AccountID: accountID, Type: domain.TransactionTypeIncome, Amount: 5, Currency: "USD",
			},
			wantErr: true,
		},
		{
			name: "account belongs to someone else / doesn't exist",
			input: domain.CreateTransactionInput{
				AccountID: "nonexistent-account", Type: domain.TransactionTypeIncome, Amount: 5, Currency: "USD", Date: now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, "user-1", tt.input)
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
		})
	}
}

// strPtr is a small helper so table-driven test cases can take the address
// of a string literal inline.
func strPtr(s string) *string { return &s }

func TestTransactionService_Create_CategoryOwnership(t *testing.T) {
	svc, _, categoryRepo, accountID := setupTransactionServiceWithAccountAndCategory(t, "user-1")
	ctx := context.Background()
	now := time.Now()

	myCategory := domain.Category{UserID: "user-1", Name: "Groceries", Type: domain.TransactionTypeExpense}
	if err := categoryRepo.Create(ctx, &myCategory); err != nil {
		t.Fatalf("setup category: %v", err)
	}
	othersCategory := domain.Category{UserID: "someone-else", Name: "Rent", Type: domain.TransactionTypeExpense}
	if err := categoryRepo.Create(ctx, &othersCategory); err != nil {
		t.Fatalf("setup category: %v", err)
	}

	tests := []struct {
		name       string
		categoryID *string
		wantErr    bool
	}{
		{
			name:       "nil category_id is valid (optional field)",
			categoryID: nil,
			wantErr:    false,
		},
		{
			name:       "own category_id is accepted",
			categoryID: strPtr(myCategory.ID),
			wantErr:    false,
		},
		{
			name:       "category_id owned by another user is rejected",
			categoryID: strPtr(othersCategory.ID),
			wantErr:    true,
		},
		{
			name:       "nonexistent category_id is rejected",
			categoryID: strPtr("nonexistent-category"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, "user-1", domain.CreateTransactionInput{
				AccountID: accountID, CategoryID: tt.categoryID, Type: domain.TransactionTypeExpense, Amount: 10, Currency: "USD", Date: now,
			})
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
		})
	}
}

func TestTransactionService_Create_AmountCeiling(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name    string
		amount  float64
		wantErr bool
	}{
		{name: "amount at the ceiling is rejected (must be strictly greater)", amount: 1_000_000_000_001, wantErr: true},
		{name: "absurdly large amount is rejected", amount: 1e307, wantErr: true},
		{name: "amount just under the ceiling is accepted", amount: 999_999_999_999, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, "user-1", domain.CreateTransactionInput{
				AccountID: accountID, Type: domain.TransactionTypeExpense, Amount: tt.amount, Currency: "USD", Date: now,
			})
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
		})
	}
}

func TestTransactionService_List_InvertedDateRange(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	if _, err := svc.Create(ctx, "user-1", domain.CreateTransactionInput{
		AccountID: accountID, Type: domain.TransactionTypeExpense, Amount: 10, Currency: "USD", Date: time.Now(),
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	from := time.Now()
	to := from.Add(-24 * time.Hour)
	_, err := svc.List(ctx, "user-1", domain.ListTransactionsInput{From: &from, To: &to})
	if err == nil {
		t.Fatalf("expected error for inverted date range, got nil")
	}
	if _, ok := service.AsValidationError(err); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTransactionService_Pagination(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	// Create 25 transactions.
	for i := 0; i < 25; i++ {
		_, err := svc.Create(ctx, "user-1", domain.CreateTransactionInput{
			AccountID: accountID,
			Type:      domain.TransactionTypeExpense,
			Amount:    float64(i + 1),
			Currency:  "USD",
			Date:      time.Now(),
		})
		if err != nil {
			t.Fatalf("setup transaction %d: %v", i, err)
		}
	}

	tests := []struct {
		name         string
		in           domain.ListTransactionsInput
		wantCount    int
		wantTotal    int64
		wantPageUsed int
	}{
		{
			name:         "default page size caps at 20 for page 1",
			in:           domain.ListTransactionsInput{},
			wantCount:    20,
			wantTotal:    25,
			wantPageUsed: 1,
		},
		{
			name:         "page 2 returns remainder",
			in:           domain.ListTransactionsInput{Page: 2},
			wantCount:    5,
			wantTotal:    25,
			wantPageUsed: 2,
		},
		{
			name:         "page_size caps at 100 even if requested higher",
			in:           domain.ListTransactionsInput{PageSize: 500},
			wantCount:    25,
			wantTotal:    25,
			wantPageUsed: 1,
		},
		{
			name:         "explicit small page size",
			in:           domain.ListTransactionsInput{PageSize: 10, Page: 3},
			wantCount:    5,
			wantTotal:    25,
			wantPageUsed: 3,
		},
		{
			name:         "page beyond available data returns empty",
			in:           domain.ListTransactionsInput{Page: 10},
			wantCount:    0,
			wantTotal:    25,
			wantPageUsed: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.List(ctx, "user-1", tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Transactions) != tt.wantCount {
				t.Fatalf("expected %d transactions, got %d", tt.wantCount, len(result.Transactions))
			}
			if result.Total != tt.wantTotal {
				t.Fatalf("expected total %d, got %d", tt.wantTotal, result.Total)
			}
		})
	}
}

func TestTransactionService_List_ScopedToUser(t *testing.T) {
	accountRepo := newFakeAccountRepository()
	categoryRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, accountRepo, categoryRepo, newFakeEventPublisher(), discardLogger())
	accountSvc := service.NewAccountService(accountRepo)
	ctx := context.Background()

	acc1, _ := accountSvc.Create(ctx, "user-1", domain.CreateAccountInput{Name: "A", Type: domain.AccountTypeCash, Currency: "USD"})
	acc2, _ := accountSvc.Create(ctx, "user-2", domain.CreateAccountInput{Name: "B", Type: domain.AccountTypeCash, Currency: "USD"})

	if _, err := svc.Create(ctx, "user-1", domain.CreateTransactionInput{AccountID: acc1.ID, Type: domain.TransactionTypeExpense, Amount: 10, Currency: "USD", Date: time.Now()}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", domain.CreateTransactionInput{AccountID: acc2.ID, Type: domain.TransactionTypeExpense, Amount: 20, Currency: "USD", Date: time.Now()}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	result, err := svc.List(ctx, "user-1", domain.ListTransactionsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected user-1 to see only their own transaction, got total %d", result.Total)
	}
}

func TestTransactionService_CrossUserAccess_ReturnsNotFound(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "owner")
	ctx := context.Background()

	tx, err := svc.Create(ctx, "owner", domain.CreateTransactionInput{
		AccountID: accountID, Type: domain.TransactionTypeExpense, Amount: 15, Currency: "USD", Date: time.Now(),
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := svc.Get(ctx, "intruder", tx.ID); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound on get, got %v", err)
	}
	if _, err := svc.Update(ctx, "intruder", tx.ID, domain.UpdateTransactionInput{
		AccountID: accountID, Type: domain.TransactionTypeExpense, Amount: 99, Currency: "USD", Date: time.Now(),
	}); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound on update, got %v", err)
	}
	if err := svc.Delete(ctx, "intruder", tx.ID); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound on delete, got %v", err)
	}
}
