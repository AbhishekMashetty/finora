// Package handler implements notification-service's Gin HTTP handlers. They
// bind/validate JSON, call into the service layer, and translate results into
// the shared httpx envelope — they never touch Mongo directly.
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/finora/notification-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// NotificationHandler wires HTTP requests to a domain.NotificationService.
type NotificationHandler struct {
	svc domain.NotificationService
}

// NewNotificationHandler builds a handler around svc.
func NewNotificationHandler(svc domain.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

type createNotificationRequest struct {
	Title   string `json:"title" binding:"required,max=200"`
	Message string `json:"message" binding:"required,max=2000"`
	Type    string `json:"type" binding:"required,max=50"`
}

// List handles GET /api/v1/notifications?unread_only=&page=&page_size=
// page/page_size follow the standard pagination contract (architecture/
// api-contracts.md) — same query param names and response shape
// (<resource>, page, page_size, total) as expense-service's transaction
// list, added in Phase 6 since a long-lived user's notification history is
// genuinely unbounded, unlike e.g. accounts/budgets/goals.
func (h *NotificationHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)

	filter := domain.NotificationFilter{UnreadOnly: c.Query("unread_only") == "true"}
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil {
			filter.Page = p
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil {
			filter.PageSize = ps
		}
	}

	result, err := h.svc.List(c.Request.Context(), userID, filter)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to list notifications", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{
		"notifications": result.Notifications,
		"page":          result.Page,
		"page_size":     result.PageSize,
		"total":         result.Total,
	})
}

// Create handles POST /api/v1/notifications
func (h *NotificationHandler) Create(c *gin.Context) {
	userID := middleware.UserID(c)

	var req createNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", gin.H{"error": err.Error()})
		return
	}

	notification, err := h.svc.Create(c.Request.Context(), userID, req.Title, req.Message, req.Type)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to create notification", nil)
		return
	}

	httpx.Success(c, http.StatusCreated, gin.H{"notification": notification})
}

// MarkRead handles PATCH /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	notification, err := h.svc.MarkRead(c.Request.Context(), userID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpx.Fail(c, http.StatusNotFound, httpx.CodeNotFound, "notification not found", nil)
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to mark notification read", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"notification": notification})
}
