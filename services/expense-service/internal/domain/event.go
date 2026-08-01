package domain

import "context"

// EventsStreamName is the shared JetStream stream every Finora event
// (produced or consumed by any service) lives on — one stream for the
// whole fleet's small event set (two subjects total as of Phase 7), not
// one stream per event type, since operating N near-empty streams for N
// event types isn't worth it at this scale. Every service that touches
// this stream (producer or consumer) calls EnsureStream with this exact
// name at its own startup; CreateOrUpdateStream is idempotent, so which
// service happens to "provision" it first doesn't matter.
//
// This name and TransactionCreatedSubject below are a cross-service
// contract — see architecture/api-contracts.md's Async Events section for
// the authoritative definition budget-service's own domain package (a
// separate Go module) independently defines matching literals against,
// the same way this fleet already agrees on the X-User-Id header contract
// without a shared Go constant across service boundaries (CLAUDE.md §10:
// every service is independently buildable, so constants can't be
// literally shared, only documented and kept in sync).
const EventsStreamName = "FINORA_EVENTS"

// TransactionCreatedSubject is the NATS subject expense-service publishes
// to whenever a transaction is created (via POST /transactions or the CSV
// importer) — Phase 7's replacement for the synchronous REST call
// budget-service used to rely on to learn "something changed."
const TransactionCreatedSubject = "finora.transaction.created"

// TransactionCreatedEvent is the payload published on
// TransactionCreatedSubject. Kept intentionally close to Transaction
// itself (not a trimmed-down projection) since the one current consumer
// (budget-service, recomputing a category's overspend status) needs
// UserID/AccountID/CategoryID/Type/Amount/Date — there's no projection
// worth maintaining separately yet for a single consumer.
type TransactionCreatedEvent struct {
	TransactionID string          `json:"transaction_id"`
	UserID        string          `json:"user_id"`
	AccountID     string          `json:"account_id"`
	CategoryID    *string         `json:"category_id,omitempty"`
	Type          TransactionType `json:"type"`
	Amount        float64         `json:"amount"`
	Currency      string          `json:"currency"`
	Date          string          `json:"date"` // RFC3339, matching every other date this API already serializes as a string
}

// EventPublisher publishes expense-service's domain events. Kept as a
// domain-level interface — same Dependency Inversion pattern as
// AccountRepository/CategoryRepository — so transactionService stays
// testable against a fake and ignorant of the concrete
// outbox/Mongo/NATS mechanics behind it (internal/events.OutboxPublisher
// is the real implementation, wired in cmd/server/main.go).
type EventPublisher interface {
	PublishTransactionCreated(ctx context.Context, tx *Transaction) error
}
