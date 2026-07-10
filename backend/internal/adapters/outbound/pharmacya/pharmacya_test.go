package pharmacya_test

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

	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/pharmacya"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
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
