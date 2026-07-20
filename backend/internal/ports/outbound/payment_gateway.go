package outbound

import (
	"context"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"

	"github.com/google/uuid"
)

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
