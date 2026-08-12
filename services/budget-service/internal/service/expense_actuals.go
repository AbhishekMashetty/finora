package service

import (
	"strings"

	"github.com/finora/budget-service/internal/domain"
)

// actualsByLowerCategory indexes a slice of domain.ExpenseSummary by a
// lower-cased category name, so a budget's own Category field can be
// looked up against it case-insensitively (also lower-cased at the call
// site) — the same matching behavior the pre-F01 per-category lookup gave
// via strings.EqualFold. A budget's category name and expense-service's
// category name are independently user-typed strings that happen to
// usually match in casing, not guaranteed to.
func actualsByLowerCategory(actuals []domain.ExpenseSummary) map[string]float64 {
	m := make(map[string]float64, len(actuals))
	for _, a := range actuals {
		m[strings.ToLower(a.Category)] = a.Actual
	}
	return m
}
