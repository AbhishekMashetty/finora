//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/repository"
	"github.com/finora/shared/mongotest"
)

func TestCategoryRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("expense_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewCategoryRepository(db)
	ctx := context.Background()

	t.Run("create then list round-trips through real Mongo", func(t *testing.T) {
		cat := &domain.Category{UserID: "user-1", Name: "Groceries", Type: domain.TransactionTypeExpense}
		if err := repo.Create(ctx, cat); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if cat.ID == "" {
			t.Fatal("Create did not populate an ID")
		}

		list, err := repo.ListByUser(ctx, "user-1")
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		var found bool
		for _, c := range list {
			if c.ID == cat.ID && c.Name == "Groceries" && c.Type == domain.TransactionTypeExpense {
				found = true
			}
		}
		if !found {
			t.Errorf("ListByUser(user-1) = %+v, want to include the just-created category", list)
		}
	})

	t.Run("ownership is enforced by the real Mongo query", func(t *testing.T) {
		cat := &domain.Category{UserID: "owner", Name: "Private Category", Type: domain.TransactionTypeExpense}
		if err := repo.Create(ctx, cat); err != nil {
			t.Fatalf("Create: %v", err)
		}

		list, err := repo.ListByUser(ctx, "someone-else")
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		for _, c := range list {
			if c.ID == cat.ID {
				t.Errorf("ListByUser(someone-else) leaked owner's category %s", cat.ID)
			}
		}
	})

	t.Run("EnsureIndexes actually created the documented user_id index", func(t *testing.T) {
		cursor, err := db.Collection("categories").Indexes().List(ctx)
		if err != nil {
			t.Fatalf("Indexes().List: %v", err)
		}
		defer cursor.Close(ctx)

		var found bool
		for cursor.Next(ctx) {
			var idx struct {
				Name string `bson:"name"`
			}
			if err := cursor.Decode(&idx); err != nil {
				t.Fatalf("decode index: %v", err)
			}
			if idx.Name == "user_id_1" {
				found = true
			}
		}
		if !found {
			t.Error("expected an index named user_id_1 on categories — EnsureIndexes did not create it")
		}
	})
}
