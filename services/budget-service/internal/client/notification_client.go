// Package client (see expense_client.go's doc comment) also holds the
// Phase 4 outbound adapter to notification-service: the budget-overspend
// trigger inside report_service.Summary calls this to create a visible
// in-app notification for the user, forwarding the caller's X-User-Id
// exactly like expense_client.go does for expense-service. This is the
// second (and, as of Phase 4, last) sanctioned service-to-service call in
// the fleet — see architecture/api-contracts.md's
// budget-service -> notification-service subsection.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/finora/shared/middleware"
)

// NotificationHTTPClient implements domain.NotificationClient against
// notification-service's real HTTP API.
type NotificationHTTPClient struct {
	baseURL string
	http    *http.Client
}

// NewNotificationHTTPClient builds a client that calls notification-service
// at baseURL (e.g. "http://notification-service:8084", from
// NOTIFICATION_SERVICE_URL).
func NewNotificationHTTPClient(baseURL string) *NotificationHTTPClient {
	return &NotificationHTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
}

type createNotificationBody struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// Notify implements domain.NotificationClient: POSTs
// /api/v1/notifications on notification-service, bounded by requestTimeout
// (the same 3s bound expense_client.go uses for its own internal-network
// calls) and forwarding userID as X-User-Id, the same trust mechanism
// expense_client.go already established for budget-service's outbound
// calls.
func (c *NotificationHTTPClient) Notify(ctx context.Context, userID, title, message string) error {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	payload, err := json.Marshal(createNotificationBody{
		Title:   title,
		Message: message,
		Type:    "overspend",
	})
	if err != nil {
		return fmt.Errorf("encoding notification request: %w", err)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/api/v1/notifications", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building notification-service request: %w", err)
	}
	req.Header.Set(middleware.UserIDHeader, userID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling notification-service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading notification-service response: %w", err)
	}

	// Reuses the same envelope shape expense_client.go decodes
	// (shared/httpx.Envelope's wire shape) rather than a second convention.
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("decoding notification-service response (status %d): %w", resp.StatusCode, err)
	}
	if !env.Success {
		msg := "unknown error"
		if env.Error != nil {
			msg = env.Error.Message
		}
		return fmt.Errorf("notification-service returned an error (status %d): %s", resp.StatusCode, msg)
	}
	return nil
}
