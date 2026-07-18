package service

import (
	"context"

	"github.com/finora/user-service/internal/domain"
)

// fakeUserRepository is an in-memory domain.UserRepository for unit tests —
// no live Mongo, no testcontainers.
type fakeUserRepository struct {
	byID    map[string]*domain.User
	byEmail map[string]*domain.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		byID:    make(map[string]*domain.User),
		byEmail: make(map[string]*domain.User),
	}
}

func (f *fakeUserRepository) Create(ctx context.Context, u *domain.User) error {
	if _, ok := f.byEmail[u.Email]; ok {
		return domain.ErrDuplicateEmail
	}
	cp := *u
	f.byID[u.ID] = &cp
	f.byEmail[u.Email] = &cp
	return nil
}

func (f *fakeUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUserRepository) Update(ctx context.Context, u *domain.User) error {
	if _, ok := f.byID[u.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *u
	f.byID[u.ID] = &cp
	f.byEmail[u.Email] = &cp
	return nil
}

// fakeSettingsRepository is an in-memory domain.SettingsRepository.
type fakeSettingsRepository struct {
	byUserID map[string]*domain.Settings
}

func newFakeSettingsRepository() *fakeSettingsRepository {
	return &fakeSettingsRepository{byUserID: make(map[string]*domain.Settings)}
}

func (f *fakeSettingsRepository) FindByUserID(ctx context.Context, userID string) (*domain.Settings, error) {
	s, ok := f.byUserID[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeSettingsRepository) Upsert(ctx context.Context, s *domain.Settings) error {
	cp := *s
	f.byUserID[s.UserID] = &cp
	return nil
}

// fakeRefreshTokenRepository is an in-memory domain.RefreshTokenRepository.
type fakeRefreshTokenRepository struct {
	byID   map[string]*domain.RefreshToken
	byHash map[string]*domain.RefreshToken
}

func newFakeRefreshTokenRepository() *fakeRefreshTokenRepository {
	return &fakeRefreshTokenRepository{
		byID:   make(map[string]*domain.RefreshToken),
		byHash: make(map[string]*domain.RefreshToken),
	}
}

func (f *fakeRefreshTokenRepository) Create(ctx context.Context, rt *domain.RefreshToken) error {
	cp := *rt
	f.byID[rt.ID] = &cp
	f.byHash[rt.JTIHash] = &cp
	return nil
}

func (f *fakeRefreshTokenRepository) FindByJTIHash(ctx context.Context, jtiHash string) (*domain.RefreshToken, error) {
	rt, ok := f.byHash[jtiHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *rt
	return &cp, nil
}

func (f *fakeRefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	rt, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	rt.Revoked = true
	f.byHash[rt.JTIHash].Revoked = true
	return nil
}

// RevokeAllForUser revokes every token record belonging to userID. byID and
// byHash share the same underlying *domain.RefreshToken per record (see
// Create), so mutating via byID is visible through byHash too.
func (f *fakeRefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	for _, rt := range f.byID {
		if rt.UserID == userID {
			rt.Revoked = true
		}
	}
	return nil
}
