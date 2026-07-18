package authmw_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/finora/shared/jwtx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"

	"github.com/finora/gateway/internal/authmw"
)

const testSecret = "test-access-secret"

func newTestRouter(t *testing.T, called *bool, gotUserID *string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// stand in for middleware.RequestID() setting the context value
		// authmw reads to propagate X-Request-Id.
		c.Set("request_id", "test-request-id")
		c.Next()
	})
	r.Use(authmw.RequireAccessToken(testSecret))
	r.GET("/protected", func(c *gin.Context) {
		*called = true
		*gotUserID = c.Request.Header.Get(middleware.UserIDHeader)
		c.Status(http.StatusOK)
	})
	return r
}

func TestRequireAccessToken_ValidTokenPassesThroughAndSetsUserID(t *testing.T) {
	token, err := jwtx.GenerateAccessToken(testSecret, "user-123", "user@example.com", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var called bool
	var gotUserID string
	r := newTestRouter(t, &called, &gotUserID)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !called {
		t.Fatal("expected downstream handler to be called")
	}
	if gotUserID != "user-123" {
		t.Fatalf("expected X-User-Id=user-123, got %q", gotUserID)
	}
}

func TestRequireAccessToken_MissingHeaderRejected(t *testing.T) {
	var called bool
	var gotUserID string
	r := newTestRouter(t, &called, &gotUserID)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if called {
		t.Fatal("downstream handler must not be called without a token")
	}
}

func TestRequireAccessToken_MalformedHeaderRejected(t *testing.T) {
	var called bool
	var gotUserID string
	r := newTestRouter(t, &called, &gotUserID)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "not-a-bearer-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if called {
		t.Fatal("downstream handler must not be called with a malformed header")
	}
}

func TestRequireAccessToken_ExpiredTokenRejected(t *testing.T) {
	token, err := jwtx.GenerateAccessToken(testSecret, "user-123", "user@example.com", -1*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var called bool
	var gotUserID string
	r := newTestRouter(t, &called, &gotUserID)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if called {
		t.Fatal("downstream handler must not be called with an expired token")
	}
}

func TestRequireAccessToken_WrongTokenTypeRejected(t *testing.T) {
	// A refresh token must never be accepted where an access token is required.
	token, err := jwtx.GenerateRefreshToken(testSecret, "user-123", "some-jti", time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var called bool
	var gotUserID string
	r := newTestRouter(t, &called, &gotUserID)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if called {
		t.Fatal("downstream handler must not be called with a refresh token")
	}
}

func TestRequireAccessToken_InvalidSignatureRejected(t *testing.T) {
	token, err := jwtx.GenerateAccessToken("a-different-secret", "user-123", "user@example.com", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var called bool
	var gotUserID string
	r := newTestRouter(t, &called, &gotUserID)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if called {
		t.Fatal("downstream handler must not be called with a bad signature")
	}
}
