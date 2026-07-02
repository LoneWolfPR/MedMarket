// Package jwt handles adapters for managing jwts
package jwt

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenIssuer is an adapter for generating a signed JWT
type TokenIssuer struct {
	logger *slog.Logger
	ttl    time.Duration
	secret []byte
}

var _ outbound.TokenIssuer = (*TokenIssuer)(nil)

// NewTokenIssuerParams are in the need inputs
type NewTokenIssuerParams struct {
	Logger *slog.Logger
	TTL    time.Duration
	Secret []byte
}

// NewTokenIssuer is the constructor
func NewTokenIssuer(p NewTokenIssuerParams) (*TokenIssuer, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}

	if len(p.Secret) == 0 {
		return nil, errors.New("secret is empty")
	}

	if p.TTL <= 0 {
		return nil, errors.New("ttl is invalid")
	}

	return &TokenIssuer{
		logger: p.Logger,
		ttl:    p.TTL,
		secret: p.Secret,
	}, nil
}

// Issue takes a user's ID and generates a signed JWT for auth use in the app
func (i *TokenIssuer) Issue(userID uuid.UUID) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(i.ttl)),
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signedToken, err := token.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("error signing token: %w", err)
	}
	return signedToken, nil
}
