package http_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/inbound/http/openapi"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
)

// stubShippingService satisfies inbound.ShippingService; each test wires only the
// behavior it drives through the HTTP stack.
type stubShippingService struct {
	sendFn func(context.Context, uuid.UUID, inbound.ShippingUpdate) error
}

func (s stubShippingService) SendShippingUpdate(
	ctx context.Context, orderID uuid.UUID, update inbound.ShippingUpdate,
) error {
	return s.sendFn(ctx, orderID, update)
}

func newShippingStack(t *testing.T, shippingSvc inbound.ShippingService) http.Handler {
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
	oh, err := httpapi.NewOrderHandler(httpapi.NewOrderHandlerParams{Logger: logger, Svc: stubOrderService{}})
	require.NoError(t, err)
	wh, err := httpapi.NewShippingHandler(httpapi.NewShippingHandlerParams{Logger: logger, Svc: shippingSvc})
	require.NoError(t, err)

	return httpapi.NewAPI(httpapi.NewAPIParams{
		Auth:         ah,
		Prescription: ph,
		Search:       sh,
		Order:        oh,
		Shipping:     wh,
		Logger:       logger,
		TokenIssuer:  stubIssuer{}, // the webhook is unauthenticated, so this is never consulted
	}, http.NewServeMux())
}

func webhookPath(orderID uuid.UUID) string {
	return "/webhooks/shipping/" + orderID.String()
}

func webhookBody(t *testing.T, status, trackingID string) []byte {
	t.Helper()
	body, err := json.Marshal(openapi.ShippingWebhookRequest{
		Status:     status,
		TrackingId: trackingID,
		OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	return body
}

func TestShippingWebhook_UnauthenticatedSucceeds(t *testing.T) {
	// The receiver is called by the shipping provider, not the SPA, so it must work
	// with no Authorization header — unlike every /api route.
	orderID := uuid.New()

	var gotOrderID uuid.UUID
	var gotUpdate inbound.ShippingUpdate
	svc := stubShippingService{
		sendFn: func(_ context.Context, oid uuid.UUID, u inbound.ShippingUpdate) error {
			gotOrderID, gotUpdate = oid, u
			return nil
		},
	}
	handler := newShippingStack(t, svc)

	rec := do(t, handler, http.MethodPost, webhookPath(orderID), webhookBody(t, "picked_up", "trk-1"), nil)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, orderID, gotOrderID, "the {orderID} path parameter should be bound and forwarded")
	assert.Equal(t, "picked_up", gotUpdate.Status)
	assert.Equal(t, "trk-1", gotUpdate.TrackingID)
}

func TestShippingWebhook_ErrorsMapToStatus(t *testing.T) {
	tests := map[string]struct {
		svcErr   error
		wantCode int
	}{
		"workflow not found -> 404": {svcErr: inbound.ErrOrderWorkflowNotFound, wantCode: http.StatusNotFound},
		"unexpected -> 500":         {svcErr: errBoom, wantCode: http.StatusInternalServerError},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := stubShippingService{
				sendFn: func(context.Context, uuid.UUID, inbound.ShippingUpdate) error { return tc.svcErr },
			}
			handler := newShippingStack(t, svc)

			rec := do(t, handler, http.MethodPost, webhookPath(uuid.New()),
				webhookBody(t, "in_transit", "trk-1"), nil)

			require.Equal(t, tc.wantCode, rec.Code)
			if tc.wantCode == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "boom", "internal error text must not leak")
			}
		})
	}
}

func TestShippingWebhook_MalformedOrderIDYields400(t *testing.T) {
	// A non-uuid orderID fails path-param binding and becomes a JSON 400 via NewAPI's
	// ErrorHandlerFunc, never reaching the service.
	svc := stubShippingService{
		sendFn: func(context.Context, uuid.UUID, inbound.ShippingUpdate) error {
			t.Fatal("service must not be called when the order id is malformed")
			return nil
		},
	}
	handler := newShippingStack(t, svc)

	rec := do(t, handler, http.MethodPost, "/webhooks/shipping/not-a-uuid",
		webhookBody(t, "delivered", "trk-1"), nil)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotEmpty(t, decodeMessage(t, rec))
}
