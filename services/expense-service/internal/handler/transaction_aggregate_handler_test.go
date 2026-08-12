package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/handler"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// fakeTransactionServiceForAggregate is a hand-written stand-in for
// domain.TransactionService, focused on exercising Aggregate — the other
// methods are unused by these tests and just return zero values, matching
// the minimal-fake style already used elsewhere in this package (see
// transaction_import_test.go's fakeTransactionServiceForImport).
type fakeTransactionServiceForAggregate struct {
	lastUserID string
	lastInput  domain.AggregateByCategoryInput

	totals []domain.CategoryTotal
	err    error
}

func (f *fakeTransactionServiceForAggregate) Create(context.Context, string, domain.CreateTransactionInput) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionServiceForAggregate) List(context.Context, string, domain.ListTransactionsInput) (domain.TransactionPage, error) {
	return domain.TransactionPage{}, nil
}
func (f *fakeTransactionServiceForAggregate) Get(context.Context, string, string) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionServiceForAggregate) Update(context.Context, string, string, domain.UpdateTransactionInput) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionServiceForAggregate) Delete(context.Context, string, string) error {
	return nil
}
func (f *fakeTransactionServiceForAggregate) Import(context.Context, string, string, []domain.ImportRow) (domain.ImportResult, error) {
	return domain.ImportResult{}, nil
}

func (f *fakeTransactionServiceForAggregate) AggregateByCategory(_ context.Context, userID string, in domain.AggregateByCategoryInput) ([]domain.CategoryTotal, error) {
	f.lastUserID = userID
	f.lastInput = in
	if f.err != nil {
		return nil, f.err
	}
	return f.totals, nil
}

var _ domain.TransactionService = (*fakeTransactionServiceForAggregate)(nil)

func newAggregateTestRouter(svc domain.TransactionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewTransactionHandler(svc)

	api := r.Group("/api/v1")
	api.Use(middleware.RequireIdentity())
	{
		transactions := api.Group("/transactions")
		transactions.GET("/aggregate", h.Aggregate)
		transactions.GET("/:id", h.Get) // registered alongside "aggregate" to prove gin resolves the static route, not the :id wildcard
	}
	return r
}

func TestTransactionHandler_Aggregate(t *testing.T) {
	t.Run("returns the service's per-category totals under the envelope", func(t *testing.T) {
		svc := &fakeTransactionServiceForAggregate{
			totals: []domain.CategoryTotal{
				{CategoryID: "cat-groceries", Total: 123.45, Count: 3},
			},
		}
		r := newAggregateTestRouter(svc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/aggregate?from=2026-01-01&to=2026-01-31&type=expense", nil)
		req.Header.Set(middleware.UserIDHeader, "user-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
		}
		if svc.lastUserID != "user-1" {
			t.Errorf("service called with userID = %q, want user-1", svc.lastUserID)
		}
		if svc.lastInput.Type != domain.TransactionTypeExpense {
			t.Errorf("service called with Type = %q, want expense", svc.lastInput.Type)
		}

		var env httpx.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		if !env.Success {
			t.Fatalf("envelope.Success = false, want true (error: %+v)", env.Error)
		}
	})

	t.Run("defaults type to expense when omitted", func(t *testing.T) {
		svc := &fakeTransactionServiceForAggregate{}
		r := newAggregateTestRouter(svc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/aggregate?from=2026-01-01&to=2026-01-31", nil)
		req.Header.Set(middleware.UserIDHeader, "user-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
		}
		if svc.lastInput.Type != domain.TransactionTypeExpense {
			t.Errorf("default Type = %q, want expense", svc.lastInput.Type)
		}
	})

	t.Run("missing from/to is a 400 validation error, never reaching the service", func(t *testing.T) {
		svc := &fakeTransactionServiceForAggregate{}
		r := newAggregateTestRouter(svc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/aggregate?to=2026-01-31", nil)
		req.Header.Set(middleware.UserIDHeader, "user-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400, body: %s", w.Code, w.Body.String())
		}
		if svc.lastUserID != "" {
			t.Error("service should never have been called with an invalid request")
		}
	})

	t.Run("GET /transactions/aggregate resolves to Aggregate, not the :id route", func(t *testing.T) {
		svc := &fakeTransactionServiceForAggregate{totals: []domain.CategoryTotal{}}
		r := newAggregateTestRouter(svc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/aggregate?from=2026-01-01&to=2026-01-31", nil)
		req.Header.Set(middleware.UserIDHeader, "user-1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Get (the :id handler) always returns nil, nil from this fake, which
		// the handler would turn into a 200 with a null "transaction" body —
		// distinguishable from Aggregate's "categories" body. Asserting the
		// service recorded lastInput (only AggregateByCategory sets it) is a
		// direct proof the static route won, not the wildcard.
		if svc.lastUserID == "" {
			t.Fatal("expected AggregateByCategory to have been called (proving the static route matched), but it wasn't")
		}
	})
}
