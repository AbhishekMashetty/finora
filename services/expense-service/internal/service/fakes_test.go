package service_test

import (
	"context"
	"fmt"

	"github.com/finora/expense-service/internal/domain"
)

// fakeAccountRepository is a hand-written in-memory stand-in for
// domain.AccountRepository, used so the service layer is unit-testable
// without a live MongoDB.
type fakeAccountRepository struct {
	accounts map[string]domain.Account
	nextID   int
}

func newFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{accounts: make(map[string]domain.Account)}
}

func (f *fakeAccountRepository) Create(_ context.Context, account *domain.Account) error {
	f.nextID++
	id := idFromInt(f.nextID)
	account.ID = id
	f.accounts[id] = *account
	return nil
}

func (f *fakeAccountRepository) ListByUser(_ context.Context, userID string) ([]domain.Account, error) {
	out := make([]domain.Account, 0)
	for _, a := range f.accounts {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAccountRepository) GetByIDForUser(_ context.Context, id, userID string) (*domain.Account, error) {
	a, ok := f.accounts[id]
	if !ok || a.UserID != userID {
		return nil, domain.ErrNotFound
	}
	cp := a
	return &cp, nil
}

func (f *fakeAccountRepository) Update(_ context.Context, account *domain.Account) error {
	existing, ok := f.accounts[account.ID]
	if !ok || existing.UserID != account.UserID {
		return domain.ErrNotFound
	}
	f.accounts[account.ID] = *account
	return nil
}

func (f *fakeAccountRepository) DeleteByIDForUser(_ context.Context, id, userID string) error {
	existing, ok := f.accounts[id]
	if !ok || existing.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.accounts, id)
	return nil
}

// fakeTransactionRepository is a hand-written in-memory stand-in for
// domain.TransactionRepository.
type fakeTransactionRepository struct {
	transactions map[string]domain.Transaction
	nextID       int
}

func newFakeTransactionRepository() *fakeTransactionRepository {
	return &fakeTransactionRepository{transactions: make(map[string]domain.Transaction)}
}

func (f *fakeTransactionRepository) Create(_ context.Context, tx *domain.Transaction) error {
	f.nextID++
	id := idFromInt(f.nextID)
	tx.ID = id
	f.transactions[id] = *tx
	return nil
}

func (f *fakeTransactionRepository) ListByUser(_ context.Context, userID string, filter domain.TransactionFilter) (domain.TransactionPage, error) {
	matches := make([]domain.Transaction, 0)
	for _, t := range f.transactions {
		if t.UserID != userID {
			continue
		}
		if filter.AccountID != "" && t.AccountID != filter.AccountID {
			continue
		}
		if filter.Category != "" && (t.CategoryID == nil || *t.CategoryID != filter.Category) {
			continue
		}
		if filter.From != nil && t.Date.Before(*filter.From) {
			continue
		}
		if filter.To != nil && t.Date.After(*filter.To) {
			continue
		}
		matches = append(matches, t)
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

	return domain.TransactionPage{Transactions: matches[start:end], Total: total}, nil
}

func (f *fakeTransactionRepository) GetByIDForUser(_ context.Context, id, userID string) (*domain.Transaction, error) {
	t, ok := f.transactions[id]
	if !ok || t.UserID != userID {
		return nil, domain.ErrNotFound
	}
	cp := t
	return &cp, nil
}

func (f *fakeTransactionRepository) Update(_ context.Context, tx *domain.Transaction) error {
	existing, ok := f.transactions[tx.ID]
	if !ok || existing.UserID != tx.UserID {
		return domain.ErrNotFound
	}
	f.transactions[tx.ID] = *tx
	return nil
}

func (f *fakeTransactionRepository) DeleteByIDForUser(_ context.Context, id, userID string) error {
	existing, ok := f.transactions[id]
	if !ok || existing.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.transactions, id)
	return nil
}

// fakeCategoryRepository is a hand-written in-memory stand-in for
// domain.CategoryRepository.
type fakeCategoryRepository struct {
	categories map[string]domain.Category
	nextID     int
}

func newFakeCategoryRepository() *fakeCategoryRepository {
	return &fakeCategoryRepository{categories: make(map[string]domain.Category)}
}

func (f *fakeCategoryRepository) Create(_ context.Context, category *domain.Category) error {
	f.nextID++
	id := idFromInt(f.nextID)
	category.ID = id
	f.categories[id] = *category
	return nil
}

func (f *fakeCategoryRepository) ListByUser(_ context.Context, userID string) ([]domain.Category, error) {
	out := make([]domain.Category, 0)
	for _, cat := range f.categories {
		if cat.UserID == userID {
			out = append(out, cat)
		}
	}
	return out, nil
}

func idFromInt(n int) string {
	return fmt.Sprintf("id-%d", n)
}
