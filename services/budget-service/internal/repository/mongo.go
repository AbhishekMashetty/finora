// Package repository implements domain's repository interfaces against
// MongoDB. Nothing outside this package touches *mongo.Collection directly.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	budgetsCollection = "budgets"
	goalsCollection   = "goals"
)

// EnsureIndexes creates the indexes budget-service relies on for correctness
// and query performance: an owner-scoping index on every collection, plus
// the compound index the budget list/period-filter access pattern needs (see
// architecture/database-design.md). Safe to call on every boot —
// CreateIndexes is idempotent.
//
// Note: architecture/database-design.md's original sketch called the goals
// collection "savings_goals", but internal/repository/mongo_goal_repository.go
// has always used db.Collection("goals") — this indexes the collection the
// code actually uses, and the doc has been corrected to match (same kind of
// doc/code drift Phase 2 caught for expense-service's category_id field).
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := db.Collection(budgetsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]int{"user_id": 1},
	}); err != nil {
		return err
	}

	// Field order matters for compound indexes (leftmost-prefix rule), so
	// this uses bson.D rather than a Go map — map iteration order is not
	// guaranteed and the driver's map codec would silently reorder keys
	// (the exact bug Phase 2 caught in expense-service's EnsureIndexes).
	if _, err := db.Collection(budgetsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "period", Value: 1},
		},
	}); err != nil {
		return err
	}

	if _, err := db.Collection(goalsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: map[string]int{"user_id": 1},
	}); err != nil {
		return err
	}

	return nil
}
