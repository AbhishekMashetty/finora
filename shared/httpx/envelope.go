// Package httpx implements the one response envelope every Finora service
// must use, so clients (the gateway, the frontend) never have to special-case
// a service's JSON shape. See architecture/api-contracts.md for the contract.
package httpx

import (
	"github.com/gin-gonic/gin"
)

// Envelope is the top-level shape of every JSON response.
type Envelope struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Error     *ErrorBody  `json:"error"`
	RequestID string      `json:"request_id"`
}

// ErrorBody carries a machine-readable code alongside a human message.
type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Standard error codes. Keep in sync with architecture/api-contracts.md.
const (
	CodeValidation   = "VALIDATION_ERROR"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeConflict     = "CONFLICT"
	CodeInternal     = "INTERNAL_ERROR"
)

func requestID(c *gin.Context) string {
	if id, ok := c.Get("request_id"); ok {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// Success writes a 2xx envelope with the given payload.
func Success(c *gin.Context, status int, data interface{}) {
	c.JSON(status, Envelope{
		Success:   true,
		Data:      data,
		Error:     nil,
		RequestID: requestID(c),
	})
}

// Fail writes an error envelope with the given HTTP status and error code.
func Fail(c *gin.Context, status int, code, message string, details interface{}) {
	c.JSON(status, Envelope{
		Success:   false,
		Data:      nil,
		Error:     &ErrorBody{Code: code, Message: message, Details: details},
		RequestID: requestID(c),
	})
}
