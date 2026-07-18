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
