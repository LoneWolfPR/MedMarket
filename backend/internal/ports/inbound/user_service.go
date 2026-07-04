// Package inbound creates the interfaces used by inbound adapters
package inbound

import (
	"context"
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"

	"github.com/google/uuid"
)

// Sentinel errors returned by UserService for the inbound adapters to map
// onto transport-level responses (e.g. HTTP status codes). The service
// normalizes domain and outbound errors into these so driving adapters never
// depend on domain/outbound error identities.
//
//nolint:revive // sentinel errors are self-documenting
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrValidation         = errors.New("validation failed")
	ErrEmailTaken         = errors.New("email already in use")
)

// RegisterInput includes the raw values for creating a new user
type RegisterInput struct {
	FirstName string
	LastName  string
	Email     string // raw, unvalidated - the service will call NewEmail
	Password  string // plaintext - hashed by the service
	Phone     string
	Address   shared.Address
}

// UserService is the application's inbound API for user operations —
// implemented by the application service and called by driving adapters
// (e.g. the HTTP handler).
type UserService interface {
	Register(ctx context.Context, input RegisterInput) (*user.User, error)
	Login(ctx context.Context, email, password string) (string, error)
	GetProfile(ctx context.Context, id uuid.UUID) (*user.User, error)
}
