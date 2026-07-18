// Package handler holds Gin handlers: they bind/validate JSON, call the
// service layer via domain interfaces, and respond via httpx.Success/Fail.
// They never touch Mongo directly.
package handler

import (
	"errors"
	"net/http"

	"github.com/finora/shared/httpx"
	"github.com/finora/user-service/internal/domain"
	"github.com/gin-gonic/gin"
)

// AuthHandler implements the /api/v1/auth/* endpoints.
type AuthHandler struct {
	svc domain.AuthService
}

// NewAuthHandler constructs an AuthHandler from a domain.AuthService.
func NewAuthHandler(svc domain.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type registerRequest struct {
	Email string `json:"email" binding:"required,email"`
	// max=72: bcrypt (see internal/service/auth_service.go) silently
	// truncates/ignores any input past 72 bytes, so this cap isn't
	// arbitrary — it matches the real limit of the hash function actually
	// used, rather than leaving an unbounded field on a public,
	// unauthenticated route.
	Password string `json:"password" binding:"required,min=8,max=72"`
	// max=100 matches maxNameLength in internal/service/user_service.go, so
	// register and update-profile agree on what a valid name is.
	Name string `json:"name" binding:"required,max=100"`
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}

	user, err := h.svc.Register(c.Request.Context(), domain.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateEmail) {
			httpx.Fail(c, http.StatusConflict, httpx.CodeConflict, "email already registered", nil)
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
		return
	}

	httpx.Success(c, http.StatusCreated, gin.H{"user": user})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}

	result, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid credentials", nil)
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"user":          result.User,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}

	result, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidToken) {
			httpx.Fail(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or expired refresh token", nil)
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Logout handles POST /api/v1/auth/logout. It authenticates via the
// refresh token in the body, not via X-User-Id, and is idempotent: always
// 204 unless a genuine internal error occurs.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}

	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
		return
	}

	c.Status(http.StatusNoContent)
}

type passwordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RequestPasswordReset handles POST /api/v1/auth/password-reset. Phase 1
// stub — see domain.AuthService.RequestPasswordReset. Always responds 202
// regardless of whether the email is registered, so this endpoint can't be
// used to enumerate accounts.
func (h *AuthHandler) RequestPasswordReset(c *gin.Context) {
	var req passwordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}

	if err := h.svc.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
		return
	}

	httpx.Success(c, http.StatusAccepted, gin.H{
		"message": "if an account with that email exists, a reset link has been sent",
	})
}
