// Package domain holds the plain data model and repository/service interfaces
// for notification-service. It intentionally has zero dependency on any web
// framework or database driver so it stays trivially unit-testable and so the
// service layer can depend on these interfaces instead of concrete infra.
package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by repository implementations when a notification
// does not exist, or exists but is not owned by the requesting user (the two
// cases are deliberately indistinguishable to callers, matching the
// cross-user-access-returns-404 convention used across Finora services).
var ErrNotFound = errors.New("notification not found")

// Notification is a single in-app notification owned by exactly one user.
type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationRepository persists and queries notifications. Every method is
// scoped by userID so ownership can never accidentally be bypassed by a
// service-layer bug — the repository itself enforces the filter.
type NotificationRepository interface {
	Create(ctx context.Context, n *Notification) error
	ListByUser(ctx context.Context, userID string, unreadOnly bool) ([]Notification, error)
	MarkRead(ctx context.Context, userID, id string) (*Notification, error)
}

// NotificationService is the business-logic surface consumed by handlers.
type NotificationService interface {
	Create(ctx context.Context, userID, title, message, notifType string) (*Notification, error)
	List(ctx context.Context, userID string, unreadOnly bool) ([]Notification, error)
	MarkRead(ctx context.Context, userID, id string) (*Notification, error)
}

// EmailSender is invoked by notificationService.Create (see
// internal/service/notification_service.go) as a best-effort side effect
// after every successful notification creation — a Send error is logged and
// never fails Create's own result. Today the only implementation is
// LoggingEmailSender, which sends nothing and just logs what would have
// been sent, so this genuinely satisfies the roadmap's "structured log"
// alternative to a real SMTP sender (see
// architecture/development-roadmap.md Phase 4) without requiring a real
// mail provider yet. A real SMTP-backed implementation can be dropped in
// later without touching the service layer's constructor signature.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}
