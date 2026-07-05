package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
)

// fakeTokenIssuer stubs the outbound.TokenIssuer port; the middleware only calls Verify.
type fakeTokenIssuer struct {
	verifyFn func(token string) (uuid.UUID, error)
}

func (f fakeTokenIssuer) Issue(uuid.UUID) (string, error)        { panic("not used") }
func (f fakeTokenIssuer) Verify(token string) (uuid.UUID, error) { return f.verifyFn(token) }

const protectedOp = "GetProfile"

// spyNext records whether the wrapped handler was reached and the context it saw.
type spyNext struct {
	called bool
	ctx    context.Context
}

func (s *spyNext) fn() openapi.StrictHandlerFunc {
	return func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		s.called = true
		s.ctx = ctx
		return "downstream-response", nil
	}
}

// invoke wires the middleware around the spy for the given operation and calls it.
func invoke(
	ti fakeTokenIssuer, operationID, authHeader string,
) (spy *spyNext, rec *httptest.ResponseRecorder, resp any, err error) {
	spy = &spyNext{}
	mw := newAuthMiddleware(ti, map[string]struct{}{protectedOp: {}})
	wrapped := mw(spy.fn(), operationID)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil)
	if authHeader != "" {
		req.Header.Set(headerNameAuth, authHeader)
	}
	rec = httptest.NewRecorder()

	resp, err = wrapped(req.Context(), rec, req, nil)
	return spy, rec, resp, err
}

func TestAuthMiddleware_ValidTokenInjectsUserID(t *testing.T) {
	id := uuid.New()
	ti := fakeTokenIssuer{verifyFn: func(token string) (uuid.UUID, error) {
		assert.Equal(t, "good-token", token, "the bearer token should be passed to Verify")
		return id, nil
	}}

	spy, rec, resp, err := invoke(ti, protectedOp, "Bearer good-token")

	require.NoError(t, err)
	require.True(t, spy.called, "next handler should be invoked on success")
	assert.Equal(t, "downstream-response", resp)
	// the verified user ID is placed into the context the downstream handler sees
	gotID, ok := getUserIDKeyValue(spy.ctx)
	require.True(t, ok)
	assert.Equal(t, id, gotID)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_LowercaseSchemeAccepted(t *testing.T) {
	id := uuid.New()
	ti := fakeTokenIssuer{verifyFn: func(string) (uuid.UUID, error) { return id, nil }}

	spy, _, _, err := invoke(ti, protectedOp, "bearer good-token")

	require.NoError(t, err)
	assert.True(t, spy.called, "scheme match should be case-insensitive")
}

func TestAuthMiddleware_UnprotectedOperationSkipsAuth(t *testing.T) {
	// an operation absent from the protected set passes straight through, even
	// with no Authorization header — this is our gating logic.
	ti := fakeTokenIssuer{verifyFn: func(string) (uuid.UUID, error) {
		t.Fatal("Verify must not be called for an unprotected operation")
		return uuid.Nil, nil
	}}

	spy, rec, resp, err := invoke(ti, "LoginUser", "")

	require.NoError(t, err)
	assert.True(t, spy.called)
	assert.Equal(t, "downstream-response", resp)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_Rejects(t *testing.T) {
	tests := map[string]struct {
		authHeader string
		verifyFn   func(string) (uuid.UUID, error)
	}{
		"missing header": {
			authHeader: "",
		},
		"no scheme separator": {
			authHeader: "Bearertoken",
		},
		"wrong scheme": {
			authHeader: "Basic dXNlcjpwYXNz",
		},
		"verify fails": {
			authHeader: "Bearer bad-token",
			verifyFn:   func(string) (uuid.UUID, error) { return uuid.Nil, errors.New("expired") },
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ti := fakeTokenIssuer{verifyFn: tc.verifyFn} // nil unless the case reaches Verify

			spy, rec, resp, err := invoke(ti, protectedOp, tc.authHeader)

			// short-circuit contract: no error, nil response, 401 written, next untouched
			require.NoError(t, err)
			assert.Nil(t, resp)
			assert.False(t, spy.called, "next handler must not run when auth fails")
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Contains(t, rec.Body.String(), msgUnauthorized)
		})
	}
}
