// Package service implements domain's service interfaces. It depends only
// on domain's repository interfaces (never a concrete Mongo type), so it can
// be unit-tested against hand-written fakes with no live database.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/finora/shared/jwtx"
	"github.com/finora/user-service/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	users    domain.UserRepository
	settings domain.SettingsRepository
	tokens   domain.RefreshTokenRepository

	accessSecret  string
	refreshSecret string
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// NewAuthService constructs domain.AuthService from repository interfaces
// and the JWT secrets/TTLs read from env by internal/config.
func NewAuthService(
	users domain.UserRepository,
	settings domain.SettingsRepository,
	tokens domain.RefreshTokenRepository,
	accessSecret, refreshSecret string,
	accessTTL, refreshTTL time.Duration,
) domain.AuthService {
	return &authService{
		users:         users,
		settings:      settings,
		tokens:        tokens,
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func hashJTI(jti string) string {
	sum := sha256.Sum256([]byte(jti))
	return hex.EncodeToString(sum[:])
}

func (s *authService) Register(ctx context.Context, in domain.RegisterInput) (*domain.User, error) {
	email := normalizeEmail(in.Email)

	existing, err := s.users.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrDuplicateEmail
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	u := &domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		Name:         in.Name,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	// Seed default settings so GET /users/me/settings never 404s for a
	// freshly registered user.
	if err := s.settings.Upsert(ctx, &domain.Settings{
		UserID:    u.ID,
		Currency:  "USD",
		Timezone:  "UTC",
		UpdatedAt: now,
	}); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*domain.LoginResult, error) {
	u, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	return s.issueTokenPair(ctx, u)
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*domain.LoginResult, error) {
	claims, err := jwtx.Parse(s.refreshSecret, refreshToken, jwtx.TypeRefresh)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	rt, err := s.tokens.FindByJTIHash(ctx, hashJTI(claims.JTI))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}
	if rt.Revoked {
		// A request presenting an already-revoked (already-rotated-away)
		// refresh token is evidence of token theft/replay: the legitimate
		// client already moved on to the next token in the rotation chain,
		// so anyone still presenting this old one is not the legitimate
		// client. Contain the blast radius by revoking every refresh token
		// for this user, forcing full re-authentication on all
		// devices/sessions — not just failing this one request. The
		// client-visible response is unchanged (still just ErrInvalidToken).
		if err := s.tokens.RevokeAllForUser(ctx, rt.UserID); err != nil {
			return nil, err
		}
		return nil, domain.ErrInvalidToken
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, domain.ErrInvalidToken
	}

	u, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}

	// Rotate: revoke the presented token before issuing a new pair so a
	// replayed old refresh token can never succeed twice.
	if err := s.tokens.Revoke(ctx, rt.ID); err != nil {
		return nil, err
	}

	return s.issueTokenPair(ctx, u)
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	claims, err := jwtx.Parse(s.refreshSecret, refreshToken, jwtx.TypeRefresh)
	if err != nil {
		// Malformed/expired token: nothing to revoke, but logout is
		// idempotent and must not leak whether the token was ever valid.
		return nil
	}

	rt, err := s.tokens.FindByJTIHash(ctx, hashJTI(claims.JTI))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	if rt.Revoked {
		return nil
	}

	return s.tokens.Revoke(ctx, rt.ID)
}

// issueTokenPair generates a fresh access+refresh token pair for u, persists
// the new refresh token's hashed JTI, and returns both tokens.
func (s *authService) issueTokenPair(ctx context.Context, u *domain.User) (*domain.LoginResult, error) {
	jti := uuid.NewString()

	access, err := jwtx.GenerateAccessToken(s.accessSecret, u.ID, u.Email, s.accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := jwtx.GenerateRefreshToken(s.refreshSecret, u.ID, jti, s.refreshTTL)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	rt := &domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		JTIHash:   hashJTI(jti),
		ExpiresAt: now.Add(s.refreshTTL),
		Revoked:   false,
		CreatedAt: now,
	}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, err
	}

	return &domain.LoginResult{
		AccessToken:  access,
		RefreshToken: refresh,
		User:         u,
	}, nil
}

// RequestPasswordReset is a Phase 1 stub (see domain.AuthService for the
// full rationale): it looks the user up only to keep the shape of a real
// implementation, but always returns nil regardless of whether the email
// is registered, so the endpoint can never be used to enumerate accounts.
// No reset token is issued and no email is sent yet.
func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	_, _ = s.users.FindByEmail(ctx, normalizeEmail(email))
	return nil
}
