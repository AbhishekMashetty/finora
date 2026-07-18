// Package domain holds Finora user-service's core business types and the
// interfaces (repositories, services) that the rest of the service depends
// on. It imports no framework (no gin, no mongo-driver) so it stays fully
// unit-testable in isolation — every other layer depends on domain, domain
// depends on nothing.
package domain

import "time"

// User is the account record. PasswordHash is tagged json:"-" so it can
// never leak into an HTTP response even if a handler serializes the struct
// directly.
type User struct {
	ID           string    `json:"id" bson:"_id"`
	Email        string    `json:"email" bson:"email"`
	PasswordHash string    `json:"-" bson:"password_hash"`
	Name         string    `json:"name" bson:"name"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" bson:"updated_at"`
}

// Settings holds per-user preferences. Stored in its own Mongo collection
// (keyed by user_id) rather than embedded in the user document, so settings
// can be read/written independently of the account record without a partial
// update on the user doc — see README for the rationale.
type Settings struct {
	UserID    string    `json:"user_id" bson:"user_id"`
	Currency  string    `json:"currency" bson:"currency"`
	Timezone  string    `json:"timezone" bson:"timezone"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// RefreshToken is the server-side record backing a refresh token. Only a
// SHA-256 hash of the token's JTI is stored (never the raw JTI or the raw
// token), so a database leak alone can't be used to forge a refresh token.
type RefreshToken struct {
	ID        string    `json:"id" bson:"_id"`
	UserID    string    `json:"user_id" bson:"user_id"`
	JTIHash   string    `json:"-" bson:"jti_hash"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
	Revoked   bool      `json:"revoked" bson:"revoked"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}
