package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/finora/notification-service/internal/domain"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// fakeService is a hand-written fake of domain.NotificationService so the
// handler layer can be tested without any real service/repository/Mongo.
type fakeService struct {
	createFn   func(ctx context.Context, userID, title, message, notifType string) (*domain.Notification, error)
	listFn     func(ctx context.Context, userID string, unreadOnly bool) ([]domain.Notification, error)
	markReadFn func(ctx context.Context, userID, id string) (*domain.Notification, error)
}

func (f *fakeService) Create(ctx context.Context, userID, title, message, notifType string) (*domain.Notification, error) {
	return f.createFn(ctx, userID, title, message, notifType)
}

func (f *fakeService) List(ctx context.Context, userID string, unreadOnly bool) ([]domain.Notification, error) {
	return f.listFn(ctx, userID, unreadOnly)
}

func (f *fakeService) MarkRead(ctx context.Context, userID, id string) (*domain.Notification, error) {
	return f.markReadFn(ctx, userID, id)
}

// newTestRouter builds a minimal gin.Engine that mimics the real router's
// relevant piece: RequireIdentity guarding the notification routes, with
// userID injected via the X-User-Id header exactly like the gateway does.
func newTestRouter(svc *fakeService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := NewNotificationHandler(svc)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RequireIdentity())
	{
		notifications := v1.Group("/notifications")
		notifications.GET("", h.List)
		notifications.POST("", h.Create)
		notifications.PATCH("/:id/read", h.MarkRead)
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

func TestNotificationHandler_Create(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		body        string
		createFn    func(ctx context.Context, userID, title, message, notifType string) (*domain.Notification, error)
		wantStatus  int
		wantSuccess bool
	}{
		{
			name:   "creates a notification for the caller",
			userID: "user-1",
			body:   `{"title":"Budget alert","message":"You are over budget","type":"budget_alert"}`,
			createFn: func(_ context.Context, userID, title, message, notifType string) (*domain.Notification, error) {
				return &domain.Notification{
					ID: "abc123", UserID: userID, Title: title, Message: message,
					Type: notifType, Read: false, CreatedAt: time.Now().UTC(),
				}, nil
			},
			wantStatus:  http.StatusCreated,
			wantSuccess: true,
		},
		{
			name:        "missing user id header is rejected before reaching the service",
			userID:      "",
			body:        `{"title":"x","message":"y","type":"z"}`,
			wantStatus:  http.StatusUnauthorized,
			wantSuccess: false,
		},
		{
			name:        "invalid body is a validation error",
			userID:      "user-1",
			body:        `{"title":""}`,
			wantStatus:  http.StatusBadRequest,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{createFn: tt.createFn}
			r := newTestRouter(svc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.userID != "" {
				req.Header.Set("X-User-Id", tt.userID)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}

			var env envelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if env.Success != tt.wantSuccess {
				t.Errorf("success = %v, want %v", env.Success, tt.wantSuccess)
			}
		})
	}
}

func TestNotificationHandler_List(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		query          string
		listFn         func(ctx context.Context, userID string, unreadOnly bool) ([]domain.Notification, error)
		wantStatus     int
		wantUnreadOnly bool
	}{
		{
			name:   "lists notifications for the caller",
			userID: "user-1",
			query:  "",
			listFn: func(_ context.Context, userID string, unreadOnly bool) ([]domain.Notification, error) {
				return []domain.Notification{
					{ID: "1", UserID: userID, Title: "A", Read: false},
					{ID: "2", UserID: userID, Title: "B", Read: true},
				}, nil
			},
			wantStatus:     http.StatusOK,
			wantUnreadOnly: false,
		},
		{
			name:   "unread_only=true is forwarded to the service",
			userID: "user-1",
			query:  "?unread_only=true",
			listFn: func(_ context.Context, userID string, unreadOnly bool) ([]domain.Notification, error) {
				return []domain.Notification{{ID: "1", UserID: userID, Title: "A", Read: false}}, nil
			},
			wantStatus:     http.StatusOK,
			wantUnreadOnly: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUnreadOnly bool
			var called bool
			svc := &fakeService{
				listFn: func(ctx context.Context, userID string, unreadOnly bool) ([]domain.Notification, error) {
					called = true
					gotUnreadOnly = unreadOnly
					return tt.listFn(ctx, userID, unreadOnly)
				},
			}
			r := newTestRouter(svc)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications"+tt.query, nil)
			req.Header.Set("X-User-Id", tt.userID)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if !called {
				t.Fatalf("service.List was not called")
			}
			if gotUnreadOnly != tt.wantUnreadOnly {
				t.Errorf("unreadOnly passed to service = %v, want %v", gotUnreadOnly, tt.wantUnreadOnly)
			}
		})
	}
}

func TestNotificationHandler_MarkRead(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		markReadFn func(ctx context.Context, userID, id string) (*domain.Notification, error)
		wantStatus int
	}{
		{
			name: "marks a notification as read",
			id:   "abc123",
			markReadFn: func(_ context.Context, userID, id string) (*domain.Notification, error) {
				return &domain.Notification{ID: id, UserID: userID, Read: true}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "not found bubbles up as 404",
			id:   "missing",
			markReadFn: func(_ context.Context, _, _ string) (*domain.Notification, error) {
				return nil, domain.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{markReadFn: tt.markReadFn}
			r := newTestRouter(svc)

			req := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/"+tt.id+"/read", nil)
			req.Header.Set("X-User-Id", "user-1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
