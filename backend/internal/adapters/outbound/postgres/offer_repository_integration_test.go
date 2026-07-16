//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupOfferRepo returns an offer repository wired to a throwaway Postgres,
// plus the client behind it so tests can seed the prescription and pharmacy
// rows that offers foreign-key to. Offers have no unique constraints, so one
// container is safely shared across the subtests.
func setupOfferRepo(t *testing.T) (*postgres.OfferRepository, *ent.Client) {
	t.Helper()

	client := newTestClient(t)
	repo, err := postgres.NewOfferRepository(postgres.NewOfferRepositoryParams{
		Client: client,
		Logger: discardLogger(),
	})
	require.NoError(t, err)
	return repo, client
}

// newDomainOffer builds a valid domain offer for persistence.
func newDomainOffer(
	t *testing.T,
	rxID, pharmID uuid.UUID,
	cents int64,
	expiresAt time.Time,
) *pharmacy.Offer {
	t.Helper()

	price, err := shared.NewMoneyFromCents(cents)
	require.NoError(t, err)
	quote, err := pharmacy.NewPriceQuote(pharmacy.NewPriceQuoteParams{
		Price:          price,
		PharmacyID:     pharmID,
		PharmacyItemID: "RXD-ATO-20",
	})
	require.NoError(t, err)
	o, err := pharmacy.NewOffer(pharmacy.NewOfferParams{
		Quote:          quote,
		ExpiresAt:      expiresAt,
		PrescriptionID: rxID,
	})
	require.NoError(t, err)
	return o
}

func TestOfferRepository(t *testing.T) {
	repo, client := setupOfferRepo(t)
	ctx := context.Background()

	// FK parents, seeded once and shared: offers have no unique constraints, so
	// subtests cannot collide through them.
	userID := seedUser(t, client)
	rxID := seedPrescription(t, client, userID)
	pharmID := seedPharmacy(t, client)

	t.Run("Create assigns an ID and round-trips via GetByID", func(t *testing.T) {
		expiresAt := time.Now().Add(15 * time.Minute)

		created, err := repo.Create(ctx, newDomainOffer(t, rxID, pharmID, 1299, expiresAt))
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, created.ID, "Create should surface the DB-assigned ID")

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, rxID, got.PrescriptionID)
		assert.Equal(t, int64(1299), got.Quote.Price().Cents())
		assert.Equal(t, pharmID, got.Quote.PharmacyID())
		assert.Equal(t, "RXD-ATO-20", got.Quote.PharmacyItemID())
		// Postgres timestamptz truncates to microseconds and comes back in UTC, so
		// the round-tripped instant is close but never byte-equal to what went in.
		assert.WithinDuration(t, expiresAt, got.ExpiresAt, time.Millisecond)
	})

	// A $0.00 offer is legitimate (discounts / full insurance coverage). This
	// pins that end-to-end: the domain allows it, price_cents is NonNegative
	// rather than Positive, and it reconstructs on the way back out.
	t.Run("a zero-price offer round-trips", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOffer(t, rxID, pharmID, 0, time.Now().Add(time.Minute)))
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, got.Quote.Price().IsZero())
		assert.False(t, got.Quote.IsZero(), "a $0 quote is built, not a zero value")
	})

	t.Run("GetByID returns ErrOfferNotFound for an unknown ID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, uuid.New())
		require.ErrorIs(t, err, outbound.ErrOfferNotFound)
		assert.Nil(t, got)
	})

	// The two FK constraints are the reason offers cannot reference rows that do
	// not exist. Nothing maps these to a sentinel: the service loads the
	// prescription before building the offer, so a violation is a bug rather than
	// bad input, and every caller wants the same generic failure.
	t.Run("Create rejects an unknown prescription_id", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOffer(t, uuid.New(), pharmID, 500, time.Now().Add(time.Minute)))
		require.Error(t, err)
		assert.Nil(t, created)
	})

	t.Run("Create rejects an unknown pharmacy_id", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOffer(t, rxID, uuid.New(), 500, time.Now().Add(time.Minute)))
		require.Error(t, err)
		assert.Nil(t, created)
	})
}
