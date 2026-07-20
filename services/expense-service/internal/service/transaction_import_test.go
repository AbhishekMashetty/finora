package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/service"
)

func TestTransactionService_Import_HappyPath(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	rows := []domain.ImportRow{
		{Date: "2026-01-05", Description: "Grocery Store", Amount: "-45.20"},
		{Date: "01/06/2026", Description: "Paycheck", Amount: "1200.00"},
		{Date: "2026-01-07T00:00:00Z", Description: "Coffee Shop", Amount: "$12.50"},
	}

	result, err := svc.Import(ctx, "user-1", accountID, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 3 {
		t.Errorf("expected 3 imported, got %d (errors: %+v)", result.Imported, result.Errors)
	}
	if result.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.Skipped)
	}

	page, err := svc.List(ctx, "user-1", domain.ListTransactionsInput{AccountID: accountID, PageSize: 10})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(page.Transactions) != 3 {
		t.Fatalf("expected 3 persisted transactions, got %d", len(page.Transactions))
	}

	byNote := make(map[string]domain.Transaction)
	for _, tx := range page.Transactions {
		byNote[tx.Note] = tx
	}

	grocery, ok := byNote["Grocery Store"]
	if !ok {
		t.Fatal("grocery transaction not found")
	}
	if grocery.Type != domain.TransactionTypeExpense {
		t.Errorf("expected negative amount to infer expense, got %s", grocery.Type)
	}
	if grocery.Amount != 45.20 {
		t.Errorf("expected amount 45.20 (absolute value), got %v", grocery.Amount)
	}
	if grocery.Currency != "USD" {
		t.Errorf("expected imported transaction to inherit account currency USD, got %s", grocery.Currency)
	}

	paycheck, ok := byNote["Paycheck"]
	if !ok {
		t.Fatal("paycheck transaction not found")
	}
	if paycheck.Type != domain.TransactionTypeIncome {
		t.Errorf("expected positive amount to infer income, got %s", paycheck.Type)
	}

	coffee, ok := byNote["Coffee Shop"]
	if !ok {
		t.Fatal("coffee transaction not found")
	}
	if coffee.Amount != 12.50 {
		t.Errorf("expected $ prefix stripped, amount 12.50, got %v", coffee.Amount)
	}
}

// TestTransactionService_Import_DebitCreditColumns_RealCapitalOneShape is a
// direct regression test for a real bug a user hit: a genuine Capital One
// statement export has separate Debit/Credit columns holding UNSIGNED
// magnitudes (Debit for purchases, Credit for payments) — not a single
// signed Amount column. Debit was originally treated as just another alias
// for Amount, so every purchase (a positive Debit value, no Amount column
// present at all) was sign-inferred as income. This test uses rows shaped
// exactly like that real file: a Debit-only purchase and a Credit-only
// payment, with Amount always empty.
func TestTransactionService_Import_DebitCreditColumns_RealCapitalOneShape(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	rows := []domain.ImportRow{
		{Date: "2026-06-23", Description: "SAFEWAY #2776", Debit: "29.68"},
		{Date: "2026-06-01", Description: "CAPITAL ONE MOBILE PYMT", Credit: "1377.33"},
	}

	result, err := svc.Import(ctx, "user-1", accountID, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 0 {
		t.Fatalf("expected both rows imported, got imported=%d skipped=%d errors=%+v", result.Imported, result.Skipped, result.Errors)
	}

	page, err := svc.List(ctx, "user-1", domain.ListTransactionsInput{AccountID: accountID, PageSize: 10})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	byNote := make(map[string]domain.Transaction)
	for _, tx := range page.Transactions {
		byNote[tx.Note] = tx
	}

	purchase, ok := byNote["SAFEWAY #2776"]
	if !ok {
		t.Fatal("purchase transaction not found")
	}
	if purchase.Type != domain.TransactionTypeExpense {
		t.Errorf("a Debit-only row must be an expense, got %s (this is the exact bug: Debit used to be sign-inferred as income)", purchase.Type)
	}
	if purchase.Amount != 29.68 {
		t.Errorf("expected amount 29.68, got %v", purchase.Amount)
	}

	payment, ok := byNote["CAPITAL ONE MOBILE PYMT"]
	if !ok {
		t.Fatal("payment transaction not found")
	}
	if payment.Type != domain.TransactionTypeIncome {
		t.Errorf("a Credit-only row must be income, got %s", payment.Type)
	}
	if payment.Amount != 1377.33 {
		t.Errorf("expected amount 1377.33, got %v", payment.Amount)
	}
}

// TestTransactionService_Import_ZeroErrorsSerializesAsEmptyArrayNotNull
// guards a real bug caught only in a real browser, not by any Go-level
// assertion: a Go `len(result.Errors) == 0` is true for both a nil slice
// and an empty one, but `encoding/json` marshals them differently
// ("errors":null vs "errors":[]) — and the frontend calls
// importResult.errors.length unconditionally, which throws on null. This
// test marshals the real result to JSON (not just inspecting the Go
// struct) specifically so it would catch a regression back to the zero
// value's nil slice.
func TestTransactionService_Import_ZeroErrorsSerializesAsEmptyArrayNotNull(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	result, err := svc.Import(ctx, "user-1", accountID, []domain.ImportRow{
		{Date: "2026-01-05", Description: "Good row", Amount: "10.00"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Skipped != 0 {
		t.Fatalf("expected 0 skipped for this test's setup, got %d", result.Skipped)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	if strings.Contains(string(b), `"errors":null`) {
		t.Fatalf(`expected "errors":[] when there are no row errors, got null: %s`, b)
	}
	if !strings.Contains(string(b), `"errors":[]`) {
		t.Fatalf(`expected "errors":[] in the marshaled JSON, got: %s`, b)
	}
}

func TestTransactionService_Import_ExplicitTypeOverridesSignInference(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	// A positive amount explicitly marked as an expense (e.g. a refund
	// tracked as a positive "expense" adjustment rather than income) must
	// respect the explicit column, not the sign-based default.
	rows := []domain.ImportRow{
		{Date: "2026-01-05", Description: "Refund", Amount: "10.00", Type: "expense"},
	}

	result, err := svc.Import(ctx, "user-1", accountID, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("expected 1 imported, got %d (errors: %+v)", result.Imported, result.Errors)
	}

	page, _ := svc.List(ctx, "user-1", domain.ListTransactionsInput{AccountID: accountID, PageSize: 10})
	if len(page.Transactions) != 1 || page.Transactions[0].Type != domain.TransactionTypeExpense {
		t.Fatalf("expected explicit type=expense to be respected, got %+v", page.Transactions)
	}
}

func TestTransactionService_Import_SkipsBadRowsButKeepsGoingAndReportsRowNumbers(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	rows := []domain.ImportRow{
		{Date: "not-a-date", Description: "Bad date", Amount: "10.00"},                     // row 2 (header is row 1)
		{Date: "2026-01-05", Description: "Good row", Amount: "10.00"},                     // row 3
		{Date: "2026-01-06", Description: "Bad amount", Amount: "abc"},                     // row 4
		{Date: "2026-01-07", Description: "Zero amount", Amount: "0"},                      // row 5
		{Date: "2026-01-08", Description: "Bad type", Amount: "10.00", Type: "not-a-type"}, // row 6
	}

	result, err := svc.Import(ctx, "user-1", accountID, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 1 {
		t.Errorf("expected 1 imported (the good row), got %d", result.Imported)
	}
	if result.Skipped != 4 {
		t.Errorf("expected 4 skipped, got %d", result.Skipped)
	}
	if len(result.Errors) != 4 {
		t.Fatalf("expected 4 row errors, got %d: %+v", len(result.Errors), result.Errors)
	}

	gotRows := map[int]bool{}
	for _, e := range result.Errors {
		gotRows[e.Row] = true
	}
	for _, wantRow := range []int{2, 4, 5, 6} {
		if !gotRows[wantRow] {
			t.Errorf("expected an error for row %d, got rows: %+v", wantRow, result.Errors)
		}
	}
}

func TestTransactionService_Import_UnknownOrForeignAccountIsRejectedUpFront(t *testing.T) {
	svc, accountRepo, _ := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	_, err := svc.Import(ctx, "user-1", "does-not-exist", []domain.ImportRow{
		{Date: "2026-01-05", Description: "x", Amount: "10.00"},
	})
	if err == nil {
		t.Fatal("expected an error for a nonexistent account")
	}

	// A second user's account, in the SAME underlying repository, must be
	// rejected the same way — this is what actually exercises the
	// ownership check (accountRepo.GetByIDForUser scoped by user_id), not
	// just "the ID doesn't exist anywhere."
	accountSvc := service.NewAccountService(accountRepo)
	otherAccount, err := accountSvc.Create(ctx, "user-2", domain.CreateAccountInput{
		Name: "User 2's account", Type: domain.AccountTypeChecking, Currency: "USD",
	})
	if err != nil {
		t.Fatalf("setup other user's account: %v", err)
	}

	_, err = svc.Import(ctx, "user-1", otherAccount.ID, []domain.ImportRow{
		{Date: "2026-01-05", Description: "x", Amount: "10.00"},
	})
	if err == nil {
		t.Fatal("expected an error importing into another user's account")
	}
}

func TestTransactionService_Import_EmptyAccountIDIsValidationError(t *testing.T) {
	svc, _, _ := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	_, err := svc.Import(ctx, "user-1", "", []domain.ImportRow{
		{Date: "2026-01-05", Description: "x", Amount: "10.00"},
	})
	if err == nil {
		t.Fatal("expected a validation error for an empty account_id")
	}
}

func TestTransactionService_Import_TooManyRowsIsRejectedUpFront(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	rows := make([]domain.ImportRow, 5001)
	for i := range rows {
		rows[i] = domain.ImportRow{Date: "2026-01-05", Description: "x", Amount: "10.00"}
	}

	_, err := svc.Import(ctx, "user-1", accountID, rows)
	if err == nil {
		t.Fatal("expected an error for a file exceeding the max row count")
	}
}

func TestTransactionService_Import_LongDescriptionIsTruncatedNotRejected(t *testing.T) {
	svc, _, accountID := setupTransactionServiceWithAccount(t, "user-1")
	ctx := context.Background()

	longDesc := strings.Repeat("a", 2000)
	result, err := svc.Import(ctx, "user-1", accountID, []domain.ImportRow{
		{Date: "2026-01-05", Description: longDesc, Amount: "10.00"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("expected the row to be imported (truncated, not rejected), got imported=%d skipped=%d errors=%+v", result.Imported, result.Skipped, result.Errors)
	}

	page, _ := svc.List(ctx, "user-1", domain.ListTransactionsInput{AccountID: accountID, PageSize: 10})
	if len(page.Transactions) != 1 || len(page.Transactions[0].Note) != 1000 {
		t.Fatalf("expected note truncated to 1000 chars, got length %d", len(page.Transactions[0].Note))
	}
}
