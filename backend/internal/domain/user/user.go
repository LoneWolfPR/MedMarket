// Package user provides the basic structure for user information
package user

import (
	"errors"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

//nolint:revive // sentinel errors are self-documenting
var (
	ErrMissingFirstName    = errors.New("first name missing")
	ErrMissingLastName     = errors.New("last name missing")
	ErrMissingEmail        = errors.New("email missing")
	ErrMissingPasswordHash = errors.New("passwordhash missing")
)

// User represents a user's profile information
type User struct {
	ID           uuid.UUID
	Email        Email
	PasswordHash PasswordHash
	FirstName    string
	LastName     string
	Phone        Phone
	Address      shared.Address
}

// NewUserParams contains the parameters an outside package needs to create
// a new instance of a user
type NewUserParams struct {
	Email        Email
	PasswordHash PasswordHash
	FirstName    string
	LastName     string
	Phone        Phone
	Address      shared.Address
}

// NewUser is the constructor for populating a User object
func NewUser(p NewUserParams) (*User, error) {
	trimmedFirstName := strings.TrimSpace(p.FirstName)
	if trimmedFirstName == "" {
		return nil, ErrMissingFirstName
	}

	trimmedLastName := strings.TrimSpace(p.LastName)
	if trimmedLastName == "" {
		return nil, ErrMissingLastName
	}

	if p.Email.IsZero() {
		return nil, ErrMissingEmail
	}

	if p.PasswordHash.IsZero() {
		return nil, ErrMissingPasswordHash
	}

	newUser := User{
		Email:        p.Email,
		PasswordHash: p.PasswordHash,
		FirstName:    trimmedFirstName,
		LastName:     trimmedLastName,
		Phone:        p.Phone,
		Address:      p.Address,
	}

	return &newUser, nil
}
