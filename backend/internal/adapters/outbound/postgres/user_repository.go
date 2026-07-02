// Package postgres handles all communication with the postgres db
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	entuser "github.com/LoneWolfPR/MedMarket/backend/ent/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// UserRepository is the adapter for connecting to user information in postgres
type UserRepository struct {
	client *ent.Client
	logger *slog.Logger
}

var _ outbound.UserRepository = (*UserRepository)(nil)

// NewUserRepositoryParams contains the input params for the constructor
type NewUserRepositoryParams struct {
	Client *ent.Client
	Logger *slog.Logger
}

// NewUserRepository is the constructor for initializing the adapter
func NewUserRepository(p NewUserRepositoryParams) (*UserRepository, error) {
	if p.Client == nil {
		return nil, errors.New("ent client is missing")
	}

	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	userRepo := UserRepository{
		client: p.Client,
		logger: p.Logger,
	}

	return &userRepo, nil
}

// Create takes the supplied information and creates a new user record in the database
func (r *UserRepository) Create(ctx context.Context, u *user.User) (*user.User, error) {
	userRecord, err := r.client.User.Create().
		SetFirstName(u.FirstName).
		SetLastName(u.LastName).
		SetEmail(u.Email.String()).
		SetPasswordHash(u.PasswordHash.String()).
		SetPhone(u.Phone).
		SetAddressStreet1(u.Address.Street1).
		SetAddressStreet2(u.Address.Street2).
		SetAddressCity(u.Address.City).
		SetAddressState(u.Address.State).
		SetAddressZip(u.Address.Zip).
		Save(ctx)

	if err != nil {
		// This assumes email is the only unique field in the user record.
		// If additional unique fields are added later this logic will need
		// to be refactored to be more precise.
		if ent.IsConstraintError(err) {
			return nil, outbound.ErrEmailTaken
		}
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return mapToDomainUser(userRecord)
}

// GetByID fetches a user from the database by the unique ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	userRecord, err := r.client.User.Query().
		Where(entuser.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, outbound.ErrUserNotFound
		}
		return nil, fmt.Errorf("error searching for user: %w", err)
	}

	return mapToDomainUser(userRecord)
}

// GetByEmail fetches a user record by the supplied email address
func (r *UserRepository) GetByEmail(ctx context.Context, email user.Email) (*user.User, error) {
	userRecord, err := r.client.User.Query().
		Where(entuser.EmailEQ(email.String())).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, outbound.ErrUserNotFound
		}
		return nil, fmt.Errorf("error searching for user: %w", err)
	}

	return mapToDomainUser(userRecord)
}

func mapToDomainUser(userRecord *ent.User) (*user.User, error) {
	userEmail, err := user.NewEmail(userRecord.Email)
	if err != nil {
		return nil, fmt.Errorf("error with the email address on file: %w", err)
	}
	userPasswordHash, err := user.NewPasswordHash(userRecord.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("error with passwordhash on file: %w", err)
	}
	domainUser := &user.User{
		ID:           userRecord.ID,
		FirstName:    userRecord.FirstName,
		LastName:     userRecord.LastName,
		Email:        userEmail,
		PasswordHash: userPasswordHash,
		Phone:        userRecord.Phone,
		Address: shared.Address{
			Street1: userRecord.AddressStreet1,
			Street2: userRecord.AddressStreet2,
			City:    userRecord.AddressCity,
			State:   userRecord.AddressState,
			Zip:     userRecord.AddressZip,
		},
	}
	return domainUser, nil
}
