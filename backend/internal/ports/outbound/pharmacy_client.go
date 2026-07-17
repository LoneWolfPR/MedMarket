package outbound

import (
	"context"
	"fmt"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
)

// PlaceOrderKind represents the possible error types when placing an order
type PlaceOrderKind string

//nolint:revive // these are obvious
const (
	KindRejected       PlaceOrderKind = "rejected"
	KindOutcomeUnknown PlaceOrderKind = "outcome_unknown"
)

// PlaceOrderError holds information for errors occurring while make calls
// out to the api to place an order
type PlaceOrderError struct {
	Kind  PlaceOrderKind
	cause error
}

// NewPlaceOrderError constructs a representation of an error occurring during
// an order
func NewPlaceOrderError(kind PlaceOrderKind, cause error) *PlaceOrderError {
	return &PlaceOrderError{
		Kind:  kind,
		cause: cause,
	}
}
func (e *PlaceOrderError) Unwrap() error { return e.cause }
func (e *PlaceOrderError) Error() string {
	return fmt.Sprintf("%s: %v", e.Kind, e.cause)
}

// PharmacyClient declares all the methods the adapter must implement
type PharmacyClient interface {
	Search(ctx context.Context, c pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error)
	PlaceOrder(ctx context.Context, i pharmacy.OrderInput) (pharmacy.OrderResult, error)
}
