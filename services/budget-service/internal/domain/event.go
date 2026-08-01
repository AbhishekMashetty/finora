package domain

import "context"

// EventsStreamName is the shared JetStream stream every Finora event lives
// on — must match expense-service's domain.EventsStreamName exactly (see
// that file's doc comment for why this is a documented cross-service
// string contract, not a shared Go constant across module boundaries).
const EventsStreamName = "FINORA_EVENTS"

// TransactionCreatedSubject is the event budget-service CONSUMES —
// published by expense-service whenever a transaction is created. Must
// match expense-service's domain.TransactionCreatedSubject exactly.
const TransactionCreatedSubject = "finora.transaction.created"

// BudgetOverspentSubject is the event budget-service PRODUCES — Phase 7's
// replacement for the synchronous REST call this service used to make to
// notification-service from inside Summary(). notification-service
// consumes this directly.
const BudgetOverspentSubject = "finora.budget.overspent"

// TransactionCreatedEvent is the payload budget-service deserializes from
// TransactionCreatedSubject. Mirrors expense-service's own
// domain.TransactionCreatedEvent field-for-field (the actual wire
// contract), even though this consumer only reads UserID today — every
// event this service publishes/consumes documents its full real shape, not
// just the fields a given consumer happens to use yet, so a future
// consumer (or a debugging session reading a captured message) never has
// to cross-reference the producer's source to know what's on the wire.
type TransactionCreatedEvent struct {
	TransactionID string          `json:"transaction_id"`
	UserID        string          `json:"user_id"`
	AccountID     string          `json:"account_id"`
	CategoryID    *string         `json:"category_id,omitempty"`
	Type          TransactionType `json:"type"`
	Amount        float64         `json:"amount"`
	Currency      string          `json:"currency"`
	Date          string          `json:"date"`
}

// TransactionType mirrors expense-service's domain.TransactionType for
// deserializing TransactionCreatedEvent — budget-service never validates
// or acts on this field itself (see overspend_service.go: recomputation
// goes through the existing ExpenseClient REST call, not this event's own
// amount), it's carried only because it's part of the real wire payload.
type TransactionType string

// BudgetOverspentEvent is the payload published on BudgetOverspentSubject.
// notification-service builds its notification title/message from this,
// replacing the plain-text title/message the old REST call used to send
// pre-formatted — formatting the user-facing message is presentation
// concern, better owned by the service that actually creates
// notifications.
type BudgetOverspentEvent struct {
	BudgetID    string  `json:"budget_id"`
	UserID      string  `json:"user_id"`
	Category    string  `json:"category"`
	Period      string  `json:"period"`
	Budgeted    float64 `json:"budgeted"`
	Actual      float64 `json:"actual"`
	PeriodStart string  `json:"period_start"` // RFC3339 — also this event's dedup key, alongside BudgetID (see events.OutboxPublisher)
}

// EventPublisher publishes budget-service's domain events. Domain-level
// interface (Dependency Inversion), same pattern as ExpenseClient — kept
// so overspendService stays testable against a fake and ignorant of the
// concrete outbox/Mongo/NATS mechanics behind it (internal/events.OutboxPublisher
// is the real implementation, wired in cmd/server/main.go).
type EventPublisher interface {
	PublishBudgetOverspent(ctx context.Context, event BudgetOverspentEvent) error
}
