package bcrypt_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/bcrypt"

	bcryptlib "golang.org/x/crypto/bcrypt"
)

func TestNewPasswordHasher_RequiresLogger(t *testing.T) {
	h, err := bcrypt.NewPasswordHasher(bcrypt.NewPasswordHasherParams{})

	require.Error(t, err)
	assert.Nil(t, h)
}

// TestHash confirms our adapter returns the library's hash intact and usable —
// i.e. the []byte->string conversion and return value are wired correctly. The
// strength/format of the hash itself is bcrypt's concern, not ours.
func TestHash(t *testing.T) {
	h, err := bcrypt.NewPasswordHasher(bcrypt.NewPasswordHasherParams{Logger: slog.New(slog.DiscardHandler)})
	require.NoError(t, err)

	const plain = "Passw0rd!"
	hash, err := h.Hash(plain)

	require.NoError(t, err)
	assert.NotEqual(t, plain, hash, "hash must not be the plaintext")
	assert.True(t, strings.HasPrefix(hash, "$2"), "expected a bcrypt hash prefix, got %q", hash)
	// verify with the underlying library that the returned string is a real, usable hash
	assert.NoError(t, bcryptlib.CompareHashAndPassword([]byte(hash), []byte(plain)))
}
