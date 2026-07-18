package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/finora/user-service/internal/domain"
)

func TestUserServiceProfile(t *testing.T) {
	users := newFakeUserRepository()
	settings := newFakeSettingsRepository()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	_ = users.Create(ctx, &domain.User{ID: "u1", Email: "leo@example.com", Name: "Leo"})

	t.Run("GetProfile returns existing user", func(t *testing.T) {
		u, err := svc.GetProfile(ctx, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Name != "Leo" {
			t.Errorf("expected Leo, got %q", u.Name)
		}
	})

	t.Run("GetProfile unknown user returns ErrNotFound", func(t *testing.T) {
		_, err := svc.GetProfile(ctx, "unknown")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("UpdateProfile changes name", func(t *testing.T) {
		u, err := svc.UpdateProfile(ctx, "u1", domain.UpdateProfileInput{Name: "Leo Updated"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Name != "Leo Updated" {
			t.Errorf("expected updated name, got %q", u.Name)
		}
	})

	t.Run("UpdateProfile trims surrounding whitespace", func(t *testing.T) {
		u, err := svc.UpdateProfile(ctx, "u1", domain.UpdateProfileInput{Name: "  Leo Trimmed  "})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Name != "Leo Trimmed" {
			t.Errorf("expected trimmed name, got %q", u.Name)
		}
	})

	t.Run("UpdateProfile whitespace-only name is rejected", func(t *testing.T) {
		_, err := svc.UpdateProfile(ctx, "u1", domain.UpdateProfileInput{Name: "   "})
		ve, ok := AsValidationError(err)
		if !ok {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Field != "name" {
			t.Errorf("expected field=name, got %q", ve.Field)
		}
	})

	t.Run("UpdateProfile name over max length is rejected", func(t *testing.T) {
		longName := strings.Repeat("a", 101)
		_, err := svc.UpdateProfile(ctx, "u1", domain.UpdateProfileInput{Name: longName})
		ve, ok := AsValidationError(err)
		if !ok {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Field != "name" {
			t.Errorf("expected field=name, got %q", ve.Field)
		}
	})

	t.Run("UpdateProfile name at exactly max length succeeds", func(t *testing.T) {
		maxName := strings.Repeat("a", 100)
		u, err := svc.UpdateProfile(ctx, "u1", domain.UpdateProfileInput{Name: maxName})
		if err != nil {
			t.Fatalf("unexpected error at exactly max length: %v", err)
		}
		if u.Name != maxName {
			t.Errorf("expected name preserved at max length, got %q", u.Name)
		}
	})
}

func TestUserServiceSettings(t *testing.T) {
	users := newFakeUserRepository()
	settings := newFakeSettingsRepository()
	svc := NewUserService(users, settings)
	ctx := context.Background()

	t.Run("GetSettings returns defaults when none exist", func(t *testing.T) {
		s, err := svc.GetSettings(ctx, "u2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Currency != "USD" || s.Timezone != "UTC" {
			t.Errorf("expected USD/UTC defaults, got %q/%q", s.Currency, s.Timezone)
		}
	})

	t.Run("UpdateSettings partially updates without clobbering", func(t *testing.T) {
		if _, err := svc.UpdateSettings(ctx, "u2", domain.UpdateSettingsInput{Currency: "EUR", Timezone: "Europe/Berlin"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		s, err := svc.UpdateSettings(ctx, "u2", domain.UpdateSettingsInput{Timezone: "Europe/Paris"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Currency != "EUR" {
			t.Errorf("expected currency to remain EUR, got %q", s.Currency)
		}
		if s.Timezone != "Europe/Paris" {
			t.Errorf("expected timezone updated to Europe/Paris, got %q", s.Timezone)
		}
	})

	t.Run("UpdateSettings rejects invalid currency codes", func(t *testing.T) {
		cases := []struct {
			name     string
			currency string
		}{
			{"lowercase", "usd"},
			{"too short", "US"},
			{"too long", "USDD"},
			{"contains digits", "US1"},
			{"contains symbols", "US$"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.UpdateSettings(ctx, "u3", domain.UpdateSettingsInput{Currency: tc.currency})
				ve, ok := AsValidationError(err)
				if !ok {
					t.Fatalf("expected ValidationError for currency %q, got %v", tc.currency, err)
				}
				if ve.Field != "currency" {
					t.Errorf("expected field=currency, got %q", ve.Field)
				}
			})
		}
	})

	t.Run("UpdateSettings accepts a valid 3-letter currency code", func(t *testing.T) {
		s, err := svc.UpdateSettings(ctx, "u4", domain.UpdateSettingsInput{Currency: "GBP"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Currency != "GBP" {
			t.Errorf("expected GBP, got %q", s.Currency)
		}
	})

	t.Run("UpdateSettings rejects an invalid IANA timezone", func(t *testing.T) {
		_, err := svc.UpdateSettings(ctx, "u5", domain.UpdateSettingsInput{Timezone: "Mars/Cydonia"})
		ve, ok := AsValidationError(err)
		if !ok {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Field != "timezone" {
			t.Errorf("expected field=timezone, got %q", ve.Field)
		}
	})

	t.Run("UpdateSettings accepts a valid IANA timezone", func(t *testing.T) {
		s, err := svc.UpdateSettings(ctx, "u6", domain.UpdateSettingsInput{Timezone: "Asia/Tokyo"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Timezone != "Asia/Tokyo" {
			t.Errorf("expected Asia/Tokyo, got %q", s.Timezone)
		}
	})

	t.Run("UpdateSettings leaves settings untouched when fields are empty", func(t *testing.T) {
		if _, err := svc.UpdateSettings(ctx, "u7", domain.UpdateSettingsInput{Currency: "JPY", Timezone: "Asia/Tokyo"}); err != nil {
			t.Fatalf("seed update failed: %v", err)
		}
		s, err := svc.UpdateSettings(ctx, "u7", domain.UpdateSettingsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Currency != "JPY" || s.Timezone != "Asia/Tokyo" {
			t.Errorf("expected settings unchanged by an empty update, got %q/%q", s.Currency, s.Timezone)
		}
	})
}
