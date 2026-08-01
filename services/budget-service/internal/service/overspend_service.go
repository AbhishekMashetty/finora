package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/finora/budget-service/internal/domain"
)

// OverspendService recomputes overspend status for a user's budgets when
// triggered by a finora.transaction.created event (Phase 7), replacing the
// old read-triggered check that used to live inside reportService.Summary.
//
// Design note — why this checks only the CURRENT period, not an arbitrary
// one: the old Summary-embedded trigger evaluated whatever [from, to] range
// the caller happened to be viewing a report for, since a human was
// actively choosing that range. An event has no such caller-supplied range
// — there's no "the user is looking at March" to key off of. So
// HandleTransactionCreated always evaluates each budget's CURRENT period
// (derived from its Period type and the current wall-clock time via
// currentPeriodStart), on the reasoning that a just-created transaction
// can only meaningfully move the needle on money "right now" — a backdated
// transaction still lands in the current period's actual-spend sum
// (SumExpensesByCategory sums by the transaction's own date, not when it
// was entered) and is correctly picked up, but nothing here re-evaluates a
// period that has already closed. GET /reports/summary?from=&to= still
// exists unchanged for on-demand historical viewing; it just no longer
// carries this side effect.
type OverspendService struct {
	budgetRepo    domain.BudgetRepository
	expenseClient domain.ExpenseClient
	publisher     domain.EventPublisher
	log           *slog.Logger
}

// NewOverspendService builds an OverspendService.
func NewOverspendService(budgetRepo domain.BudgetRepository, expenseClient domain.ExpenseClient, publisher domain.EventPublisher, log *slog.Logger) *OverspendService {
	return &OverspendService{budgetRepo: budgetRepo, expenseClient: expenseClient, publisher: publisher, log: log}
}

// HandleTransactionCreated is called for every finora.transaction.created
// event this service consumes (see cmd/server/main.go's NATS subscription
// wiring) — userID identifies whose budgets to recheck; the event's other
// fields aren't needed here since recomputation always goes through the
// existing ExpenseClient REST call for a fresh, authoritative sum, not by
// incrementally applying the new transaction's own amount. now is passed
// in (rather than read via time.Now() here) purely so tests can control
// which period gets evaluated deterministically.
//
// A single budget's expense-client failure is logged and does NOT abort
// checking the user's other budgets — one flaky lookup shouldn't prevent
// every other budget from being correctly evaluated.
func (s *OverspendService) HandleTransactionCreated(ctx context.Context, userID string, now time.Time) error {
	budgets, err := s.budgetRepo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, b := range budgets {
		from := currentPeriodStart(b.Period, now)
		actual, err := s.expenseClient.SumExpensesByCategory(ctx, userID, b.Category, from, now)
		if err != nil {
			s.log.Error("failed to sum expenses while checking for overspend",
				slog.String("user_id", userID),
				slog.String("budget_id", b.ID),
				slog.String("category", b.Category),
				slog.String("error", err.Error()),
			)
			continue
		}

		if b.Amount-actual < 0 {
			s.notifyIfOverspent(ctx, userID, b, actual, from)
		}
	}
	return nil
}

// notifyIfOverspent is the same dedup rule report_service.go's
// notifyIfOverspent used before Phase 7, now publishing an event instead
// of making a synchronous REST call: a budget that's over-spent notifies
// once per distinct period — "already notified for this period" means
// b.LastNotifiedAt != nil && b.LastNotifiedAt.Equal(from), i.e.
// LastNotifiedAt stores the period's `from` boundary last notified for,
// not a wall-clock timestamp (see the field's own doc comment in
// domain/budget.go for why exact-period-match, not time-ordering, is the
// correct comparison).
//
// A publish failure (e.g. NATS or Mongo briefly unreachable) is logged and
// swallowed, never propagated — recomputing overspend status itself
// succeeded; the same "a side effect failing shouldn't fail the primary
// operation" reasoning as expense-service's transactionService.publishCreated.
//
// Known limitation, carried over unchanged from the pre-Phase-7 version:
// this is a read-then-write with no atomic check-and-set, so two genuinely
// concurrent events for the same never-before-notified period could both
// notify. Low consequence (an extra notification, not data loss) and
// narrow — not worth a locking mechanism here.
func (s *OverspendService) notifyIfOverspent(ctx context.Context, userID string, b domain.Budget, actual float64, from time.Time) {
	if b.LastNotifiedAt != nil && b.LastNotifiedAt.Equal(from) {
		return // already notified for this exact period
	}

	event := domain.BudgetOverspentEvent{
		BudgetID:    b.ID,
		UserID:      userID,
		Category:    b.Category,
		Period:      string(b.Period),
		Budgeted:    b.Amount,
		Actual:      actual,
		PeriodStart: from.Format(time.RFC3339),
	}

	if err := s.publisher.PublishBudgetOverspent(ctx, event); err != nil {
		s.log.Error("failed to publish budget.overspent event",
			slog.String("user_id", userID),
			slog.String("budget_id", b.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	periodStart := from
	b.LastNotifiedAt = &periodStart
	if err := s.budgetRepo.Update(ctx, &b); err != nil {
		s.log.Error("failed to persist last_notified_at after publishing budget.overspent",
			slog.String("user_id", userID),
			slog.String("budget_id", b.ID),
			slog.String("error", err.Error()),
		)
	}
}

// currentPeriodStart returns the start of the period containing now, for
// the given cadence — e.g. PeriodMonthly with now = 2026-03-15 returns
// 2026-03-01T00:00:00Z. Weekly starts on Monday (ISO 8601 week start,
// consistent with Go's own time.Monday-anchored Weekday semantics rather
// than picking Sunday arbitrarily).
func currentPeriodStart(period domain.Period, now time.Time) time.Time {
	now = now.UTC()
	switch period {
	case domain.PeriodWeekly:
		offset := (int(now.Weekday()) + 6) % 7 // days since the most recent Monday (Sunday=0 -> 6 days since Monday)
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -offset)
	case domain.PeriodYearly:
		return time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	case domain.PeriodMonthly:
		fallthrough
	default:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
}
