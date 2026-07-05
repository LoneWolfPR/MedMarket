package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
)

func TestMapToSharedAddress(t *testing.T) {
	t.Run("nil DTO yields zero address", func(t *testing.T) {
		assert.Equal(t, shared.Address{}, MapToSharedAddress(nil))
	})

	t.Run("omitted street2 becomes empty string", func(t *testing.T) {
		got := MapToSharedAddress(&openapi.Address{Street1: "1 Main St", City: "Anytown", State: "CA", Zip: "90001"})

		assert.Equal(t, shared.Address{Street1: "1 Main St", City: "Anytown", State: "CA", Zip: "90001"}, got)
		assert.Empty(t, got.Street2)
	})

	t.Run("street2 present is preserved", func(t *testing.T) {
		got := MapToSharedAddress(&openapi.Address{
			Street1: "1 Main St", Street2: ptr.To("Apt 2"), City: "Anytown", State: "CA", Zip: "90001",
		})

		assert.Equal(t, "Apt 2", got.Street2)
	})
}

func TestMapToOAPIAddress(t *testing.T) {
	t.Run("zero address yields nil DTO", func(t *testing.T) {
		assert.Nil(t, MapToOAPIAddress(shared.Address{}))
	})

	t.Run("empty street2 is omitted", func(t *testing.T) {
		got := MapToOAPIAddress(shared.Address{Street1: "1 Main St", City: "Anytown", State: "CA", Zip: "90001"})

		require.NotNil(t, got)
		assert.Nil(t, got.Street2)
	})

	t.Run("street2 present becomes a pointer", func(t *testing.T) {
		got := MapToOAPIAddress(shared.Address{
			Street1: "1 Main St", Street2: "Apt 2", City: "Anytown", State: "CA", Zip: "90001",
		})

		require.NotNil(t, got)
		require.NotNil(t, got.Street2)
		assert.Equal(t, "Apt 2", *got.Street2)
	})
}

// TestToUserResponse_OmitsEmptyOptionals verifies the optional-field convention:
// an empty phone and a zero address are omitted (nil) in the response DTO.
func TestToUserResponse_OmitsEmptyOptionals(t *testing.T) {
	email, err := user.NewEmail("jane@example.com")
	require.NoError(t, err)
	u := &user.User{ID: uuid.New(), Email: email, FirstName: "Jane", LastName: "Doe"} // no phone, zero address

	resp := toUserResponse(u)

	assert.Equal(t, u.ID, resp.Id)
	assert.Equal(t, "Jane", resp.FirstName)
	assert.Nil(t, resp.Phone, "empty phone should be omitted")
	assert.Nil(t, resp.Address, "zero address should be omitted")
}
