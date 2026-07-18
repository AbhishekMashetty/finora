package router_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/finora/shared/jwtx"
	"github.com/finora/shared/logger"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"

	gwconfig "github.com/finora/gateway/internal/config"
	"github.com/finora/gateway/internal/proxy"
	"github.com/finora/gateway/internal/router"
)

const testSecret = "test-access-secret"

// echoBackend records the last request it received (path, query, headers)
// and replies 200 with a tiny JSON echo, standing in for a real backend
// service in these tests — no real user-service/expense-service/etc. need
// to be running.
type echoBackend struct {
	lastPath      string
	lastRawQuery  string
	lastUserID    string
	lastRequestID string
}

func newEchoServer(rec *echoBackend) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.lastPath = r.URL.Path
		rec.lastRawQuery = r.URL.RawQuery
		rec.lastUserID = r.Header.Get(middleware.UserIDHeader)
		rec.lastRequestID = r.Header.Get(middleware.RequestIDHeader)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
}

// gatewayTestServer wraps the real gin engine in an httptest.Server. A
// reverse proxy streaming a real response needs a real http.ResponseWriter
// (one that implements http.CloseNotifier) — calling engine.ServeHTTP
// directly against an httptest.ResponseRecorder panics inside
// httputil.ReverseProxy, so every test that expects to actually reach a
// backend goes over a real HTTP round trip via this server instead.
func buildGatewayTestServer(t *testing.T, user, expense, budget, notification *echoBackend) (*httptest.Server, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	userSrv := newEchoServer(user)
	expenseSrv := newEchoServer(expense)
	budgetSrv := newEchoServer(budget)
	notificationSrv := newEchoServer(notification)

	log := logger.New("gateway-test", "error")

	userProxy, err := proxy.New(userSrv.URL, log)
	if err != nil {
		t.Fatalf("failed building user proxy: %v", err)
	}
	expenseProxy, err := proxy.New(expenseSrv.URL, log)
	if err != nil {
		t.Fatalf("failed building expense proxy: %v", err)
	}
	budgetProxy, err := proxy.New(budgetSrv.URL, log)
	if err != nil {
		t.Fatalf("failed building budget proxy: %v", err)
	}
	notificationProxy, err := proxy.New(notificationSrv.URL, log)
	if err != nil {
		t.Fatalf("failed building notification proxy: %v", err)
	}

	cfg := gwconfig.Config{
		GatewayPort:            "8080",
		UserServiceURL:         userSrv.URL,
		ExpenseServiceURL:      expenseSrv.URL,
		BudgetServiceURL:       budgetSrv.URL,
		NotificationServiceURL: notificationSrv.URL,
		JWTAccessSecret:        testSecret,
		LogLevel:               "error",
		ShutdownTimeout:        5 * time.Second,
		CORSAllowedOrigins:     []string{"http://localhost:3000"},
	}

	engine := router.New(cfg, log, router.Backends{
		User:         userProxy,
		Expense:      expenseProxy,
		Budget:       budgetProxy,
		Notification: notificationProxy,
	}, nil)

	gwSrv := httptest.NewServer(engine)

	cleanup := func() {
		gwSrv.Close()
		userSrv.Close()
		expenseSrv.Close()
		budgetSrv.Close()
		notificationSrv.Close()
	}
	return gwSrv, cleanup
}

func mustAccessToken(t *testing.T, userID string) string {
	t.Helper()
	tok, err := jwtx.GenerateAccessToken(testSecret, userID, "user@example.com", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return tok
}

func doRequest(t *testing.T, method, url, bearer string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("failed building request: %v", err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func TestGateway_ProtectedRoute_ReachesExpenseServiceWithUserIDAndPath(t *testing.T) {
	var user, expense, budget, notification echoBackend
	gwSrv, cleanup := buildGatewayTestServer(t, &user, &expense, &budget, &notification)
	defer cleanup()

	token := mustAccessToken(t, "user-42")

	req, err := http.NewRequest(http.MethodGet, gwSrv.URL+"/api/v1/accounts/abc123?page=2", nil)
	if err != nil {
		t.Fatalf("failed building request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(middleware.UserIDHeader, "attacker-supplied-id") // must be overwritten
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if expense.lastPath != "/api/v1/accounts/abc123" {
		t.Fatalf("expected backend to see unchanged path, got %q", expense.lastPath)
	}
	if expense.lastRawQuery != "page=2" {
		t.Fatalf("expected query string to be preserved, got %q", expense.lastRawQuery)
	}
	if expense.lastUserID != "user-42" {
		t.Fatalf("expected X-User-Id=user-42 (overwritten from validated token), got %q", expense.lastUserID)
	}
	if expense.lastRequestID == "" {
		t.Fatal("expected X-Request-Id to be propagated to the backend")
	}
	if user.lastPath != "" {
		t.Fatal("request to /accounts must not reach user-service")
	}
}

func TestGateway_ProtectedRoute_MissingTokenNeverReachesBackend(t *testing.T) {
	var user, expense, budget, notification echoBackend
	gwSrv, cleanup := buildGatewayTestServer(t, &user, &expense, &budget, &notification)
	defer cleanup()

	resp := doRequest(t, http.MethodGet, gwSrv.URL+"/api/v1/accounts", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if expense.lastPath != "" {
		t.Fatal("expense-service must never be reached without a valid token")
	}
}

func TestGateway_PublicRoute_LoginReachesUserServiceWithoutToken(t *testing.T) {
	var user, expense, budget, notification echoBackend
	gwSrv, cleanup := buildGatewayTestServer(t, &user, &expense, &budget, &notification)
	defer cleanup()

	resp := doRequest(t, http.MethodPost, gwSrv.URL+"/api/v1/auth/login", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if user.lastPath != "/api/v1/auth/login" {
		t.Fatalf("expected user-service to receive /api/v1/auth/login, got %q", user.lastPath)
	}
}

func TestGateway_ProtectedRoute_LogoutRequiresTokenEvenThoughUnderAuthPrefix(t *testing.T) {
	var user, expense, budget, notification echoBackend
	gwSrv, cleanup := buildGatewayTestServer(t, &user, &expense, &budget, &notification)
	defer cleanup()

	resp := doRequest(t, http.MethodPost, gwSrv.URL+"/api/v1/auth/logout", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for logout without token, got %d", resp.StatusCode)
	}

	token := mustAccessToken(t, "user-7")
	resp2 := doRequest(t, http.MethodPost, gwSrv.URL+"/api/v1/auth/logout", token)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for logout with valid token, got %d", resp2.StatusCode)
	}
	if user.lastUserID != "user-7" {
		t.Fatalf("expected X-User-Id=user-7 on logout, got %q", user.lastUserID)
	}
}

func TestGateway_HealthEndpointsArePublic(t *testing.T) {
	var user, expense, budget, notification echoBackend
	gwSrv, cleanup := buildGatewayTestServer(t, &user, &expense, &budget, &notification)
	defer cleanup()

	for _, path := range []string{"/live", "/ready", "/health"} {
		resp := doRequest(t, http.MethodGet, gwSrv.URL+path, "")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, resp.StatusCode)
		}
	}
}

// TestGateway_RateLimitAppliesThroughRealConfig proves the full wiring —
// gwconfig.Config's RateLimitRequestsPerSecond/RateLimitBurst actually
// reaching shared/middleware.RateLimit through router.New — not just the
// middleware in isolation (already covered by
// shared/middleware/ratelimit_test.go). buildGatewayTestServer's shared cfg
// leaves these at their zero value (rate limiting disabled, per
// RateLimit's fail-open guard) so every other test in this file is
// unaffected; this test builds its own minimal router with a real, tiny
// limit instead.
func TestGateway_RateLimitAppliesThroughRealConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("gateway-test", "error")

	userSrv := newEchoServer(&echoBackend{})
	defer userSrv.Close()
	userProxy, err := proxy.New(userSrv.URL, log)
	if err != nil {
		t.Fatalf("failed building user proxy: %v", err)
	}

	cfg := gwconfig.Config{
		GatewayPort:                "8080",
		UserServiceURL:             userSrv.URL,
		ExpenseServiceURL:          userSrv.URL,
		BudgetServiceURL:           userSrv.URL,
		NotificationServiceURL:     userSrv.URL,
		JWTAccessSecret:            testSecret,
		LogLevel:                   "error",
		ShutdownTimeout:            5 * time.Second,
		CORSAllowedOrigins:         []string{"http://localhost:3000"},
		RateLimitRequestsPerSecond: 1,
		RateLimitBurst:             1,
	}

	engine := router.New(cfg, log, router.Backends{
		User:         userProxy,
		Expense:      userProxy,
		Budget:       userProxy,
		Notification: userProxy,
	}, nil)
	gwSrv := httptest.NewServer(engine)
	defer gwSrv.Close()

	first := doRequest(t, http.MethodGet, gwSrv.URL+"/live", "")
	first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first request: expected 200 (within burst of 1), got %d", first.StatusCode)
	}

	second := doRequest(t, http.MethodGet, gwSrv.URL+"/live", "")
	defer second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429 (burst of 1 exhausted), got %d", second.StatusCode)
	}
	body, _ := io.ReadAll(second.Body)
	if !strings.Contains(string(body), `"RATE_LIMITED"`) {
		t.Errorf("429 response body doesn't carry the RATE_LIMITED code: %s", body)
	}
}

// TestGateway_BodyLimitAppliesThroughRealConfig proves the full wiring —
// gwconfig.Config's MaxRequestBodyBytes actually reaching
// shared/middleware.BodyLimit through router.New — not just the middleware
// in isolation (already covered by shared/middleware/bodylimit_test.go).
// buildGatewayTestServer's shared cfg leaves this at its zero value (body
// limit disabled, per BodyLimit's fail-open guard) so every other test in
// this file is unaffected; this test builds its own minimal router with a
// real, tiny limit instead, and posts an oversized body to the public
// (unauthenticated) register route — exactly the attack surface this fix
// targets.
func TestGateway_BodyLimitAppliesThroughRealConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := logger.New("gateway-test", "error")

	userSrv := newEchoServer(&echoBackend{})
	defer userSrv.Close()
	userProxy, err := proxy.New(userSrv.URL, log)
	if err != nil {
		t.Fatalf("failed building user proxy: %v", err)
	}

	cfg := gwconfig.Config{
		GatewayPort:            "8080",
		UserServiceURL:         userSrv.URL,
		ExpenseServiceURL:      userSrv.URL,
		BudgetServiceURL:       userSrv.URL,
		NotificationServiceURL: userSrv.URL,
		JWTAccessSecret:        testSecret,
		LogLevel:               "error",
		ShutdownTimeout:        5 * time.Second,
		CORSAllowedOrigins:     []string{"http://localhost:3000"},
		MaxRequestBodyBytes:    64,
	}

	engine := router.New(cfg, log, router.Backends{
		User:         userProxy,
		Expense:      userProxy,
		Budget:       userProxy,
		Notification: userProxy,
	}, nil)
	gwSrv := httptest.NewServer(engine)
	defer gwSrv.Close()

	oversized := strings.NewReader(strings.Repeat("a", 10_000))
	req, err := http.NewRequest(http.MethodPost, gwSrv.URL+"/api/v1/auth/register", oversized)
	if err != nil {
		t.Fatalf("failed building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// Asserts the exact envelope, not just "not 200" — proxy.go's
	// ErrorHandler special-cases *http.MaxBytesError to report this
	// precisely so an oversized body isn't misreported as a backend outage
	// (502 INTERNAL_ERROR). A prior version of this test only checked
	// StatusCode != 200, which passed even while the gateway was reporting
	// the wrong error code entirely.
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", resp.StatusCode, respBody)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatalf("failed to decode response body: %v (body: %s)", err, respBody)
	}
	if env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected error code VALIDATION_ERROR, got %q", env.Error.Code)
	}
}
