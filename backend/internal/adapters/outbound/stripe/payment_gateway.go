// Package stripe is the adapter for implementing a payment gateway specific
// to the stripe client
package stripe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/stripe/stripe-go/v86"
)

// PaymentGateway is the adapter for making payment related calls out
// to Stripe
type PaymentGateway struct {
	client *stripe.Client
	logger *slog.Logger
}

var _ outbound.PaymentGateway = (*PaymentGateway)(nil)

// NewPaymentGatewayParams holds the parameters necessary to construct the adapter
type NewPaymentGatewayParams struct {
	Client *stripe.Client
	Logger *slog.Logger
}

// NewPaymentGateway constructs a new instance of the adapter
func NewPaymentGateway(p NewPaymentGatewayParams) (*PaymentGateway, error) {
	if p.Client == nil {
		return nil, errors.New("client is missing")
	}
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	return &PaymentGateway{
		client: p.Client,
		logger: p.Logger,
	}, nil
}

// Authorize sends a transaction to Stripe for authorization. Note: This does not actually transfer funds
func (g *PaymentGateway) Authorize(ctx context.Context, i outbound.AuthorizeInput) (outbound.AuthorizeResult, error) {
	authParams := stripe.PaymentIntentCreateParams{
		Amount:        stripe.Int64(i.Amount.Cents()),
		Currency:      stripe.String("usd"),
		PaymentMethod: stripe.String(i.PaymentMethod),
		CaptureMethod: stripe.String(stripe.PaymentIntentCaptureMethodManual),
		Confirm:       stripe.Bool(true),
	}
	authParams.AddMetadata("order_id", i.OrderID.String())
	authParams.SetIdempotencyKey("order-" + i.OrderID.String() + ":authorize")

	pi, err := g.client.V1PaymentIntents.Create(ctx, &authParams)
	if err != nil {
		var stripeErr *stripe.Error
		if errors.As(err, &stripeErr) {
			switch stripeErr.Type {
			case stripe.ErrorTypeCard:
				return outbound.AuthorizeResult{}, outbound.NewPaymentError(outbound.PaymentKindDeclined, err)
			case stripe.ErrorTypeInvalidRequest, stripe.ErrorTypeIdempotency:
				return outbound.AuthorizeResult{},
					outbound.NewPaymentError(outbound.PaymentKindUnexpected, err)
			}
		}
		return outbound.AuthorizeResult{}, fmt.Errorf("error authorizing payment: %w", err)
	}
	return outbound.AuthorizeResult{
		AuthorizationID: pi.ID,
	}, nil
}

// Capture takes an auth id and initiates the capture. This actually transfers the funds
func (g *PaymentGateway) Capture(
	ctx context.Context,
	authID string,
	amount shared.Money,
) (outbound.CaptureResult, error) {
	captureParams := stripe.PaymentIntentCaptureParams{
		AmountToCapture: stripe.Int64(amount.Cents()),
	}
	captureParams.SetIdempotencyKey(authID + ":capture")
	pi, err := g.client.V1PaymentIntents.Capture(ctx, authID, &captureParams)
	if err != nil {
		var stripeErr *stripe.Error
		if errors.As(err, &stripeErr) {
			if stripeErr.Type == stripe.ErrorTypeInvalidRequest ||
				stripeErr.Type == stripe.ErrorTypeIdempotency {
				return outbound.CaptureResult{},
					outbound.NewPaymentError(outbound.PaymentKindUnexpected, err)
			}
		}
		return outbound.CaptureResult{}, fmt.Errorf("error capturing payment: %w", err)
	}
	return outbound.CaptureResult{
		AuthorizationID: pi.ID,
	}, nil
}

// Void takes an auth id and voids the transaction
func (g *PaymentGateway) Void(ctx context.Context, authID string) error {
	voidParams := stripe.PaymentIntentCancelParams{}
	voidParams.SetIdempotencyKey(authID + ":void")
	_, err := g.client.V1PaymentIntents.Cancel(ctx, authID, &voidParams)
	if err != nil {
		return fmt.Errorf("error voiding payment: %w", err)
	}
	return nil
}
