package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

const validHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

var errBoom = errors.New("boom")

// stubService and stubIssuer are minimal port stubs; each test wires only the
// behavior it drives through the HTTP stack.
type stubService struct {
	registerFn   func(context.Context, inbound.RegisterInput) (*user.User, error)
	loginFn      func(context.Context, string, string) (string, error)
	getProfileFn func(context.Context, uuid.UUID) (*user.User, error)
}

func (s stubService) Register(ctx context.Context, in inbound.RegisterInput) (*user.User, error) {
	return s.registerFn(ctx, in)
}
func (s stubService) Login(ctx context.Context, email, password string) (string, error) {
	return s.loginFn(ctx, email, password)
}
func (s stubService) GetProfile(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return s.getProfileFn(ctx, id)
}

type stubIssuer struct {
	verifyFn func(string) (uuid.UUID, error)
}

func (s stubIssuer) Issue(uuid.UUID) (string, error)        { return "", nil }
func (s stubIssuer) Verify(token string) (uuid.UUID, error) { return s.verifyFn(token) }

// newStack assembles the real HTTP stack (handler -> NewAPI -> mux) the same way
// main.go does, so tests exercise the actual routing, middleware, and error policy.
func newStack(t *testing.T, svc inbound.UserService, ti outbound.TokenIssuer) http.Handler {
	t.Helper()

	h, err := httpapi.NewAuthHandler(httpapi.NewAuthHandlerParams{Logger: slog.New(slog.DiscardHandler), Svc: svc})
	require.NoError(t, err)

	mux := http.NewServeMux()
	openapi.HandlerFromMux(httpapi.NewAPI(h, ti), mux)
	return mux
}

func buildUser(t *testing.T) *user.User {
	t.Helper()

	e, err := user.NewEmail("jane@example.com")
	require.NoError(t, err)
	h, err := user.NewPasswordHash(validHash)
	require.NoError(t, err)
	u, err := user.NewUser(user.NewUserParams{Email: e, PasswordHash: h, FirstName: "Jane", LastName: "Doe"})
	require.NoError(t, err)
	u.ID = uuid.New()
	return u
}

func do(
	t *testing.T,
	handler http.Handler,
	method, target string,
	body []byte,
	header http.Header,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, vals := range header {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestStack_RegisterRoute(t *testing.T) {
	created := buildUser(t)
	svc := stubService{registerFn: func(context.Context, inbound.RegisterInput) (*user.User, error) {
		return created, nil
	}}
	handler := newStack(t, svc, stubIssuer{})

	body, err := json.Marshal(openapi.RegisterRequest{
		FirstName: "Jane", LastName: "Doe",
		Email: openapi_types.Email("jane@example.com"), Password: "Passw0rd!",
	})
	require.NoError(t, err)

	rec := do(t, handler, http.MethodPost, "/api/auth/register", body, nil)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp openapi.UserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, created.ID, resp.Id)
	assert.Equal(t, openapi_types.Email("jane@example.com"), resp.Email)
}

func TestStack_LoginRoute(t *testing.T) {
	svc := stubService{loginFn: func(context.Context, string, string) (string, error) {
		return "signed.jwt.token", nil
	}}
	handler := newStack(t, svc, stubIssuer{})

	body, err := json.Marshal(openapi.LoginRequest{
		Email:    openapi_types.Email("jane@example.com"),
		Password: "Passw0rd!",
	})
	require.NoError(t, err)

	rec := do(t, handler, http.MethodPost, "/api/auth/login", body, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp openapi.TokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "signed.jwt.token", resp.Token)
}

func TestStack_MalformedJSONYields400(t *testing.T) {
	// RequestErrorHandlerFunc wired in NewAPI: a body-binding failure becomes a
	// JSON 400, not a panic or an empty response. The service must never be hit.
	svc := stubService{registerFn: func(context.Context, inbound.RegisterInput) (*user.User, error) {
		t.Fatal("service should not be called when the body is malformed")
		return nil, nil
	}}
	handler := newStack(t, svc, stubIssuer{})

	rec := do(t, handler, http.MethodPost, "/api/auth/register", []byte(`{"email":`), nil)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var e openapi.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	assert.NotEmpty(t, e.Message)
}

func TestStack_UnexpectedErrorYieldsGeneric500(t *testing.T) {
	// ResponseErrorHandlerFunc wired in NewAPI: a raw handler error becomes a
	// generic 500 that does not leak the underlying error text.
	svc := stubService{loginFn: func(context.Context, string, string) (string, error) {
		return "", errBoom
	}}
	handler := newStack(t, svc, stubIssuer{})

	body, err := json.Marshal(openapi.LoginRequest{
		Email:    openapi_types.Email("jane@example.com"),
		Password: "Passw0rd!",
	})
	require.NoError(t, err)

	rec := do(t, handler, http.MethodPost, "/api/auth/login", body, nil)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "boom", "internal error text must not leak to the client")
}

func TestStack_ProfileRequiresAuth(t *testing.T) {
	// getProfileFn nil: an unauthenticated request must be blocked by the
	// middleware before ever reaching the handler/service.
	handler := newStack(t, stubService{}, stubIssuer{})

	rec := do(t, handler, http.MethodGet, "/api/auth/profile", nil, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStack_ProfileWithValidTokenSucceeds(t *testing.T) {
	profile := buildUser(t)
	var askedID uuid.UUID
	svc := stubService{getProfileFn: func(_ context.Context, id uuid.UUID) (*user.User, error) {
		askedID = id
		return profile, nil
	}}
	// the issuer resolves any token to the profile's ID; this exercises the full
	// middleware -> context -> handler path end to end.
	ti := stubIssuer{verifyFn: func(string) (uuid.UUID, error) { return profile.ID, nil }}
	handler := newStack(t, svc, ti)

	rec := do(t, handler, http.MethodGet, "/api/auth/profile", nil, http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, profile.ID, askedID, "the ID verified from the token should reach the service")
	var resp openapi.UserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, profile.ID, resp.Id)
}
