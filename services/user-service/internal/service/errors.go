package service

import "errors"

// ValidationError signals a request failed input validation. Handlers map it
// to httpx.CodeValidation / 400. Mirrors expense-service's
// internal/service.ValidationError so both services share the same
// service-layer-owns-business-validation convention.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError builds a ValidationError for a specific field.
func NewValidationError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// AsValidationError reports whether err is (or wraps) a *ValidationError.
func AsValidationError(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}
