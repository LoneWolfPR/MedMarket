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
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// stubPriceSearchService satisfies inbound.PriceSearchService; each test wires
// only the behavior it drives.
type stubPriceSearchService struct {
	getQuoteListFn func(context.Context, uuid.UUID, uuid.UUID) ([]inbound.QuoteView, error)
}

func (s stubPriceSearchService) GetQuoteList(
	ctx context.Context, userID, rxID uuid.UUID,
) ([]inbound.QuoteView, error) {
	return s.getQuoteListFn(ctx, userID, rxID)
}

// newSearchStack assembles the real HTTP stack with a configurable price search
// service, so tests exercise the actual routing, middleware, and error policy.
func newSearchStack(
	t *testing.T, searchSvc inbound.PriceSearchService, ti outbound.TokenIssuer,
) http.Handler {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	h, err := httpapi.NewAuthHandler(httpapi.NewAuthHandlerParams{Logger: logger, Svc: stubService{}})
	require.NoError(t, err)
	ph, err := httpapi.NewPrescriptionHandler(httpapi.NewPrescriptionHandlerParams{
		Logger: logger,
		Svc:    stubPrescriptionService{},
	})
	require.NoError(t, err)
	sh, err := httpapi.NewPriceSearchHandler(httpapi.NewPriceSearchHandlerParams{
		Logger: logger,
		Svc:    searchSvc,
	})
	require.NoError(t, err)

	return httpapi.NewAPI(httpapi.NewAPIParams{
		Auth:         h,
		Prescription: ph,
		Search:       sh,
		Logger:       logger,
		TokenIssuer:  ti,
	}, http.NewServeMux())
}

// buildQuoteView builds a QuoteView the service stub can return.
func buildQuoteView(t *testing.T, pharmacyName, itemID string, unitCents, totalCents int64) inbound.QuoteView {
	t.Helper()

	price, err := shared.NewMoneyFromCents(unitCents)
	require.NoError(t, err)
	quote, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     uuid.New(),
		PharmacyItemID: itemID,
	})
	require.NoError(t, err)
	total, err := shared.NewMoneyFromCents(totalCents)
	require.NoError(t, err)

	return inbound.QuoteView{
		OfferID:      uuid.New(),
		Quote:        quote,
		PharmacyName: pharmacyName,
		Total:        total,
	}
}

// searchPath builds the search route for a prescription id.
func searchPath(rxID uuid.UUID) string {
	return "/api/prescriptions/" + rxID.String() + "/search"
}

func TestSearchRoute_RequiresAuth(t *testing.T) {
	svc := stubPriceSearchService{
		getQuoteListFn: func(context.Context, uuid.UUID, uuid.UUID) ([]inbound.QuoteView, error) {
			t.Fatal("service must not be reached without auth")
			return nil, nil
		},
	}
	handler := newSearchStack(t, svc, stubIssuer{})

	rec := do(t, handler, http.MethodPost, searchPath(uuid.New()), nil, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSearchRoute_Success(t *testing.T) {
	userID := uuid.New()
	rxID := uuid.New()
	views := []inbound.QuoteView{
		buildQuoteView(t, "Mock Pharmacy A", "sku-1", 880, 26400),
		buildQuoteView(t, "Mock Pharmacy B", "code-1", 1161, 34830),
	}

	var gotUserID, gotRxID uuid.UUID
	svc := stubPriceSearchService{
		getQuoteListFn: func(_ context.Context, uid, rid uuid.UUID) ([]inbound.QuoteView, error) {
			gotUserID, gotRxID = uid, rid
			return views, nil
		},
	}
	handler := newSearchStack(t, svc, tokenIssuer(userID))

	rec := do(t, handler, http.MethodPost, searchPath(rxID), nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)

	// The authenticated user's ID and the path's prescription ID both reach the service.
	assert.Equal(t, userID, gotUserID)
	assert.Equal(t, rxID, gotRxID, "the {id} path parameter should be bound and forwarded")

	var resp []openapi.QuoteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)

	// Order is preserved and every field is mapped off the view.
	assert.Equal(t, "Mock Pharmacy A", resp[0].PharmacyName)
	assert.Equal(t, "sku-1", resp[0].PharmacyItemId)
	assert.Equal(t, int64(880), resp[0].UnitPriceCents)
	assert.Equal(t, int64(26400), resp[0].TotalCents)
	assert.Equal(t, views[0].Quote.PharmacyID(), resp[0].PharmacyId)
	// The offer id is what POST /api/orders references, so a quote must carry the
	// one its own view was built with — not the other quote's, and not the zero uuid.
	assert.Equal(t, views[0].OfferID, resp[0].OfferId)
	assert.Equal(t, "Mock Pharmacy B", resp[1].PharmacyName)
	assert.Equal(t, int64(1161), resp[1].UnitPriceCents)
	assert.Equal(t, views[1].OfferID, resp[1].OfferId)
}

func TestSearchRoute_NoQuotesYieldsEmptyJSONArray(t *testing.T) {
	// An empty result must marshal as [] rather than null, so clients can iterate
	// the response unconditionally.
	svc := stubPriceSearchService{
		getQuoteListFn: func(context.Context, uuid.UUID, uuid.UUID) ([]inbound.QuoteView, error) {
			return nil, nil
		},
	}
	handler := newSearchStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodPost, searchPath(uuid.New()), nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestSearchRoute_ServiceErrorsMapToStatus(t *testing.T) {
	tests := map[string]struct {
		svcErr   error
		wantCode int
	}{
		"prescription not found -> 404": {
			svcErr:   inbound.ErrPrescriptionNotFound,
			wantCode: http.StatusNotFound,
		},
		"unexpected -> 500": {svcErr: errBoom, wantCode: http.StatusInternalServerError},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := stubPriceSearchService{
				getQuoteListFn: func(context.Context, uuid.UUID, uuid.UUID) ([]inbound.QuoteView, error) {
					return nil, tc.svcErr
				},
			}
			handler := newSearchStack(t, svc, tokenIssuer(uuid.New()))

			rec := do(t, handler, http.MethodPost, searchPath(uuid.New()), nil,
				http.Header{"Authorization": {"Bearer any-token"}})

			require.Equal(t, tc.wantCode, rec.Code)
			if tc.wantCode == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "boom", "internal error text must not leak")
			}
		})
	}
}

func TestSearchRoute_MalformedPrescriptionIDYields400(t *testing.T) {
	// The {id} path parameter is a uuid; a non-uuid fails binding and is turned
	// into a JSON 400 by NewAPI's RequestErrorHandlerFunc, never reaching the service.
	svc := stubPriceSearchService{
		getQuoteListFn: func(context.Context, uuid.UUID, uuid.UUID) ([]inbound.QuoteView, error) {
			t.Fatal("service must not be called when the prescription id is malformed")
			return nil, nil
		},
	}
	handler := newSearchStack(t, svc, tokenIssuer(uuid.New()))

	rec := do(t, handler, http.MethodPost, "/api/prescriptions/not-a-uuid/search", nil,
		http.Header{"Authorization": {"Bearer any-token"}})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotEmpty(t, decodeMessage(t, rec))
}
