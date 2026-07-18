package handler

import (
	"net/http"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// AccountHandler wires HTTP requests for /api/v1/accounts to domain.AccountService.
type AccountHandler struct {
	svc domain.AccountService
}

// NewAccountHandler builds an AccountHandler backed by svc.
func NewAccountHandler(svc domain.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

type createAccountRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
}

type updateAccountRequest struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Currency string   `json:"currency"`
	Balance  *float64 `json:"balance"`
}

// List handles GET /api/v1/accounts.
func (h *AccountHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)
	accounts, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"accounts": accounts})
}

// Create handles POST /api/v1/accounts.
func (h *AccountHandler) Create(c *gin.Context) {
	userID := middleware.UserID(c)

	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", gin.H{"error": err.Error()})
		return
	}

	account, err := h.svc.Create(c.Request.Context(), userID, domain.CreateAccountInput{
		Name:     req.Name,
		Type:     domain.AccountType(req.Type),
		Currency: req.Currency,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, gin.H{"account": account})
}

// Get handles GET /api/v1/accounts/:id.
func (h *AccountHandler) Get(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	account, err := h.svc.Get(c.Request.Context(), userID, id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"account": account})
}

// Update handles PUT /api/v1/accounts/:id.
func (h *AccountHandler) Update(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	var req updateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", gin.H{"error": err.Error()})
		return
	}

	account, err := h.svc.Update(c.Request.Context(), userID, id, domain.UpdateAccountInput{
		Name:     req.Name,
		Type:     domain.AccountType(req.Type),
		Currency: req.Currency,
		Balance:  req.Balance,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"account": account})
}

// Delete handles DELETE /api/v1/accounts/:id.
func (h *AccountHandler) Delete(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	if err := h.svc.Delete(c.Request.Context(), userID, id); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
