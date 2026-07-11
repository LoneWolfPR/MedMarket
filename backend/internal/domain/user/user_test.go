package user_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
)

// validParams returns a fully-populated, valid NewUserParams that each test
// mutates to isolate the one invariant under test.
func validParams(t *testing.T) user.NewUserParams {
	t.Helper()

	email, err := shared.NewEmail("jane@example.com")
	require.NoError(t, err)

	hash, err := user.NewPasswordHash("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")
	require.NoError(t, err)

	phone, err := shared.NewPhone("5551234567")
	require.NoError(t, err)

	return user.NewUserParams{
		Email:        email,
		PasswordHash: hash,
		FirstName:    "Jane",
		LastName:     "Doe",
		Phone:        phone,
		Address: shared.Address{
			Street1: "1 Main St",
			City:    "Anytown",
			State:   "CA",
			Zip:     "90001",
		},
	}
}

func TestNewUser_Valid(t *testing.T) {
	p := validParams(t)

	got, err := user.NewUser(p)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.Email, got.Email)
	assert.Equal(t, p.PasswordHash, got.PasswordHash)
	assert.Equal(t, "Jane", got.FirstName)
	assert.Equal(t, "Doe", got.LastName)
	assert.Equal(t, p.Phone, got.Phone)
	assert.Equal(t, p.Address, got.Address)
	// NewUser does not assign an ID — that is the persistence layer's job.
	assert.Equal(t, uuid.Nil, got.ID)
}

func TestNewUser_TrimsNames(t *testing.T) {
	p := validParams(t)
	p.FirstName = "  Jane  "
	p.LastName = "  Doe  "

	got, err := user.NewUser(p)

	require.NoError(t, err)
	assert.Equal(t, "Jane", got.FirstName)
	assert.Equal(t, "Doe", got.LastName)
}

func TestNewUser_Invalid(t *testing.T) {
	tests := map[string]struct {
		mutate  func(p *user.NewUserParams)
		wantErr error
	}{
		"empty first name": {
			mutate:  func(p *user.NewUserParams) { p.FirstName = "" },
			wantErr: user.ErrMissingFirstName,
		},
		"whitespace first name": {
			mutate:  func(p *user.NewUserParams) { p.FirstName = "   " },
			wantErr: user.ErrMissingFirstName,
		},
		"empty last name": {
			mutate:  func(p *user.NewUserParams) { p.LastName = "" },
			wantErr: user.ErrMissingLastName,
		},
		"whitespace last name": {
			mutate:  func(p *user.NewUserParams) { p.LastName = "   " },
			wantErr: user.ErrMissingLastName,
		},
		"zero email": {
			mutate:  func(p *user.NewUserParams) { p.Email = shared.Email{} },
			wantErr: user.ErrMissingEmail,
		},
		"zero password hash": {
			mutate:  func(p *user.NewUserParams) { p.PasswordHash = user.PasswordHash{} },
			wantErr: user.ErrMissingPasswordHash,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := validParams(t)
			tc.mutate(&p)

			got, err := user.NewUser(p)

			require.ErrorIs(t, err, tc.wantErr)
			assert.Nil(t, got, "expected nil *User on error")
		})
	}
}

// TestNewUser_ValidationOrder documents the precedence of the invariant checks:
// first name is validated before last name, so a params value that violates
// both surfaces the first-name error.
func TestNewUser_ValidationOrder(t *testing.T) {
	p := validParams(t)
	p.FirstName = ""
	p.LastName = ""

	_, err := user.NewUser(p)

	require.ErrorIs(t, err, user.ErrMissingFirstName)
}
