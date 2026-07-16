package app_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/app"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/inbound"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// testOfferTTL is the TTL every price-search test is built with; tests asserting
// expiry compare against it rather than the production default.
const testOfferTTL = 15 * time.Minute

// newPriceSearchService constructs a PriceSearchService with the given fakes, a
// discard logger, and an offer repo that stands in for the DB assigning an id.
// Tests needing to observe or fail offer persistence use newPriceSearchServiceWithOffers.
func newPriceSearchService(
	t *testing.T,
	rxRepo outbound.PrescriptionRepository,
	pharmRepo outbound.PharmacyRepository,
	searcher outbound.PriceSearcher,
) *app.PriceSearchService {
	t.Helper()
	return newPriceSearchServiceWithOffers(t, rxRepo, pharmRepo, searcher, offerRepoAssigningID())
}

// newPriceSearchServiceWithOffers is newPriceSearchService with the offer repo
// under the test's control.
func newPriceSearchServiceWithOffers(
	t *testing.T,
	rxRepo outbound.PrescriptionRepository,
	pharmRepo outbound.PharmacyRepository,
	searcher outbound.PriceSearcher,
	offerRepo outbound.OfferRepository,
) *app.PriceSearchService {
	t.Helper()

	svc, err := app.NewPriceSearchService(app.NewPriceSearchServiceParams{
		Logger:    slog.New(slog.DiscardHandler),
		RxRepo:    rxRepo,
		PharmRepo: pharmRepo,
		Searcher:  searcher,
		OfferRepo: offerRepo,
		OfferTTL:  testOfferTTL,
	})
	require.NoError(t, err)
	return svc
}

// buildPharmacy builds a persisted-looking pharmacy with the given id and name.
// The regulatory IDs carry real checksums, so NewPharmacy accepts them.
func buildPharmacy(t *testing.T, id uuid.UUID, code, name string) pharmacy.Pharmacy {
	t.Helper()

	phone, err := shared.NewPhone("5551234567")
	require.NoError(t, err)
	npi, err := pharmacy.NewNPI("1234567893")
	require.NoError(t, err)
	dea, err := pharmacy.NewDEA("AF1234563")
	require.NoError(t, err)
	ncpdp, err := pharmacy.NewNCPDP("1234567")
	require.NoError(t, err)

	p, err := pharmacy.NewPharmacy(pharmacy.NewPharmacyParams{
		Code:         code,
		Name:         name,
		ContactPhone: phone,
		NPINum:       npi,
		DEANum:       dea,
		NCPDPNum:     ncpdp,
		Address: shared.Address{
			Street1: "100 Market St",
			City:    "Springfield",
			State:   "IL",
			Zip:     "62704",
		},
	})
	require.NoError(t, err)
	p.ID = id
	return *p
}

// buildQuote builds a PriceQuote at the given unit price in cents.
func buildQuote(t *testing.T, pharmacyID uuid.UUID, itemID string, cents int64) pharmacy.PriceQuote {
	t.Helper()

	price, err := shared.NewMoneyFromCents(cents)
	require.NoError(t, err)
	q, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     pharmacyID,
		PharmacyItemID: itemID,
	})
	require.NoError(t, err)
	return q
}

// --- NewPriceSearchService --------------------------------------------------

func TestNewPriceSearchService_Validation(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	rxRepo := fakePrescriptionRepo{}
	pharmRepo := fakePharmacyRepo{}
	searcher := fakePriceSearcher{}
	offerRepo := fakeOfferRepo{}
	ttl := testOfferTTL

	tests := map[string]app.NewPriceSearchServiceParams{
		"missing logger": {
			RxRepo: rxRepo, PharmRepo: pharmRepo, Searcher: searcher, OfferRepo: offerRepo, OfferTTL: ttl,
		},
		"missing rx repo": {
			Logger: logger, PharmRepo: pharmRepo, Searcher: searcher, OfferRepo: offerRepo, OfferTTL: ttl,
		},
		"missing pharmacy repo": {
			Logger: logger, RxRepo: rxRepo, Searcher: searcher, OfferRepo: offerRepo, OfferTTL: ttl,
		},
		"missing price searcher": {
			Logger: logger, RxRepo: rxRepo, PharmRepo: pharmRepo, OfferRepo: offerRepo, OfferTTL: ttl,
		},
		"missing offer repo": {
			Logger: logger, RxRepo: rxRepo, PharmRepo: pharmRepo, Searcher: searcher, OfferTTL: ttl,
		},
		// A zero TTL is what an unset field looks like, and it would mint offers
		// that expire the instant they are created.
		"zero offer ttl": {
			Logger: logger, RxRepo: rxRepo, PharmRepo: pharmRepo, Searcher: searcher, OfferRepo: offerRepo,
		},
		"negative offer ttl": {
			Logger: logger, RxRepo: rxRepo, PharmRepo: pharmRepo, Searcher: searcher, OfferRepo: offerRepo,
			OfferTTL: -time.Minute,
		},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			svc, err := app.NewPriceSearchService(params)
			require.Error(t, err)
			assert.Nil(t, svc)
		})
	}
}

// --- GetQuoteList: prescription lookup --------------------------------------

func TestGetQuoteList_PrescriptionNotFound(t *testing.T) {
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return nil, outbound.ErrPrescriptionNotFound
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			t.Fatal("search must not run when the prescription is not found")
			return nil, nil
		},
	}
	svc := newPriceSearchService(t, rxRepo, fakePharmacyRepo{}, searcher)

	_, err := svc.GetQuoteList(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, inbound.ErrPrescriptionNotFound)
}

func TestGetQuoteList_RepoErrorSurfacesGeneric(t *testing.T) {
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return nil, errBoom
		},
	}
	svc := newPriceSearchService(t, rxRepo, fakePharmacyRepo{}, fakePriceSearcher{})

	_, err := svc.GetQuoteList(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, inbound.ErrPrescriptionNotFound)
}

func TestGetQuoteList_OwnershipMismatchHidesRecord(t *testing.T) {
	// Searching someone else's prescription is indistinguishable from searching a
	// missing one: no existence leak, and no pharmacy calls are made on its behalf.
	rx := buildRx(t, uuid.New()) // owned by someone else
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			t.Fatal("search must not run for a prescription the caller does not own")
			return nil, nil
		},
	}
	svc := newPriceSearchService(t, rxRepo, fakePharmacyRepo{}, searcher)

	_, err := svc.GetQuoteList(context.Background(), uuid.New(), rx.ID)
	require.ErrorIs(t, err, inbound.ErrPrescriptionNotFound)
}

// --- GetQuoteList: downstream failures --------------------------------------

func TestGetQuoteList_PharmacyListError(t *testing.T) {
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) { return nil, errBoom },
	}
	svc := newPriceSearchService(t, rxRepo, pharmRepo, fakePriceSearcher{})

	_, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.ErrorIs(t, err, errBoom)
}

func TestGetQuoteList_SearchError(t *testing.T) {
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) { return nil, nil },
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return nil, errBoom
		},
	}
	svc := newPriceSearchService(t, rxRepo, pharmRepo, searcher)

	_, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.ErrorIs(t, err, errBoom)
}

// --- GetQuoteList: success --------------------------------------------------

func TestGetQuoteList_BuildsCriteriaFromPrescription(t *testing.T) {
	// The prescription's medication name and strength ARE the search input.
	owner := uuid.New()
	rx := buildRx(t, owner) // atorvastatin 20mg
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) { return nil, nil },
	}

	var got pharmacy.SearchCriteria
	searcher := fakePriceSearcher{
		searchFn: func(_ context.Context, c pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			got = c
			return nil, nil
		},
	}
	svc := newPriceSearchService(t, rxRepo, pharmRepo, searcher)

	_, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err)

	assert.Equal(t, rx.MedName, got.MedName())
	assert.Equal(t, "20", got.MedStrength().Value())
	assert.Equal(t, "mg", got.MedStrength().Unit())
}

func TestGetQuoteList_ResolvesNameAndComputesTotal(t *testing.T) {
	owner := uuid.New()
	rx := buildRx(t, owner) // Qty 30
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}

	pharmID := uuid.New()
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
			return []pharmacy.Pharmacy{buildPharmacy(t, pharmID, "mock-pharmacy-a", "Mock Pharmacy A")}, nil
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return []pharmacy.PriceQuote{buildQuote(t, pharmID, "sku-1", 880)}, nil
		},
	}
	svc := newPriceSearchService(t, rxRepo, pharmRepo, searcher)

	views, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)

	// The name is resolved from the repo, and the total is unit price x quantity.
	assert.Equal(t, "Mock Pharmacy A", views[0].PharmacyName)
	assert.Equal(t, int64(880), views[0].Quote.Price().Cents())
	assert.Equal(t, int64(880*30), views[0].Total.Cents())
}

func TestGetQuoteList_PreservesSearcherOrder(t *testing.T) {
	// The workflow already sorts by price; the service must not reorder.
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}

	cheapID, pricyID := uuid.New(), uuid.New()
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
			return []pharmacy.Pharmacy{
				buildPharmacy(t, cheapID, "mock-pharmacy-a", "Cheap Pharmacy"),
				buildPharmacy(t, pricyID, "mock-pharmacy-b", "Pricy Pharmacy"),
			}, nil
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return []pharmacy.PriceQuote{
				buildQuote(t, cheapID, "sku-1", 880),
				buildQuote(t, pricyID, "code-1", 1161),
			}, nil
		},
	}
	svc := newPriceSearchService(t, rxRepo, pharmRepo, searcher)

	views, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, "Cheap Pharmacy", views[0].PharmacyName)
	assert.Equal(t, "Pricy Pharmacy", views[1].PharmacyName)
}

func TestGetQuoteList_UnknownPharmacyIsSkipped(t *testing.T) {
	// A quote naming a pharmacy the repo does not know cannot be rendered (there
	// is no name for it), so it is logged and dropped rather than failing the call.
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}

	knownID := uuid.New()
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
			return []pharmacy.Pharmacy{buildPharmacy(t, knownID, "mock-pharmacy-a", "Known Pharmacy")}, nil
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return []pharmacy.PriceQuote{
				buildQuote(t, uuid.New(), "orphan-sku", 500), // no matching pharmacy row
				buildQuote(t, knownID, "sku-1", 880),
			}, nil
		},
	}
	svc := newPriceSearchService(t, rxRepo, pharmRepo, searcher)

	views, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "Known Pharmacy", views[0].PharmacyName)
}

// --- GetQuoteList: offer persistence ----------------------------------------

func TestGetQuoteList_PersistsAnOfferPerQuote(t *testing.T) {
	// Each returned quote is persisted as an offer built from that quote, tied to
	// the prescription, and stamped now+TTL. The view carries the resulting id so
	// the client can order against the price it was shown.
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}

	pharmID := uuid.New()
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
			return []pharmacy.Pharmacy{buildPharmacy(t, pharmID, "mock-pharmacy-a", "Mock Pharmacy A")}, nil
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return []pharmacy.PriceQuote{buildQuote(t, pharmID, "sku-1", 880)}, nil
		},
	}

	var saved []pharmacy.Offer
	assignedID := uuid.New()
	offerRepo := fakeOfferRepo{
		createFn: func(_ context.Context, o *pharmacy.Offer) (*pharmacy.Offer, error) {
			saved = append(saved, *o)
			out := *o
			out.ID = assignedID
			return &out, nil
		},
	}
	svc := newPriceSearchServiceWithOffers(t, rxRepo, pharmRepo, searcher, offerRepo)

	before := time.Now()
	views, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err)

	require.Len(t, saved, 1)
	assert.Equal(t, rx.ID, saved[0].PrescriptionID)
	assert.Equal(t, int64(880), saved[0].Quote.Price().Cents())
	assert.Equal(t, "sku-1", saved[0].Quote.PharmacyItemID())
	assert.Equal(t, pharmID, saved[0].Quote.PharmacyID())
	// Expiry is now+TTL, so it lands between the TTL measured before and after the
	// call. A wrong or unset TTL (the field-drop bug) fails this.
	assert.WithinRange(t, saved[0].ExpiresAt, before.Add(testOfferTTL), time.Now().Add(testOfferTTL))

	require.Len(t, views, 1)
	assert.Equal(t, assignedID, views[0].OfferID, "the view carries the DB-assigned offer id")
}

func TestGetQuoteList_QuoteIsSkippedWhenOfferCannotBePersisted(t *testing.T) {
	// A quote whose offer fails to save has no id to order against, so returning it
	// would hand the client something it cannot act on. The good quote still lands.
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}

	goodID, badID := uuid.New(), uuid.New()
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
			return []pharmacy.Pharmacy{
				buildPharmacy(t, goodID, "mock-pharmacy-a", "Good Pharmacy"),
				buildPharmacy(t, badID, "mock-pharmacy-b", "Doomed Pharmacy"),
			}, nil
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return []pharmacy.PriceQuote{
				buildQuote(t, goodID, "sku-1", 880),
				buildQuote(t, badID, "code-1", 1161),
			}, nil
		},
	}
	offerRepo := fakeOfferRepo{
		createFn: func(_ context.Context, o *pharmacy.Offer) (*pharmacy.Offer, error) {
			if o.Quote.PharmacyID() == badID {
				return nil, errBoom
			}
			out := *o
			out.ID = uuid.New()
			return &out, nil
		},
	}
	svc := newPriceSearchServiceWithOffers(t, rxRepo, pharmRepo, searcher, offerRepo)

	views, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err, "one unpersistable quote must not fail the whole search")
	require.Len(t, views, 1)
	assert.Equal(t, "Good Pharmacy", views[0].PharmacyName)
	assert.NotEqual(t, uuid.Nil, views[0].OfferID)
}

func TestGetQuoteList_QuoteIsSkippedWhenOfferCannotBeBuilt(t *testing.T) {
	// A zero-value quote cannot become an offer (NewOffer rejects it), so the quote
	// is dropped before it ever reaches the repo.
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) {
			return []pharmacy.Pharmacy{buildPharmacy(t, uuid.Nil, "mock-pharmacy-a", "Zero Pharmacy")}, nil
		},
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return []pharmacy.PriceQuote{{}}, nil // zero value: never built by a constructor
		},
	}
	offerRepo := fakeOfferRepo{
		createFn: func(context.Context, *pharmacy.Offer) (*pharmacy.Offer, error) {
			t.Fatal("an offer must not be persisted for a quote that cannot build one")
			return nil, nil
		},
	}
	svc := newPriceSearchServiceWithOffers(t, rxRepo, pharmRepo, searcher, offerRepo)

	views, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err)
	assert.Empty(t, views)
}

func TestGetQuoteList_NoQuotesYieldsEmptySlice(t *testing.T) {
	// No pharmacy stocking the medication is a normal outcome, not an error.
	owner := uuid.New()
	rx := buildRx(t, owner)
	rxRepo := fakePrescriptionRepo{
		getByIDFn: func(context.Context, uuid.UUID) (*prescription.Prescription, error) {
			return &rx, nil
		},
	}
	pharmRepo := fakePharmacyRepo{
		listFn: func(context.Context) ([]pharmacy.Pharmacy, error) { return nil, nil },
	}
	searcher := fakePriceSearcher{
		searchFn: func(context.Context, pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error) {
			return nil, nil
		},
	}
	svc := newPriceSearchService(t, rxRepo, pharmRepo, searcher)

	views, err := svc.GetQuoteList(context.Background(), owner, rx.ID)
	require.NoError(t, err)
	assert.Empty(t, views)
}
