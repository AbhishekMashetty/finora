// Package service implements expense-service's business logic. It depends
// only on domain interfaces (never on the concrete Mongo repositories), so it
// can be unit-tested against hand-written fakes with no live database.
package service

import "errors"

// ValidationError signals a request failed input validation. Handlers map it
// to httpx.CodeValidation / 400.
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
