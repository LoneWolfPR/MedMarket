// Package app implements the inbound service ports, orchestrating
// the domain and outbound adapters.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
)

// UserService implements the port allow outside reguests against user data
type UserService struct {
	logger         *slog.Logger
	userRepository outbound.UserRepository
	passwordHasher outbound.PasswordHasher
	tokenIssuer    outbound.TokenIssuer
}

var _ inbound.UserService = (*UserService)(nil)

// NewUserServiceParams contains the input params for the constructor
type NewUserServiceParams struct {
	Logger         *slog.Logger
	UserRepository outbound.UserRepository
	PasswordHasher outbound.PasswordHasher
	TokenIssuer    outbound.TokenIssuer
}

// NewUserService is the constructor
func NewUserService(p NewUserServiceParams) (*UserService, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}

	if p.UserRepository == nil {
		return nil, errors.New("userRepository is missing")
	}

	if p.PasswordHasher == nil {
		return nil, errors.New("passwordHasher is missing")
	}

	if p.TokenIssuer == nil {
		return nil, errors.New("tokenIssuer is missing")
	}
	userService := UserService{
		logger:         p.Logger,
		userRepository: p.UserRepository,
		passwordHasher: p.PasswordHasher,
		tokenIssuer:    p.TokenIssuer,
	}

	return &userService, nil
}

// Register takes incoming user information and uses the UserRepository adapter to create
// a new user record
func (s *UserService) Register(ctx context.Context, input inbound.RegisterInput) (*user.User, error) {
	emailAddress, err := user.NewEmail(input.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", inbound.ErrValidation, err)
	}
	password, err := user.NewPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", inbound.ErrValidation, err)
	}
	passwordHashString, err := s.passwordHasher.Hash(password.String())
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}
	passwordHash, err := user.NewPasswordHash(passwordHashString)
	if err != nil {
		return nil, fmt.Errorf("error in the password hash: %w", err)
	}
	phoneNumber, err := user.NewPhone(input.Phone)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", inbound.ErrValidation, err)
	}
	newUser, err := user.NewUser(user.NewUserParams{
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Email:        emailAddress,
		PasswordHash: passwordHash,
		Phone:        phoneNumber,
		Address:      input.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", inbound.ErrValidation, err)
	}
	createdUser, err := s.userRepository.Create(ctx, newUser)
	if err != nil {
		if errors.Is(err, outbound.ErrEmailTaken) {
			return nil, fmt.Errorf("%w: %w", inbound.ErrEmailTaken, err)
		}
		return nil, fmt.Errorf("error creating user: %w", err)
	}
	return createdUser, nil
}

// Login takes a provided email and password, fetches a profile by email, and validates the
// password that is provided against the hash returned
func (s *UserService) Login(ctx context.Context, email, password string) (string, error) {
	userEmail, err := user.NewEmail(email)
	if err != nil {
		return "", inbound.ErrInvalidCredentials
	}
	userProfile, err := s.userRepository.GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, outbound.ErrUserNotFound) {
			return "", inbound.ErrInvalidCredentials
		}
		return "", fmt.Errorf("error fetching profile by email: %w", err)
	}

	err = s.passwordHasher.Compare(userProfile.PasswordHash.String(), password)
	if err != nil {
		return "", inbound.ErrInvalidCredentials
	}

	signedToken, err := s.tokenIssuer.Issue(userProfile.ID)
	if err != nil {
		return "", fmt.Errorf("error generating signed jwt: %w", err)
	}
	return signedToken, nil
}

// GetProfile takes a user's unique ID and utilizes the UserRepository adapter to fetch
// the profile information
func (s *UserService) GetProfile(ctx context.Context, id uuid.UUID) (*user.User, error) {
	profile, err := s.userRepository.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, outbound.ErrUserNotFound) {
			return nil, fmt.Errorf("%w: %w", inbound.ErrInvalidCredentials, err)
		}
		return nil, fmt.Errorf("error fetching user profile: %w", err)
	}
	return profile, nil
}
