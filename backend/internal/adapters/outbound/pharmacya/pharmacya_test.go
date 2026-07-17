package pharmacya_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/pharmacya"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

const testSecret = "test-api-key"

// testCriteria builds a valid SearchCriteria for driving Search.
func testCriteria(t *testing.T) pharmacy.SearchCriteria {
	t.Helper()
	strength, err := shared.NewMedStrength("20", "mg")
	require.NoError(t, err)
	c, err := pharmacy.NewSearchCriteria(pharmacy.NewSearchCriteriaParams{
		MedName:     "atorvastatin",
		MedStrength: strength,
	})
	require.NoError(t, err)
	return c
}

// newAdapter wires a PharmacyA adapter to a stub server.
func newAdapter(t *testing.T, handler http.HandlerFunc) *pharmacya.PharmacyA {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	a, err := pharmacya.NewPharmacyA(pharmacya.NewPharmacyAParams{
		Client:  srv.Client(),
		Logger:  slog.New(slog.DiscardHandler),
		ID:      uuid.New(),
		Secret:  testSecret,
		BaseURL: srv.URL,
	})
	require.NoError(t, err)
	return a
}

func TestSearch_HappyPathSendsCorrectRequest(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify the adapter's request translation: method, path, auth header,
		// and the criteria payload mapped onto pharmacy A's field names.
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/search", r.URL.Path)
		assert.Equal(t, testSecret, r.Header.Get("X-Api-Key"))

		var body struct {
			Drug     string `json:"drug"`
			Strength string `json:"strength"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "atorvastatin", body.Drug)
		assert.Equal(t, "20mg", body.Strength)

		_, _ = io.WriteString(w, `{"results":[
			{"sku":"A1","priceCents":1299},
			{"sku":"A2","priceCents":500}
		]}`)
	})

	quotes, err := adapter.Search(context.Background(), testCriteria(t))

	require.NoError(t, err)
	assert.Len(t, quotes, 2)
}

func TestSearch_NonSuccessStatusReturnsError(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "upstream boom")
	})

	quotes, err := adapter.Search(context.Background(), testCriteria(t))

	require.Error(t, err)
	assert.Nil(t, quotes)
}

func TestSearch_SkipsMalformedItems(t *testing.T) {
	// One valid item, one with an empty sku, one with a negative price. Only the
	// valid item should survive; the other two are skipped, not fatal.
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"results":[
			{"sku":"A1","priceCents":1299},
			{"sku":"","priceCents":500},
			{"sku":"A3","priceCents":-5}
		]}`)
	})

	quotes, err := adapter.Search(context.Background(), testCriteria(t))

	require.NoError(t, err)
	assert.Len(t, quotes, 1)
}

func TestSearch_EmptyResultsReturnsEmptySlice(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"results":[]}`)
	})

	quotes, err := adapter.Search(context.Background(), testCriteria(t))

	require.NoError(t, err)
	assert.Empty(t, quotes)
}

// testOrderInput builds a valid OrderInput for driving PlaceOrder.
func testOrderInput(t *testing.T) pharmacy.OrderInput {
	t.Helper()
	in, err := pharmacy.NewOrderInput(pharmacy.NewOrderInputParams{
		PharmacyItemID: "A1",
		Qty:            2,
		ShippingAddress: shared.Address{
			Street1: "123 Main St",
			Street2: "Apt 4",
			City:    "Springfield",
			State:   "IL",
			Zip:     "62704",
		},
		RecipientName:  "Jane Doe",
		IdempotencyKey: "idem-key-123",
	})
	require.NoError(t, err)
	return in
}

func TestPlaceOrder_HappyPathSendsCorrectRequest(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify request translation: method, path, both headers (including the
		// idempotency key A relies on), and the flat shipping payload.
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/order", r.URL.Path)
		assert.Equal(t, testSecret, r.Header.Get("X-Api-Key"))
		assert.Equal(t, "idem-key-123", r.Header.Get("Idempotency-Key"))

		var body struct {
			SKU      string `json:"sku"`
			Quantity int    `json:"quantity"`
			Shipping struct {
				Name    string `json:"name"`
				Street1 string `json:"street1"`
				Street2 string `json:"street2"`
				City    string `json:"city"`
				State   string `json:"state"`
				Zip     string `json:"zip"`
			} `json:"shipping"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "A1", body.SKU)
		assert.Equal(t, 2, body.Quantity)
		assert.Equal(t, "Jane Doe", body.Shipping.Name)
		assert.Equal(t, "123 Main St", body.Shipping.Street1)
		assert.Equal(t, "Apt 4", body.Shipping.Street2)
		assert.Equal(t, "Springfield", body.Shipping.City)
		assert.Equal(t, "IL", body.Shipping.State)
		assert.Equal(t, "62704", body.Shipping.Zip)

		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w,
			`{"orderId":"ord_abc","trackingId":"RXD123","status":"CONFIRMED","totalCents":8899}`)
	})

	result, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))

	require.NoError(t, err)
	assert.Equal(t, "ord_abc", result.PharmacyOrderID())
	assert.Equal(t, "RXD123", result.TrackingID())
	assert.Equal(t, "CONFIRMED", result.OrderStatus())
	assert.Equal(t, int64(8899), result.NewTotal().Cents())
}

func TestPlaceOrder_StatusClassification(t *testing.T) {
	// terminal => a *PlaceOrderError with KindRejected (don't retry);
	// !terminal => a plain error the activity leaves retryable.
	cases := map[string]struct {
		status   int
		terminal bool
	}{
		"400 invalid shipping": {http.StatusBadRequest, true},
		"404 unknown sku":      {http.StatusNotFound, true},
		"500 server error":     {http.StatusInternalServerError, false},
		"503 unavailable":      {http.StatusServiceUnavailable, false},
		"429 rate limited":     {http.StatusTooManyRequests, false},
		"408 request timeout":  {http.StatusRequestTimeout, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"error":"boom"}`)
			})

			_, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))
			require.Error(t, err)

			var poErr *outbound.PlaceOrderError
			if tc.terminal {
				require.ErrorAs(t, err, &poErr)
				assert.Equal(t, outbound.KindRejected, poErr.Kind)
			} else {
				assert.False(t, errors.As(err, &poErr),
					"retryable failures must not surface as a terminal PlaceOrderError")
			}
		})
	}
}

func TestPlaceOrder_MalformedResponseIsRetryable(t *testing.T) {
	// A 2xx we can't parse is ambiguous — but A is idempotent, so a retry is
	// safe. The adapter leaves it a plain (retryable) error, not terminal.
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"orderId":`) // truncated JSON
	})

	_, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))

	require.Error(t, err)
	var poErr *outbound.PlaceOrderError
	assert.False(t, errors.As(err, &poErr))
}
