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

// fakeGoalService is a hand-written fake of domain.GoalService.
type fakeGoalService struct {
	createFn func(ctx context.Context, userID string, in domain.CreateGoalInput) (*domain.Goal, error)
	listFn   func(ctx context.Context, userID string) ([]domain.Goal, error)
	getFn    func(ctx context.Context, userID, id string) (*domain.Goal, error)
	updateFn func(ctx context.Context, userID, id string, in domain.UpdateGoalInput) (*domain.Goal, error)
	deleteFn func(ctx context.Context, userID, id string) error
}

func (f *fakeGoalService) Create(ctx context.Context, userID string, in domain.CreateGoalInput) (*domain.Goal, error) {
	return f.createFn(ctx, userID, in)
}
func (f *fakeGoalService) List(ctx context.Context, userID string) ([]domain.Goal, error) {
	return f.listFn(ctx, userID)
}
func (f *fakeGoalService) Get(ctx context.Context, userID, id string) (*domain.Goal, error) {
	return f.getFn(ctx, userID, id)
}
func (f *fakeGoalService) Update(ctx context.Context, userID, id string, in domain.UpdateGoalInput) (*domain.Goal, error) {
	return f.updateFn(ctx, userID, id, in)
}
func (f *fakeGoalService) Delete(ctx context.Context, userID, id string) error {
	return f.deleteFn(ctx, userID, id)
}

func newGoalTestRouter(svc domain.GoalService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := NewGoalHandler(svc)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RequireIdentity())
	{
		goals := v1.Group("/goals")
		goals.GET("", h.List)
		goals.POST("", h.Create)
		goals.GET("/:id", h.Get)
		goals.PUT("/:id", h.Update)
		goals.DELETE("/:id", h.Delete)
	}

	return r
}

func TestGoalHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createFn   func(ctx context.Context, userID string, in domain.CreateGoalInput) (*domain.Goal, error)
		wantStatus int
	}{
		{
			name: "creates a goal for the caller",
			body: `{"name":"Emergency fund","target_amount":5000,"target_date":"2030-01-01"}`,
			createFn: func(_ context.Context, userID string, in domain.CreateGoalInput) (*domain.Goal, error) {
				return &domain.Goal{ID: "g1", UserID: userID, Name: in.Name, TargetAmount: in.TargetAmount, TargetDate: in.TargetDate, CreatedAt: time.Now().UTC()}, nil
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid body is a validation error",
			body:       `{"name":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed target_date is a validation error",
			body:       `{"name":"Trip","target_amount":100,"target_date":"not-a-date"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeGoalService{createFn: tt.createFn}
			r := newGoalTestRouter(svc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/goals", bytes.NewBufferString(tt.body))
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

func TestGoalHandler_List(t *testing.T) {
	svc := &fakeGoalService{
		listFn: func(_ context.Context, userID string) ([]domain.Goal, error) {
			return []domain.Goal{{ID: "g1", UserID: userID, Name: "Trip", TargetAmount: 2000}}, nil
		},
	}
	r := newGoalTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/goals", nil)
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

func TestGoalHandler_Get_CrossUserIs404(t *testing.T) {
	svc := &fakeGoalService{
		getFn: func(_ context.Context, _, _ string) (*domain.Goal, error) {
			return nil, domain.ErrNotFound
		},
	}
	r := newGoalTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/goals/other-users-goal", nil)
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

func TestGoalHandler_Update(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		updateFn   func(ctx context.Context, userID, id string, in domain.UpdateGoalInput) (*domain.Goal, error)
		wantStatus int
	}{
		{
			name: "owner update succeeds, including current_amount progress",
			body: `{"name":"Trip","target_amount":2000,"target_date":"2030-01-01","current_amount":500}`,
			updateFn: func(_ context.Context, userID, id string, in domain.UpdateGoalInput) (*domain.Goal, error) {
				return &domain.Goal{ID: id, UserID: userID, Name: in.Name, TargetAmount: in.TargetAmount, TargetDate: in.TargetDate, CurrentAmount: in.CurrentAmount}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "current_amount of zero is accepted, not treated as missing",
			body: `{"name":"Trip","target_amount":2000,"target_date":"2030-01-01","current_amount":0}`,
			updateFn: func(_ context.Context, userID, id string, in domain.UpdateGoalInput) (*domain.Goal, error) {
				return &domain.Goal{ID: id, UserID: userID, Name: in.Name, TargetAmount: in.TargetAmount, TargetDate: in.TargetDate, CurrentAmount: in.CurrentAmount}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "cross-user update is 404, not 403",
			body: `{"name":"Trip","target_amount":2000,"target_date":"2030-01-01"}`,
			updateFn: func(_ context.Context, _, _ string, _ domain.UpdateGoalInput) (*domain.Goal, error) {
				return nil, domain.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "malformed target_date is a validation error",
			body:       `{"name":"Trip","target_amount":2000,"target_date":"not-a-date"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing name is a validation error",
			body:       `{"name":"","target_amount":2000,"target_date":"2030-01-01"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeGoalService{updateFn: tt.updateFn}
			r := newGoalTestRouter(svc)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/goals/g1", bytes.NewBufferString(tt.body))
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

func TestGoalHandler_Delete(t *testing.T) {
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
			svc := &fakeGoalService{deleteFn: tt.deleteFn}
			r := newGoalTestRouter(svc)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/goals/g1", nil)
			req.Header.Set("X-User-Id", "user-1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
