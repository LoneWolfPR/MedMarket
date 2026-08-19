package order_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	orderwf "github.com/LoneWolfPR/MedMarket/backend/workflows/order"
)

// errBoom stands in for an unexpected infrastructure failure (a retryable one:
// a plain error the activity does not classify as terminal).
var errBoom = errors.New("boom")

// Hand-rolled fakes for the outbound ports the activities depend on. Each method
// delegates to a function field so a test sets only the behavior it exercises;
// an unset field panics, surfacing an unexpected call as a test failure.

type fakeOrderRepo struct {
	createFn  func(ctx context.Context, o *order.Order) (*order.Order, error)
	updateFn  func(ctx context.Context, o *order.Order) (*order.Order, error)
	getByIDFn func(ctx context.Context, id uuid.UUID) (*order.Order, error)
	listFn    func(ctx context.Context, rxIDs []uuid.UUID) ([]order.Order, error)
}

func (f fakeOrderRepo) Create(ctx context.Context, o *order.Order) (*order.Order, error) {
	return f.createFn(ctx, o)
}

func (f fakeOrderRepo) Update(ctx context.Context, o *order.Order) (*order.Order, error) {
	return f.updateFn(ctx, o)
}

func (f fakeOrderRepo) GetByID(ctx context.Context, id uuid.UUID) (*order.Order, error) {
	return f.getByIDFn(ctx, id)
}

// No order activity lists orders — the method exists only to satisfy the port.
// Leaving listFn nil means a call panics rather than passing silently.
func (f fakeOrderRepo) List(ctx context.Context, rxIDs []uuid.UUID) ([]order.Order, error) {
	return f.listFn(ctx, rxIDs)
}

type fakeShippingClient struct {
	registerFn func(ctx context.Context, trackingID, callbackURL string) error
}

func (f fakeShippingClient) RegisterWebhook(ctx context.Context, trackingID, callbackURL string) error {
	return f.registerFn(ctx, trackingID, callbackURL)
}

type fakeEmailSender struct {
	sendFn func(ctx context.Context, params outbound.EmailSenderParams) error
}

func (f fakeEmailSender) Send(ctx context.Context, params outbound.EmailSenderParams) error {
	return f.sendFn(ctx, params)
}

type fakePaymentGateway struct {
	authorizeFn func(ctx context.Context, i outbound.AuthorizeInput) (outbound.AuthorizeResult, error)
	captureFn   func(ctx context.Context, authID string, amount shared.Money) (outbound.CaptureResult, error)
	voidFn      func(ctx context.Context, authID string) error
}

func (f fakePaymentGateway) Authorize(
	ctx context.Context, i outbound.AuthorizeInput,
) (outbound.AuthorizeResult, error) {
	return f.authorizeFn(ctx, i)
}

func (f fakePaymentGateway) Capture(
	ctx context.Context, authID string, amount shared.Money,
) (outbound.CaptureResult, error) {
	return f.captureFn(ctx, authID, amount)
}

func (f fakePaymentGateway) Void(ctx context.Context, authID string) error {
	return f.voidFn(ctx, authID)
}

type fakePharmacyClient struct {
	searchFn     func(ctx context.Context, c pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error)
	placeOrderFn func(ctx context.Context, i pharmacy.OrderInput) (pharmacy.OrderResult, error)
}

func (f fakePharmacyClient) Search(
	ctx context.Context, c pharmacy.SearchCriteria,
) ([]pharmacy.PriceQuote, error) {
	return f.searchFn(ctx, c)
}

func (f fakePharmacyClient) PlaceOrder(
	ctx context.Context, i pharmacy.OrderInput,
) (pharmacy.OrderResult, error) {
	return f.placeOrderFn(ctx, i)
}

// --- shared helpers ---------------------------------------------------------

// isNonRetryable reports whether err is an ApplicationError Temporal will not retry.
func isNonRetryable(t *testing.T, err error) bool {
	t.Helper()
	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr, "expected a temporal ApplicationError")
	return appErr.NonRetryable()
}

// appErrType returns the ApplicationError Type the workflow branches on.
func appErrType(t *testing.T, err error) string {
	t.Helper()
	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr, "expected a temporal ApplicationError")
	return appErr.Type()
}

// --- fixtures ---------------------------------------------------------------

// validInput builds a workflow Input whose fields rebuild into valid domain
// objects. amountCents drives the $0-skip vs paid branches.
func validInput(amountCents int64) orderwf.Input {
	return orderwf.Input{
		IdempotencyKey: "order-idem-key",
		OrderID:        uuid.New(),
		PharmacyCode:   "mock-pharmacy-a",
		PharmacyItemID: "sku-1",
		ItemName:       "Atorvastatin 20mg",
		RecipientName:  "Jane Doe",
		RecipientEmail: "jane@example.com",
		Qty:            2,
		Address: orderwf.ShippingAddress{
			Street1: "100 Market St",
			City:    "Springfield",
			State:   "IL",
			Zip:     "62704",
		},
		WebhookBaseURL: "http://backend:8080",
		AmountCents:    amountCents,
		PaymentMethod:  "pm_card_visa",
	}
}

// buildOrderResult builds a valid pharmacy OrderResult for the place-order fake.
func buildOrderResult(t *testing.T, cents int64) pharmacy.OrderResult {
	t.Helper()
	total, err := shared.NewMoneyFromCents(cents)
	require.NoError(t, err)
	res, err := pharmacy.NewOrderResult(pharmacy.NewOrderResultParams{
		PharmacyOrderID: "pharm-order-1",
		TrackingID:      "trk-1",
		NewTotal:        total,
		OrderStatus:     "placed",
	})
	require.NoError(t, err)
	return res
}

// buildOrder builds a persisted-looking order for the order repo fakes.
func buildOrder(t *testing.T, id uuid.UUID) *order.Order {
	t.Helper()
	o, err := order.NewOrder(order.NewOrderParams{
		PrescriptionID: uuid.New(),
		OfferID:        uuid.New(),
		Qty:            2,
	})
	require.NoError(t, err)
	o.ID = id
	return o
}
