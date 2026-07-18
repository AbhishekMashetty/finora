package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/finora/shared/jwtx"
	"github.com/finora/user-service/internal/domain"
)

const (
	testAccessSecret  = "test-access-secret"
	testRefreshSecret = "test-refresh-secret"
	testAccessTTL     = 15 * time.Minute
	testRefreshTTL    = 168 * time.Hour
)

func newTestAuthService() (domain.AuthService, *fakeUserRepository, *fakeSettingsRepository, *fakeRefreshTokenRepository) {
	users := newFakeUserRepository()
	settings := newFakeSettingsRepository()
	tokens := newFakeRefreshTokenRepository()
	svc := NewAuthService(users, settings, tokens, testAccessSecret, testRefreshSecret, testAccessTTL, testRefreshTTL)
	return svc, users, settings, tokens
}

func TestRegister(t *testing.T) {
	t.Run("success creates user and default settings", func(t *testing.T) {
		svc, _, settings, _ := newTestAuthService()

		u, err := svc.Register(context.Background(), domain.RegisterInput{
			Email:    "Alice@Example.com",
			Password: "supersecret1",
			Name:     "Alice",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Email != "alice@example.com" {
			t.Errorf("expected normalized email, got %q", u.Email)
		}
		if u.PasswordHash == "" || u.PasswordHash == "supersecret1" {
			t.Errorf("expected password to be hashed, got %q", u.PasswordHash)
		}

		s, err := settings.FindByUserID(context.Background(), u.ID)
		if err != nil {
			t.Fatalf("expected default settings to be seeded, got err: %v", err)
		}
		if s.Currency != "USD" || s.Timezone != "UTC" {
			t.Errorf("expected USD/UTC defaults, got %q/%q", s.Currency, s.Timezone)
		}
	})

	t.Run("duplicate email returns ErrDuplicateEmail", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()
		ctx := context.Background()

		in := domain.RegisterInput{Email: "bob@example.com", Password: "supersecret1", Name: "Bob"}
		if _, err := svc.Register(ctx, in); err != nil {
			t.Fatalf("first registration should succeed: %v", err)
		}

		_, err := svc.Register(ctx, in)
		if !errors.Is(err, domain.ErrDuplicateEmail) {
			t.Fatalf("expected ErrDuplicateEmail, got %v", err)
		}
	})

	t.Run("duplicate email case-insensitive returns ErrDuplicateEmail", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "carol@example.com", Password: "supersecret1", Name: "Carol"}); err != nil {
			t.Fatalf("first registration should succeed: %v", err)
		}

		_, err := svc.Register(ctx, domain.RegisterInput{Email: "Carol@Example.com", Password: "anotherpass1", Name: "Carol 2"})
		if !errors.Is(err, domain.ErrDuplicateEmail) {
			t.Fatalf("expected ErrDuplicateEmail, got %v", err)
		}
	})
}

func TestLogin(t *testing.T) {
	t.Run("wrong password returns ErrInvalidCredentials", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "dave@example.com", Password: "correcthorse1", Name: "Dave"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}

		_, err := svc.Login(ctx, "dave@example.com", "wrongpassword")
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("unknown email returns ErrInvalidCredentials", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()

		_, err := svc.Login(context.Background(), "nobody@example.com", "whatever123")
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("success generates valid access and refresh tokens", func(t *testing.T) {
		svc, _, _, tokens := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "erin@example.com", Password: "correcthorse1", Name: "Erin"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}

		result, err := svc.Login(ctx, "erin@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.AccessToken == "" || result.RefreshToken == "" {
			t.Fatal("expected non-empty tokens")
		}
		if result.User.Email != "erin@example.com" {
			t.Errorf("unexpected user in result: %+v", result.User)
		}

		accessClaims, err := jwtx.Parse(testAccessSecret, result.AccessToken, jwtx.TypeAccess)
		if err != nil {
			t.Fatalf("access token should parse as access type: %v", err)
		}
		if accessClaims.UserID != result.User.ID {
			t.Errorf("access token sub mismatch: got %q want %q", accessClaims.UserID, result.User.ID)
		}

		refreshClaims, err := jwtx.Parse(testRefreshSecret, result.RefreshToken, jwtx.TypeRefresh)
		if err != nil {
			t.Fatalf("refresh token should parse as refresh type: %v", err)
		}
		if refreshClaims.JTI == "" {
			t.Fatal("expected refresh token to carry a jti")
		}

		if len(tokens.byID) != 1 {
			t.Fatalf("expected exactly one persisted refresh token record, got %d", len(tokens.byID))
		}
	})
}

func TestRefresh(t *testing.T) {
	t.Run("valid token rotates and issues a new pair", func(t *testing.T) {
		svc, _, _, tokens := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "frank@example.com", Password: "correcthorse1", Name: "Frank"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}
		login, err := svc.Login(ctx, "frank@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}

		result, err := svc.Refresh(ctx, login.RefreshToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.RefreshToken == login.RefreshToken {
			t.Error("expected a newly rotated refresh token, got the same one back")
		}
		if result.AccessToken == "" {
			t.Error("expected a new access token")
		}

		// The original token's record must now be revoked.
		oldClaims, err := jwtx.Parse(testRefreshSecret, login.RefreshToken, jwtx.TypeRefresh)
		if err != nil {
			t.Fatalf("failed to parse old refresh token: %v", err)
		}
		oldRecord, err := tokens.FindByJTIHash(ctx, hashJTI(oldClaims.JTI))
		if err != nil {
			t.Fatalf("expected old token record to still exist: %v", err)
		}
		if !oldRecord.Revoked {
			t.Error("expected old refresh token record to be revoked after rotation")
		}

		// Before any replay, the newly issued refresh token works normally.
		second, err := svc.Refresh(ctx, result.RefreshToken)
		if err != nil {
			t.Fatalf("expected new refresh token to be usable, got %v", err)
		}

		// Reusing the old (now revoked) refresh token must fail. This also
		// triggers reuse-detection containment (see the dedicated
		// "revoked token reuse revokes every refresh token for the user"
		// test below), which revokes every other refresh token for this
		// user — including the one just issued by the successful refresh
		// above — as the deliberate security response.
		if _, err := svc.Refresh(ctx, login.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken reusing a rotated token, got %v", err)
		}
		if _, err := svc.Refresh(ctx, second.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected the latest refresh token to be revoked too, as the containment response to the detected replay, got %v", err)
		}
	})

	t.Run("garbage token rejected", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()

		_, err := svc.Refresh(context.Background(), "not-a-real-token")
		if !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("access token used as refresh token rejected", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "grace@example.com", Password: "correcthorse1", Name: "Grace"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}
		login, err := svc.Login(ctx, "grace@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}

		_, err = svc.Refresh(ctx, login.AccessToken)
		if !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken using an access token as a refresh token, got %v", err)
		}
	})

	t.Run("revoked token rejected", func(t *testing.T) {
		svc, _, _, tokens := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "heidi@example.com", Password: "correcthorse1", Name: "Heidi"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}
		login, err := svc.Login(ctx, "heidi@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}

		claims, err := jwtx.Parse(testRefreshSecret, login.RefreshToken, jwtx.TypeRefresh)
		if err != nil {
			t.Fatalf("failed to parse refresh token: %v", err)
		}
		rec, err := tokens.FindByJTIHash(ctx, hashJTI(claims.JTI))
		if err != nil {
			t.Fatalf("failed to find refresh token record: %v", err)
		}
		if err := tokens.Revoke(ctx, rec.ID); err != nil {
			t.Fatalf("failed to revoke: %v", err)
		}

		_, err = svc.Refresh(ctx, login.RefreshToken)
		if !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken for a revoked token, got %v", err)
		}
	})

	t.Run("revoked token reuse revokes every refresh token for the user", func(t *testing.T) {
		svc, _, _, tokens := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "mallory@example.com", Password: "correcthorse1", Name: "Mallory"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}

		// Two independent sessions/devices for the same user.
		firstLogin, err := svc.Login(ctx, "mallory@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("first login failed: %v", err)
		}
		secondLogin, err := svc.Login(ctx, "mallory@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("second login failed: %v", err)
		}

		// Rotate the first session's token — the legitimate client moving
		// forward in the chain. This revokes the presented token server-side.
		rotated, err := svc.Refresh(ctx, firstLogin.RefreshToken)
		if err != nil {
			t.Fatalf("first refresh should succeed: %v", err)
		}

		// An attacker (or a stale client) replays the now-revoked original
		// token. It must still just fail with ErrInvalidToken...
		if _, err := svc.Refresh(ctx, firstLogin.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken replaying a revoked token, got %v", err)
		}

		// ...but the important, harder-to-see behavior: every other refresh
		// token belonging to this user — including the still-valid second
		// session's token, and the just-issued rotated token — must now be
		// revoked too, forcing full re-authentication everywhere.
		secondClaims, err := jwtx.Parse(testRefreshSecret, secondLogin.RefreshToken, jwtx.TypeRefresh)
		if err != nil {
			t.Fatalf("failed to parse second session's refresh token: %v", err)
		}
		secondRecord, err := tokens.FindByJTIHash(ctx, hashJTI(secondClaims.JTI))
		if err != nil {
			t.Fatalf("expected second session's token record to still exist: %v", err)
		}
		if !secondRecord.Revoked {
			t.Error("expected the second, unrelated session's refresh token to be revoked after a revoked-token replay was detected")
		}

		rotatedClaims, err := jwtx.Parse(testRefreshSecret, rotated.RefreshToken, jwtx.TypeRefresh)
		if err != nil {
			t.Fatalf("failed to parse rotated refresh token: %v", err)
		}
		rotatedRecord, err := tokens.FindByJTIHash(ctx, hashJTI(rotatedClaims.JTI))
		if err != nil {
			t.Fatalf("expected rotated token record to still exist: %v", err)
		}
		if !rotatedRecord.Revoked {
			t.Error("expected the newly-rotated token to be revoked too — full re-auth should be forced")
		}

		// Confirming the point: the second session's token, though it was
		// never itself replayed, can no longer be used either.
		if _, err := svc.Refresh(ctx, secondLogin.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken using the second session's now-revoked token, got %v", err)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		settings := newFakeSettingsRepository()
		tokens := newFakeRefreshTokenRepository()
		svc := NewAuthService(users, settings, tokens, testAccessSecret, testRefreshSecret, testAccessTTL, testRefreshTTL)
		ctx := context.Background()

		u, err := svc.Register(ctx, domain.RegisterInput{Email: "ivan@example.com", Password: "correcthorse1", Name: "Ivan"})
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}

		jti := "expired-jti"
		refreshToken, err := jwtx.GenerateRefreshToken(testRefreshSecret, u.ID, jti, time.Hour)
		if err != nil {
			t.Fatalf("failed to generate refresh token: %v", err)
		}
		if err := tokens.Create(ctx, &domain.RefreshToken{
			ID:        "rt-1",
			UserID:    u.ID,
			JTIHash:   hashJTI(jti),
			ExpiresAt: time.Now().Add(-time.Minute), // already expired
			Revoked:   false,
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}); err != nil {
			t.Fatalf("failed to persist expired token record: %v", err)
		}

		_, err = svc.Refresh(ctx, refreshToken)
		if !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken for an expired-record token, got %v", err)
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("valid token gets revoked", func(t *testing.T) {
		svc, _, _, tokens := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "judy@example.com", Password: "correcthorse1", Name: "Judy"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}
		login, err := svc.Login(ctx, "judy@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}

		if err := svc.Logout(ctx, login.RefreshToken); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		claims, err := jwtx.Parse(testRefreshSecret, login.RefreshToken, jwtx.TypeRefresh)
		if err != nil {
			t.Fatalf("failed to parse refresh token: %v", err)
		}
		rec, err := tokens.FindByJTIHash(ctx, hashJTI(claims.JTI))
		if err != nil {
			t.Fatalf("expected token record to still exist: %v", err)
		}
		if !rec.Revoked {
			t.Error("expected refresh token to be revoked after logout")
		}

		// Re-refreshing after logout must fail.
		if _, err := svc.Refresh(ctx, login.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken refreshing a logged-out token, got %v", err)
		}
	})

	t.Run("idempotent for already-revoked token", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()
		ctx := context.Background()

		if _, err := svc.Register(ctx, domain.RegisterInput{Email: "kim@example.com", Password: "correcthorse1", Name: "Kim"}); err != nil {
			t.Fatalf("register failed: %v", err)
		}
		login, err := svc.Login(ctx, "kim@example.com", "correcthorse1")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}

		if err := svc.Logout(ctx, login.RefreshToken); err != nil {
			t.Fatalf("first logout unexpected error: %v", err)
		}
		if err := svc.Logout(ctx, login.RefreshToken); err != nil {
			t.Fatalf("second logout should be idempotent, got error: %v", err)
		}
	})

	t.Run("idempotent for garbage token", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()

		if err := svc.Logout(context.Background(), "not-a-real-token"); err != nil {
			t.Fatalf("expected logout with a garbage token to be a no-op, got error: %v", err)
		}
	})
}

func TestRequestPasswordReset(t *testing.T) {
	t.Run("registered email succeeds", func(t *testing.T) {
		svc, users, _, _ := newTestAuthService()
		_ = users.Create(context.Background(), &domain.User{
			ID: "u1", Email: "demo@finora.dev", Name: "Demo",
		})

		if err := svc.RequestPasswordReset(context.Background(), "demo@finora.dev"); err != nil {
			t.Fatalf("expected no error for a registered email, got: %v", err)
		}
	})

	t.Run("unregistered email still succeeds, never leaks existence", func(t *testing.T) {
		svc, _, _, _ := newTestAuthService()

		if err := svc.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
			t.Fatalf("expected no error for an unregistered email, got: %v", err)
		}
	})
}
