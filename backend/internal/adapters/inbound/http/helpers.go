package http

import (
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	msgInternalServerError = "internal server error"
)

// toUserResponse maps a domain user onto the OpenAPI UserResponse DTO, applying
// the optional-field conventions (empty phone / zero address are omitted).
// Shared by the register and profile handlers, whose success responses are both
// defined as UserResponse.
func toUserResponse(u *user.User) openapi.UserResponse {
	resp := openapi.UserResponse{
		Id:        u.ID,
		Email:     openapi_types.Email(u.Email.String()),
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Address:   MapToOAPIAddress(u.Address),
	}
	if u.Phone != "" {
		resp.Phone = ptr.To(u.Phone)
	}
	return resp
}

// MapToSharedAddress converts an optional OpenAPI address DTO into the domain
// address value. A nil DTO (address omitted by the client) yields a zero-value
// address; an omitted street2 becomes an empty string.
func MapToSharedAddress(addr *openapi.Address) shared.Address {
	if addr == nil {
		return shared.Address{}
	}
	return shared.Address{
		Street1: addr.Street1,
		Street2: ptr.Deref(addr.Street2),
		City:    addr.City,
		State:   addr.State,
		Zip:     addr.Zip,
	}
}

// MapToOAPIAddress converts a domain address into the optional OpenAPI address
// DTO. A zero-value address returns nil so the field is omitted entirely; an
// empty street2 is likewise omitted.
func MapToOAPIAddress(addr shared.Address) *openapi.Address {
	if addr == (shared.Address{}) {
		return nil
	}
	oapiAddr := &openapi.Address{
		Street1: addr.Street1,
		City:    addr.City,
		State:   addr.State,
		Zip:     addr.Zip,
	}
	if addr.Street2 != "" {
		oapiAddr.Street2 = ptr.To(addr.Street2)
	}
	return oapiAddr
}
