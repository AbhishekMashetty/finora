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

	// Phase 6 shipped {user_id: 1, account_id: 1, date: 1} and a sparse
	// {user_id: 1, category_id: 1} as the only transaction indexes. Neither
	// can serve ListByUser's actual query shape: it always sorts
	// {date: -1}, but when the caller doesn't filter by account_id (the
	// default dashboard view — the single most-executed query in this
	// service), account_id sits between the equality prefix and the sort
	// field in that compound index, so Mongo can't use it to produce
	// sorted output and falls back to a blocking in-memory SORT over the
	// user's entire matched set on every page, including page 1 — cost
	// grows with total transaction history, not page size, and Mongo
	// aborts the sort outright past its 32MB memory limit (roughly 160k
	// transactions for one user) unless allowDiskUse is set, which nothing
	// here does. Drop the old indexes by their auto-generated names (a
	// fresh database has neither, so DropOne failing is expected and
	// ignored) and replace them with three indexes shaped for the actual
	// ESR (Equality, Sort, Range) order of every access pattern
	// ListByUser's buildTransactionFilter can produce, each ending in
	// {date: -1} to match the query's own sort direction so Mongo can walk
	// the index directly instead of sorting in memory.
	_, _ = db.Collection(transactionsCollection).Indexes().DropOne(ctx, "user_id_1_account_id_1_date_1")
	_, _ = db.Collection(transactionsCollection).Indexes().DropOne(ctx, "user_id_1_category_id_1")

	// Field order matters for compound indexes (leftmost-prefix rule), so
	// these use bson.D rather than a Go map — map iteration order is not
	// guaranteed and the driver's map codec would silently reorder keys.

	// Serves the default list query (user_id only, no account/category
	// filter) and any date-range-only query.
	if _, err := db.Collection(transactionsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "date", Value: -1},
		},
		Options: options.Index().SetName("idx_user_date"),
	}); err != nil {
		return err
	}

	// Serves the account-filtered list query.
	if _, err := db.Collection(transactionsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "account_id", Value: 1},
			{Key: "date", Value: -1},
		},
		Options: options.Index().SetName("idx_user_account_date"),
	}); err != nil {
		return err
	}

	// Serves the category-filtered list query, and the per-category
	// aggregation endpoint (AggregateByCategory) — a $match on
	// {user_id, date range} followed by a $group on category_id benefits
	// from the same leading {user_id, ...date} shape, and category_id
	// stays sparse: not every transaction carries one.
	if _, err := db.Collection(transactionsCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "category_id", Value: 1},
			{Key: "date", Value: -1},
		},
		Options: options.Index().SetName("idx_user_category_date").SetSparse(true),
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
