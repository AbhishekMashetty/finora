package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// fakeBudgetService is a hand-written fake of domain.BudgetService so the
// handler layer can be tested without any real service/repository/Mongo.
type fakeBudgetService struct {
	createFn func(ctx context.Context, userID string, in domain.CreateBudgetInput) (*domain.Budget, error)
	listFn   func(ctx context.Context, userID string) ([]domain.Budget, error)
	getFn    func(ctx context.Context, userID, id string) (*domain.Budget, error)
	updateFn func(ctx context.Context, userID, id string, in domain.UpdateBudgetInput) (*domain.Budget, error)
	deleteFn func(ctx context.Context, userID, id string) error
}

func (f *fakeBudgetService) Create(ctx context.Context, userID string, in domain.CreateBudgetInput) (*domain.Budget, error) {
	return f.createFn(ctx, userID, in)
}
func (f *fakeBudgetService) List(ctx context.Context, userID string) ([]domain.Budget, error) {
	return f.listFn(ctx, userID)
}
func (f *fakeBudgetService) Get(ctx context.Context, userID, id string) (*domain.Budget, error) {
	return f.getFn(ctx, userID, id)
}
func (f *fakeBudgetService) Update(ctx context.Context, userID, id string, in domain.UpdateBudgetInput) (*domain.Budget, error) {
	return f.updateFn(ctx, userID, id, in)
}
func (f *fakeBudgetService) Delete(ctx context.Context, userID, id string) error {
	return f.deleteFn(ctx, userID, id)
}

// newTestRouter builds a minimal gin.Engine that mimics the real router's
// relevant piece: RequireIdentity guarding the budget routes, with userID
// injected via the X-User-Id header exactly like the gateway does.
func newTestRouter(svc domain.BudgetService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := NewBudgetHandler(svc)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RequireIdentity())
	{
		budgets := v1.Group("/budgets")
		budgets.GET("", h.List)
		budgets.POST("", h.Create)
		budgets.GET("/:id", h.Get)
		budgets.PUT("/:id", h.Update)
		budgets.DELETE("/:id", h.Delete)
	}

	return r
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestBudgetHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		body       string
		createFn   func(ctx context.Context, userID string, in domain.CreateBudgetInput) (*domain.Budget, error)
		wantStatus int
	}{
		{
			name:   "creates a budget for the caller",
			userID: "user-1",
			body:   `{"category":"groceries","amount":500,"period":"monthly"}`,
			createFn: func(_ context.Context, userID string, in domain.CreateBudgetInput) (*domain.Budget, error) {
				return &domain.Budget{ID: "b1", UserID: userID, Category: in.Category, Amount: in.Amount, Period: in.Period, CreatedAt: time.Now().UTC()}, nil
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing user id header is rejected before reaching the service",
			userID:     "",
			body:       `{"category":"groceries","amount":500,"period":"monthly"}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid body is a validation error",
			userID:     "user-1",
			body:       `{"category":""}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeBudgetService{createFn: tt.createFn}
			r := newTestRouter(svc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/budgets", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.userID != "" {
				req.Header.Set("X-User-Id", tt.userID)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestBudgetHandler_List(t *testing.T) {
	svc := &fakeBudgetService{
		listFn: func(_ context.Context, userID string) ([]domain.Budget, error) {
			return []domain.Budget{{ID: "b1", UserID: userID, Category: "rent", Amount: 1000, Period: domain.PeriodMonthly}}, nil
		},
	}
	r := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/budgets", nil)
	req.Header.Set("X-User-Id", "user-1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if !env.Success {
		t.Errorf("success = %v, want true", env.Success)
	}
}

func TestBudgetHandler_Get_CrossUserIs404(t *testing.T) {
	svc := &fakeBudgetService{
		getFn: func(_ context.Context, _, _ string) (*domain.Budget, error) {
			return nil, domain.ErrNotFound
		},
	}
	r := newTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/budgets/other-users-budget", nil)
	req.Header.Set("X-User-Id", "user-1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusNotFound, w.Body.String())
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("error code = %+v, want NOT_FOUND", env.Error)
	}
}

func TestBudgetHandler_Update(t *testing.T) {
	tests := []struct {
		name       string
		updateFn   func(ctx context.Context, userID, id string, in domain.UpdateBudgetInput) (*domain.Budget, error)
		wantStatus int
	}{
		{
			name: "owner update succeeds",
			updateFn: func(_ context.Context, userID, id string, in domain.UpdateBudgetInput) (*domain.Budget, error) {
				return &domain.Budget{ID: id, UserID: userID, Category: in.Category, Amount: in.Amount, Period: in.Period}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "cross-user update is 404, not 403",
			updateFn: func(_ context.Context, _, _ string, _ domain.UpdateBudgetInput) (*domain.Budget, error) {
				return nil, domain.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeBudgetService{updateFn: tt.updateFn}
			r := newTestRouter(svc)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/budgets/b1", bytes.NewBufferString(`{"category":"rent","amount":1200,"period":"monthly"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-User-Id", "user-1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestBudgetHandler_Delete(t *testing.T) {
	tests := []struct {
		name       string
		deleteFn   func(ctx context.Context, userID, id string) error
		wantStatus int
	}{
		{
			name:       "owner delete succeeds with 204",
			deleteFn:   func(_ context.Context, _, _ string) error { return nil },
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "cross-user delete is 404, not 403",
			deleteFn:   func(_ context.Context, _, _ string) error { return domain.ErrNotFound },
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeBudgetService{deleteFn: tt.deleteFn}
			r := newTestRouter(svc)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/b1", nil)
			req.Header.Set("X-User-Id", "user-1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
