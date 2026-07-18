package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/finora/shared/httpx"
	"github.com/finora/user-service/internal/domain"
	"github.com/gin-gonic/gin"
)

// fakeAuthService is a hand-written stub implementing domain.AuthService,
// so handler tests never depend on Mongo or the real service layer.
type fakeAuthService struct {
	registerErr error
	registerRes *domain.User

	loginErr error
	loginRes *domain.LoginResult

	refreshErr error
	refreshRes *domain.LoginResult

	logoutErr error

	passwordResetErr error
}

func (f *fakeAuthService) Register(ctx context.Context, in domain.RegisterInput) (*domain.User, error) {
	return f.registerRes, f.registerErr
}

func (f *fakeAuthService) Login(ctx context.Context, email, password string) (*domain.LoginResult, error) {
	return f.loginRes, f.loginErr
}

func (f *fakeAuthService) Refresh(ctx context.Context, refreshToken string) (*domain.LoginResult, error) {
	return f.refreshRes, f.refreshErr
}

func (f *fakeAuthService) Logout(ctx context.Context, refreshToken string) error {
	return f.logoutErr
}

func (f *fakeAuthService) RequestPasswordReset(ctx context.Context, email string) error {
	return f.passwordResetErr
}

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestRouter(authSvc domain.AuthService) *gin.Engine {
	r := gin.New()
	h := NewAuthHandler(authSvc)
	v1 := r.Group("/api/v1/auth")
	v1.POST("/register", h.Register)
	v1.POST("/login", h.Login)
	v1.POST("/refresh", h.Refresh)
	v1.POST("/logout", h.Logout)
	v1.POST("/password-reset", h.RequestPasswordReset)
	return r
}

func doJSONRequest(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) httpx.Envelope {
	t.Helper()
	var env httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v (body: %s)", err, rec.Body.String())
	}
	return env
}

func TestAuthHandlerRegister(t *testing.T) {
	t.Run("duplicate email returns 409 CONFLICT envelope", func(t *testing.T) {
		fake := &fakeAuthService{registerErr: domain.ErrDuplicateEmail}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
			"email":    "dup@example.com",
			"password": "supersecret1",
			"name":     "Dup",
		})

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		env := decodeEnvelope(t, rec)
		if env.Success {
			t.Error("expected success=false")
		}
		if env.Error == nil || env.Error.Code != httpx.CodeConflict {
			t.Errorf("expected CONFLICT error code, got %+v", env.Error)
		}
	})

	t.Run("missing fields returns 400 VALIDATION_ERROR", func(t *testing.T) {
		fake := &fakeAuthService{}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
			"email": "onlyemail@example.com",
		})

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		env := decodeEnvelope(t, rec)
		if env.Error == nil || env.Error.Code != httpx.CodeValidation {
			t.Errorf("expected VALIDATION_ERROR, got %+v", env.Error)
		}
	})

	t.Run("success returns 201 with user, no password hash leaked", func(t *testing.T) {
		fake := &fakeAuthService{registerRes: &domain.User{ID: "u1", Email: "new@example.com", Name: "New", PasswordHash: "should-never-appear"}}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
			"email":    "new@example.com",
			"password": "supersecret1",
			"name":     "New",
		})

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if bytes.Contains(rec.Body.Bytes(), []byte("should-never-appear")) {
			t.Error("password hash leaked into response body")
		}
	})
}

func TestAuthHandlerLogin(t *testing.T) {
	t.Run("wrong password returns 401 UNAUTHORIZED", func(t *testing.T) {
		fake := &fakeAuthService{loginErr: domain.ErrInvalidCredentials}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
			"email":    "someone@example.com",
			"password": "wrong",
		})

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		env := decodeEnvelope(t, rec)
		if env.Error == nil || env.Error.Code != httpx.CodeUnauthorized {
			t.Errorf("expected UNAUTHORIZED, got %+v", env.Error)
		}
	})

	t.Run("success returns tokens and user", func(t *testing.T) {
		fake := &fakeAuthService{loginRes: &domain.LoginResult{
			AccessToken:  "access-tok",
			RefreshToken: "refresh-tok",
			User:         &domain.User{ID: "u1", Email: "someone@example.com", Name: "Someone"},
		}}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
			"email":    "someone@example.com",
			"password": "correct",
		})

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		env := decodeEnvelope(t, rec)
		data, ok := env.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected data to be an object, got %T", env.Data)
		}
		if data["access_token"] != "access-tok" || data["refresh_token"] != "refresh-tok" {
			t.Errorf("unexpected token fields: %+v", data)
		}
	})
}

func TestAuthHandlerRefresh(t *testing.T) {
	t.Run("invalid token returns 401 UNAUTHORIZED", func(t *testing.T) {
		fake := &fakeAuthService{refreshErr: domain.ErrInvalidToken}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
			"refresh_token": "garbage",
		})

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("valid token returns rotated pair", func(t *testing.T) {
		fake := &fakeAuthService{refreshRes: &domain.LoginResult{AccessToken: "new-access", RefreshToken: "new-refresh"}}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
			"refresh_token": "old-refresh",
		})

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

func TestAuthHandlerLogout(t *testing.T) {
	t.Run("always 204 when service reports no internal error", func(t *testing.T) {
		fake := &fakeAuthService{logoutErr: nil}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/logout", map[string]string{
			"refresh_token": "whatever",
		})

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("internal error surfaces as 500", func(t *testing.T) {
		fake := &fakeAuthService{logoutErr: errors.New("db exploded")}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/logout", map[string]string{
			"refresh_token": "whatever",
		})

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

func TestAuthHandlerRequestPasswordReset(t *testing.T) {
	t.Run("unknown email still returns 202, never leaks existence", func(t *testing.T) {
		fake := &fakeAuthService{passwordResetErr: nil}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/password-reset", map[string]string{
			"email": "nobody@example.com",
		})

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid email fails validation", func(t *testing.T) {
		fake := &fakeAuthService{}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/password-reset", map[string]string{
			"email": "not-an-email",
		})

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("internal error surfaces as 500", func(t *testing.T) {
		fake := &fakeAuthService{passwordResetErr: errors.New("db exploded")}
		r := newTestRouter(fake)

		rec := doJSONRequest(t, r, http.MethodPost, "/api/v1/auth/password-reset", map[string]string{
			"email": "demo@finora.dev",
		})

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})
}
