package user_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
)

func TestNewPassword_Valid(t *testing.T) {
	tests := map[string]string{
		"minimum length boundary": "Aa1!aaaa",                           // exactly 8 runes
		"maximum length boundary": strings.Repeat("Aa1!", 8),            // exactly 32 runes
		"typical password":        "Passw0rd!",                          // 9 runes
		"every allowed special":   "Aa1" + user.AllowedSpecialChars[:5], // pulls specials from the allow-list
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := user.NewPassword(raw)

			require.NoError(t, err)
			assert.Equal(t, raw, got.String(), "password should be stored verbatim")
			assert.False(t, got.IsZero())
		})
	}
}

func TestNewPassword_Invalid(t *testing.T) {
	tests := map[string]struct {
		raw     string
		wantErr error
	}{
		// length is checked first, before character/class rules
		"empty":     {raw: "", wantErr: user.ErrInvalidPasswordLength},
		"too short": {raw: "Aa1!aaa", wantErr: user.ErrInvalidPasswordLength},                       // 7 runes
		"too long":  {raw: strings.Repeat("Aa1!", 8) + "A", wantErr: user.ErrInvalidPasswordLength}, // 33 runes

		// disallowed characters (still within length bounds)
		"contains space": {raw: "Passw0rd !", wantErr: user.ErrInvalidCharacter},
		"contains emoji": {raw: "Passw0rd\U0001F600", wantErr: user.ErrInvalidCharacter},

		// all characters allowed, but a required class is missing
		"no upper":   {raw: "passw0rd!", wantErr: user.ErrInvalidPassword},
		"no lower":   {raw: "PASSW0RD!", wantErr: user.ErrInvalidPassword},
		"no digit":   {raw: "Password!", wantErr: user.ErrInvalidPassword},
		"no special": {raw: "Passw0rd", wantErr: user.ErrInvalidPassword},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := user.NewPassword(tc.raw)

			require.ErrorIs(t, err, tc.wantErr)
			assert.True(t, got.IsZero(), "expected zero Password on error")
		})
	}
}
