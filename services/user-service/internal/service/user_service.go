package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/finora/user-service/internal/domain"
)

// maxNameLength bounds the profile name field. Deliberately generous (not a
// display-layout constraint) — it exists only to reject obviously-abusive
// input, not to police real names.
const maxNameLength = 100

// currencyPattern is a lightweight format check (3 uppercase ASCII letters),
// not a real ISO-4217 lookup table — deliberately so, per CLAUDE.md's "no
// clever code" / don't-over-engineer guidance for this scale of app.
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type userService struct {
	users    domain.UserRepository
	settings domain.SettingsRepository
}

// NewUserService constructs domain.UserService from repository interfaces.
func NewUserService(users domain.UserRepository, settings domain.SettingsRepository) domain.UserService {
	return &userService{users: users, settings: settings}
}

func (s *userService) GetProfile(ctx context.Context, userID string) (*domain.User, error) {
	return s.users.FindByID(ctx, userID)
}

func (s *userService) UpdateProfile(ctx context.Context, userID string, in domain.UpdateProfileInput) (*domain.User, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, NewValidationError("name", "name must not be blank")
	}
	if len(name) > maxNameLength {
		return nil, NewValidationError("name", "name must be at most 100 characters")
	}

	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	u.Name = name
	u.UpdatedAt = time.Now().UTC()

	if err := s.users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *userService) GetSettings(ctx context.Context, userID string) (*domain.Settings, error) {
	settings, err := s.settings.FindByUserID(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		// A user created before settings existed (or whose seed upsert
		// somehow didn't land) still gets sane defaults instead of a 404.
		return &domain.Settings{UserID: userID, Currency: "USD", Timezone: "UTC"}, nil
	}
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *userService) UpdateSettings(ctx context.Context, userID string, in domain.UpdateSettingsInput) (*domain.Settings, error) {
	currency := strings.TrimSpace(in.Currency)
	if currency != "" && !currencyPattern.MatchString(currency) {
		return nil, NewValidationError("currency", "currency must be a 3-letter uppercase code (e.g. USD)")
	}

	timezone := strings.TrimSpace(in.Timezone)
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return nil, NewValidationError("timezone", "timezone must be a valid IANA timezone (e.g. America/New_York)")
		}
	}

	existing, err := s.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	if currency != "" {
		existing.Currency = currency
	}
	if timezone != "" {
		existing.Timezone = timezone
	}
	existing.UserID = userID
	existing.UpdatedAt = time.Now().UTC()

	if err := s.settings.Upsert(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}
