// Package events adapts domain.EventPublisher to shared/outbox — same
// shape as expense-service's own internal/events package, and the same
// "small concrete adapter behind a domain interface" pattern internal/client
// already used for the (now-removed) REST call to notification-service.
package events

import (
	"context"
	"encoding/json"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/shared/outbox"
)

// OutboxPublisher implements domain.EventPublisher by enqueueing onto a
// shared/outbox.Store — budget-service's own outbox_events collection, in
// its own finora_budgets database.
type OutboxPublisher struct {
	store *outbox.Store
}

// NewOutboxPublisher builds an OutboxPublisher backed by store.
func NewOutboxPublisher(store *outbox.Store) *OutboxPublisher {
	return &OutboxPublisher{store: store}
}

var _ domain.EventPublisher = (*OutboxPublisher)(nil)

func (p *OutboxPublisher) PublishBudgetOverspent(ctx context.Context, event domain.BudgetOverspentEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// msgID = budget_id + period_start: the exact same pair
	// notifyIfOverspent's own LastNotifiedAt dedup already keys on, reused
	// here as JetStream's redelivery-safety-net dedup key too — if this
	// exact (budget, period) combination is ever enqueued twice (e.g. two
	// concurrent events landing before LastNotifiedAt is persisted, the
	// known narrow race documented in overspend_service.go), NATS
	// collapses them into one delivered message within its dedup window,
	// rather than notification-service seeing two.
	msgID := event.BudgetID + ":" + event.PeriodStart
	return p.store.Enqueue(ctx, domain.BudgetOverspentSubject, payload, msgID)
}
