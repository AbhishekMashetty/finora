package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// TransactionHandler wires HTTP requests for /api/v1/transactions to
// domain.TransactionService.
type TransactionHandler struct {
	svc domain.TransactionService
}

// NewTransactionHandler builds a TransactionHandler backed by svc.
func NewTransactionHandler(svc domain.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

type transactionRequest struct {
	AccountID  string  `json:"account_id"`
	CategoryID *string `json:"category_id"`
	Type       string  `json:"type"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	Date       string  `json:"date"`
	Note       string  `json:"note" binding:"max=1000"`
}

// parseDate parses a request date field as RFC3339, falling back to the
// simpler YYYY-MM-DD form so clients can send either.
func parseDate(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// List handles GET /api/v1/transactions.
func (h *TransactionHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)

	in := domain.ListTransactionsInput{
		AccountID: c.Query("account_id"),
		Category:  c.Query("category"),
	}

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil {
			in.Page = p
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil {
			in.PageSize = ps
		}
	}
	if fromStr := c.Query("from"); fromStr != "" {
		if t, ok := parseDate(fromStr); ok {
			in.From = &t
		} else {
			httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid from date", gin.H{"field": "from"})
			return
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, ok := parseDate(toStr); ok {
			in.To = &t
		} else {
			httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid to date", gin.H{"field": "to"})
			return
		}
	}

	result, err := h.svc.List(c.Request.Context(), userID, in)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{
		"transactions": result.Transactions,
		"page":         result.Page,
		"page_size":    result.PageSize,
		"total":        result.Total,
	})
}

// Create handles POST /api/v1/transactions.
func (h *TransactionHandler) Create(c *gin.Context) {
	userID := middleware.UserID(c)

	var req transactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", gin.H{"error": err.Error()})
		return
	}

	date, ok := parseDate(req.Date)
	if !ok {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "date is required and must be RFC3339 or YYYY-MM-DD", gin.H{"field": "date"})
		return
	}

	tx, err := h.svc.Create(c.Request.Context(), userID, domain.CreateTransactionInput{
		AccountID:  req.AccountID,
		CategoryID: req.CategoryID,
		Type:       domain.TransactionType(req.Type),
		Amount:     req.Amount,
		Currency:   req.Currency,
		Date:       date,
		Note:       req.Note,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, gin.H{"transaction": tx})
}

// Get handles GET /api/v1/transactions/:id.
func (h *TransactionHandler) Get(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	tx, err := h.svc.Get(c.Request.Context(), userID, id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"transaction": tx})
}

// Update handles PUT /api/v1/transactions/:id.
func (h *TransactionHandler) Update(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	var req transactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", gin.H{"error": err.Error()})
		return
	}

	date, ok := parseDate(req.Date)
	if !ok {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "date is required and must be RFC3339 or YYYY-MM-DD", gin.H{"field": "date"})
		return
	}

	tx, err := h.svc.Update(c.Request.Context(), userID, id, domain.UpdateTransactionInput{
		AccountID:  req.AccountID,
		CategoryID: req.CategoryID,
		Type:       domain.TransactionType(req.Type),
		Amount:     req.Amount,
		Currency:   req.Currency,
		Date:       date,
		Note:       req.Note,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"transaction": tx})
}

// Delete handles DELETE /api/v1/transactions/:id.
func (h *TransactionHandler) Delete(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	if err := h.svc.Delete(c.Request.Context(), userID, id); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
