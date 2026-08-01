package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/service"
)

// setupTransactionServiceWithPublisher mirrors
// setupTransactionServiceWithAccount but exposes the fakeEventPublisher
// directly, for tests asserting on what Create/Import publish (Phase 7) —
// kept as its own small helper rather than changing the widely-used shared
// helpers' return signature, which 15+ existing call sites depend on.
func setupTransactionServiceWithPublisher(t *testing.T, userID string) (domain.TransactionService, *fakeEventPublisher, string) {
	t.Helper()
	accountRepo := newFakeAccountRepository()
	categoryRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	pub := newFakeEventPublisher()
	svc := service.NewTransactionService(txRepo, accountRepo, categoryRepo, pub, discardLogger())

	accountSvc := service.NewAccountService(accountRepo)
	account, err := accountSvc.Create(context.Background(), userID, domain.CreateAccountInput{
		Name: "Checking", Type: domain.AccountTypeChecking, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("setup account: %v", err)
	}
	return svc, pub, account.ID
}

func TestTransactionService_Create_PublishesTransactionCreatedEvent(t *testing.T) {
	svc, pub, accountID := setupTransactionServiceWithPublisher(t, "user-1")
	ctx := context.Background()

	tx, err := svc.Create(ctx, "user-1", domain.CreateTransactionInput{
		AccountID: accountID,
		Type:      domain.TransactionTypeExpense,
		Amount:    42.50,
		Currency:  "USD",
		Date:      time.Now(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(pub.published) != 1 {
		t.Fatalf("expected exactly 1 published event, got %d", len(pub.published))
	}
	published := pub.published[0]
	if published.ID != tx.ID {
		t.Errorf("expected the published event to reference the created transaction's ID %q, got %q", tx.ID, published.ID)
	}
	if published.Amount != 42.50 || published.Type != domain.TransactionTypeExpense {
		t.Errorf("published event doesn't match the created transaction: %+v", published)
	}
}

func TestTransactionService_Create_SucceedsEvenIfEventPublishFails(t *testing.T) {
	accountRepo := newFakeAccountRepository()
	categoryRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	pub := &fakeEventPublisher{err: errors.New("outbox enqueue failed")}
	svc := service.NewTransactionService(txRepo, accountRepo, categoryRepo, pub, discardLogger())

	accountSvc := service.NewAccountService(accountRepo)
	account, err := accountSvc.Create(context.Background(), "user-1", domain.CreateAccountInput{
		Name: "Checking", Type: domain.AccountTypeChecking, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("setup account: %v", err)
	}

	// The whole point of publishCreated logging (not returning) a publish
	// failure: Create must still report success, since the transaction
	// itself was genuinely persisted — a caller that saw an error here and
	// retried would create a duplicate transaction.
	tx, err := svc.Create(context.Background(), "user-1", domain.CreateTransactionInput{
		AccountID: account.ID,
		Type:      domain.TransactionTypeExpense,
		Amount:    10,
		Currency:  "USD",
		Date:      time.Now(),
	})
	if err != nil {
		t.Fatalf("expected Create to succeed despite the event publish failing, got error: %v", err)
	}
	if tx == nil || tx.ID == "" {
		t.Fatal("expected a real created transaction back")
	}
}

func TestTransactionService_Import_PublishesOneEventPerImportedRow(t *testing.T) {
	svc, pub, accountID := setupTransactionServiceWithPublisher(t, "user-1")
	ctx := context.Background()

	rows := []domain.ImportRow{
		{Date: "2026-01-05", Description: "Good row 1", Amount: "10.00"},
		{Date: "not-a-date", Description: "Bad row", Amount: "10.00"}, // should be skipped, not published
		{Date: "2026-01-06", Description: "Good row 2", Amount: "20.00"},
	}

	result, err := svc.Import(ctx, "user-1", accountID, rows)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("expected 2 imported, got %d", result.Imported)
	}
	if len(pub.published) != 2 {
		t.Fatalf("expected exactly 2 published events (one per successfully imported row, none for the skipped row), got %d", len(pub.published))
	}
}
