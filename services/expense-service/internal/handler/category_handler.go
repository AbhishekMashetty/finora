package handler

import (
	"net/http"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// CategoryHandler wires HTTP requests for /api/v1/categories to
// domain.CategoryService. Intentionally minimal: create + list only.
type CategoryHandler struct {
	svc domain.CategoryService
}

// NewCategoryHandler builds a CategoryHandler backed by svc.
func NewCategoryHandler(svc domain.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

type createCategoryRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// List handles GET /api/v1/categories.
func (h *CategoryHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)
	categories, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, gin.H{"categories": categories})
}

// Create handles POST /api/v1/categories.
func (h *CategoryHandler) Create(c *gin.Context) {
	userID := middleware.UserID(c)

	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", gin.H{"error": err.Error()})
		return
	}

	category, err := h.svc.Create(c.Request.Context(), userID, domain.CreateCategoryInput{
		Name: req.Name,
		Type: domain.TransactionType(req.Type),
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	httpx.Success(c, http.StatusCreated, gin.H{"category": category})
}
