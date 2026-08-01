package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/finora/expense-service/internal/domain"
)

const (
	// maxImportErrors caps the per-row error list in the response — the
	// Skipped count still reflects the true total even past this cap.
	maxImportErrors = 20

	// maxImportRows is a defense-in-depth ceiling on a single import
	// request: every row becomes its own Mongo insert (no bulk-write path
	// exists here, matching the rest of this service's "no clever code"
	// preference for simple, explicit per-row calls over a batch API this
	// app's scale doesn't need), so an unbounded file would mean an
	// unbounded amount of sequential work inside one HTTP request.
	maxImportRows = 5000

	// maxImportNoteLength mirrors transactionRequest.Note's own binding cap
	// (transaction_handler.go) — imported notes are truncated, not rejected,
	// since a long merchant description in a real statement isn't a reason
	// to drop an otherwise-valid transaction.
	maxImportNoteLength = 1000
)

// importDateLayouts are the date formats accepted in a CSV cell, tried in
// order. RFC3339/YYYY-MM-DD match what the rest of this API already accepts
// (transaction_handler.go's parseDate); MM/DD/YYYY and M/D/YYYY are added
// because they're what most US credit card issuers actually export.
var importDateLayouts = []string{
	time.RFC3339,
	"2006-01-02",
	"01/02/2006",
	"1/2/2006",
}

// Import validates accountID once up front (ownership + its currency, which
// every imported transaction inherits — CSV statements don't carry a
// per-row currency column), then processes rows independently: a bad row is
// skipped and recorded, never aborts the rest of the file.
func (s *transactionService) Import(ctx context.Context, userID, accountID string, rows []domain.ImportRow) (domain.ImportResult, error) {
	if strings.TrimSpace(accountID) == "" {
		return domain.ImportResult{}, NewValidationError("account_id", "account_id is required")
	}
	if len(rows) > maxImportRows {
		return domain.ImportResult{}, NewValidationError("file", fmt.Sprintf("file has more than %d rows; split it into smaller files", maxImportRows))
	}

	account, err := s.accountRepo.GetByIDForUser(ctx, accountID, userID)
	if err != nil {
		if err == domain.ErrNotFound {
			return domain.ImportResult{}, NewValidationError("account_id", "account does not exist")
		}
		return domain.ImportResult{}, err
	}

	// Errors starts as an empty (non-nil) slice, not the zero value's nil,
	// so a fully-successful import serializes as "errors":[] — matching
	// every other list-shaped field in this API (e.g. fakeAccountRepository
	// ListByUser's `make([]domain.Account, 0)`) — rather than "errors":null.
	// A nil slice is functionally identical in Go but NOT in JSON, and the
	// frontend calls importResult.errors.length unconditionally; this was
	// caught as a real browser crash (not by any Go test, since
	// len(nilSlice) == 0 too) importing a CSV with zero bad rows.
	result := domain.ImportResult{Errors: []domain.ImportRowError{}}
	now := time.Now().UTC()

	for i, row := range rows {
		rowNum := i + 2 // header is row 1, so the first data row is row 2

		tx, rowErr := buildImportedTransaction(userID, account, row, now)
		if rowErr != nil {
			result.Skipped++
			appendImportError(&result, rowNum, rowErr.Error())
			continue
		}
		if err := s.repo.Create(ctx, tx); err != nil {
			result.Skipped++
			appendImportError(&result, rowNum, "failed to save: "+err.Error())
			continue
		}
		s.publishCreated(ctx, tx)
		result.Imported++
	}

	return result, nil
}

// buildImportedTransaction turns one raw CSV row into a Transaction ready to
// persist, or an error explaining why the row is unusable. Kept as a
// standalone function (not a method) since it has no need for repository
// access — every dependency it needs (the account, the current time) is
// already resolved once by the caller, not re-fetched per row.
func buildImportedTransaction(userID string, account *domain.Account, row domain.ImportRow, now time.Time) (*domain.Transaction, error) {
	date, ok := parseImportDate(row.Date)
	if !ok {
		return nil, errors.New("invalid or missing date")
	}

	absAmount, inferredType, err := resolveImportAmount(row)
	if err != nil {
		return nil, err
	}

	txType := domain.TransactionType(strings.ToLower(strings.TrimSpace(row.Type)))
	if txType == "" {
		txType = inferredType
	}
	if !domain.ValidTransactionType(txType) {
		return nil, fmt.Errorf("type must be income or expense, got %q", row.Type)
	}

	if absAmount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}
	if absAmount > maxAmount {
		return nil, errors.New("amount exceeds the maximum allowed value")
	}

	note := strings.TrimSpace(row.Description)
	if len(note) > maxImportNoteLength {
		note = note[:maxImportNoteLength]
	}

	return &domain.Transaction{
		UserID:    userID,
		AccountID: account.ID,
		Type:      txType,
		Amount:    absAmount,
		Currency:  account.Currency,
		Date:      date,
		Note:      note,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// resolveImportAmount handles the two amount shapes a CSV can carry
// (domain.ImportRow's own doc comment explains why both exist): a single
// signed Amount column, or a separate Debit/Credit pair. Amount wins if a
// row somehow has both. Returns the absolute value to store plus the
// inferred type — the caller still lets an explicit Type column override
// this, exactly like the single-Amount path always did.
func resolveImportAmount(row domain.ImportRow) (float64, domain.TransactionType, error) {
	if strings.TrimSpace(row.Amount) != "" {
		amount, err := parseImportAmount(row.Amount)
		if err != nil {
			return 0, "", err
		}
		if amount < 0 {
			return math.Abs(amount), domain.TransactionTypeExpense, nil
		}
		return amount, domain.TransactionTypeIncome, nil
	}

	if strings.TrimSpace(row.Debit) != "" {
		debit, err := parseImportAmount(row.Debit)
		if err != nil {
			return 0, "", fmt.Errorf("debit: %w", err)
		}
		if debit != 0 {
			return math.Abs(debit), domain.TransactionTypeExpense, nil
		}
	}
	if strings.TrimSpace(row.Credit) != "" {
		credit, err := parseImportAmount(row.Credit)
		if err != nil {
			return 0, "", fmt.Errorf("credit: %w", err)
		}
		if credit != 0 {
			return math.Abs(credit), domain.TransactionTypeIncome, nil
		}
	}

	return 0, "", errors.New("no amount, debit, or credit value for this row")
}

// parseImportDate tries every layout in importDateLayouts in order.
func parseImportDate(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	for _, layout := range importDateLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseImportAmount accepts the punctuation real statement exports actually
// use: a leading currency symbol ("$45.00"), thousands separators
// ("$1,234.56"), and parenthesized negatives ("(45.00)" — a common
// accounting-export convention for a debit/charge), on top of a plain
// signed number.
func parseImportAmount(v string) (float64, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, errors.New("amount is required")
	}

	negative := false
	if strings.HasPrefix(v, "(") && strings.HasSuffix(v, ")") {
		negative = true
		v = strings.TrimSuffix(strings.TrimPrefix(v, "("), ")")
	}

	v = strings.NewReplacer("$", "", ",", "", " ", "").Replace(v)
	if v == "" {
		return 0, errors.New("amount is required")
	}

	amount, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q is not a valid number", strings.TrimSpace(v))
	}
	if negative {
		amount = -math.Abs(amount)
	}
	return amount, nil
}

// appendImportError records a row failure, respecting maxImportErrors —
// Skipped (incremented by the caller) always reflects the true count even
// once the error list itself stops growing.
func appendImportError(result *domain.ImportResult, row int, message string) {
	if len(result.Errors) < maxImportErrors {
		result.Errors = append(result.Errors, domain.ImportRowError{Row: row, Message: message})
	}
}
