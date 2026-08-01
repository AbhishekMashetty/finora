package service

import (
	"context"
	"testing"

	"github.com/finora/notification-service/internal/domain"
)

func TestOverspendConsumer_HandleBudgetOverspent_CreatesANotification(t *testing.T) {
	repo := &fakeRepository{}
	email := &fakeEmailSender{}
	notifications := NewNotificationService(repo, email, discardLogger())
	consumer := NewOverspendConsumer(notifications)

	event := domain.BudgetOverspentEvent{
		BudgetID:    "b1",
		UserID:      "user-1",
		Category:    "groceries",
		Period:      "monthly",
		Budgeted:    100,
		Actual:      150,
		PeriodStart: "2026-01-01T00:00:00Z",
	}

	if err := consumer.HandleBudgetOverspent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	page, err := notifications.List(context.Background(), "user-1", domain.NotificationFilter{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(page.Notifications) != 1 {
		t.Fatalf("expected 1 notification created, got %d", len(page.Notifications))
	}

	n := page.Notifications[0]
	if n.Type != "overspend" {
		t.Errorf("Type = %q, want %q", n.Type, "overspend")
	}
	if n.Title != "Budget exceeded" {
		t.Errorf("Title = %q, want %q", n.Title, "Budget exceeded")
	}
	wantMessage := "You've spent 150.00 of your 100.00 monthly groceries budget."
	if n.Message != wantMessage {
		t.Errorf("Message = %q, want %q", n.Message, wantMessage)
	}
}

func TestOverspendConsumer_HandleBudgetOverspent_PropagatesCreateErrors(t *testing.T) {
	repo := &fakeRepository{}
	repo.createErr = context.DeadlineExceeded
	notifications := NewNotificationService(repo, &fakeEmailSender{}, discardLogger())
	consumer := NewOverspendConsumer(notifications)

	err := consumer.HandleBudgetOverspent(context.Background(), domain.BudgetOverspentEvent{
		UserID: "user-1", Category: "groceries", Period: "monthly", Budgeted: 100, Actual: 150,
	})
	if err == nil {
		t.Fatal("expected an error to propagate when notification creation fails")
	}
}
