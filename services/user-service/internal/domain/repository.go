package domain

import "context"

// UserRepository persists User records. Implemented against MongoDB in
// internal/repository; internal/service depends only on this interface so it
// can be unit-tested against a hand-written fake.
type UserRepository interface {
	Create(ctx context.Context, u *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	Update(ctx context.Context, u *User) error
}

// SettingsRepository persists per-user Settings records.
type SettingsRepository interface {
	FindByUserID(ctx context.Context, userID string) (*Settings, error)
	// Upsert creates or replaces the settings document for s.UserID.
	Upsert(ctx context.Context, s *Settings) error
}

// RefreshTokenRepository persists RefreshToken records, keyed by the SHA-256
// hash of the token's JTI (never the raw JTI).
type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *RefreshToken) error
	FindByJTIHash(ctx context.Context, jtiHash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id string) error
	// RevokeAllForUser revokes every non-revoked refresh token belonging to
	// userID. Used as the containment response when a request presents an
	// already-revoked (already-rotated-away) refresh token: that's evidence
	// of token theft/replay, since the legitimate client already moved on to
	// the next token in the rotation chain. Revoking everything forces full
	// re-authentication on all of that user's devices/sessions.
	RevokeAllForUser(ctx context.Context, userID string) error
}
