// Package inbound creates the interfaces used by inbound adapters
package inbound

import (
	"context"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"

	"github.com/google/uuid"
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
