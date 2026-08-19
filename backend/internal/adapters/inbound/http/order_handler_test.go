package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
)

const orderPath = "/api/orders"

// stubOrderService satisfies inbound.OrderService; each test wires only the
// behavior it drives through the HTTP stack.
type stubOrderService struct {
	placeOrderFn     func(context.Context, inbound.OrderInput) (inbound.OrderView, error)
	getOrderStatusFn func(context.Context, uuid.UUID, uuid.UUID) (inbound.OrderStatusView, error)
	listOrdersFn     func(context.Context, uuid.UUID) ([]inbound.OrderSummaryView, error)
}

func (s stubOrderService) PlaceOrder(
	ctx context.Context, i inbound.OrderInput,
) (inbound.OrderView, error) {
	return s.placeOrderFn(ctx, i)
}

func (s stubOrderService) GetOrderStatus(
	ctx context.Context, userID, orderID uuid.UUID,
) (inbound.OrderStatusView, error) {
	return s.getOrderStatusFn(ctx, userID, orderID)
}

func (s stubOrderService) ListOrders(
	ctx context.Context, userID uuid.UUID,
) ([]inbound.OrderSummaryView, error) {
	return s.listOrdersFn(ctx, userID)
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
		// A deleted account holding a still-valid token is an auth failure, not a 500.
		"deleted account -> 401": {svcErr: inbound.ErrInvalidCredentials, wantCode: http.StatusUnauthorized},
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

func statusPath(orderID uuid.UUID) string {
	return "/api/orders/" + orderID.String() + "/status"
}

func TestGetOrderStatusRoute_RequiresAuth(t *testing.T) {
	svc := stubOrderService{
		getOrderStatusFn: func(context.Context, uuid.UUID, uuid.UUID) (inbound.OrderStatusView, error) {
			t.Fatal("service must not be reached without auth")
			return inbound.OrderStatusView{}, nil
		},
	}
	handler := newOrderStack(t, svc, stubIssuer{})

	rec := do(t, handler, http.MethodGet, statusPath(uuid.New()), nil, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetOrderStatusRoute_SuccessWithLiveShipping(t *testing.T) {
	userID := uuid.New()
	orderID := uuid.New()

	var gotUserID, gotOrderID uuid.UUID
	svc := stubOrderService{
		getOrderStatusFn: func(_ context.Context, uid, oid uuid.UUID) (inbound.OrderStatusView, error) {
			gotUserID, gotOrderID = uid, oid
			return inbound.OrderStatusView{OrderID: oid, Status: "shipped", ShippingStatus: "in_transit"}, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(userID))

	rec := do(t, handler, http.MethodGet, statusPath(orderID), nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID, gotUserID)
	assert.Equal(t, orderID, gotOrderID, "the {id} path parameter should be bound and forwarded")

	var resp openapi.OrderStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, orderID, resp.OrderId)
	assert.Equal(t, "shipped", resp.Status)
	require.NotNil(t, resp.ShippingStatus)
	assert.Equal(t, "in_transit", *resp.ShippingStatus)
}

func TestGetOrderStatusRoute_OmitsShippingStatusWhenAbsent(t *testing.T) {
	// A closed/aged-out workflow yields no live detail; the field must be omitted,
	// not serialized as an empty string.
	svc := stubOrderService{
		getOrderStatusFn: func(context.Context, uuid.UUID, uuid.UUID) (inbound.OrderStatusView, error) {
			return inbound.OrderStatusView{OrderID: uuid.New(), Status: "delivered", ShippingStatus: ""}, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodGet, statusPath(uuid.New()), nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)

	var resp openapi.OrderStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Nil(t, resp.ShippingStatus, "shippingStatus must be omitted when there is no live detail")
	assert.NotContains(t, rec.Body.String(), "shippingStatus")
}

func TestGetOrderStatusRoute_ErrorsMapToStatus(t *testing.T) {
	tests := map[string]struct {
		svcErr   error
		wantCode int
	}{
		"order not found -> 404": {svcErr: inbound.ErrOrderNotFound, wantCode: http.StatusNotFound},
		"unexpected -> 500":      {svcErr: errBoom, wantCode: http.StatusInternalServerError},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := stubOrderService{
				getOrderStatusFn: func(context.Context, uuid.UUID, uuid.UUID) (inbound.OrderStatusView, error) {
					return inbound.OrderStatusView{}, tc.svcErr
				},
			}
			handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

			rec := do(t, handler, http.MethodGet, statusPath(uuid.New()), nil,
				http.Header{"Authorization": {"Bearer any-token"}})

			require.Equal(t, tc.wantCode, rec.Code)
			if tc.wantCode == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "boom", "internal error text must not leak")
			}
		})
	}
}

func TestGetOrderStatusRoute_MalformedIDYields400(t *testing.T) {
	svc := stubOrderService{
		getOrderStatusFn: func(context.Context, uuid.UUID, uuid.UUID) (inbound.OrderStatusView, error) {
			t.Fatal("service must not be called when the order id is malformed")
			return inbound.OrderStatusView{}, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodGet, "/api/orders/not-a-uuid/status", nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotEmpty(t, decodeMessage(t, rec))
}

// --- GET /api/orders --------------------------------------------------------

func TestListOrdersRoute_RequiresAuth(t *testing.T) {
	svc := stubOrderService{
		listOrdersFn: func(context.Context, uuid.UUID) ([]inbound.OrderSummaryView, error) {
			t.Fatal("service must not be reached without auth")
			return nil, nil
		},
	}
	handler := newOrderStack(t, svc, stubIssuer{})

	rec := do(t, handler, http.MethodGet, orderPath, nil, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListOrdersRoute_Success(t *testing.T) {
	userID := uuid.New()
	paid, err := shared.NewMoneyFromCents(26400)
	require.NoError(t, err)
	placedAt := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)

	views := []inbound.OrderSummaryView{
		{
			OrderID:   uuid.New(),
			ItemName:  "atorvastatin",
			Qty:       30,
			Status:    "delivered",
			PlacedAt:  placedAt,
			PricePaid: ptr.To(paid),
		},
	}

	var gotUserID uuid.UUID
	svc := stubOrderService{
		listOrdersFn: func(_ context.Context, id uuid.UUID) ([]inbound.OrderSummaryView, error) {
			gotUserID = id
			return views, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(userID))

	rec := do(t, handler, http.MethodGet, orderPath, nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID, gotUserID, "the authenticated user's id should reach the service")

	var resp openapi.OrderListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Orders, 1)

	got := resp.Orders[0]
	assert.Equal(t, views[0].OrderID, got.OrderId)
	assert.Equal(t, "atorvastatin", got.ItemName)
	assert.Equal(t, 30, got.Quantity)
	assert.Equal(t, "delivered", got.Status)
	assert.True(t, placedAt.Equal(got.PlacedAt))
	require.NotNil(t, got.PricePaidCents)
	assert.Equal(t, int64(26400), *got.PricePaidCents)
}

func TestListOrdersRoute_UncapturedOrderOmitsPrice(t *testing.T) {
	// PricePaid is nil until capture succeeds. It must be omitted rather than sent as
	// 0 — $0.00 is a legitimate captured amount, so a zero would misreport a free
	// order as unpaid and an unpaid one as free.
	svc := stubOrderService{
		listOrdersFn: func(context.Context, uuid.UUID) ([]inbound.OrderSummaryView, error) {
			return []inbound.OrderSummaryView{{
				OrderID:   uuid.New(),
				ItemName:  "atorvastatin",
				Qty:       30,
				Status:    "placed",
				PlacedAt:  time.Now(),
				PricePaid: nil,
			}}, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodGet, orderPath, nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)

	var resp openapi.OrderListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Orders, 1)
	assert.Nil(t, resp.Orders[0].PricePaidCents)

	// Decoding into the typed struct cannot tell an omitted key from a null one, so
	// assert on the wire form: the key must be absent entirely.
	assert.NotContains(t, rec.Body.String(), "pricePaidCents")
}

func TestListOrdersRoute_EmptyListIsNotNull(t *testing.T) {
	// A user with no orders gets an empty array, not null — clients iterate it.
	svc := stubOrderService{
		listOrdersFn: func(context.Context, uuid.UUID) ([]inbound.OrderSummaryView, error) {
			return []inbound.OrderSummaryView{}, nil
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodGet, orderPath, nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"orders":[]}`, rec.Body.String())
}

func TestListOrdersRoute_ServiceErrorYields500(t *testing.T) {
	// There is no not-found for a collection, so any failure is a server fault and
	// the generic handler renders it without leaking the cause.
	svc := stubOrderService{
		listOrdersFn: func(context.Context, uuid.UUID) ([]inbound.OrderSummaryView, error) {
			return nil, errors.New("database exploded")
		},
	}
	handler := newOrderStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodGet, orderPath, nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "database exploded")
}
