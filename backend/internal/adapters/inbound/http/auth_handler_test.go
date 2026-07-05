package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
)

const validHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// errBoom stands in for an unexpected service error the handler must surface as
// a raw error (for the global 500), not a typed response.
var errBoom = errors.New("boom")

// fakeUserService is a hand-rolled stub of the inbound.UserService port; each
// method delegates to a function field so a test wires only what it exercises.
type fakeUserService struct {
	registerFn   func(ctx context.Context, in inbound.RegisterInput) (*user.User, error)
	loginFn      func(ctx context.Context, email, password string) (string, error)
	getProfileFn func(ctx context.Context, id uuid.UUID) (*user.User, error)
}

func (f fakeUserService) Register(ctx context.Context, in inbound.RegisterInput) (*user.User, error) {
	return f.registerFn(ctx, in)
}

func (f fakeUserService) Login(ctx context.Context, email, password string) (string, error) {
	return f.loginFn(ctx, email, password)
}

func (f fakeUserService) GetProfile(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return f.getProfileFn(ctx, id)
}

func newHandler(t *testing.T, svc inbound.UserService) *AuthHandler {
	t.Helper()

	h, err := NewAuthHandler(NewAuthHandlerParams{Logger: slog.New(slog.DiscardHandler), Svc: svc})
	require.NoError(t, err)
	return h
}

// makeUser builds a fully populated domain user for the service fakes to return.
func makeUser(t *testing.T) *user.User {
	t.Helper()

	e, err := user.NewEmail("jane@example.com")
	require.NoError(t, err)
	h, err := user.NewPasswordHash(validHash)
	require.NoError(t, err)
	p, err := user.NewPhone("5551234567")
	require.NoError(t, err)
	u, err := user.NewUser(user.NewUserParams{
		Email:        e,
		PasswordHash: h,
		FirstName:    "Jane",
		LastName:     "Doe",
		Phone:        p,
		Address:      shared.Address{Street1: "1 Main St", City: "Anytown", State: "CA", Zip: "90001"},
	})
	require.NoError(t, err)
	u.ID = uuid.New()
	return u
}

func validRegisterReq() openapi.RegisterUserRequestObject {
	return openapi.RegisterUserRequestObject{Body: &openapi.RegisterRequest{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     openapi_types.Email("jane@example.com"),
		Password:  "Passw0rd!",
		Phone:     ptr.To("5551234567"),
		Address:   &openapi.Address{Street1: "1 Main St", City: "Anytown", State: "CA", Zip: "90001"},
	}}
}

// --- RegisterUser -----------------------------------------------------------

func TestRegisterUser_Success(t *testing.T) {
	var captured inbound.RegisterInput
	created := makeUser(t)
	svc := fakeUserService{registerFn: func(_ context.Context, in inbound.RegisterInput) (*user.User, error) {
		captured = in
		return created, nil
	}}

	got, err := newHandler(t, svc).RegisterUser(context.Background(), validRegisterReq())

	require.NoError(t, err)
	// DTO -> service input mapping, including the phone that used to be dropped
	assert.Equal(t, "Jane", captured.FirstName)
	assert.Equal(t, "Doe", captured.LastName)
	assert.Equal(t, "jane@example.com", captured.Email)
	assert.Equal(t, "Passw0rd!", captured.Password)
	assert.Equal(t, "5551234567", captured.Phone)
	assert.Equal(t, "1 Main St", captured.Address.Street1)
	// domain user -> 201 response DTO mapping
	resp, ok := got.(openapi.RegisterUser201JSONResponse)
	require.True(t, ok, "want 201 response, got %T", got)
	assert.Equal(t, created.ID, resp.Id)
	assert.Equal(t, openapi_types.Email("jane@example.com"), resp.Email)
	require.NotNil(t, resp.Phone)
	assert.Equal(t, "5551234567", *resp.Phone)
}

func TestRegisterUser_ValidationError(t *testing.T) {
	svc := fakeUserService{registerFn: func(context.Context, inbound.RegisterInput) (*user.User, error) {
		return nil, fmt.Errorf("%w: bad email", inbound.ErrValidation)
	}}

	got, err := newHandler(t, svc).RegisterUser(context.Background(), validRegisterReq())

	require.NoError(t, err)
	resp, ok := got.(openapi.RegisterUser400JSONResponse)
	require.True(t, ok, "want 400 response, got %T", got)
	assert.Equal(t, inbound.ErrValidation.Error(), resp.Message)
}

func TestRegisterUser_EmailTaken(t *testing.T) {
	svc := fakeUserService{registerFn: func(context.Context, inbound.RegisterInput) (*user.User, error) {
		return nil, fmt.Errorf("%w", inbound.ErrEmailTaken)
	}}

	got, err := newHandler(t, svc).RegisterUser(context.Background(), validRegisterReq())

	require.NoError(t, err)
	resp, ok := got.(openapi.RegisterUser409JSONResponse)
	require.True(t, ok, "want 409 response, got %T", got)
	assert.Equal(t, inbound.ErrEmailTaken.Error(), resp.Message)
}

func TestRegisterUser_UnexpectedErrorBubbles(t *testing.T) {
	svc := fakeUserService{registerFn: func(context.Context, inbound.RegisterInput) (*user.User, error) {
		return nil, errBoom
	}}

	got, err := newHandler(t, svc).RegisterUser(context.Background(), validRegisterReq())

	// unexpected errors are returned raw for the global 500 handler, not typed
	require.Error(t, err)
	assert.Nil(t, got)
}

// --- LoginUser --------------------------------------------------------------

func TestLoginUser_Success(t *testing.T) {
	var gotEmail, gotPassword string
	svc := fakeUserService{loginFn: func(_ context.Context, email, password string) (string, error) {
		gotEmail, gotPassword = email, password
		return "signed.jwt.token", nil
	}}
	req := openapi.LoginUserRequestObject{Body: &openapi.LoginRequest{
		Email: openapi_types.Email("jane@example.com"), Password: "Passw0rd!",
	}}

	got, err := newHandler(t, svc).LoginUser(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "jane@example.com", gotEmail)
	assert.Equal(t, "Passw0rd!", gotPassword)
	resp, ok := got.(openapi.LoginUser200JSONResponse)
	require.True(t, ok, "want 200 response, got %T", got)
	assert.Equal(t, "signed.jwt.token", resp.Token)
}

func TestLoginUser_InvalidCredentials(t *testing.T) {
	svc := fakeUserService{loginFn: func(context.Context, string, string) (string, error) {
		return "", inbound.ErrInvalidCredentials
	}}
	req := openapi.LoginUserRequestObject{Body: &openapi.LoginRequest{
		Email: openapi_types.Email("jane@example.com"), Password: "wrong",
	}}

	got, err := newHandler(t, svc).LoginUser(context.Background(), req)

	require.NoError(t, err)
	resp, ok := got.(openapi.LoginUser401JSONResponse)
	require.True(t, ok, "want 401 response, got %T", got)
	assert.Equal(t, inbound.ErrInvalidCredentials.Error(), resp.Message)
}

func TestLoginUser_UnexpectedErrorBubbles(t *testing.T) {
	svc := fakeUserService{loginFn: func(context.Context, string, string) (string, error) {
		return "", errBoom
	}}
	req := openapi.LoginUserRequestObject{Body: &openapi.LoginRequest{
		Email: openapi_types.Email("jane@example.com"), Password: "Passw0rd!",
	}}

	got, err := newHandler(t, svc).LoginUser(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, got)
}

// --- GetProfile -------------------------------------------------------------

func TestGetProfile_Success(t *testing.T) {
	profile := makeUser(t)
	var askedID uuid.UUID
	svc := fakeUserService{getProfileFn: func(_ context.Context, id uuid.UUID) (*user.User, error) {
		askedID = id
		return profile, nil
	}}
	// the middleware contract: the authenticated user's ID lives in the context
	ctx := setUserIDKeyValue(context.Background(), profile.ID)

	got, err := newHandler(t, svc).GetProfile(ctx, openapi.GetProfileRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, profile.ID, askedID, "handler should read the ID from context")
	resp, ok := got.(openapi.GetProfile200JSONResponse)
	require.True(t, ok, "want 200 response, got %T", got)
	assert.Equal(t, profile.ID, resp.Id)
}

func TestGetProfile_MissingContextIsUnauthorized(t *testing.T) {
	// getProfileFn is nil on purpose: it must never be called when the context
	// carries no user ID, so a call would panic and fail the test.
	svc := fakeUserService{}

	got, err := newHandler(t, svc).GetProfile(context.Background(), openapi.GetProfileRequestObject{})

	require.NoError(t, err)
	_, ok := got.(openapi.GetProfile401JSONResponse)
	assert.True(t, ok, "want 401 response, got %T", got)
}

func TestGetProfile_InvalidCredentials(t *testing.T) {
	id := uuid.New()
	svc := fakeUserService{getProfileFn: func(context.Context, uuid.UUID) (*user.User, error) {
		return nil, inbound.ErrInvalidCredentials
	}}
	ctx := setUserIDKeyValue(context.Background(), id)

	got, err := newHandler(t, svc).GetProfile(ctx, openapi.GetProfileRequestObject{})

	require.NoError(t, err)
	_, ok := got.(openapi.GetProfile401JSONResponse)
	assert.True(t, ok, "want 401 response, got %T", got)
}

func TestGetProfile_UnexpectedErrorBubbles(t *testing.T) {
	id := uuid.New()
	svc := fakeUserService{getProfileFn: func(context.Context, uuid.UUID) (*user.User, error) {
		return nil, errBoom
	}}
	ctx := setUserIDKeyValue(context.Background(), id)

	got, err := newHandler(t, svc).GetProfile(ctx, openapi.GetProfileRequestObject{})

	require.Error(t, err)
	assert.Nil(t, got)
}
