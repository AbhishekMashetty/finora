// Package client holds outbound adapters to other Finora services. Today
// that's just expense-service, called synchronously over REST for the
// reports feature (architecture/development-roadmap.md Phase 3) — the only
// sanctioned service-to-service call in the fleet today, per CLAUDE.md §3.
// This package implements domain.ExpenseClient; internal/service depends
// only on that interface, never on this concrete type (Dependency
// Inversion), so report_service stays unit-testable with a fake.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/shared/httpclient"
	"github.com/finora/shared/middleware"
)

// requestTimeout bounds each individual outbound call to expense-service.
// This is an internal, same-datacenter call (docker/K8s network), so a
// couple of seconds is generous, not tight.
const requestTimeout = 3 * time.Second

// ExpenseHTTPClient implements domain.ExpenseClient against expense-service's
// real HTTP API, reusing the gateway's trusted X-User-Id header contract
// (shared/middleware.RequireIdentity) rather than any token — this is an
// internal-network call, not a client-facing one.
type ExpenseHTTPClient struct {
	baseURL string
	http    *http.Client
}

// NewExpenseHTTPClient builds a client that calls expense-service at baseURL
// (e.g. "http://expense-service:8082", from EXPENSE_SERVICE_URL).
func NewExpenseHTTPClient(baseURL string) *ExpenseHTTPClient {
	return &ExpenseHTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		// &http.Client{} alone inherits http.DefaultTransport's
		// MaxIdleConnsPerHost: 2 — see shared/httpclient's doc comment for
		// what that default does under sustained load. This is the busiest
		// internal-network client in the fleet outside the gateway itself.
		// Each individual call still gets its own bounded deadline via
		// doGet's context.WithTimeout(ctx, requestTimeout) — this only
		// fixes connection reuse, not per-call timeouts.
		http: &http.Client{Transport: httpclient.NewTransport()},
	}
}

// envelope mirrors shared/httpx.Envelope's wire shape, just enough of it to
// unwrap expense-service's responses.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type categoryDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// categoryTotalDTO mirrors one entry of expense-service's GET
// /transactions/aggregate response — see that endpoint's openapi.yaml
// entry for the wire shape.
type categoryTotalDTO struct {
	CategoryID string  `json:"category_id"`
	Total      float64 `json:"total"`
	Count      int64   `json:"count"`
}

// doGet performs an authenticated (X-User-Id) GET against expense-service and
// returns the decoded envelope, bounded by requestTimeout.
func (c *ExpenseHTTPClient) doGet(ctx context.Context, userID, path string, query url.Values) (*envelope, error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building expense-service request: %w", err)
	}
	req.Header.Set(middleware.UserIDHeader, userID)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling expense-service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading expense-service response: %w", err)
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decoding expense-service response (status %d): %w", resp.StatusCode, err)
	}
	if !env.Success {
		msg := "unknown error"
		if env.Error != nil {
			msg = env.Error.Message
		}
		return nil, fmt.Errorf("expense-service returned an error (status %d): %s", resp.StatusCode, msg)
	}
	return &env, nil
}

// categoriesByID fetches the caller's expense-service categories and
// returns a map from category id to category name. Needed because the
// aggregation endpoint groups by category_id (what transactions actually
// store), while budgets identify their category by name — this is what
// lets SumExpensesByCategory translate one to the other.
func (c *ExpenseHTTPClient) categoriesByID(ctx context.Context, userID string) (map[string]string, error) {
	env, err := c.doGet(ctx, userID, "/api/v1/categories", nil)
	if err != nil {
		return nil, err
	}

	var body struct {
		Categories []categoryDTO `json:"categories"`
	}
	if err := json.Unmarshal(env.Data, &body); err != nil {
		return nil, fmt.Errorf("decoding categories response: %w", err)
	}

	names := make(map[string]string, len(body.Categories))
	for _, cat := range body.Categories {
		names[cat.ID] = cat.Name
	}
	return names, nil
}

// SumExpensesByCategory implements domain.ExpenseClient. See the
// interface's doc comment for the full contract.
//
// Exactly two calls, regardless of how many budgets or categories the
// caller has: one to resolve category id -> name (categoriesByID), one to
// expense-service's GET /transactions/aggregate for every category's
// total in [from, to]. This replaces what used to be a category-name-to-id
// lookup PLUS a full paginated transaction walk, per call, per budget (see
// infrastructure/scale-readiness-review.html finding F01) — B budgets used
// to mean B*(1+P) internal HTTP requests; this is 2, independent of B.
func (c *ExpenseHTTPClient) SumExpensesByCategory(ctx context.Context, userID string, from, to time.Time) ([]domain.ExpenseSummary, error) {
	names, err := c.categoriesByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("from", from.Format(time.RFC3339))
	query.Set("to", to.Format(time.RFC3339))
	query.Set("type", "expense")

	env, err := c.doGet(ctx, userID, "/api/v1/transactions/aggregate", query)
	if err != nil {
		return nil, err
	}

	var body struct {
		Categories []categoryTotalDTO `json:"categories"`
	}
	if err := json.Unmarshal(env.Data, &body); err != nil {
		return nil, fmt.Errorf("decoding aggregate response: %w", err)
	}

	summaries := make([]domain.ExpenseSummary, 0, len(body.Categories))
	for _, ct := range body.Categories {
		name, ok := names[ct.CategoryID]
		if !ok {
			// A category the aggregation grouped by no longer exists in the
			// caller's category list (e.g. deleted after a transaction that
			// referenced it was created) — nothing meaningful to match it
			// against, so skip it rather than surfacing an unnamed entry no
			// budget could ever match.
			continue
		}
		summaries = append(summaries, domain.ExpenseSummary{Category: name, Actual: ct.Total})
	}
	return summaries, nil
}
