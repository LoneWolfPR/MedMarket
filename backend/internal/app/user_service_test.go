package app_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/app"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

const validHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// errBoom stands in for an unexpected infrastructure failure — the kind the
// service should surface as a generic (non-sentinel) error rather than
// normalize into the inbound vocabulary.
var errBoom = errors.New("boom")

// newService constructs a UserService with the given fakes and a discard logger.
func newService(t *testing.T, repo fakeUserRepo, hasher fakePasswordHasher, issuer fakeTokenIssuer) *app.UserService {
	t.Helper()

	svc, err := app.NewUserService(app.NewUserServiceParams{
		Logger:         slog.New(slog.DiscardHandler),
		UserRepository: repo,
		PasswordHasher: hasher,
		TokenIssuer:    issuer,
	})
	require.NoError(t, err)
	return svc
}

// validRegisterInput returns a RegisterInput that passes all domain validation.
func validRegisterInput() inbound.RegisterInput {
	return inbound.RegisterInput{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "Jane@Example.com", // mixed case on purpose — service normalizes it
		Password:  "Passw0rd!",
		Phone:     "5551234567",
	}
}

// regInput returns a valid RegisterInput with the given mutation applied, for
// isolating one bad field per case.
func regInput(mutate func(in *inbound.RegisterInput)) inbound.RegisterInput {
	in := validRegisterInput()
	mutate(&in)
	return in
}

// makeStoredUser builds a persisted-looking *user.User (with an ID) for repo
// fakes to return.
func makeStoredUser(t *testing.T, email string) *user.User {
	t.Helper()

	e, err := user.NewEmail(email)
	require.NoError(t, err)
	h, err := user.NewPasswordHash(validHash)
	require.NoError(t, err)
	u, err := user.NewUser(user.NewUserParams{
		Email:        e,
		PasswordHash: h,
		FirstName:    "Jane",
		LastName:     "Doe",
	})
	require.NoError(t, err)
	u.ID = uuid.New()
	return u
}

// --- NewUserService ---------------------------------------------------------

func TestNewUserService_Validation(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	repo := fakeUserRepo{}
	hasher := fakePasswordHasher{}
	issuer := fakeTokenIssuer{}

	tests := map[string]app.NewUserServiceParams{
		"missing logger":       {UserRepository: repo, PasswordHasher: hasher, TokenIssuer: issuer},
		"missing repository":   {Logger: logger, PasswordHasher: hasher, TokenIssuer: issuer},
		"missing hasher":       {Logger: logger, UserRepository: repo, TokenIssuer: issuer},
		"missing token issuer": {Logger: logger, UserRepository: repo, PasswordHasher: hasher},
	}

	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			svc, err := app.NewUserService(params)

			require.Error(t, err)
			assert.Nil(t, svc)
		})
	}

	t.Run("all dependencies present", func(t *testing.T) {
		svc, err := app.NewUserService(app.NewUserServiceParams{
			Logger: logger, UserRepository: repo, PasswordHasher: hasher, TokenIssuer: issuer,
		})

		require.NoError(t, err)
		assert.NotNil(t, svc)
	})
}

// --- Register ---------------------------------------------------------------

func TestRegister_Success(t *testing.T) {
	var (
		hashedPlain string     // captures the plaintext handed to the hasher
		created     *user.User // captures the user handed to the repo
	)
	stored := makeStoredUser(t, "jane@example.com")

	repo := fakeUserRepo{
		createFn: func(_ context.Context, u *user.User) (*user.User, error) {
			created = u
			return stored, nil
		},
	}
	hasher := fakePasswordHasher{
		hashFn: func(pw string) (string, error) {
			hashedPlain = pw
			return validHash, nil
		},
	}
	svc := newService(t, repo, hasher, fakeTokenIssuer{})

	got, err := svc.Register(context.Background(), validRegisterInput())

	require.NoError(t, err)
	assert.Same(t, stored, got, "service should return the repository's persisted user")
	// the plaintext password (not the raw input struct) reaches the hasher
	assert.Equal(t, "Passw0rd!", hashedPlain)
	// the user handed to the repo carries the normalized email and the hash
	require.NotNil(t, created)
	assert.Equal(t, "jane@example.com", created.Email.String())
	assert.Equal(t, validHash, created.PasswordHash.String())
}

func TestRegister_NormalizesErrors(t *testing.T) {
	// A hasher that succeeds by default; individual cases override the fakes.
	okHasher := fakePasswordHasher{hashFn: func(string) (string, error) { return validHash, nil }}

	tests := map[string]struct {
		input   inbound.RegisterInput
		repo    fakeUserRepo
		hasher  fakePasswordHasher
		wantErr error // sentinel the error must match, or nil for "generic (non-sentinel)"
	}{
		"invalid email": {
			input:   regInput(func(in *inbound.RegisterInput) { in.Email = "not-an-email" }),
			hasher:  okHasher,
			wantErr: inbound.ErrValidation,
		},
		"invalid password": {
			input:   regInput(func(in *inbound.RegisterInput) { in.Password = "weak" }),
			hasher:  okHasher,
			wantErr: inbound.ErrValidation,
		},
		"missing name fails NewUser invariant": {
			input:   regInput(func(in *inbound.RegisterInput) { in.FirstName = "" }),
			hasher:  okHasher,
			wantErr: inbound.ErrValidation,
		},
		"duplicate email": {
			input: validRegisterInput(),
			repo: fakeUserRepo{createFn: func(context.Context, *user.User) (*user.User, error) {
				return nil, outbound.ErrEmailTaken
			}},
			hasher:  okHasher,
			wantErr: inbound.ErrEmailTaken,
		},
		"hasher failure is generic": {
			input:   validRegisterInput(),
			hasher:  fakePasswordHasher{hashFn: func(string) (string, error) { return "", errBoom }},
			wantErr: nil,
		},
		"hasher returns unusable hash is generic": {
			input:   validRegisterInput(),
			hasher:  fakePasswordHasher{hashFn: func(string) (string, error) { return "", nil }},
			wantErr: nil,
		},
		"repository failure is generic": {
			input: validRegisterInput(),
			repo: fakeUserRepo{createFn: func(context.Context, *user.User) (*user.User, error) {
				return nil, errBoom
			}},
			hasher:  okHasher,
			wantErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := newService(t, tc.repo, tc.hasher, fakeTokenIssuer{})

			got, err := svc.Register(context.Background(), tc.input)

			require.Error(t, err)
			assert.Nil(t, got)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				// generic failures must not masquerade as any inbound sentinel
				assert.NotErrorIs(t, err, inbound.ErrValidation)
				assert.NotErrorIs(t, err, inbound.ErrEmailTaken)
				assert.NotErrorIs(t, err, inbound.ErrInvalidCredentials)
			}
		})
	}
}

// --- Login ------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	var (
		comparedHash  string
		comparedPlain string
		issuedFor     uuid.UUID
	)
	stored := makeStoredUser(t, "jane@example.com")

	repo := fakeUserRepo{
		getByEmailFn: func(_ context.Context, _ user.Email) (*user.User, error) { return stored, nil },
	}
	hasher := fakePasswordHasher{
		compareFn: func(hash, plain string) error {
			comparedHash, comparedPlain = hash, plain
			return nil
		},
	}
	issuer := fakeTokenIssuer{
		issueFn: func(id uuid.UUID) (string, error) {
			issuedFor = id
			return "signed.jwt.token", nil
		},
	}
	svc := newService(t, repo, hasher, issuer)

	token, err := svc.Login(context.Background(), "jane@example.com", "Passw0rd!")

	require.NoError(t, err)
	assert.Equal(t, "signed.jwt.token", token)
	// Compare receives the stored hash and the supplied plaintext
	assert.Equal(t, validHash, comparedHash)
	assert.Equal(t, "Passw0rd!", comparedPlain)
	// the token is issued for the resolved user's ID
	assert.Equal(t, stored.ID, issuedFor)
}

func TestLogin_NormalizesErrors(t *testing.T) {
	stored := makeStoredUser(t, "jane@example.com")
	okCompare := fakePasswordHasher{compareFn: func(string, string) error { return nil }}
	foundRepo := fakeUserRepo{
		getByEmailFn: func(context.Context, user.Email) (*user.User, error) { return stored, nil },
	}

	tests := map[string]struct {
		email   string
		repo    fakeUserRepo
		hasher  fakePasswordHasher
		issuer  fakeTokenIssuer
		wantErr error // sentinel to match, or nil for "generic (non-sentinel)"
	}{
		"malformed email never touches the repo": {
			email:   "not-an-email",
			wantErr: inbound.ErrInvalidCredentials,
		},
		"unknown user": {
			email: "jane@example.com",
			repo: fakeUserRepo{getByEmailFn: func(context.Context, user.Email) (*user.User, error) {
				return nil, outbound.ErrUserNotFound
			}},
			wantErr: inbound.ErrInvalidCredentials,
		},
		"wrong password": {
			email:   "jane@example.com",
			repo:    foundRepo,
			hasher:  fakePasswordHasher{compareFn: func(string, string) error { return errBoom }},
			wantErr: inbound.ErrInvalidCredentials,
		},
		"repository lookup failure is generic": {
			email: "jane@example.com",
			repo: fakeUserRepo{getByEmailFn: func(context.Context, user.Email) (*user.User, error) {
				return nil, errBoom
			}},
			wantErr: nil,
		},
		"token issue failure is generic": {
			email:   "jane@example.com",
			repo:    foundRepo,
			hasher:  okCompare,
			issuer:  fakeTokenIssuer{issueFn: func(uuid.UUID) (string, error) { return "", errBoom }},
			wantErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := newService(t, tc.repo, tc.hasher, tc.issuer)

			token, err := svc.Login(context.Background(), tc.email, "Passw0rd!")

			require.Error(t, err)
			assert.Empty(t, token)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NotErrorIs(t, err, inbound.ErrInvalidCredentials)
			}
		})
	}
}

// --- GetProfile -------------------------------------------------------------

func TestGetProfile_Success(t *testing.T) {
	stored := makeStoredUser(t, "jane@example.com")
	var askedFor uuid.UUID

	repo := fakeUserRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*user.User, error) {
			askedFor = id
			return stored, nil
		},
	}
	svc := newService(t, repo, fakePasswordHasher{}, fakeTokenIssuer{})

	got, err := svc.GetProfile(context.Background(), stored.ID)

	require.NoError(t, err)
	assert.Same(t, stored, got)
	assert.Equal(t, stored.ID, askedFor)
}

func TestGetProfile_NormalizesErrors(t *testing.T) {
	tests := map[string]struct {
		repo    fakeUserRepo
		wantErr error // sentinel to match, or nil for "generic (non-sentinel)"
	}{
		// a valid token for a since-deleted user must read as unauthorized, not 500
		"unknown user maps to invalid credentials": {
			repo: fakeUserRepo{getByIDFn: func(context.Context, uuid.UUID) (*user.User, error) {
				return nil, outbound.ErrUserNotFound
			}},
			wantErr: inbound.ErrInvalidCredentials,
		},
		"repository failure is generic": {
			repo: fakeUserRepo{getByIDFn: func(context.Context, uuid.UUID) (*user.User, error) {
				return nil, errBoom
			}},
			wantErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := newService(t, tc.repo, fakePasswordHasher{}, fakeTokenIssuer{})

			got, err := svc.GetProfile(context.Background(), uuid.New())

			require.Error(t, err)
			assert.Nil(t, got)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NotErrorIs(t, err, inbound.ErrInvalidCredentials)
			}
		})
	}
}
