// Package repository implements domain's repository interfaces against
// MongoDB. Nothing outside this package touches *mongo.Collection directly.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	accountsCollection     = "accounts"
	transactionsCollection = "transactions"
	categoriesCollection   = "categories"
)

// EnsureIndexes creates the indexes expense-service relies on for correctness
// and query performance: an owner-scoping index on every collection, plus the
// compound indexes the transaction list endpoint's access patterns need (see
// architecture/database-design.md). Safe to call on every boot —
// CreateIndexes is idempotent.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := db.Collection(accountsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]int{"user_id": 1},
	}); err != nil {
		return err
	}

	// Field order matters for compound indexes (leftmost-prefix rule), so
	// these use bson.D rather than a Go map — map iteration order is not
	// guaranteed and the driver's map codec would silently reorder keys.
	if _, err := db.Collection(transactionsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "account_id", Value: 1},
			{Key: "date", Value: 1},
		},
	}); err != nil {
		return err
	}

	if _, err := db.Collection(transactionsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "category_id", Value: 1},
		},
		Options: options.Index().SetSparse(true),
	}); err != nil {
		return err
	}

	if _, err := db.Collection(categoriesCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]int{"user_id": 1},
	}); err != nil {
		return err
	}

	return nil
}
