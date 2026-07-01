// Package outbound creates the interfaces for use by outbound adapters
package outbound

import (
	"context"
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"

	"github.com/google/uuid"
)

//nolint:revive // Sentinel errors are self documenting
var (
	ErrUserNotFound = errors.New("user not found")
)

// UserRepository declares all the methods an adapter needs to implement
type UserRepository interface {
	Create(ctx context.Context, u *user.User) (*user.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*user.User, error)
	GetByEmail(ctx context.Context, email user.Email) (*user.User, error)
}
