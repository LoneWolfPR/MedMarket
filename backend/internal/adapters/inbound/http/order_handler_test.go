package http_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

const orderPath = "/api/orders"

// stubOrderService satisfies inbound.OrderService; each test wires only the
// behavior it drives through the HTTP stack.
type stubOrderService struct {
	placeOrderFn func(context.Context, inbound.OrderInput) (inbound.OrderView, error)
}

func (s stubOrderService) PlaceOrder(
	ctx context.Context, i inbound.OrderInput,
) (inbound.OrderView, error) {
	return s.placeOrderFn(ctx, i)
}

// newOrderStack assembles the real HTTP stack with a configurable order service,
// so tests exercise the actual routing, auth middleware, and error policy.
func newOrderStack(
	t *testing.T, orderSvc inbound.OrderService, ti outbound.TokenIssuer,
) http.Handler {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	ah, err := httpapi.NewAuthHandler(httpapi.NewAuthHandlerParams{Logger: logger, Svc: stubService{}})
	require.NoError(t, err)
	ph, err := httpapi.NewPrescriptionHandler(httpapi.NewPrescriptionHandlerParams{
		Logger: logger,
		Svc:    stubPrescriptionService{},
	})
	require.NoError(t, err)
	sh, err := httpapi.NewPriceSearchHandler(httpapi.NewPriceSearchHandlerParams{
		Logger: logger,
		Svc:    stubPriceSearchService{},
	})
	require.NoError(t, err)
	oh, err := httpapi.NewOrderHandler(httpapi.NewOrderHandlerParams{Logger: logger, Svc: orderSvc})
	require.NoError(t, err)

	return httpapi.NewAPI(httpapi.NewAPIParams{
		Auth:         ah,
		Prescription: ph,
		Search:       sh,
		Order:        oh,
		Logger:       logger,
		TokenIssuer:  ti,
	}, http.NewServeMux())
}

func orderBody(t *testing.T, offerID uuid.UUID, paymentMethod string) []byte {
	t.Helper()
	body, err := json.Marshal(openapi.OrderRequest{OfferId: offerID, PaymentMethod: paymentMethod})
	require.NoError(t, err)
	return body
}

func TestCreateOrderRoute_RequiresAuth(t *testing.T) {
	svc := stubOrderService{
		placeOrderFn: func(context.Context, inbound.OrderInput) (inbound.OrderView, error) {
			t.Fatal("service must not be reached without auth")
			return inbound.OrderView{}, nil
		},
	}
	handler := newOrderStack(t, svc, stubIssuer{})

	rec := do(t, handler, http.MethodPost, orderPath, orderBody(t, uuid.New(), "pm_card_visa"), nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCreateOrderRoute_Success(t *testing.T) {
	userID := uuid.New()
	offerID := uuid.New()
	view := inbound.OrderView{
		OrderID:  uuid.New(),
		ItemName: "atorvastatin",
		Qty:      30,
		Status:   "placed",
	}

	var gotInput inbound.OrderInput
	svc := stubOrderService{
		placeOrderFn: func(_ context.Context, i inbound.OrderInput) (inbound.OrderView, error) {
			gotInput = i
			return view, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(userID))

	rec := do(t, handler, http.MethodPost, orderPath, orderBody(t, offerID, "pm_card_visa"),
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusCreated, rec.Code)

	// The authenticated user's id plus the request body fields all reach the service.
	assert.Equal(t, userID, gotInput.UserID)
	assert.Equal(t, offerID, gotInput.OfferID)
	assert.Equal(t, "pm_card_visa", gotInput.PaymentMethod)

	// Every response field is mapped off the view.
	var resp openapi.OrderResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, view.OrderID, resp.OrderId)
	assert.Equal(t, "atorvastatin", resp.ItemName)
	assert.Equal(t, 30, resp.Quantity)
	assert.Equal(t, "placed", resp.Status)
}

func TestCreateOrderRoute_ServiceErrorsMapToStatus(t *testing.T) {
	tests := map[string]struct {
		svcErr   error
		wantCode int
	}{
		"offer not found -> 404": {svcErr: inbound.ErrOfferNotFound, wantCode: http.StatusNotFound},
		"already placed -> 409":  {svcErr: inbound.ErrOrderAlreadyPlaced, wantCode: http.StatusConflict},
		"offer expired -> 410":   {svcErr: inbound.ErrOfferExpired, wantCode: http.StatusGone},
		"unexpected -> 500":      {svcErr: errBoom, wantCode: http.StatusInternalServerError},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := stubOrderService{
				placeOrderFn: func(context.Context, inbound.OrderInput) (inbound.OrderView, error) {
					return inbound.OrderView{}, tc.svcErr
				},
			}
			handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

			rec := do(t, handler, http.MethodPost, orderPath, orderBody(t, uuid.New(), "pm_card_visa"),
				http.Header{"Authorization": {"Bearer any-token"}})

			require.Equal(t, tc.wantCode, rec.Code)
			if tc.wantCode == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "boom", "internal error text must not leak")
			}
		})
	}
}

func TestCreateOrderRoute_MalformedBodyYields400(t *testing.T) {
	// A non-uuid offerId fails body binding and is turned into a JSON 400 by NewAPI's
	// RequestErrorHandlerFunc, never reaching the service.
	svc := stubOrderService{
		placeOrderFn: func(context.Context, inbound.OrderInput) (inbound.OrderView, error) {
			t.Fatal("service must not be called when the body is malformed")
			return inbound.OrderView{}, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodPost, orderPath, []byte(`{"offerId":"not-a-uuid","paymentMethod":"pm"}`),
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotEmpty(t, decodeMessage(t, rec))
}
