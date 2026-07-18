// Package repository implements domain.NotificationRepository against
// MongoDB. Nothing outside this package touches *mongo.Collection directly.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// EnsureIndexes creates the indexes notification-service relies on for
// correctness and query performance, same idiom as the Phase 2/3 fixes in
// expense-service and budget-service's internal/repository/mongo.go. Safe
// to call on every boot — CreateOne is idempotent.
//
// notifications -> compound (user_id, read): the feed's primary query is
// "this user's unread notifications" (GET /api/v1/notifications?unread_only),
// which this index serves directly (see architecture/database-design.md).
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Field order matters for compound indexes (leftmost-prefix rule), so
	// this uses bson.D rather than a Go map — map iteration order is not
	// guaranteed and the driver's map codec would silently reorder keys
	// (the exact bug Phase 2 caught in expense-service's EnsureIndexes).
	_, err := db.Collection(collectionName).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "read", Value: 1},
		},
	})
	return err
}
