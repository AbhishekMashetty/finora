package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/finora/budget-service/internal/domain"
)

// reportService implements domain.ReportService: a real, computed
// budget-vs-actual report built from this service's own budgets plus a
// cross-service REST call to expense-service (via domain.ExpenseClient) for
// actual spend per category. Depends only on domain interfaces, never a
// concrete repository or HTTP client type, so it's unit-testable with fakes.
//
// Since Phase 4, Summary also triggers a best-effort overspend notification
// (via domain.NotificationClient) the first time a budget is found over
// spent for the requested report period — see the doc comment above the
// notifyIfOverspent helper for the full dedup rule and the trade-off this
// represents (a read endpoint with a side effect, in lieu of a real event
// bus that doesn't exist until Phase 7).
type reportService struct {
	budgetRepo    domain.BudgetRepository
	expenseClient domain.ExpenseClient
	notifier      domain.NotificationClient
	log           *slog.Logger
}

// NewReportService builds a ReportService backed by budgetRepo (for the
// caller's budgets), expenseClient (for actual spend per category), and
// notifier (for the Phase 4 overspend-notification trigger). log is used
// only to record a notify failure — Notify errors never fail Summary's own
// result, since the report itself computed successfully regardless (see
// notifyIfOverspent).
func NewReportService(budgetRepo domain.BudgetRepository, expenseClient domain.ExpenseClient, notifier domain.NotificationClient, log *slog.Logger) domain.ReportService {
	return &reportService{budgetRepo: budgetRepo, expenseClient: expenseClient, notifier: notifier, log: log}
}

func (s *reportService) Summary(ctx context.Context, userID string, from, to time.Time) (*domain.ReportSummary, error) {
	budgets, err := s.budgetRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary := &domain.ReportSummary{
		From:       from.Format(time.RFC3339),
		To:         to.Format(time.RFC3339),
		Categories: make([]domain.CategorySummary, 0, len(budgets)),
	}

	for _, b := range budgets {
		// A budget whose category name has no match in expense-service's
		// categories returns actual=0, not an error — the user simply
		// hasn't logged anything under that name yet (see
		// domain.ExpenseClient's doc comment).
		actual, err := s.expenseClient.SumExpensesByCategory(ctx, userID, b.Category, from, to)
		if err != nil {
			return nil, err
		}
		remaining := b.Amount - actual

		summary.Categories = append(summary.Categories, domain.CategorySummary{
			Category:  b.Category,
			Period:    b.Period,
			Budgeted:  b.Amount,
			Actual:    actual,
			Remaining: remaining,
		})
		summary.TotalBudgeted += b.Amount
		summary.TotalActual += actual

		if remaining < 0 {
			s.notifyIfOverspent(ctx, userID, b, actual, from)
		}
	}

	return summary, nil
}

// notifyIfOverspent implements the Phase 4 overspend-notification trigger
// and its dedup rule: this is a deliberate, documented trade-off (a read
// endpoint — GET /reports/summary — having a side effect), accepted only
// because Finora has no event bus yet (Phase 7 replaces this with a real
// domain event). See architecture/api-contracts.md's
// budget-service -> notification-service subsection.
//
// Dedup rule: a budget that's over-spent notifies once per distinct
// reporting period. "Already notified for this period" means
// b.LastNotifiedAt != nil && b.LastNotifiedAt.Equal(from) — i.e.
// LastNotifiedAt stores the *period's `from` boundary* that was last
// notified for, not the wall-clock time the notification fired. Comparing
// by equality-with-the-period (rather than "was notified at-or-after some
// timestamp") is deliberate: an earlier draft compared LastNotifiedAt
// against `from` as a time ordering, which broke for out-of-order queries —
// e.g. a user notified today for the current month, who then loads an
// *older*, never-before-viewed over-budget month, would have that
// legitimate first notification wrongly suppressed, since "today" is after
// that older month's `from`. Keying on exact-period-match instead means
// every distinct [from] a report is ever run for gets its own independent
// notify-once behavior, regardless of the order periods are viewed in.
//
// A Notify failure (e.g. notification-service unreachable) is logged and
// swallowed, never propagated to the caller — the report itself already
// computed correctly, and a transient notification-delivery failure
// shouldn't turn a working read endpoint into a 500.
//
// Known limitation (caught in review, not fixed here): this is a
// read-then-write with no atomic check-and-set. Two concurrent Summary
// calls for the same never-before-notified period could both observe
// LastNotifiedAt == nil and both notify, producing a duplicate. Low
// consequence (an extra notification, not data loss or a security issue)
// and narrow (requires a genuine race on the very first notify for a given
// period) — not worth a locking/optimistic-concurrency mechanism for a
// side-effecting-GET workaround that Phase 7's real event bus replaces
// entirely. Revisit only if it's ever observed in practice.
func (s *reportService) notifyIfOverspent(ctx context.Context, userID string, b domain.Budget, actual float64, from time.Time) {
	if b.LastNotifiedAt != nil && b.LastNotifiedAt.Equal(from) {
		return // already notified for this exact reporting period
	}

	title := "Budget exceeded"
	message := fmt.Sprintf("You've spent %.2f of your %.2f %s %s budget.", actual, b.Amount, b.Period, b.Category)

	if err := s.notifier.Notify(ctx, userID, title, message); err != nil {
		s.log.Error("failed to send overspend notification",
			slog.String("user_id", userID),
			slog.String("budget_id", b.ID),
			slog.String("category", b.Category),
			slog.String("error", err.Error()),
		)
		return
	}

	periodStart := from
	b.LastNotifiedAt = &periodStart
	if err := s.budgetRepo.Update(ctx, &b); err != nil {
		s.log.Error("failed to persist last_notified_at after overspend notification",
			slog.String("user_id", userID),
			slog.String("budget_id", b.ID),
			slog.String("error", err.Error()),
		)
	}
}
