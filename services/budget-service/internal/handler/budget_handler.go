// Package handler implements Gin HTTP handlers. Handlers only bind/validate
// JSON, call the service layer, and respond via httpx.Success/Fail — they
// never touch MongoDB directly.
package handler

import (
	"errors"
	"net/http"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// BudgetHandler exposes budget CRUD endpoints.
type BudgetHandler struct {
	service domain.BudgetService
}

// NewBudgetHandler builds a BudgetHandler backed by service.
func NewBudgetHandler(service domain.BudgetService) *BudgetHandler {
	return &BudgetHandler{service: service}
}

type createBudgetRequest struct {
	Category string  `json:"category" binding:"required"`
	Amount   float64 `json:"amount" binding:"required"`
	Period   string  `json:"period" binding:"required"`
}

type updateBudgetRequest struct {
	Category string  `json:"category" binding:"required"`
	Amount   float64 `json:"amount" binding:"required"`
	Period   string  `json:"period" binding:"required"`
}

// List handles GET /api/v1/budgets.
func (h *BudgetHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)

	budgets, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to list budgets", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"budgets": budgets})
}

// Create handles POST /api/v1/budgets.
func (h *BudgetHandler) Create(c *gin.Context) {
	userID := middleware.UserID(c)

	var req createBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", err.Error())
		return
	}

	budget, err := h.service.Create(c.Request.Context(), userID, domain.CreateBudgetInput{
		Category: req.Category,
		Amount:   req.Amount,
		Period:   domain.Period(req.Period),
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusCreated, gin.H{"budget": budget})
}

// Get handles GET /api/v1/budgets/:id.
func (h *BudgetHandler) Get(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	budget, err := h.service.Get(c.Request.Context(), userID, id)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"budget": budget})
}

// Update handles PUT /api/v1/budgets/:id.
func (h *BudgetHandler) Update(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	var req updateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", err.Error())
		return
	}

	budget, err := h.service.Update(c.Request.Context(), userID, id, domain.UpdateBudgetInput{
		Category: req.Category,
		Amount:   req.Amount,
		Period:   domain.Period(req.Period),
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"budget": budget})
}

// Delete handles DELETE /api/v1/budgets/:id.
func (h *BudgetHandler) Delete(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		writeServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// writeServiceError maps a domain/service error to the right HTTP status and
// error code. domain.ErrNotFound covers both "doesn't exist" and "belongs to
// another user" — those must never be distinguished in the response.
func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Fail(c, http.StatusNotFound, httpx.CodeNotFound, "resource not found", nil)
	case errors.Is(err, domain.ErrValidation):
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
	default:
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
	}
}
