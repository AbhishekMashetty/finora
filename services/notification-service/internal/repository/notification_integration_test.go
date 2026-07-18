//go:build integration

// Real-Mongo integration tests (CLAUDE.md §7's Phase 6 seam). Excluded from
// the default `go test ./...` run; run via `go test -tags=integration
// ./...` (see the root Makefile's `test-integration` target).
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/finora/notification-service/internal/domain"
	"github.com/finora/notification-service/internal/repository"
	"github.com/finora/shared/mongotest"
)

func TestNotificationRepository_Integration(t *testing.T) {
	client := mongotest.StartClient(t)
	db := client.Database("notification_service_test")
	if err := repository.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	repo := repository.NewMongoNotificationRepository(db)
	ctx := context.Background()

	t.Run("create then list round-trips through real Mongo, newest first", func(t *testing.T) {
		// CreatedAt is set by the *service* layer (notification_service.go's
		// Create), not the repository — calling the repository directly, as
		// this test does, means CreatedAt must be set explicitly here, or
		// both documents get Go's zero-value timestamp and the sort order
		// between them is genuinely undefined (caught live: the first draft
		// of this test omitted it and failed intermittently on tie-broken
		// ordering, not because ListByUser's sort is wrong).
		userID := "user-1"
		now := time.Now().UTC()
		first := &domain.Notification{UserID: userID, Title: "First", Message: "m1", Type: "overspend", CreatedAt: now}
		if err := repo.Create(ctx, first); err != nil {
			t.Fatalf("Create(first): %v", err)
		}
		second := &domain.Notification{UserID: userID, Title: "Second", Message: "m2", Type: "overspend", CreatedAt: now.Add(time.Second)}
		if err := repo.Create(ctx, second); err != nil {
			t.Fatalf("Create(second): %v", err)
		}
		if first.ID == "" || second.ID == "" {
			t.Fatal("Create did not populate an ID")
		}

		list, err := repo.ListByUser(ctx, userID, false)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		if len(list) < 2 {
			t.Fatalf("ListByUser returned %d notifications, want at least 2", len(list))
		}
		if list[0].Title != "Second" {
			t.Errorf("list[0].Title = %q, want %q (newest first, per ListByUser's SetSort)", list[0].Title, "Second")
		}
	})

	t.Run("unreadOnly filters at the real Mongo query level", func(t *testing.T) {
		userID := "user-2"
		unread := &domain.Notification{UserID: userID, Title: "Unread", Message: "m", Type: "overspend"}
		if err := repo.Create(ctx, unread); err != nil {
			t.Fatalf("Create: %v", err)
		}
		read := &domain.Notification{UserID: userID, Title: "Read", Message: "m", Type: "overspend"}
		if err := repo.Create(ctx, read); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := repo.MarkRead(ctx, userID, read.ID); err != nil {
			t.Fatalf("MarkRead: %v", err)
		}

		unreadOnly, err := repo.ListByUser(ctx, userID, true)
		if err != nil {
			t.Fatalf("ListByUser(unreadOnly=true): %v", err)
		}
		for _, n := range unreadOnly {
			if n.Read {
				t.Errorf("ListByUser(unreadOnly=true) returned a read notification: %+v", n)
			}
			if n.ID == read.ID {
				t.Errorf("ListByUser(unreadOnly=true) leaked the already-read notification %s", read.ID)
			}
		}

		all, err := repo.ListByUser(ctx, userID, false)
		if err != nil {
			t.Fatalf("ListByUser(unreadOnly=false): %v", err)
		}
		if len(all) != len(unreadOnly)+1 {
			t.Errorf("ListByUser(false) returned %d, want %d (unread) + 1 (the read one)", len(all), len(unreadOnly))
		}
	})

	t.Run("MarkRead is scoped by user_id at the real Mongo query — a cross-user mark-read is ErrNotFound and does not mutate the document", func(t *testing.T) {
		n := &domain.Notification{UserID: "owner", Title: "Owner's", Message: "m", Type: "overspend"}
		if err := repo.Create(ctx, n); err != nil {
			t.Fatalf("Create: %v", err)
		}

		if _, err := repo.MarkRead(ctx, "someone-else", n.ID); err != domain.ErrNotFound {
			t.Errorf("cross-user MarkRead: err = %v, want ErrNotFound", err)
		}

		list, err := repo.ListByUser(ctx, "owner", false)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		for _, x := range list {
			if x.ID == n.ID && x.Read {
				t.Error("cross-user MarkRead must not mutate the document, but it was marked read anyway")
			}
		}
	})

	t.Run("MarkRead on the owner actually persists read=true", func(t *testing.T) {
		n := &domain.Notification{UserID: "user-3", Title: "To read", Message: "m", Type: "overspend"}
		if err := repo.Create(ctx, n); err != nil {
			t.Fatalf("Create: %v", err)
		}

		updated, err := repo.MarkRead(ctx, "user-3", n.ID)
		if err != nil {
			t.Fatalf("MarkRead: %v", err)
		}
		if !updated.Read {
			t.Error("MarkRead's returned document has Read=false, want true")
		}

		list, err := repo.ListByUser(ctx, "user-3", true)
		if err != nil {
			t.Fatalf("ListByUser(unreadOnly=true): %v", err)
		}
		for _, x := range list {
			if x.ID == n.ID {
				t.Error("the just-marked-read notification still appears in the unread-only list")
			}
		}
	})

	t.Run("EnsureIndexes created the documented compound (user_id, read) index", func(t *testing.T) {
		cursor, err := db.Collection("notifications").Indexes().List(ctx)
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
			if idx.Name == "user_id_1_read_1" {
				found = true
			}
		}
		if !found {
			t.Error("expected compound index user_id_1_read_1 on notifications — EnsureIndexes did not create it")
		}
	})
}
