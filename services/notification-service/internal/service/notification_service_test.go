package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/finora/notification-service/internal/domain"
)

// discardLogger is a *slog.Logger that writes nowhere, for tests that don't
// care about log output but need a non-nil logger passed to
// NewNotificationService.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeRepository is a hand-written in-memory fake of domain.NotificationRepository,
// scoped by user id just like the real Mongo implementation, so tests never
// need a live Mongo / testcontainers.
type fakeRepository struct {
	notifications []domain.Notification
	nextID        int
	createErr     error
	listErr       error
}

func (f *fakeRepository) Create(_ context.Context, n *domain.Notification) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	n.ID = itoa(f.nextID)
	f.notifications = append(f.notifications, *n)
	return nil
}

func (f *fakeRepository) ListByUser(_ context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error) {
	if f.listErr != nil {
		return domain.NotificationPage{}, f.listErr
	}
	matches := make([]domain.Notification, 0)
	for _, n := range f.notifications {
		if n.UserID != userID {
			continue
		}
		if filter.UnreadOnly && n.Read {
			continue
		}
		matches = append(matches, n)
	}

	total := int64(len(matches))
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > len(matches) {
		start = len(matches)
	}
	end := start + pageSize
	if end > len(matches) {
		end = len(matches)
	}

	return domain.NotificationPage{Notifications: matches[start:end], Total: total}, nil
}

func (f *fakeRepository) MarkRead(_ context.Context, userID, id string) (*domain.Notification, error) {
	for i := range f.notifications {
		if f.notifications[i].ID == id && f.notifications[i].UserID == userID {
			f.notifications[i].Read = true
			result := f.notifications[i]
			return &result, nil
		}
	}
	return nil, domain.ErrNotFound
}

// itoa avoids importing strconv just for test IDs.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// fakeEmailSender records calls (and their arguments) so tests can verify
// Create invokes the email seam, and can be made to fail so tests can
// verify a Send error never fails Create's own result.
type fakeEmailSender struct {
	calls   int
	lastTo  string
	lastSub string
	lastMsg string
	err     error
}

func (f *fakeEmailSender) Send(_ context.Context, to, subject, body string) error {
	f.calls++
	f.lastTo, f.lastSub, f.lastMsg = to, subject, body
	if f.err != nil {
		return f.err
	}
	return nil
}

func TestNotificationService_Create(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		title     string
		message   string
		notifType string
		wantErr   bool
	}{
		{
			name:      "creates a notification for the caller",
			userID:    "user-1",
			title:     "Budget exceeded",
			message:   "You have exceeded your groceries budget",
			notifType: "budget_alert",
			wantErr:   false,
		},
		{
			name:      "creates a second notification type",
			userID:    "user-2",
			title:     "Welcome",
			message:   "Thanks for signing up",
			notifType: "info",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			email := &fakeEmailSender{}
			svc := NewNotificationService(repo, email, discardLogger())

			got, err := svc.Create(context.Background(), tt.userID, tt.title, tt.message, tt.notifType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if got.UserID != tt.userID {
				t.Errorf("UserID = %q, want %q", got.UserID, tt.userID)
			}
			if got.Title != tt.title {
				t.Errorf("Title = %q, want %q", got.Title, tt.title)
			}
			if got.Message != tt.message {
				t.Errorf("Message = %q, want %q", got.Message, tt.message)
			}
			if got.Type != tt.notifType {
				t.Errorf("Type = %q, want %q", got.Type, tt.notifType)
			}
			if got.Read {
				t.Errorf("Read = true, want false for a freshly created notification")
			}
			if got.ID == "" {
				t.Errorf("ID was not populated by repository.Create")
			}
			if got.CreatedAt.IsZero() {
				t.Errorf("CreatedAt was not set")
			}

			// Since Phase 4, Create invokes the email seam as a best-effort
			// side effect after a successful repo.Create.
			if email.calls != 1 {
				t.Errorf("email seam was invoked %d times, want 1", email.calls)
			}
			if email.lastTo != tt.userID {
				t.Errorf("email 'to' = %q, want %q (userID is used as a placeholder recipient — see Create's doc comment)", email.lastTo, tt.userID)
			}
			if email.lastSub != tt.title {
				t.Errorf("email subject = %q, want %q", email.lastSub, tt.title)
			}
			if email.lastMsg != tt.message {
				t.Errorf("email body = %q, want %q", email.lastMsg, tt.message)
			}
		})
	}
}

// TestNotificationService_Create_EmailSendErrorDoesNotFailCreate proves
// email is genuinely best-effort: a Send error must never surface as a
// Create error, since the notification itself was already persisted
// successfully by repo.Create.
func TestNotificationService_Create_EmailSendErrorDoesNotFailCreate(t *testing.T) {
	repo := &fakeRepository{}
	email := &fakeEmailSender{err: errors.New("smtp: connection refused")}
	svc := NewNotificationService(repo, email, discardLogger())

	got, err := svc.Create(context.Background(), "user-1", "Budget exceeded", "You have exceeded your groceries budget", "overspend")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil (email errors must not fail Create)", err)
	}
	if got.ID == "" {
		t.Errorf("expected the notification to still be persisted despite the email error")
	}
	if email.calls != 1 {
		t.Errorf("email.calls = %d, want 1 (Send should still have been attempted)", email.calls)
	}
}

func TestNotificationService_List(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewNotificationService(repo, &fakeEmailSender{}, discardLogger())
	ctx := context.Background()

	// Seed notifications across two different users to verify scoping.
	if _, err := svc.Create(ctx, "user-1", "A", "msg-a", "info"); err != nil {
		t.Fatalf("seed Create failed: %v", err)
	}
	if _, err := svc.Create(ctx, "user-1", "B", "msg-b", "info"); err != nil {
		t.Fatalf("seed Create failed: %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", "C", "msg-c", "info"); err != nil {
		t.Fatalf("seed Create failed: %v", err)
	}

	tests := []struct {
		name       string
		userID     string
		unreadOnly bool
		wantCount  int
		wantTitles []string
	}{
		{
			name:       "lists only the caller's own notifications",
			userID:     "user-1",
			unreadOnly: false,
			wantCount:  2,
			wantTitles: []string{"A", "B"},
		},
		{
			name:       "a different user only sees their own notification",
			userID:     "user-2",
			unreadOnly: false,
			wantCount:  1,
			wantTitles: []string{"C"},
		},
		{
			name:       "a user with no notifications gets an empty (not nil) slice",
			userID:     "user-3",
			unreadOnly: false,
			wantCount:  0,
			wantTitles: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := svc.List(ctx, tt.userID, domain.NotificationFilter{UnreadOnly: tt.unreadOnly})
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			got := page.Notifications
			if got == nil {
				t.Errorf("List() returned nil, want a non-nil (possibly empty) slice")
			}
			if len(got) != tt.wantCount {
				t.Fatalf("List() returned %d notifications, want %d", len(got), tt.wantCount)
			}
			if int(page.Total) != tt.wantCount {
				t.Errorf("List() Total = %d, want %d", page.Total, tt.wantCount)
			}
			if page.Page != 1 {
				t.Errorf("List() resolved Page = %d, want 1 (default)", page.Page)
			}
			if page.PageSize != 20 {
				t.Errorf("List() resolved PageSize = %d, want 20 (default)", page.PageSize)
			}
			for i, title := range tt.wantTitles {
				if got[i].Title != title {
					t.Errorf("notification[%d].Title = %q, want %q", i, got[i].Title, title)
				}
				if got[i].UserID != tt.userID {
					t.Errorf("notification[%d].UserID = %q, want %q (leaked cross-user)", i, got[i].UserID, tt.userID)
				}
			}
		})
	}
}

func TestNotificationService_List_UnreadOnlyFilter(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewNotificationService(repo, &fakeEmailSender{}, discardLogger())
	ctx := context.Background()

	first, err := svc.Create(ctx, "user-1", "Unread one", "msg", "info")
	if err != nil {
		t.Fatalf("seed Create failed: %v", err)
	}
	if _, err := svc.Create(ctx, "user-1", "Unread two", "msg", "info"); err != nil {
		t.Fatalf("seed Create failed: %v", err)
	}

	if _, err := svc.MarkRead(ctx, "user-1", first.ID); err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}

	allPage, err := svc.List(ctx, "user-1", domain.NotificationFilter{UnreadOnly: false})
	if err != nil {
		t.Fatalf("List(unreadOnly=false) error = %v", err)
	}
	if len(allPage.Notifications) != 2 {
		t.Fatalf("List(unreadOnly=false) returned %d, want 2", len(allPage.Notifications))
	}

	unreadPage, err := svc.List(ctx, "user-1", domain.NotificationFilter{UnreadOnly: true})
	if err != nil {
		t.Fatalf("List(unreadOnly=true) error = %v", err)
	}
	unread := unreadPage.Notifications
	if len(unread) != 1 {
		t.Fatalf("List(unreadOnly=true) returned %d, want 1", len(unread))
	}
	if unread[0].Title != "Unread two" {
		t.Errorf("unread[0].Title = %q, want %q", unread[0].Title, "Unread two")
	}
	if unread[0].Read {
		t.Errorf("unread[0].Read = true, want false")
	}
}

func TestNotificationService_List_Pagination(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewNotificationService(repo, &fakeEmailSender{}, discardLogger())
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		if _, err := svc.Create(ctx, "user-1", itoa(i), "msg", "info"); err != nil {
			t.Fatalf("seed Create %d failed: %v", i, err)
		}
	}

	page1, err := svc.List(ctx, "user-1", domain.NotificationFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List(page=1, page_size=10) error = %v", err)
	}
	if len(page1.Notifications) != 10 || page1.Total != 25 || page1.Page != 1 || page1.PageSize != 10 {
		t.Errorf("page1 = %+v, want 10 notifications, Total=25, Page=1, PageSize=10", page1)
	}

	page3, err := svc.List(ctx, "user-1", domain.NotificationFilter{Page: 3, PageSize: 10})
	if err != nil {
		t.Fatalf("List(page=3, page_size=10) error = %v", err)
	}
	if len(page3.Notifications) != 5 {
		t.Errorf("page3 = %d notifications, want 5 (the remainder)", len(page3.Notifications))
	}

	// A page_size beyond the service's cap must be clamped, not passed
	// through as-is — the same maxPageSize convention transaction_service.go
	// uses.
	capped, err := svc.List(ctx, "user-1", domain.NotificationFilter{Page: 1, PageSize: 1000})
	if err != nil {
		t.Fatalf("List(page_size=1000) error = %v", err)
	}
	if capped.PageSize != maxPageSize {
		t.Errorf("resolved PageSize = %d, want the cap of %d", capped.PageSize, maxPageSize)
	}
}

func TestNotificationService_MarkRead(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewNotificationService(repo, &fakeEmailSender{}, discardLogger())
	ctx := context.Background()

	created, err := svc.Create(ctx, "user-1", "Title", "msg", "info")
	if err != nil {
		t.Fatalf("seed Create failed: %v", err)
	}

	tests := []struct {
		name     string
		userID   string
		id       string
		wantErr  error
		wantRead bool
	}{
		{
			name:     "marks the caller's own notification as read",
			userID:   "user-1",
			id:       created.ID,
			wantErr:  nil,
			wantRead: true,
		},
		{
			name:    "a different user cannot mark someone else's notification as read",
			userID:  "user-2",
			id:      created.ID,
			wantErr: domain.ErrNotFound,
		},
		{
			name:    "unknown id returns not found",
			userID:  "user-1",
			id:      "does-not-exist",
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.MarkRead(ctx, tt.userID, tt.id)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("MarkRead() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MarkRead() unexpected error = %v", err)
			}
			if got.Read != tt.wantRead {
				t.Errorf("Read = %v, want %v", got.Read, tt.wantRead)
			}
		})
	}
}
