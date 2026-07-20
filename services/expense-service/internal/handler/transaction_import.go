package handler

import (
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// importColumnAliases maps a domain.ImportRow field to the case-insensitive
// header names this endpoint recognizes for it. Real statement exports
// don't agree on column names (a bank's "Description" is another's
// "Merchant"), so matching a small alias list is meaningfully more useful
// than requiring one exact header — without trying to auto-detect every
// possible export format, which would be exactly the kind of complexity
// CLAUDE.md says not to add before there's a legitimate reason.
//
// "amount" and "debit" are deliberately NOT aliases of each other, even
// though earlier they were treated as one — a real Capital One export has
// separate Debit/Credit columns holding unsigned magnitudes (Debit for
// purchases, Credit for payments), not a single signed Amount column, and
// treating a lone "Debit" header as if it were signed silently mislabeled
// every purchase as income (Debit values are always positive). Debit and
// Credit are their own concept, resolved in the service layer
// (resolveImportAmount), not folded into "amount" here.
var importColumnAliases = map[string][]string{
	"date":        {"date", "transaction date", "posted date"},
	"description": {"description", "merchant", "payee", "note"},
	"amount":      {"amount"},
	"debit":       {"debit"},
	"credit":      {"credit"},
	"type":        {"type"},
}

// ImportCSV handles POST /api/v1/transactions/import — a multipart/form-data
// request with an `account_id` field and a `file` field (the CSV). Every
// imported transaction is attached to account_id and inherits its currency;
// the CSV itself carries no currency column, matching how a single credit
// card statement is always in one currency.
func (h *TransactionHandler) ImportCSV(c *gin.Context) {
	userID := middleware.UserID(c)

	accountID := strings.TrimSpace(c.PostForm("account_id"))
	if accountID == "" {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "account_id is required", gin.H{"field": "account_id"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, `a CSV file is required (multipart field "file")`, gin.H{"field": "file"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "could not read uploaded file", nil)
		return
	}
	defer f.Close()

	rows, parseErr := parseImportCSV(f)
	if parseErr != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, parseErr.Error(), gin.H{"field": "file"})
		return
	}

	result, err := h.svc.Import(c.Request.Context(), userID, accountID, rows)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{
		"imported": result.Imported,
		"skipped":  result.Skipped,
		"errors":   result.Errors,
	})
}

// parseImportCSV reads the header row to locate the date/description/
// amount/type columns (by alias, case-insensitively), then reads every
// remaining row into a domain.ImportRow. It never rejects an individual
// data row itself — a row shorter than the header (a ragged CSV) just
// yields empty strings for its missing columns, which the service layer's
// value-level validation (parseImportDate/parseImportAmount) will then
// reject with a specific message. Only a structurally unusable file (empty,
// or missing the date/amount columns entirely) is a parse-level error here.
func parseImportCSV(r io.Reader) ([]domain.ImportRow, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; each is still parsed, just with blanks for missing fields

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("the uploaded file is empty")
		}
		return nil, errors.New("could not parse the uploaded file as CSV")
	}

	columnIndex := make(map[string]int, len(header))
	for i, name := range header {
		columnIndex[normalizeHeader(name)] = i
	}

	dateIdx, ok := findColumn(columnIndex, importColumnAliases["date"])
	if !ok {
		return nil, errors.New(`CSV must include a date column (e.g. "Date")`)
	}
	amountIdx, hasAmount := findColumn(columnIndex, importColumnAliases["amount"])
	debitIdx, hasDebit := findColumn(columnIndex, importColumnAliases["debit"])
	creditIdx, hasCredit := findColumn(columnIndex, importColumnAliases["credit"])
	if !hasAmount && !hasDebit && !hasCredit {
		return nil, errors.New(`CSV must include an amount column (e.g. "Amount"), or separate "Debit"/"Credit" columns`)
	}
	descIdx, _ := findColumn(columnIndex, importColumnAliases["description"])
	typeIdx, _ := findColumn(columnIndex, importColumnAliases["type"])

	var rows []domain.ImportRow
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// A genuinely malformed line (e.g. an unterminated quote) —
			// encoding/csv can't recover a byte offset back into a clean
			// row here, so the whole file is rejected rather than silently
			// dropping everything after the bad line.
			return nil, errors.New("could not parse the uploaded file as CSV: " + err.Error())
		}
		rows = append(rows, domain.ImportRow{
			Date:        field(record, dateIdx),
			Description: field(record, descIdx),
			Amount:      field(record, amountIdx),
			Debit:       field(record, debitIdx),
			Credit:      field(record, creditIdx),
			Type:        field(record, typeIdx),
		})
	}
	return rows, nil
}

func normalizeHeader(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func findColumn(columnIndex map[string]int, aliases []string) (int, bool) {
	for _, alias := range aliases {
		if idx, ok := columnIndex[alias]; ok {
			return idx, true
		}
	}
	return -1, false
}

func field(record []string, idx int) string {
	if idx < 0 || idx >= len(record) {
		return ""
	}
	return record[idx]
}
