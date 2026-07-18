// Package repository implements domain's repository interfaces against
// MongoDB. Nothing outside this package touches *mongo.Collection directly.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	usersCollection    = "users"
	settingsCollection = "settings"
	tokensCollection   = "refresh_tokens"
)

// EnsureIndexes creates the indexes user-service relies on for correctness:
// a unique index on users.email (so a race between two concurrent
// registrations for the same address can't both succeed), a unique index
// on refresh_tokens.jti_hash (one record per token), and a unique index on
// settings.user_id. Safe to call on every boot — CreateIndexes is
// idempotent.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := db.Collection(usersCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    map[string]int{"email": 1},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	if _, err := db.Collection(tokensCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    map[string]int{"jti_hash": 1},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	// Real gap found while writing Phase 6 integration tests: these two
	// were documented in architecture/database-design.md since Phase 0
	// ("TTL index on expires_at", "index on user_id — supports 'revoke all
	// sessions for this user'") but never actually implemented. The
	// user_id index isn't cosmetic — RevokeAllForUser (the token-theft
	// containment path, see this package's refresh_token_repository.go)
	// queries {user_id, revoked: false} on every replay-detection event,
	// which was doing a full collection scan without it. Both added now,
	// matching the doc exactly.
	if _, err := db.Collection(tokensCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]int{"user_id": 1},
	}); err != nil {
		return err
	}
	if _, err := db.Collection(tokensCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    map[string]int{"expires_at": 1},
		Options: options.Index().SetExpireAfterSeconds(0),
	}); err != nil {
		return err
	}

	if _, err := db.Collection(settingsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    map[string]int{"user_id": 1},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	return nil
}
