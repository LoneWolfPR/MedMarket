package outbound

import "context"

// ShippingClient declares all the methods the adapters must implement
type ShippingClient interface {
	RegisterWebhook(ctx context.Context, trackingID, callbackURL string) error
}
