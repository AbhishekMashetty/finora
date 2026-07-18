//go:build integration

package repository_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
)

// assertIndexExists fails the test if collection does not have an index
// named indexName. Mongo's default naming (field_direction, e.g.
// "user_id_1") is what EnsureIndexes' unnamed IndexModels produce, so
// asserting on that name is asserting the index genuinely exists — not
// inferring it from the code that's supposed to create it.
func assertIndexExists(t *testing.T, ctx context.Context, db *mongo.Database, collection, indexName string) {
	t.Helper()

	cursor, err := db.Collection(collection).Indexes().List(ctx)
	if err != nil {
		t.Fatalf("Indexes().List(%s): %v", collection, err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var idx struct {
			Name string `bson:"name"`
		}
		if err := cursor.Decode(&idx); err != nil {
			t.Fatalf("decode index on %s: %v", collection, err)
		}
		if idx.Name == indexName {
			return
		}
	}
	t.Errorf("expected an index named %q on %s, per architecture/database-design.md — it does not exist", indexName, collection)
}

// assertTTLIndexExists fails the test unless collection has an index named
// indexName that is genuinely a TTL index expiring documents at their exact
// stored timestamp (expireAfterSeconds: 0) — not just an index that happens
// to share that name. A plain, non-expiring index (or a TTL index with a
// nonzero grace period) would satisfy assertIndexExists alone but silently
// fail to reap expired refresh tokens, which is the entire point of this
// index existing.
func assertTTLIndexExists(t *testing.T, ctx context.Context, db *mongo.Database, collection, indexName string) {
	t.Helper()

	cursor, err := db.Collection(collection).Indexes().List(ctx)
	if err != nil {
		t.Fatalf("Indexes().List(%s): %v", collection, err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var idx struct {
			Name               string `bson:"name"`
			ExpireAfterSeconds *int32 `bson:"expireAfterSeconds"`
		}
		if err := cursor.Decode(&idx); err != nil {
			t.Fatalf("decode index on %s: %v", collection, err)
		}
		if idx.Name == indexName {
			if idx.ExpireAfterSeconds == nil {
				t.Errorf("index %q on %s exists but has no expireAfterSeconds — it's a plain index, not a TTL index", indexName, collection)
				return
			}
			if *idx.ExpireAfterSeconds != 0 {
				t.Errorf("index %q on %s has expireAfterSeconds=%d, want 0 (expire exactly at the stored timestamp)", indexName, collection, *idx.ExpireAfterSeconds)
			}
			return
		}
	}
	t.Errorf("expected a TTL index named %q on %s, per architecture/database-design.md — it does not exist", indexName, collection)
}
