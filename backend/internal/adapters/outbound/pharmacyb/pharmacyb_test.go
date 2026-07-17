package pharmacyb_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/pharmacyb"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

const testSecret = "test-api-token"

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

// newAdapter wires a PharmacyB adapter to a stub server.
func newAdapter(t *testing.T, handler http.HandlerFunc) *pharmacyb.PharmacyB {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	b, err := pharmacyb.NewPharmacyB(pharmacyb.NewPharmacyBParams{
		Client:  srv.Client(),
		Logger:  slog.New(slog.DiscardHandler),
		ID:      uuid.New(),
		Secret:  testSecret,
		BaseURL: srv.URL,
	})
	require.NoError(t, err)
	return b
}

func TestSearch_HappyPathSendsCorrectRequest(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify the adapter's request translation: method, GraphQL path, Bearer
		// auth, and the criteria mapped into the query variables.
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/graphql", r.URL.Path)
		assert.Equal(t, "Bearer "+testSecret, r.Header.Get("Authorization"))

		var body struct {
			Query     string `json:"query"`
			Variables struct {
				Name     string `json:"name"`
				Strength string `json:"strength"`
			} `json:"variables"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Contains(t, body.Query, "medications(")
		assert.Equal(t, "atorvastatin", body.Variables.Name)
		assert.Equal(t, "20mg", body.Variables.Strength)

		_, _ = io.WriteString(w, `{"data":{"medications":[
			{"code":"B1","unitPrice":{"amount":"12.99"}},
			{"code":"B2","unitPrice":{"amount":"5.00"}}
		]}}`)
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
	// One valid, one with an empty code, one with a malformed dollar amount
	// ("12.9" fails the two-decimal format). Only the valid item survives.
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"medications":[
			{"code":"B1","unitPrice":{"amount":"12.99"}},
			{"code":"","unitPrice":{"amount":"5.00"}},
			{"code":"B3","unitPrice":{"amount":"12.9"}}
		]}}`)
	})

	quotes, err := adapter.Search(context.Background(), testCriteria(t))

	require.NoError(t, err)
	assert.Len(t, quotes, 1)
}

func TestSearch_GraphQLErrorsWithPartialDataReturnsQuotes(t *testing.T) {
	// GraphQL errors alongside usable data: partial-data policy keeps the quotes.
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"data":{"medications":[{"code":"B1","unitPrice":{"amount":"12.99"}}]},
			"errors":[{"message":"partial failure"}]
		}`)
	})

	quotes, err := adapter.Search(context.Background(), testCriteria(t))

	require.NoError(t, err)
	assert.Len(t, quotes, 1)
}

func TestSearch_GraphQLErrorsWithNoDataReturnsError(t *testing.T) {
	// GraphQL errors and no usable quotes: the whole search fails.
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"data":{"medications":[]},
			"errors":[{"message":"boom"}]
		}`)
	})

	quotes, err := adapter.Search(context.Background(), testCriteria(t))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Nil(t, quotes)
}

// testOrderInput builds a valid OrderInput for driving PlaceOrder. Street2 is
// set so the tests can assert B's fold of it into line1.
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

// assertKind asserts err is a terminal *PlaceOrderError of the wanted kind.
func assertKind(t *testing.T, err error, want outbound.PlaceOrderKind) {
	t.Helper()
	require.Error(t, err)
	var poErr *outbound.PlaceOrderError
	require.ErrorAs(t, err, &poErr)
	assert.Equal(t, want, poErr.Kind)
}

func TestPlaceOrder_HappyPathSendsCorrectRequest(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify request translation: GraphQL path, Bearer auth, the absence of
		// an idempotency key (B has no support), and the nested recipient with
		// Street2 folded into line1.
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/graphql", r.URL.Path)
		assert.Equal(t, "Bearer "+testSecret, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("Idempotency-Key"))

		var body struct {
			Query     string `json:"query"`
			Variables struct {
				Input struct {
					Code      string `json:"code"`
					Count     int    `json:"count"`
					Recipient struct {
						Name       string `json:"name"`
						Line1      string `json:"line1"`
						City       string `json:"city"`
						State      string `json:"state"`
						PostalCode string `json:"postalCode"`
					} `json:"recipient"`
				} `json:"input"`
			} `json:"variables"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Contains(t, body.Query, "placeOrder(")
		assert.Equal(t, "A1", body.Variables.Input.Code)
		assert.Equal(t, 2, body.Variables.Input.Count)
		assert.Equal(t, "Jane Doe", body.Variables.Input.Recipient.Name)
		assert.Equal(t, "123 Main St, Apt 4", body.Variables.Input.Recipient.Line1)
		assert.Equal(t, "Springfield", body.Variables.Input.Recipient.City)
		assert.Equal(t, "IL", body.Variables.Input.Recipient.State)
		assert.Equal(t, "62704", body.Variables.Input.Recipient.PostalCode)

		_, _ = io.WriteString(w, `{"data":{"placeOrder":{
			"reference":"mc_abc","tracking":"MDC123","state":"ACCEPTED",
			"grandTotal":{"amount":"88.99"}
		}}}`)
	})

	result, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))

	require.NoError(t, err)
	assert.Equal(t, "mc_abc", result.PharmacyOrderID())
	assert.Equal(t, "MDC123", result.TrackingID())
	assert.Equal(t, "ACCEPTED", result.OrderStatus())
	assert.Equal(t, int64(8899), result.NewTotal().Cents())
}

func TestPlaceOrder_InBandErrorsReturnOutcomeUnknown(t *testing.T) {
	// A 200 with in-band errors is ambiguous and B can't dedupe a retry.
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":"boom"}]}`)
	})

	_, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))

	assertKind(t, err, outbound.KindOutcomeUnknown)
}

func TestPlaceOrder_MissingDataReturnsOutcomeUnknown(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"placeOrder":null}}`)
	})

	_, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))

	assertKind(t, err, outbound.KindOutcomeUnknown)
}

func TestPlaceOrder_StatusClassification(t *testing.T) {
	// B has no idempotency: a deterministic 4xx is a clean rejection, but a 5xx
	// may have executed and can't be safely retried, so it parks as unknown.
	cases := map[string]struct {
		status int
		want   outbound.PlaceOrderKind
	}{
		"400 bad request":  {http.StatusBadRequest, outbound.KindRejected},
		"401 unauthorized": {http.StatusUnauthorized, outbound.KindRejected},
		"500 server error": {http.StatusInternalServerError, outbound.KindOutcomeUnknown},
		"503 unavailable":  {http.StatusServiceUnavailable, outbound.KindOutcomeUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"errors":[{"message":"boom"}]}`)
			})

			_, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))
			assertKind(t, err, tc.want)
		})
	}
}

func TestPlaceOrder_MalformedResponseReturnsOutcomeUnknown(t *testing.T) {
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"placeOrder":`) // truncated JSON
	})

	_, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))

	assertKind(t, err, outbound.KindOutcomeUnknown)
}

func TestPlaceOrder_BadAmountReturnsOutcomeUnknown(t *testing.T) {
	// "88.9" fails the two-decimal dollar format; on an order the whole thing is
	// unknown (unlike a search item, which would just be skipped).
	adapter := newAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"placeOrder":{
			"reference":"mc_abc","tracking":"MDC123","state":"ACCEPTED",
			"grandTotal":{"amount":"88.9"}
		}}}`)
	})

	_, err := adapter.PlaceOrder(context.Background(), testOrderInput(t))

	assertKind(t, err, outbound.KindOutcomeUnknown)
}
