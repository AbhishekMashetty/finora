// Package jwtx is the single source of truth for Finora's JWT claim shape.
// Only user-service generates tokens; only the gateway parses access tokens
// for routing. Both import this package so the claim contract can't drift
// between the two (see architecture/api-contracts.md, JWT Contract).
package jwtx

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrWrongType    = errors.New("unexpected token type")
)

// Claims is the shared JWT payload shape for both access and refresh tokens.
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email,omitempty"`
	Type   string `json:"type"`
	JTI    string `json:"jti,omitempty"`
	jwt.RegisteredClaims
}

// GenerateAccessToken issues a short-lived access token identifying userID/email.
func GenerateAccessToken(secret, userID, email string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Type:   TypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// GenerateRefreshToken issues a longer-lived refresh token carrying a jti so
// the caller (user-service) can persist/revoke it independently of expiry.
func GenerateRefreshToken(secret, userID, jti string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Type:   TypeRefresh,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Parse validates the token's signature/expiry and that its type matches
// expectedType ("access" or "refresh"), returning its claims.
func Parse(secret, tokenString, expectedType string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Type != expectedType {
		return nil, ErrWrongType
	}
	return claims, nil
}
