package domain

import "context"

// RegisterInput carries the fields needed to create a new account.
type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

// LoginResult is returned by any flow that issues a fresh token pair
// (login, refresh).
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *User
}

// AuthService owns registration, login, refresh-token rotation and logout.
// internal/handler depends only on this interface, never on the concrete
// internal/service implementation or on Mongo.
type AuthService interface {
	Register(ctx context.Context, in RegisterInput) (*User, error)
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	// Refresh rotates the refresh token: the old one is revoked and a new
	// access+refresh pair is issued and persisted.
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	// Logout is idempotent: an already-revoked or unknown token still
	// succeeds, so callers can't probe token validity via this endpoint.
	Logout(ctx context.Context, refreshToken string) error
	// RequestPasswordReset is a Phase 1 stub: it always succeeds regardless
	// of whether the email is registered (so the endpoint can't be used to
	// enumerate accounts) and logs the would-be reset action instead of
	// sending an email. Real delivery — a reset-token record plus a
	// consume/confirm endpoint and a real notification-service email send —
	// is follow-up Phase 1 work; this establishes the route and contract so
	// the frontend can be wired against it now.
	RequestPasswordReset(ctx context.Context, email string) error
}

// UpdateProfileInput carries the mutable profile fields for PUT /users/me.
type UpdateProfileInput struct {
	Name string
}

// UpdateSettingsInput carries the mutable settings fields for
// PUT /users/me/settings. Empty fields leave the existing value untouched.
type UpdateSettingsInput struct {
	Currency string
	Timezone string
}

// UserService owns profile and settings reads/writes for the authenticated
// caller (identified by middleware.RequireIdentity upstream).
type UserService interface {
	GetProfile(ctx context.Context, userID string) (*User, error)
	UpdateProfile(ctx context.Context, userID string, in UpdateProfileInput) (*User, error)
	GetSettings(ctx context.Context, userID string) (*Settings, error)
	UpdateSettings(ctx context.Context, userID string, in UpdateSettingsInput) (*Settings, error)
}
