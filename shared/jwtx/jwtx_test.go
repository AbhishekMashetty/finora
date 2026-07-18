package jwtx

import (
	"testing"
	"time"
)

func TestGenerateAndParseAccessToken(t *testing.T) {
	token, err := GenerateAccessToken("secret", "user-1", "a@b.com", time.Minute)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	claims, err := Parse("secret", token, TypeAccess)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", claims.UserID)
	}
	if claims.Email != "a@b.com" {
		t.Errorf("expected email a@b.com, got %s", claims.Email)
	}
}

func TestParseRejectsWrongType(t *testing.T) {
	token, err := GenerateRefreshToken("secret", "user-1", "jti-1", time.Minute)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	if _, err := Parse("secret", token, TypeAccess); err != ErrWrongType {
		t.Errorf("expected ErrWrongType, got %v", err)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	token, err := GenerateAccessToken("secret", "user-1", "a@b.com", -time.Minute)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	if _, err := Parse("secret", token, TypeAccess); err == nil {
		t.Error("expected error for expired token, got nil")
	}
}
