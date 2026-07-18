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
	listFn     func(ctx context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error)
	markReadFn func(ctx context.Context, userID, id string) (*domain.Notification, error)
}

func (f *fakeService) Create(ctx context.Context, userID, title, message, notifType string) (*domain.Notification, error) {
	return f.createFn(ctx, userID, title, message, notifType)
}

func (f *fakeService) List(ctx context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error) {
	return f.listFn(ctx, userID, filter)
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
		listFn         func(ctx context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error)
		wantStatus     int
		wantUnreadOnly bool
		wantPage       int
		wantPageSize   int
	}{
		{
			name:   "lists notifications for the caller",
			userID: "user-1",
			query:  "",
			listFn: func(_ context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error) {
				return domain.NotificationPage{
					Notifications: []domain.Notification{
						{ID: "1", UserID: userID, Title: "A", Read: false},
						{ID: "2", UserID: userID, Title: "B", Read: true},
					},
					Page:     1,
					PageSize: 20,
					Total:    2,
				}, nil
			},
			wantStatus:     http.StatusOK,
			wantUnreadOnly: false,
		},
		{
			name:   "unread_only=true is forwarded to the service",
			userID: "user-1",
			query:  "?unread_only=true",
			listFn: func(_ context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error) {
				return domain.NotificationPage{
					Notifications: []domain.Notification{{ID: "1", UserID: userID, Title: "A", Read: false}},
					Page:          1,
					PageSize:      20,
					Total:         1,
				}, nil
			},
			wantStatus:     http.StatusOK,
			wantUnreadOnly: true,
		},
		{
			name:   "page/page_size query params are forwarded to the service",
			userID: "user-1",
			query:  "?page=2&page_size=5",
			listFn: func(_ context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error) {
				return domain.NotificationPage{Notifications: []domain.Notification{}, Page: 2, PageSize: 5, Total: 12}, nil
			},
			wantStatus:   http.StatusOK,
			wantPage:     2,
			wantPageSize: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotFilter domain.NotificationFilter
			var called bool
			svc := &fakeService{
				listFn: func(ctx context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error) {
					called = true
					gotFilter = filter
					return tt.listFn(ctx, userID, filter)
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
			if gotFilter.UnreadOnly != tt.wantUnreadOnly {
				t.Errorf("unreadOnly passed to service = %v, want %v", gotFilter.UnreadOnly, tt.wantUnreadOnly)
			}
			if tt.wantPage != 0 && gotFilter.Page != tt.wantPage {
				t.Errorf("page passed to service = %d, want %d", gotFilter.Page, tt.wantPage)
			}
			if tt.wantPageSize != 0 && gotFilter.PageSize != tt.wantPageSize {
				t.Errorf("page_size passed to service = %d, want %d", gotFilter.PageSize, tt.wantPageSize)
			}

			var body struct {
				Data struct {
					Page     int `json:"page"`
					PageSize int `json:"page_size"`
					Total    int `json:"total"`
				} `json:"data"`
			}
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}
			if body.Data.Page == 0 && body.Data.PageSize == 0 {
				t.Errorf("response envelope is missing page/page_size — got %+v", body.Data)
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
