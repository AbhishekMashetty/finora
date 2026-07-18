package domain

import "errors"

// Sentinel errors returned by repositories and services. Handlers map these
// to the standard httpx error codes/status codes from api-contracts.md.
var (
	// ErrNotFound is returned by a repository when a lookup finds nothing.
	ErrNotFound = errors.New("not found")
	// ErrDuplicateEmail is returned when registering an email already in use.
	ErrDuplicateEmail = errors.New("email already registered")
	// ErrInvalidCredentials is returned on login when the email is unknown or
	// the password doesn't match — kept generic so the caller can't tell
	// which one was wrong.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidToken is returned when a refresh token is malformed, expired,
	// revoked, or unknown.
	ErrInvalidToken = errors.New("invalid or expired token")
)
