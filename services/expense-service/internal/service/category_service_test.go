package service_test

import (
	"context"
	"testing"

	"github.com/finora/expense-service/internal/domain"
	"github.com/finora/expense-service/internal/service"
)

func TestCategoryService_CreateAndList(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "user-1", domain.CreateCategoryInput{Name: "Groceries", Type: domain.TransactionTypeExpense}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.Create(ctx, "user-2", domain.CreateCategoryInput{Name: "Salary", Type: domain.TransactionTypeIncome}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cats, err := svc.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 1 || cats[0].Name != "Groceries" {
		t.Fatalf("expected only user-1's category, got %+v", cats)
	}
}

func TestCategoryService_Create_Validation(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "user-1", domain.CreateCategoryInput{Name: "", Type: domain.TransactionTypeExpense}); err == nil {
		t.Fatalf("expected validation error for empty name")
	}
	if _, err := svc.Create(ctx, "user-1", domain.CreateCategoryInput{Name: "Bad", Type: domain.TransactionType("weird")}); err == nil {
		t.Fatalf("expected validation error for invalid type")
	}
}
