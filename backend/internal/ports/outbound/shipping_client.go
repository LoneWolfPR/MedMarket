package outbound

import (
	"context"
	"errors"
)

// ErrShippingRejected marks a RegisterWebhook failure as deterministic: the
// same call will fail the same way. Implementations wrap it around invalid
// input and rejected responses, and return transient failures bare.
//
//nolint:revive // sentinel error
var ErrShippingRejected = errors.New("shipping registration rejected")

// ShippingClient declares all the methods the adapters must implement
type ShippingClient interface {
	RegisterWebhook(ctx context.Context, trackingID, callbackURL string) error
}
