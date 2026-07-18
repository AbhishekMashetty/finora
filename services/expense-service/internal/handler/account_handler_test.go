package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/handler"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// fakeAccountService is a hand-written stand-in for domain.AccountService so
// handler tests never depend on the service layer's real implementation or a
// database.
type fakeAccountService struct {
	accounts map[string]domain.Account
	nextID   int
}

func newFakeAccountService() *fakeAccountService {
	return &fakeAccountService{accounts: make(map[string]domain.Account)}
}

func (f *fakeAccountService) Create(_ context.Context, userID string, in domain.CreateAccountInput) (*domain.Account, error) {
	f.nextID++
	id := fmt.Sprintf("acc-%d", f.nextID)
	a := domain.Account{ID: id, UserID: userID, Name: in.Name, Type: in.Type, Currency: in.Currency}
	f.accounts[id] = a
	return &a, nil
}

func (f *fakeAccountService) List(_ context.Context, userID string) ([]domain.Account, error) {
	out := make([]domain.Account, 0)
	for _, a := range f.accounts {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAccountService) Get(_ context.Context, userID, id string) (*domain.Account, error) {
	a, ok := f.accounts[id]
	if !ok || a.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return &a, nil
}

func (f *fakeAccountService) Update(_ context.Context, userID, id string, in domain.UpdateAccountInput) (*domain.Account, error) {
	a, ok := f.accounts[id]
	if !ok || a.UserID != userID {
		return nil, domain.ErrNotFound
	}
	a.Name = in.Name
	f.accounts[id] = a
	return &a, nil
}

func (f *fakeAccountService) Delete(_ context.Context, userID, id string) error {
	a, ok := f.accounts[id]
	if !ok || a.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.accounts, id)
	return nil
}

var _ domain.AccountService = (*fakeAccountService)(nil)

func newTestRouter(svc domain.AccountService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewAccountHandler(svc)

	api := r.Group("/api/v1")
	api.Use(middleware.RequireIdentity())
	{
		accounts := api.Group("/accounts")
		accounts.GET("", h.List)
		accounts.POST("", h.Create)
		accounts.GET("/:id", h.Get)
		accounts.PUT("/:id", h.Update)
		accounts.DELETE("/:id", h.Delete)
	}
	return r
}

func decodeEnvelope(t *testing.T, body *bytes.Buffer) httpx.Envelope {
	t.Helper()
	var env httpx.Envelope
	if err := json.Unmarshal(body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v (body: %s)", err, body.String())
	}
	return env
}

func TestAccountHandler_Create(t *testing.T) {
	router := newTestRouter(newFakeAccountService())

	body := `{"name":"Checking","type":"checking","currency":"USD"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "user-1")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	env := decodeEnvelope(t, w.Body)
	if !env.Success {
		t.Fatalf("expected success envelope, got %+v", env)
	}
}

func TestAccountHandler_Create_MissingIdentity(t *testing.T) {
	router := newTestRouter(newFakeAccountService())

	body := `{"name":"Checking","type":"checking","currency":"USD"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-User-Id header set.
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when X-User-Id missing, got %d", w.Code)
	}
}

func TestAccountHandler_List_ScopedToUser(t *testing.T) {
	svc := newFakeAccountService()
	router := newTestRouter(svc)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "user-1", domain.CreateAccountInput{Name: "A", Type: domain.AccountTypeCash, Currency: "USD"}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", domain.CreateAccountInput{Name: "B", Type: domain.AccountTypeCash, Currency: "USD"}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("X-User-Id", "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	env := decodeEnvelope(t, w.Body)
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be an object, got %T", env.Data)
	}
	accounts, ok := data["accounts"].([]interface{})
	if !ok {
		t.Fatalf("expected accounts field to be a list, got %T", data["accounts"])
	}
	if len(accounts) != 1 {
		t.Fatalf("expected exactly 1 account scoped to user-1, got %d", len(accounts))
	}
}

func TestAccountHandler_CrossUserAccess_ReturnsNotFound(t *testing.T) {
	svc := newFakeAccountService()
	router := newTestRouter(svc)

	owned, err := svc.Create(context.Background(), "owner", domain.CreateAccountInput{Name: "Mine", Type: domain.AccountTypeCash, Currency: "USD"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+owned.ID, nil)
		req.Header.Set("X-User-Id", "intruder")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
		}
		env := decodeEnvelope(t, w.Body)
		if env.Success {
			t.Fatalf("expected failure envelope, got %+v", env)
		}
		if env.Error.Code != httpx.CodeNotFound {
			t.Fatalf("expected code %q, got %q", httpx.CodeNotFound, env.Error.Code)
		}
	})

	t.Run("update", func(t *testing.T) {
		body := `{"name":"Hijacked","type":"cash","currency":"USD"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/accounts/"+owned.ID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-Id", "intruder")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("delete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/"+owned.ID, nil)
		req.Header.Set("X-User-Id", "intruder")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
		}
	})
}

func TestAccountHandler_Delete_Success(t *testing.T) {
	svc := newFakeAccountService()
	router := newTestRouter(svc)

	owned, err := svc.Create(context.Background(), "user-1", domain.CreateAccountInput{Name: "Mine", Type: domain.AccountTypeCash, Currency: "USD"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/"+owned.ID, nil)
	req.Header.Set("X-User-Id", "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
}
