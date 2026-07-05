package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
)

func TestNewPasswordHash_Valid(t *testing.T) {
	// A representative bcrypt hash — the constructor only rejects surrounding
	// whitespace / emptiness, it does not validate hash format.
	const hash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	got, err := user.NewPasswordHash(hash)

	require.NoError(t, err)
	assert.Equal(t, hash, got.String())
	assert.False(t, got.IsZero())
}

func TestNewPasswordHash_Invalid(t *testing.T) {
	tests := map[string]struct {
		raw     string
		wantErr error
	}{
		// surrounding whitespace is rejected before the emptiness check
		"leading space":   {raw: " hash", wantErr: user.ErrInvalidPasswordHash},
		"trailing space":  {raw: "hash ", wantErr: user.ErrInvalidPasswordHash},
		"surrounding tab": {raw: "\thash\n", wantErr: user.ErrInvalidPasswordHash},
		// whitespace-only trims to "" but differs from the input, so it is "invalid", not "empty"
		"whitespace only": {raw: "   ", wantErr: user.ErrInvalidPasswordHash},
		// only a truly empty string yields the empty sentinel
		"empty": {raw: "", wantErr: user.ErrEmptyPasswordHash},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := user.NewPasswordHash(tc.raw)

			require.ErrorIs(t, err, tc.wantErr)
			assert.True(t, got.IsZero(), "expected zero PasswordHash on error")
		})
	}
}
