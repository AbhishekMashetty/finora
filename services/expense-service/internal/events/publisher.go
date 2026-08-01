// Package events adapts domain.EventPublisher to shared/outbox — the same
// shape as internal/client's REST adapters (domain.ExpenseClient,
// domain.NotificationClient, elsewhere in this fleet): a small concrete
// type behind a domain interface, so the service layer never imports
// shared/outbox or the Mongo driver directly.
package events

import (
	"context"
	"encoding/json"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/shared/outbox"
)

// OutboxPublisher implements domain.EventPublisher by enqueueing onto a
// shared/outbox.Store — expense-service's own outbox_events collection, in
// its own finora_expenses database (CLAUDE.md §2's "one MongoDB per
// service" rule applies to the outbox collection exactly like every other
// collection).
type OutboxPublisher struct {
	store *outbox.Store
}

// NewOutboxPublisher builds an OutboxPublisher backed by store.
func NewOutboxPublisher(store *outbox.Store) *OutboxPublisher {
	return &OutboxPublisher{store: store}
}

var _ domain.EventPublisher = (*OutboxPublisher)(nil)

func (p *OutboxPublisher) PublishTransactionCreated(ctx context.Context, tx *domain.Transaction) error {
	payload, err := json.Marshal(domain.TransactionCreatedEvent{
		TransactionID: tx.ID,
		UserID:        tx.UserID,
		AccountID:     tx.AccountID,
		CategoryID:    tx.CategoryID,
		Type:          tx.Type,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Date:          tx.Date.Format("2006-01-02T15:04:05Z07:00"), // RFC3339, matching every other date this API serializes as a string
	})
	if err != nil {
		return err
	}
	// tx.ID as the JetStream dedup msgID: if publishCreated's caller ever
	// retried (it doesn't today, but this is what makes that safe were it
	// to happen), the same transaction can never produce two delivered
	// events — one transaction, one event, by construction.
	return p.store.Enqueue(ctx, domain.TransactionCreatedSubject, payload, tx.ID)
}
