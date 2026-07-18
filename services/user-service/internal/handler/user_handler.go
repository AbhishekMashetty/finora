package handler

import (
	"errors"
	"net/http"

	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/finora/user-service/internal/domain"
	"github.com/finora/user-service/internal/service"
	"github.com/gin-gonic/gin"
)

// UserHandler implements the /api/v1/users/me* endpoints. All routes it
// serves sit behind middleware.RequireIdentity(), so middleware.UserID(c)
// is always populated.
type UserHandler struct {
	svc domain.UserService
}

// NewUserHandler constructs a UserHandler from a domain.UserService.
func NewUserHandler(svc domain.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// respondUserErr maps a service/domain error to the correct HTTP envelope.
// Business validation (name/currency/timezone format) lives in the service
// layer — see internal/service.ValidationError — matching the house style
// established by expense-service's internal/service, so handlers stay
// parse/validate-shape/respond only, never re-implementing business rules.
func (h *UserHandler) respondUserErr(c *gin.Context, err error) {
	if ve, ok := service.AsValidationError(err); ok {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, ve.Message, gin.H{"field": ve.Field})
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		httpx.Fail(c, http.StatusNotFound, httpx.CodeNotFound, "user not found", nil)
		return
	}
	httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
}

// Me handles GET /api/v1/users/me.
func (h *UserHandler) Me(c *gin.Context) {
	userID := middleware.UserID(c)

	u, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		h.respondUserErr(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"user": u})
}

type updateProfileRequest struct {
	Name string `json:"name" binding:"required"`
}

// UpdateMe handles PUT /api/v1/users/me.
func (h *UserHandler) UpdateMe(c *gin.Context) {
	userID := middleware.UserID(c)

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}

	u, err := h.svc.UpdateProfile(c.Request.Context(), userID, domain.UpdateProfileInput{Name: req.Name})
	if err != nil {
		h.respondUserErr(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"user": u})
}

// GetSettings handles GET /api/v1/users/me/settings.
func (h *UserHandler) GetSettings(c *gin.Context) {
	userID := middleware.UserID(c)

	s, err := h.svc.GetSettings(c.Request.Context(), userID)
	if err != nil {
		h.respondUserErr(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"settings": s})
}

type updateSettingsRequest struct {
	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
}

// UpdateSettings handles PUT /api/v1/users/me/settings.
func (h *UserHandler) UpdateSettings(c *gin.Context) {
	userID := middleware.UserID(c)

	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}

	s, err := h.svc.UpdateSettings(c.Request.Context(), userID, domain.UpdateSettingsInput{
		Currency: req.Currency,
		Timezone: req.Timezone,
	})
	if err != nil {
		h.respondUserErr(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"settings": s})
}
