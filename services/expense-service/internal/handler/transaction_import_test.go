package handler_test

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/handler"
	"github.com/finora/expense-service/internal/service"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// fakeTransactionServiceForImport is a hand-written stand-in for
// domain.TransactionService, focused on exercising ImportCSV — the other
// methods are unused by these tests and just return zero values, matching
// the minimal-fake style already used elsewhere in this package (e.g.
// account_handler_test.go's fakeAccountService only implements real
// behavior for what its own tests exercise).
type fakeTransactionServiceForImport struct {
	lastUserID    string
	lastAccountID string
	lastRows      []domain.ImportRow
	callCount     int

	result domain.ImportResult
	err    error
}

func (f *fakeTransactionServiceForImport) Create(context.Context, string, domain.CreateTransactionInput) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionServiceForImport) List(context.Context, string, domain.ListTransactionsInput) (domain.TransactionPage, error) {
	return domain.TransactionPage{}, nil
}
func (f *fakeTransactionServiceForImport) Get(context.Context, string, string) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionServiceForImport) Update(context.Context, string, string, domain.UpdateTransactionInput) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionServiceForImport) Delete(context.Context, string, string) error {
	return nil
}

func (f *fakeTransactionServiceForImport) Import(_ context.Context, userID, accountID string, rows []domain.ImportRow) (domain.ImportResult, error) {
	f.callCount++
	f.lastUserID = userID
	f.lastAccountID = accountID
	f.lastRows = rows
	if f.err != nil {
		return domain.ImportResult{}, f.err
	}
	return f.result, nil
}

var _ domain.TransactionService = (*fakeTransactionServiceForImport)(nil)

func newImportTestRouter(svc domain.TransactionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewTransactionHandler(svc)

	api := r.Group("/api/v1")
	api.Use(middleware.RequireIdentity())
	{
		transactions := api.Group("/transactions")
		transactions.POST("/import", h.ImportCSV)
	}
	return r
}

// newImportRequest builds a real multipart/form-data request, exactly what
// a browser's <input type="file"> + FormData would send — this is what
// makes these tests actually exercise ImportCSV's multipart/CSV parsing,
// not just the service call behind it.
func newImportRequest(t *testing.T, accountID string, includeFile bool, csvContent string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if accountID != "" {
		if err := writer.WriteField("account_id", accountID); err != nil {
			t.Fatalf("failed to write account_id field: %v", err)
		}
	}
	if includeFile {
		part, err := writer.CreateFormFile("file", "statement.csv")
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := part.Write([]byte(csvContent)); err != nil {
			t.Fatalf("failed to write CSV content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-Id", "user-1")
	return req
}

func TestTransactionHandler_ImportCSV_Success(t *testing.T) {
	fake := &fakeTransactionServiceForImport{result: domain.ImportResult{Imported: 2, Skipped: 0}}
	router := newImportTestRouter(fake)

	csv := "Date,Description,Amount\n2026-01-05,Grocery Store,-45.20\n2026-01-06,Paycheck,1200.00\n"
	req := newImportRequest(t, "acc-1", true, csv)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	env := decodeEnvelope(t, w.Body)
	if !env.Success {
		t.Fatalf("expected success envelope, got %+v", env)
	}

	if fake.callCount != 1 {
		t.Fatalf("expected Import to be called once, got %d", fake.callCount)
	}
	if fake.lastUserID != "user-1" {
		t.Errorf("expected userID from X-User-Id header, got %q", fake.lastUserID)
	}
	if fake.lastAccountID != "acc-1" {
		t.Errorf("expected account_id from form field, got %q", fake.lastAccountID)
	}
	if len(fake.lastRows) != 2 {
		t.Fatalf("expected 2 parsed rows, got %d: %+v", len(fake.lastRows), fake.lastRows)
	}
	if fake.lastRows[0].Date != "2026-01-05" || fake.lastRows[0].Description != "Grocery Store" || fake.lastRows[0].Amount != "-45.20" {
		t.Errorf("row 0 parsed incorrectly: %+v", fake.lastRows[0])
	}
}

func TestTransactionHandler_ImportCSV_ColumnAliasesAreCaseInsensitive(t *testing.T) {
	fake := &fakeTransactionServiceForImport{}
	router := newImportTestRouter(fake)

	// Real statement exports vary in header naming — "Transaction Date"/
	// "Merchant"/"Debit" instead of this app's own "Date"/"Description"/
	// "Amount" — and case varies too. "Debit" here is a real, standalone
	// column (a Capital One-style export), not an alias for a signed
	// "Amount" column — it must land in row.Debit, not row.Amount.
	csv := "TRANSACTION DATE,Merchant,Debit\n01/15/2026,Coffee Shop,4.50\n"
	req := newImportRequest(t, "acc-1", true, csv)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(fake.lastRows) != 1 {
		t.Fatalf("expected 1 parsed row, got %d", len(fake.lastRows))
	}
	row := fake.lastRows[0]
	if row.Date != "01/15/2026" || row.Description != "Coffee Shop" || row.Debit != "4.50" || row.Amount != "" {
		t.Errorf("aliased columns parsed incorrectly: %+v", row)
	}
}

func TestTransactionHandler_ImportCSV_MissingAccountID(t *testing.T) {
	fake := &fakeTransactionServiceForImport{}
	router := newImportTestRouter(fake)

	req := newImportRequest(t, "", true, "Date,Description,Amount\n2026-01-05,x,10.00\n")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	env := decodeEnvelope(t, w.Body)
	if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %+v", env.Error)
	}
	if fake.callCount != 0 {
		t.Error("expected Import to never be called when account_id is missing")
	}
}

func TestTransactionHandler_ImportCSV_MissingFile(t *testing.T) {
	fake := &fakeTransactionServiceForImport{}
	router := newImportTestRouter(fake)

	req := newImportRequest(t, "acc-1", false, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	if fake.callCount != 0 {
		t.Error("expected Import to never be called when no file is uploaded")
	}
}

func TestTransactionHandler_ImportCSV_DebitCreditColumnsParsedSeparatelyFromAmount(t *testing.T) {
	fake := &fakeTransactionServiceForImport{}
	router := newImportTestRouter(fake)

	// A real Capital One export shape: separate Debit/Credit columns, no
	// Amount column at all. This is the exact case that used to be
	// silently mislabeled as income (see the regression test in
	// transaction_import_test.go, service package, for the amount/type
	// resolution itself — this test only asserts the handler routes the
	// two columns into the right ImportRow fields).
	csv := "Transaction Date,Posted Date,Card No.,Description,Category,Debit,Credit\n" +
		"2026-06-23,2026-06-25,9854,SAFEWAY #2776,Merchandise,29.68,\n" +
		"2026-06-01,2026-06-01,9854,CAPITAL ONE MOBILE PYMT,Payment/Credit,,1377.33\n"
	req := newImportRequest(t, "acc-1", true, csv)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(fake.lastRows) != 2 {
		t.Fatalf("expected 2 parsed rows, got %d", len(fake.lastRows))
	}
	purchase := fake.lastRows[0]
	if purchase.Debit != "29.68" || purchase.Credit != "" || purchase.Amount != "" {
		t.Errorf("purchase row parsed incorrectly: %+v", purchase)
	}
	payment := fake.lastRows[1]
	if payment.Credit != "1377.33" || payment.Debit != "" || payment.Amount != "" {
		t.Errorf("payment row parsed incorrectly: %+v", payment)
	}
}

func TestTransactionHandler_ImportCSV_MissingRequiredColumns(t *testing.T) {
	fake := &fakeTransactionServiceForImport{}
	router := newImportTestRouter(fake)

	// No recognizable amount column at all.
	req := newImportRequest(t, "acc-1", true, "Date,Description\n2026-01-05,x\n")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	if fake.callCount != 0 {
		t.Error("expected Import to never be called when the CSV is missing required columns")
	}
}

func TestTransactionHandler_ImportCSV_EmptyFile(t *testing.T) {
	fake := &fakeTransactionServiceForImport{}
	router := newImportTestRouter(fake)

	req := newImportRequest(t, "acc-1", true, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestTransactionHandler_ImportCSV_MissingIdentity(t *testing.T) {
	fake := &fakeTransactionServiceForImport{}
	router := newImportTestRouter(fake)

	req := newImportRequest(t, "acc-1", true, "Date,Description,Amount\n2026-01-05,x,10.00\n")
	req.Header.Del("X-User-Id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when X-User-Id missing, got %d", w.Code)
	}
}

func TestTransactionHandler_ImportCSV_ServiceValidationErrorTranslatesTo400(t *testing.T) {
	fake := &fakeTransactionServiceForImport{err: service.NewValidationError("account_id", "account does not exist")}
	router := newImportTestRouter(fake)

	req := newImportRequest(t, "does-not-exist", true, "Date,Description,Amount\n2026-01-05,x,10.00\n")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	env := decodeEnvelope(t, w.Body)
	if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %+v", env.Error)
	}
}

func TestTransactionHandler_ImportCSV_UnexpectedServiceErrorTranslatesTo500(t *testing.T) {
	fake := &fakeTransactionServiceForImport{err: errors.New("boom")}
	router := newImportTestRouter(fake)

	req := newImportRequest(t, "acc-1", true, "Date,Description,Amount\n2026-01-05,x,10.00\n")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", w.Code, w.Body.String())
	}
}
