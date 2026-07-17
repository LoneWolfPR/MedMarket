package pricesearch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

// errBoom stands in for an unexpected infrastructure failure.
var errBoom = errors.New("boom")

// Hand-rolled fakes for the outbound ports the activities depend on. Each method
// delegates to a function field so a test sets only the behavior it exercises;
// an unset field panics, surfacing an unexpected call as a test failure.

type fakePharmacyRepo struct {
	createFn    func(ctx context.Context, p *pharmacy.Pharmacy) (*pharmacy.Pharmacy, error)
	getByIDFn   func(ctx context.Context, id uuid.UUID) (*pharmacy.Pharmacy, error)
	getByCodeFn func(ctx context.Context, code string) (*pharmacy.Pharmacy, error)
	listFn      func(ctx context.Context) ([]pharmacy.Pharmacy, error)
}

func (f fakePharmacyRepo) Create(
	ctx context.Context, p *pharmacy.Pharmacy,
) (*pharmacy.Pharmacy, error) {
	return f.createFn(ctx, p)
}

func (f fakePharmacyRepo) GetByID(ctx context.Context, id uuid.UUID) (*pharmacy.Pharmacy, error) {
	return f.getByIDFn(ctx, id)
}

func (f fakePharmacyRepo) GetByCode(ctx context.Context, code string) (*pharmacy.Pharmacy, error) {
	return f.getByCodeFn(ctx, code)
}

func (f fakePharmacyRepo) List(ctx context.Context) ([]pharmacy.Pharmacy, error) {
	return f.listFn(ctx)
}

type fakePharmacyClient struct {
	searchFn     func(ctx context.Context, c pharmacy.SearchCriteria) ([]pharmacy.PriceQuote, error)
	placeOrderFn func(ctx context.Context, i pharmacy.OrderInput) (pharmacy.OrderResult, error)
}

func (f fakePharmacyClient) Search(
	ctx context.Context, c pharmacy.SearchCriteria,
) ([]pharmacy.PriceQuote, error) {
	return f.searchFn(ctx, c)
}

func (f fakePharmacyClient) PlaceOrder(
	ctx context.Context, i pharmacy.OrderInput,
) (pharmacy.OrderResult, error) {
	return f.placeOrderFn(ctx, i)
}

// --- fixtures ---------------------------------------------------------------

// buildPharmacy builds a persisted-looking pharmacy. The regulatory IDs carry
// real checksums, so NewPharmacy accepts them.
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
