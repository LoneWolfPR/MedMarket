package order_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	orderwf "github.com/LoneWolfPR/MedMarket/backend/workflows/order"
)

// --- AuthorizePaymentActivity ----------------------------------------------

func TestAuthorizePaymentActivity_InvalidAmountIsNonRetryable(t *testing.T) {
	// A negative amount can't rebuild into Money; retrying can't fix bad input.
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			authorizeFn: func(context.Context, outbound.AuthorizeInput) (outbound.AuthorizeResult, error) {
				t.Fatal("gateway must not be called with an invalid amount")
				return outbound.AuthorizeResult{}, nil
			},
		},
	})

	_, err := a.AuthorizePaymentActivity(context.Background(), orderwf.AuthorizeActivityInput{
		OrderID:       uuid.New(),
		AmountCents:   -1,
		PaymentMethod: "pm_card_visa",
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "ErrInvalidAuthorizationInput", appErrType(t, err))
}

func TestAuthorizePaymentActivity_PaymentErrorIsNonRetryable(t *testing.T) {
	// A terminal payment failure carries its Kind through as the ApplicationError
	// Type so the workflow can branch (declined vs unexpected).
	tests := map[string]struct {
		kind     outbound.PaymentErrorKind
		wantType string
	}{
		"declined":   {outbound.PaymentKindDeclined, "declined"},
		"unexpected": {outbound.PaymentKindUnexpected, "unexpected"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := orderwf.NewActivities(orderwf.NewActivitiesParams{
				PaymentGateway: fakePaymentGateway{
					authorizeFn: func(context.Context, outbound.AuthorizeInput) (outbound.AuthorizeResult, error) {
						return outbound.AuthorizeResult{}, outbound.NewPaymentError(tc.kind, errBoom)
					},
				},
			})

			_, err := a.AuthorizePaymentActivity(context.Background(), orderwf.AuthorizeActivityInput{
				OrderID:       uuid.New(),
				AmountCents:   1299,
				PaymentMethod: "pm_card_visa",
			})
			require.Error(t, err)
			assert.True(t, isNonRetryable(t, err))
			assert.Equal(t, tc.wantType, appErrType(t, err))
		})
	}
}

func TestAuthorizePaymentActivity_PlainErrorIsRetryable(t *testing.T) {
	// A transient gateway failure stays a plain error so the retry policy heals it.
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			authorizeFn: func(context.Context, outbound.AuthorizeInput) (outbound.AuthorizeResult, error) {
				return outbound.AuthorizeResult{}, errBoom
			},
		},
	})

	_, err := a.AuthorizePaymentActivity(context.Background(), orderwf.AuthorizeActivityInput{
		OrderID:       uuid.New(),
		AmountCents:   1299,
		PaymentMethod: "pm_card_visa",
	})
	require.ErrorIs(t, err, errBoom)
}

func TestAuthorizePaymentActivity_HappyPathTranslatesAndReturnsAuthID(t *testing.T) {
	orderID := uuid.New()
	var got outbound.AuthorizeInput
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			authorizeFn: func(_ context.Context, i outbound.AuthorizeInput) (outbound.AuthorizeResult, error) {
				got = i
				return outbound.AuthorizeResult{AuthorizationID: "pi_123"}, nil
			},
		},
	})

	authID, err := a.AuthorizePaymentActivity(context.Background(), orderwf.AuthorizeActivityInput{
		OrderID:       orderID,
		AmountCents:   1299,
		PaymentMethod: "pm_card_visa",
	})
	require.NoError(t, err)
	assert.Equal(t, "pi_123", authID)

	assert.Equal(t, orderID, got.OrderID)
	assert.Equal(t, int64(1299), got.Amount.Cents())
	assert.Equal(t, "pm_card_visa", got.PaymentMethod)
}

// --- CapturePaymentActivity -------------------------------------------------

func TestCapturePaymentActivity_InvalidAmountIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			captureFn: func(context.Context, string, shared.Money) (outbound.CaptureResult, error) {
				t.Fatal("gateway must not be called with an invalid amount")
				return outbound.CaptureResult{}, nil
			},
		},
	})

	_, err := a.CapturePaymentActivity(context.Background(), orderwf.CaptureActivityInput{
		AuthID:      "pi_123",
		AmountCents: -1,
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "ErrInvalidCaptureInput", appErrType(t, err))
}

func TestCapturePaymentActivity_PaymentErrorIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			captureFn: func(context.Context, string, shared.Money) (outbound.CaptureResult, error) {
				return outbound.CaptureResult{}, outbound.NewPaymentError(outbound.PaymentKindUnexpected, errBoom)
			},
		},
	})

	_, err := a.CapturePaymentActivity(context.Background(), orderwf.CaptureActivityInput{
		AuthID:      "pi_123",
		AmountCents: 1299,
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "unexpected", appErrType(t, err))
}

func TestCapturePaymentActivity_PlainErrorIsRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			captureFn: func(context.Context, string, shared.Money) (outbound.CaptureResult, error) {
				return outbound.CaptureResult{}, errBoom
			},
		},
	})

	_, err := a.CapturePaymentActivity(context.Background(), orderwf.CaptureActivityInput{
		AuthID:      "pi_123",
		AmountCents: 1299,
	})
	require.ErrorIs(t, err, errBoom)
}

func TestCapturePaymentActivity_HappyPathPassesAuthIDAndAmount(t *testing.T) {
	var gotAuthID string
	var gotAmount shared.Money
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			captureFn: func(_ context.Context, authID string, amount shared.Money) (outbound.CaptureResult, error) {
				gotAuthID = authID
				gotAmount = amount
				return outbound.CaptureResult{AuthorizationID: authID}, nil
			},
		},
	})

	authID, err := a.CapturePaymentActivity(context.Background(), orderwf.CaptureActivityInput{
		AuthID:      "pi_123",
		AmountCents: 1299,
	})
	require.NoError(t, err)
	assert.Equal(t, "pi_123", authID)
	assert.Equal(t, "pi_123", gotAuthID)
	assert.Equal(t, int64(1299), gotAmount.Cents())
}

// --- VoidPaymentActivity ----------------------------------------------------

func TestVoidPaymentActivity_ErrorIsRetryable(t *testing.T) {
	// Void classifies nothing; every failure is a plain (retryable) error.
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			voidFn: func(context.Context, string) error { return errBoom },
		},
	})

	err := a.VoidPaymentActivity(context.Background(), "pi_123")
	require.ErrorIs(t, err, errBoom)
}

func TestVoidPaymentActivity_HappyPathPassesAuthID(t *testing.T) {
	var gotAuthID string
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		PaymentGateway: fakePaymentGateway{
			voidFn: func(_ context.Context, authID string) error {
				gotAuthID = authID
				return nil
			},
		},
	})

	require.NoError(t, a.VoidPaymentActivity(context.Background(), "pi_123"))
	assert.Equal(t, "pi_123", gotAuthID)
}

// --- PlaceOrderActivity -----------------------------------------------------

func TestPlaceOrderActivity_UnknownClientIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		Clients: map[string]outbound.PharmacyClient{},
	})

	_, err := a.PlaceOrderActivity(context.Background(), validInput(1299))
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "MissingPharmacyClient", appErrType(t, err))
}

func TestPlaceOrderActivity_InvalidOrderInputIsNonRetryable(t *testing.T) {
	// Client exists, but the order input can't be built (empty item id).
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		Clients: map[string]outbound.PharmacyClient{
			"mock-pharmacy-a": fakePharmacyClient{
				placeOrderFn: func(context.Context, pharmacy.OrderInput) (pharmacy.OrderResult, error) {
					t.Fatal("the pharmacy must not be called with invalid input")
					return pharmacy.OrderResult{}, nil
				},
			},
		},
	})

	in := validInput(1299)
	in.PharmacyItemID = ""
	_, err := a.PlaceOrderActivity(context.Background(), in)
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "InvalidOrderInput", appErrType(t, err))
}

func TestPlaceOrderActivity_PlaceOrderErrorIsNonRetryable(t *testing.T) {
	// A terminal place-order failure carries its Kind through as the Type so the
	// workflow can branch (rejected vs outcome-unknown).
	tests := map[string]struct {
		kind     outbound.PlaceOrderKind
		wantType string
	}{
		"rejected":        {outbound.KindRejected, "rejected"},
		"outcome unknown": {outbound.KindOutcomeUnknown, "outcome_unknown"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := orderwf.NewActivities(orderwf.NewActivitiesParams{
				Clients: map[string]outbound.PharmacyClient{
					"mock-pharmacy-a": fakePharmacyClient{
						placeOrderFn: func(context.Context, pharmacy.OrderInput) (pharmacy.OrderResult, error) {
							return pharmacy.OrderResult{}, outbound.NewPlaceOrderError(tc.kind, errBoom)
						},
					},
				},
			})

			_, err := a.PlaceOrderActivity(context.Background(), validInput(1299))
			require.Error(t, err)
			assert.True(t, isNonRetryable(t, err))
			assert.Equal(t, tc.wantType, appErrType(t, err))
		})
	}
}

func TestPlaceOrderActivity_PlainErrorIsRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		Clients: map[string]outbound.PharmacyClient{
			"mock-pharmacy-a": fakePharmacyClient{
				placeOrderFn: func(context.Context, pharmacy.OrderInput) (pharmacy.OrderResult, error) {
					return pharmacy.OrderResult{}, errBoom
				},
			},
		},
	})

	_, err := a.PlaceOrderActivity(context.Background(), validInput(1299))
	require.ErrorIs(t, err, errBoom)
}

func TestPlaceOrderActivity_HappyPathTranslatesAndMapsResult(t *testing.T) {
	in := validInput(1299)
	var got pharmacy.OrderInput
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		Clients: map[string]outbound.PharmacyClient{
			"mock-pharmacy-a": fakePharmacyClient{
				placeOrderFn: func(_ context.Context, oi pharmacy.OrderInput) (pharmacy.OrderResult, error) {
					got = oi
					return buildOrderResult(t, 1299), nil
				},
			},
		},
	})

	res, err := a.PlaceOrderActivity(context.Background(), in)
	require.NoError(t, err)

	// The workflow Input was translated into the domain OrderInput for the port.
	assert.Equal(t, in.IdempotencyKey, got.IdempotencyKey())
	assert.Equal(t, in.PharmacyItemID, got.PharmacyItemID())
	assert.Equal(t, in.Qty, got.Qty())
	assert.Equal(t, in.RecipientName, got.RecipientName())
	assert.Equal(t, in.Address.Street1, got.ShippingAddress().Street1)
	assert.Equal(t, in.Address.Zip, got.ShippingAddress().Zip)

	// The domain result was flattened back into the activity result DTO.
	assert.Equal(t, "pharm-order-1", res.PharmacyOrderID)
	assert.Equal(t, "trk-1", res.TrackingID)
	assert.Equal(t, int64(1299), res.NewTotalCents)
	assert.Equal(t, "placed", res.OrderStatus)
}

// --- UpdateOrderActivity ----------------------------------------------------

func TestUpdateOrderActivity_NotFoundIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		OrderRepo: fakeOrderRepo{
			getByIDFn: func(context.Context, uuid.UUID) (*order.Order, error) {
				return nil, outbound.ErrOrderNotFound
			},
		},
	})

	err := a.UpdateOrderActivity(context.Background(), orderwf.UpdateOrderActivityInput{
		ID:     uuid.New(),
		Status: strPtr(order.StatusConfirmed),
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "OrderNotFound", appErrType(t, err))
}

func TestUpdateOrderActivity_InvalidStatusIsNonRetryable(t *testing.T) {
	id := uuid.New()
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		OrderRepo: fakeOrderRepo{
			getByIDFn: func(_ context.Context, gid uuid.UUID) (*order.Order, error) {
				return buildOrder(t, gid), nil
			},
			updateFn: func(context.Context, *order.Order) (*order.Order, error) {
				t.Fatal("update must not run with an invalid status")
				return nil, nil
			},
		},
	})

	err := a.UpdateOrderActivity(context.Background(), orderwf.UpdateOrderActivityInput{
		ID:     id,
		Status: strPtr("bogus-status"),
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "InvalidUpdateOrderInput", appErrType(t, err))
}

func TestUpdateOrderActivity_InvalidPricePaidIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		OrderRepo: fakeOrderRepo{
			getByIDFn: func(_ context.Context, gid uuid.UUID) (*order.Order, error) {
				return buildOrder(t, gid), nil
			},
			updateFn: func(context.Context, *order.Order) (*order.Order, error) {
				t.Fatal("update must not run with an invalid price")
				return nil, nil
			},
		},
	})

	err := a.UpdateOrderActivity(context.Background(), orderwf.UpdateOrderActivityInput{
		ID:             uuid.New(),
		PricePaidCents: int64Ptr(-1),
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "InvalidUpdateOrderInput", appErrType(t, err))
}

func TestUpdateOrderActivity_GetErrorIsRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		OrderRepo: fakeOrderRepo{
			getByIDFn: func(context.Context, uuid.UUID) (*order.Order, error) {
				return nil, errBoom
			},
		},
	})

	err := a.UpdateOrderActivity(context.Background(), orderwf.UpdateOrderActivityInput{
		ID:     uuid.New(),
		Status: strPtr(order.StatusConfirmed),
	})
	require.ErrorIs(t, err, errBoom)
}

func TestUpdateOrderActivity_HappyPathAppliesFields(t *testing.T) {
	id := uuid.New()
	var updated *order.Order
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		OrderRepo: fakeOrderRepo{
			getByIDFn: func(_ context.Context, gid uuid.UUID) (*order.Order, error) {
				return buildOrder(t, gid), nil
			},
			updateFn: func(_ context.Context, o *order.Order) (*order.Order, error) {
				updated = o
				return o, nil
			},
		},
	})

	err := a.UpdateOrderActivity(context.Background(), orderwf.UpdateOrderActivityInput{
		ID:              id,
		Status:          strPtr(order.StatusConfirmed),
		PharmacyOrderID: strPtr("pharm-order-1"),
		TrackingID:      strPtr("trk-1"),
		PricePaidCents:  int64Ptr(1299),
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, order.StatusConfirmed, updated.Status)
	assert.Equal(t, "pharm-order-1", updated.PharmacyOrderID)
	assert.Equal(t, "trk-1", updated.TrackingID)
	require.NotNil(t, updated.PricePaid)
	assert.Equal(t, int64(1299), updated.PricePaid.Cents())
}

func TestUpdateOrderActivity_UpdateErrorIsRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		OrderRepo: fakeOrderRepo{
			getByIDFn: func(_ context.Context, gid uuid.UUID) (*order.Order, error) {
				return buildOrder(t, gid), nil
			},
			updateFn: func(context.Context, *order.Order) (*order.Order, error) {
				return nil, errBoom
			},
		},
	})

	err := a.UpdateOrderActivity(context.Background(), orderwf.UpdateOrderActivityInput{
		ID:     uuid.New(),
		Status: strPtr(order.StatusShipped),
	})
	require.ErrorIs(t, err, errBoom)
}

// --- RegisterWebhookActivity ------------------------------------------------

func TestRegisterWebhookActivity_MissingFieldsAreNonRetryable(t *testing.T) {
	tests := map[string]orderwf.RegisterWebhookActivityInput{
		"empty tracking": {TrackingID: "  ", CallbackURL: "http://x/cb"},
		"empty callback": {TrackingID: "trk-1", CallbackURL: " "},
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			a := orderwf.NewActivities(orderwf.NewActivitiesParams{
				ShippingClient: fakeShippingClient{
					registerFn: func(context.Context, string, string) error {
						t.Fatal("shipping client must not be called with missing fields")
						return nil
					},
				},
			})
			err := a.RegisterWebhookActivity(context.Background(), in)
			require.Error(t, err)
			assert.True(t, isNonRetryable(t, err))
		})
	}
}

func TestRegisterWebhookActivity_RejectedIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		ShippingClient: fakeShippingClient{
			registerFn: func(context.Context, string, string) error { return outbound.ErrShippingRejected },
		},
	})

	err := a.RegisterWebhookActivity(context.Background(), orderwf.RegisterWebhookActivityInput{
		TrackingID:  "trk-1",
		CallbackURL: "http://x/cb",
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "ErrShippingRejected", appErrType(t, err))
}

func TestRegisterWebhookActivity_PlainErrorIsRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		ShippingClient: fakeShippingClient{
			registerFn: func(context.Context, string, string) error { return errBoom },
		},
	})

	err := a.RegisterWebhookActivity(context.Background(), orderwf.RegisterWebhookActivityInput{
		TrackingID:  "trk-1",
		CallbackURL: "http://x/cb",
	})
	require.ErrorIs(t, err, errBoom)
}

func TestRegisterWebhookActivity_HappyPathPassesTrimmedValues(t *testing.T) {
	var gotTracking, gotCallback string
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		ShippingClient: fakeShippingClient{
			registerFn: func(_ context.Context, tracking, callback string) error {
				gotTracking = tracking
				gotCallback = callback
				return nil
			},
		},
	})

	err := a.RegisterWebhookActivity(context.Background(), orderwf.RegisterWebhookActivityInput{
		TrackingID:  "  trk-1  ",
		CallbackURL: "  http://x/cb  ",
	})
	require.NoError(t, err)
	assert.Equal(t, "trk-1", gotTracking)
	assert.Equal(t, "http://x/cb", gotCallback)
}

// --- SendEmailUpdate --------------------------------------------------------

func TestSendEmailUpdate_InvalidToIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		EmailSender: fakeEmailSender{
			sendFn: func(context.Context, outbound.EmailSenderParams) error {
				t.Fatal("sender must not be called with an invalid address")
				return nil
			},
		},
	})

	err := a.SendEmailUpdate(context.Background(), orderwf.EmailUpdateInput{
		To:      "not-an-email",
		Message: "hi",
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
}

func TestSendEmailUpdate_EmptyMessageIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		EmailSender: fakeEmailSender{
			sendFn: func(context.Context, outbound.EmailSenderParams) error {
				t.Fatal("sender must not be called with an empty message")
				return nil
			},
		},
	})

	err := a.SendEmailUpdate(context.Background(), orderwf.EmailUpdateInput{
		To:      "jane@example.com",
		Message: "",
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
}

func TestSendEmailUpdate_InvalidMessageIsNonRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		EmailSender: fakeEmailSender{
			sendFn: func(context.Context, outbound.EmailSenderParams) error { return outbound.ErrInvalidMessage },
		},
	})

	err := a.SendEmailUpdate(context.Background(), orderwf.EmailUpdateInput{
		To:      "jane@example.com",
		Message: "hi",
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(t, err))
	assert.Equal(t, "ErrInvalidMessage", appErrType(t, err))
}

func TestSendEmailUpdate_PlainErrorIsRetryable(t *testing.T) {
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		EmailSender: fakeEmailSender{
			sendFn: func(context.Context, outbound.EmailSenderParams) error { return errBoom },
		},
	})

	err := a.SendEmailUpdate(context.Background(), orderwf.EmailUpdateInput{
		To:      "jane@example.com",
		Message: "hi",
	})
	require.ErrorIs(t, err, errBoom)
}

func TestSendEmailUpdate_HappyPathPassesParams(t *testing.T) {
	var got outbound.EmailSenderParams
	a := orderwf.NewActivities(orderwf.NewActivitiesParams{
		EmailSender: fakeEmailSender{
			sendFn: func(_ context.Context, p outbound.EmailSenderParams) error {
				got = p
				return nil
			},
		},
	})

	err := a.SendEmailUpdate(context.Background(), orderwf.EmailUpdateInput{
		To:      "jane@example.com",
		Message: "your order shipped",
	})
	require.NoError(t, err)
	assert.Equal(t, "jane@example.com", got.To.String())
	assert.Equal(t, "your order shipped", got.Message)
	assert.NotEmpty(t, got.Subject)
}

// --- small pointer helpers --------------------------------------------------

func strPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64 { return &i }
