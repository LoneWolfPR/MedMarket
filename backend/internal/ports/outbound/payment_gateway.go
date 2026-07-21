package outbound

import (
	"context"
	"fmt"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

// PaymentErrorKind represents the possible error types when running a payment
type PaymentErrorKind string

//nolint:revive // these are obvious
const (
	PaymentKindDeclined   PaymentErrorKind = "declined"
	PaymentKindUnexpected PaymentErrorKind = "unexpected"
)

// PaymentError holds information on errors occurring when trying to
// process a payment
type PaymentError struct {
	Kind  PaymentErrorKind
	cause error
}

// NewPaymentError constructs a new instance of a payment error
func NewPaymentError(kind PaymentErrorKind, cause error) *PaymentError {
	return &PaymentError{
		Kind:  kind,
		cause: cause,
	}
}

func (e *PaymentError) Unwrap() error { return e.cause }
func (e *PaymentError) Error() string {
	return fmt.Sprintf("%s: %v", e.Kind, e.cause)
}

// AuthorizeInput holds the properties necessary to authorize a transaction
type AuthorizeInput struct {
	OrderID       uuid.UUID
	Amount        shared.Money
	PaymentMethod string
}

// AuthorizeResult holds the properties from the result of trying to authorize
// a transaction
type AuthorizeResult struct {
	AuthorizationID string
}

// CaptureResult holds the properties from the result of trying to capture payment
type CaptureResult struct {
	AuthorizationID string
}

// PaymentGateway defines the methods the adapter needs to implement
type PaymentGateway interface {
	Authorize(ctx context.Context, i AuthorizeInput) (AuthorizeResult, error)
	Capture(ctx context.Context, authID string, amount shared.Money) (CaptureResult, error)
	Void(ctx context.Context, authID string) error
}
