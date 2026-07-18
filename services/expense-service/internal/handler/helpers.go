// Package handler implements Gin HTTP handlers for expense-service. Handlers
// only bind/validate JSON, delegate to the service layer, and translate
// results/errors into the shared httpx envelope — they never touch Mongo
// directly.
package handler

import (
	"errors"
	"net/http"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/service"
	"github.com/finora/shared/httpx"
	"github.com/gin-gonic/gin"
)

// writeServiceError maps a service/domain error to the correct HTTP envelope.
func writeServiceError(c *gin.Context, err error) {
	if ve, ok := service.AsValidationError(err); ok {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, ve.Message, gin.H{"field": ve.Field})
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		httpx.Fail(c, http.StatusNotFound, httpx.CodeNotFound, "resource not found", nil)
		return
	}
	httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error", nil)
}
