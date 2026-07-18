// Package service implements notification-service's business logic. It
// depends only on domain interfaces (never a concrete Mongo type), which is
// what lets it be unit-tested with an in-memory fake repository.
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/finora/notification-service/internal/domain"
)

// notificationService is the concrete domain.NotificationService.
type notificationService struct {
	repo  domain.NotificationRepository
	email domain.EmailSender
	log   *slog.Logger
}

// NewNotificationService builds the service backed by repo and email. Since
// Phase 4, Create invokes email.Send as a best-effort side effect after a
// successful repo.Create (see Create's doc comment for the "to" argument's
// deliberate simplification and the error-handling rule).
func NewNotificationService(repo domain.NotificationRepository, email domain.EmailSender, log *slog.Logger) domain.NotificationService {
	return &notificationService{repo: repo, email: email, log: log}
}

// Create builds and persists a new notification owned by userID, then makes
// a best-effort attempt to email it too.
//
// The "to" argument passed to email.Send is userID itself, not a real
// resolved email address. This is a deliberate simplification: resolving
// userID to a real address would require a new notification-service ->
// user-service REST call (a new cross-service edge, new config, a new
// architecture doc section) purely to feed a log line that
// LoggingEmailSender.Send discards without ever reading it — real
// complexity for zero behavior change. Revisit only once a real SMTP sender
// lands and the recipient address starts actually mattering.
//
// email.Send's error (were LoggingEmailSender ever to return one, which it
// doesn't today) is logged and swallowed, never propagated: email is
// best-effort, and a delivery failure must never fail notification
// creation, which already succeeded via repo.Create.
func (s *notificationService) Create(ctx context.Context, userID, title, message, notifType string) (*domain.Notification, error) {
	n := &domain.Notification{
		UserID:    userID,
		Title:     title,
		Message:   message,
		Type:      notifType,
		Read:      false,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}

	if err := s.email.Send(ctx, userID, title, message); err != nil {
		s.log.Error("failed to send notification email (best-effort, not failing Create)",
			slog.String("user_id", userID),
			slog.String("notification_id", n.ID),
			slog.String("error", err.Error()),
		)
	}

	return n, nil
}

// List returns userID's notifications, optionally filtered to unread only.
func (s *notificationService) List(ctx context.Context, userID string, unreadOnly bool) ([]domain.Notification, error) {
	return s.repo.ListByUser(ctx, userID, unreadOnly)
}

// MarkRead marks the given notification as read, scoped to userID.
func (s *notificationService) MarkRead(ctx context.Context, userID, id string) (*domain.Notification, error) {
	return s.repo.MarkRead(ctx, userID, id)
}
