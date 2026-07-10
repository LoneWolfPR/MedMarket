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
