package jwt_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/jwt"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var testSecret = []byte("super-secret-test-key")

func newIssuer(t *testing.T, ttl time.Duration) *jwt.TokenIssuer {
	t.Helper()

	i, err := jwt.NewTokenIssuer(jwt.NewTokenIssuerParams{
		Logger: slog.New(slog.DiscardHandler),
		TTL:    ttl,
		Secret: testSecret,
	})
	require.NoError(t, err)
	return i
}

// makeToken crafts a signed token with the underlying library so Verify can be
// tested in isolation.
func makeToken(t *testing.T, method jwtlib.SigningMethod, key any, claims jwtlib.RegisteredClaims) string {
	t.Helper()

	signed, err := jwtlib.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestNewTokenIssuer_Validation(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	tests := map[string]jwt.NewTokenIssuerParams{
		"missing logger": {TTL: time.Hour, Secret: testSecret},
		"empty secret":   {Logger: logger, TTL: time.Hour, Secret: nil},
		"zero ttl":       {Logger: logger, TTL: 0, Secret: testSecret},
		"negative ttl":   {Logger: logger, TTL: -time.Hour, Secret: testSecret},
	}

	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			i, err := jwt.NewTokenIssuer(params)

			require.Error(t, err)
			assert.Nil(t, i)
		})
	}

	t.Run("valid params", func(t *testing.T) {
		i, err := jwt.NewTokenIssuer(jwt.NewTokenIssuerParams{Logger: logger, TTL: time.Hour, Secret: testSecret})

		require.NoError(t, err)
		assert.NotNil(t, i)
	})
}

// TestIssue checks the claims WE build — subject is the user ID, expiry is
// now+ttl — by inspecting the token with the underlying library (not Verify).
func TestIssue(t *testing.T) {
	const ttl = time.Hour
	i := newIssuer(t, ttl)
	id := uuid.New()

	token, err := i.Issue(id)
	require.NoError(t, err)

	parsed, err := jwtlib.ParseWithClaims(token, &jwtlib.RegisteredClaims{}, func(*jwtlib.Token) (any, error) {
		return testSecret, nil
	})
	require.NoError(t, err)

	claims, ok := parsed.Claims.(*jwtlib.RegisteredClaims)
	require.True(t, ok)
	assert.Equal(t, id.String(), claims.Subject)
	assert.WithinDuration(t, time.Now().Add(ttl), claims.ExpiresAt.Time, 5*time.Second)
	assert.WithinDuration(t, time.Now(), claims.IssuedAt.Time, 5*time.Second)
}

// TestVerify_Valid covers our happy path: a well-formed token yields the
// subject parsed back into a uuid.
func TestVerify_Valid(t *testing.T) {
	i := newIssuer(t, time.Hour)
	id := uuid.New()
	token := makeToken(t, jwtlib.SigningMethodHS256, testSecret, jwtlib.RegisteredClaims{
		Subject:   id.String(),
		ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
	})

	got, err := i.Verify(token)

	require.NoError(t, err)
	assert.Equal(t, id, got)
}

// TestVerify_RejectsNonHMAC covers OUR signing-method guard in the keyfunc —
// an alg-confusion defense we wrote, not library behavior.
func TestVerify_RejectsNonHMAC(t *testing.T) {
	i := newIssuer(t, time.Hour)
	token := makeToken(t, jwtlib.SigningMethodNone, jwtlib.UnsafeAllowNoneSignatureType, jwtlib.RegisteredClaims{
		Subject:   uuid.New().String(),
		ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
	})

	got, err := i.Verify(token)

	require.Error(t, err)
	assert.Equal(t, uuid.Nil, got)
}

// TestVerify_RejectsNonUUIDSubject covers OUR subject->uuid parsing: a properly
// signed token whose subject isn't a uuid must fail.
func TestVerify_RejectsNonUUIDSubject(t *testing.T) {
	i := newIssuer(t, time.Hour)
	token := makeToken(t, jwtlib.SigningMethodHS256, testSecret, jwtlib.RegisteredClaims{
		Subject:   "not-a-uuid",
		ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
	})

	got, err := i.Verify(token)

	require.Error(t, err)
	assert.Equal(t, uuid.Nil, got)
}
