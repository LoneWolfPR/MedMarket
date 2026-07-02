// Package bcrypt contains adapters for the bcrypt library
package bcrypt

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	bcryptlib "golang.org/x/crypto/bcrypt"
)

// PasswordHasher is the adapter for implementing the corresponding port
type PasswordHasher struct {
	logger *slog.Logger
}

var _ outbound.PasswordHasher = (*PasswordHasher)(nil)

// NewPasswordHasherParams contains the input params for the constructor
type NewPasswordHasherParams struct {
	Logger *slog.Logger
}

// NewPasswordHasher is the constructor for the adapter
func NewPasswordHasher(p NewPasswordHasherParams) (*PasswordHasher, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}

	return &PasswordHasher{logger: p.Logger}, nil
}

// Hash will take a plain string password and create a hash with bcrypt
func (h *PasswordHasher) Hash(pw string) (string, error) {
	passwordHashString, err := bcryptlib.GenerateFromPassword([]byte(pw), bcryptlib.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("error hashing the password: %w", err)
	}
	return string(passwordHashString), nil
}

// Compare will take a hashed and plain string of a password and compare to see
// if they match
func (h *PasswordHasher) Compare(hash, plain string) error {
	return bcryptlib.CompareHashAndPassword([]byte(hash), []byte(plain))
}
