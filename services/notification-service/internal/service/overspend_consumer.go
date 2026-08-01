package service

import (
	"context"
	"fmt"

	"github.com/finora/notification-service/internal/domain"
)

// OverspendConsumer turns a finora.budget.overspent event into a real
// in-app notification via the existing NotificationService.Create — no new
// HTTP handler needed, since this is an internal, event-triggered call
// path, not something a browser hits directly. Phase 7's replacement for
// budget-service calling POST /api/v1/notifications over REST with a
// pre-formatted title/message.
type OverspendConsumer struct {
	notifications domain.NotificationService
}

// NewOverspendConsumer builds an OverspendConsumer backed by
// notifications — the same NotificationService instance
// cmd/server/main.go already wires up for the HTTP handler, reused as-is.
func NewOverspendConsumer(notifications domain.NotificationService) *OverspendConsumer {
	return &OverspendConsumer{notifications: notifications}
}

// HandleBudgetOverspent formats a user-facing title/message from event and
// creates the notification. Formatting the message here (rather than
// receiving it pre-formatted, which is what the old REST call used to do)
// is deliberate: presentation of a domain event is this service's concern,
// not the producer's — budget-service publishes structured facts
// (budgeted/actual/category/period), and how those become human-readable
// text is entirely up to whichever consumer renders them.
func (c *OverspendConsumer) HandleBudgetOverspent(ctx context.Context, event domain.BudgetOverspentEvent) error {
	title := "Budget exceeded"
	message := fmt.Sprintf("You've spent %.2f of your %.2f %s %s budget.", event.Actual, event.Budgeted, event.Period, event.Category)

	_, err := c.notifications.Create(ctx, event.UserID, title, message, "overspend")
	return err
}
