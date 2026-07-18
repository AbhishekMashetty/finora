// Package handler implements notification-service's Gin HTTP handlers. They
// bind/validate JSON, call into the service layer, and translate results into
// the shared httpx envelope — they never touch Mongo directly.
package handler

import (
	"errors"
	"net/http"

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
	Title   string `json:"title" binding:"required"`
	Message string `json:"message" binding:"required"`
	Type    string `json:"type" binding:"required"`
}

// List handles GET /api/v1/notifications?unread_only=
func (h *NotificationHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)
	unreadOnly := c.Query("unread_only") == "true"

	notifications, err := h.svc.List(c.Request.Context(), userID, unreadOnly)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to list notifications", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"notifications": notifications})
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
