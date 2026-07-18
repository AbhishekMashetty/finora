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
	"strconv"
	"strings"
	"time"

	"github.com/finora/shared/middleware"
)

const (
	// reportPageSize is the page size requested per call to expense-service's
	// transaction list endpoint (its own max is 100, see
	// expense-service/internal/service/transaction_service.go).
	reportPageSize = 100

	// maxReportPages bounds how many pages SumExpensesByCategory will walk
	// for a single budget's actual-spend computation (100 * 50 = 5,000
	// transactions), so a pathological account can never make a report
	// request loop unboundedly.
	maxReportPages = 50

	// requestTimeout bounds each individual outbound call to expense-service.
	// This is an internal, same-datacenter call (docker/K8s network), so a
	// couple of seconds is generous, not tight.
	requestTimeout = 3 * time.Second
)

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
		http:    &http.Client{},
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

type transactionDTO struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
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

// findCategoryIDByName looks up the caller's expense-service categories and
// returns the id of the one whose name matches categoryName case-
// insensitively. found is false (with a nil error) if there's no match —
// that's a normal, non-error outcome (see domain.ExpenseClient).
func (c *ExpenseHTTPClient) findCategoryIDByName(ctx context.Context, userID, categoryName string) (id string, found bool, err error) {
	env, err := c.doGet(ctx, userID, "/api/v1/categories", nil)
	if err != nil {
		return "", false, err
	}

	var body struct {
		Categories []categoryDTO `json:"categories"`
	}
	if err := json.Unmarshal(env.Data, &body); err != nil {
		return "", false, fmt.Errorf("decoding categories response: %w", err)
	}

	for _, cat := range body.Categories {
		if strings.EqualFold(cat.Name, categoryName) {
			return cat.ID, true, nil
		}
	}
	return "", false, nil
}

// SumExpensesByCategory implements domain.ExpenseClient. See the interface
// doc comment for the category-not-found contract.
//
// expense-service's transaction list filters ?category=<id> by category_id,
// not by name (a known pre-existing wrinkle — see
// expense-service/internal/repository/mongo_transaction.go's
// buildTransactionFilter), which is why a category-name-to-id lookup happens
// first. Its ListTransactionsInput/TransactionFilter also has no server-side
// "type" filter, so income/expense filtering is done client-side here on
// each page's results instead of trusting the ?type= query param.
func (c *ExpenseHTTPClient) SumExpensesByCategory(ctx context.Context, userID, categoryName string, from, to time.Time) (float64, error) {
	categoryID, found, err := c.findCategoryIDByName(ctx, userID, categoryName)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}

	var total float64
	for page := 1; page <= maxReportPages; page++ {
		query := url.Values{}
		query.Set("category", categoryID)
		query.Set("from", from.Format(time.RFC3339))
		query.Set("to", to.Format(time.RFC3339))
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(reportPageSize))

		env, err := c.doGet(ctx, userID, "/api/v1/transactions", query)
		if err != nil {
			return 0, err
		}

		var body struct {
			Transactions []transactionDTO `json:"transactions"`
		}
		if err := json.Unmarshal(env.Data, &body); err != nil {
			return 0, fmt.Errorf("decoding transactions response: %w", err)
		}

		for _, tx := range body.Transactions {
			if tx.Type == "expense" {
				total += tx.Amount
			}
		}

		if len(body.Transactions) < reportPageSize {
			break // last page
		}
	}

	return total, nil
}
