package domain

// EventsStreamName is the shared JetStream stream every Finora event lives
// on — must match expense-service's and budget-service's own
// domain.EventsStreamName exactly (see expense-service's copy of this
// constant for why this is a documented cross-service string contract, not
// a shared Go constant across module boundaries).
const EventsStreamName = "FINORA_EVENTS"

// BudgetOverspentSubject is the event notification-service CONSUMES —
// published by budget-service whenever a budget is newly found over spent
// (Phase 7's replacement for the synchronous REST call budget-service used
// to make directly to POST /api/v1/notifications). Must match
// budget-service's domain.BudgetOverspentSubject exactly.
const BudgetOverspentSubject = "finora.budget.overspent"

// BudgetOverspentEvent is the payload notification-service deserializes
// from BudgetOverspentSubject — mirrors budget-service's own
// domain.BudgetOverspentEvent field-for-field (the actual wire contract).
type BudgetOverspentEvent struct {
	BudgetID    string  `json:"budget_id"`
	UserID      string  `json:"user_id"`
	Category    string  `json:"category"`
	Period      string  `json:"period"`
	Budgeted    float64 `json:"budgeted"`
	Actual      float64 `json:"actual"`
	PeriodStart string  `json:"period_start"`
}
