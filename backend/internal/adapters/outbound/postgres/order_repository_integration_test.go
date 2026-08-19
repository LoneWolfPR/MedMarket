//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/LoneWolfPR/MedMarket/backend/ent"
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/order"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupOrderRepo returns an order repository wired to a throwaway Postgres, plus
// the client behind it so tests can seed the prescription and offer rows that
// orders foreign-key to. The partial unique index on offer_id allows only one
// active (non-failed/canceled) order per offer, so each subtest that persists an
// order seeds its own offer; with that, one container is safely shared.
func setupOrderRepo(t *testing.T) (*postgres.OrderRepository, *ent.Client) {
	t.Helper()

	client := newTestClient(t)
	repo, err := postgres.NewOrderRepository(postgres.NewOrderRepositoryParams{
		Client: client,
		Logger: discardLogger(),
	})
	require.NoError(t, err)
	return repo, client
}

// newDomainOrder builds a valid domain order for persistence.
func newDomainOrder(t *testing.T, rxID, offerID uuid.UUID) *order.Order {
	t.Helper()

	o, err := order.NewOrder(order.NewOrderParams{
		PrescriptionID: rxID,
		OfferID:        offerID,
		Qty:            30,
	})
	require.NoError(t, err)
	return o
}

func TestOrderRepository(t *testing.T) {
	repo, client := setupOrderRepo(t)
	ctx := context.Background()

	// FK parents, seeded once and shared. Offers are the exception: the partial
	// unique index allows one active order per offer, so subtests that persist an
	// order seed their own.
	userID := seedUser(t, client)
	rxID := seedPrescription(t, client, userID)
	pharmID := seedPharmacy(t, client)
	offerID := seedOffer(t, client, rxID, pharmID)

	t.Run("Create assigns an ID and round-trips via GetByID", func(t *testing.T) {
		ownOfferID := seedOffer(t, client, rxID, pharmID)
		created, err := repo.Create(ctx, newDomainOrder(t, rxID, ownOfferID))
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, created.ID, "Create should surface the DB-assigned ID")
		assert.Equal(t, order.StatusPlaced, created.Status, "a new order starts placed")

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, rxID, got.PrescriptionID)
		assert.Equal(t, ownOfferID, got.OfferID)
		assert.Equal(t, order.StatusPlaced, got.Status)
		assert.Equal(t, 30, got.Qty)
	})

	// A freshly placed order has not been paid. price_paid_cents is NULL, and that
	// must survive the trip back as a nil *Money rather than a pointer to $0.00 —
	// the distinction the whole nullable design exists for.
	t.Run("a new order reads back unpaid, not paid-zero", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOrder(t, rxID, seedOffer(t, client, rxID, pharmID)))
		require.NoError(t, err)
		assert.Nil(t, created.PricePaid, "nothing is paid at placed")

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Nil(t, got.PricePaid, "NULL must map to nil, not to a pointer at zero")
	})

	// The saga's first update lands before any payment: the pharmacy has accepted
	// and returned its handles, but capture has not happened. PricePaid is still nil
	// here, so Update must not assume otherwise.
	t.Run("Update persists pharmacy handles while still unpaid", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOrder(t, rxID, seedOffer(t, client, rxID, pharmID)))
		require.NoError(t, err)

		created.Status = order.StatusConfirmed
		created.PharmacyOrderID = "ord_abc123"
		created.TrackingID = "RXD00324FECC48D"

		updated, err := repo.Update(ctx, created)
		require.NoError(t, err)
		require.NotNil(t, updated, "Update must return the persisted order")
		assert.Equal(t, order.StatusConfirmed, updated.Status)
		assert.Equal(t, "ord_abc123", updated.PharmacyOrderID)
		assert.Equal(t, "RXD00324FECC48D", updated.TrackingID)
		assert.Nil(t, updated.PricePaid)

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "RXD00324FECC48D", got.TrackingID)
	})

	t.Run("Update records a captured price", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOrder(t, rxID, seedOffer(t, client, rxID, pharmID)))
		require.NoError(t, err)

		paid, err := shared.NewMoneyFromCents(38970)
		require.NoError(t, err)
		created.Status = order.StatusConfirmed
		created.PricePaid = &paid

		_, err = repo.Update(ctx, created)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		require.NotNil(t, got.PricePaid)
		assert.Equal(t, int64(38970), got.PricePaid.Cents())
	})

	// A $0.00 capture is real (full insurance coverage) and must be distinguishable
	// from having paid nothing at all. This is the case that collapses if PricePaid
	// is ever built as &someValue rather than left nil.
	t.Run("a captured $0.00 is distinguishable from unpaid", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOrder(t, rxID, seedOffer(t, client, rxID, pharmID)))
		require.NoError(t, err)
		require.Nil(t, created.PricePaid)

		var free shared.Money // zero value == $0.00
		created.Status = order.StatusConfirmed
		created.PricePaid = &free

		_, err = repo.Update(ctx, created)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		require.NotNil(t, got.PricePaid, "$0.00 captured is paid, not unpaid")
		assert.True(t, got.PricePaid.IsZero())
	})

	t.Run("GetByID returns ErrOrderNotFound for an unknown ID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, uuid.New())
		require.ErrorIs(t, err, outbound.ErrOrderNotFound)
		assert.Nil(t, got)
	})

	t.Run("Update returns ErrOrderNotFound for an unknown ID", func(t *testing.T) {
		o := newDomainOrder(t, rxID, offerID)
		o.ID = uuid.New() // never persisted

		got, err := repo.Update(ctx, o)
		require.ErrorIs(t, err, outbound.ErrOrderNotFound)
		assert.Nil(t, got)
	})

	// The status CHECK is the only thing guarding the column now that NewOrder no
	// longer takes a status: the saga advances orders by assigning the field, so a
	// typo there reaches the database.
	t.Run("the status CHECK rejects a value outside the enum", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOrder(t, rxID, seedOffer(t, client, rxID, pharmID)))
		require.NoError(t, err)

		created.Status = "shipped_typo"

		got, err := repo.Update(ctx, created)
		require.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("Create rejects an unknown prescription_id", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOrder(t, uuid.New(), offerID))
		require.Error(t, err)
		assert.Nil(t, created)
	})

	t.Run("Create rejects an unknown offer_id", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainOrder(t, rxID, uuid.New()))
		require.Error(t, err)
		assert.Nil(t, created)
	})

	// The partial unique index allows one ACTIVE order per offer: a second Create
	// against the same offer maps to ErrOrderExists, but once the first order is
	// terminal-non-success (failed/canceled) the offer is re-orderable.
	t.Run("a second active order for the same offer maps to ErrOrderExists", func(t *testing.T) {
		ownOfferID := seedOffer(t, client, rxID, pharmID)
		first, err := repo.Create(ctx, newDomainOrder(t, rxID, ownOfferID))
		require.NoError(t, err)

		dup, err := repo.Create(ctx, newDomainOrder(t, rxID, ownOfferID))
		require.ErrorIs(t, err, outbound.ErrOrderExists)
		assert.Nil(t, dup)

		first.Status = order.StatusFailed
		_, err = repo.Update(ctx, first)
		require.NoError(t, err)

		retry, err := repo.Create(ctx, newDomainOrder(t, rxID, ownOfferID))
		require.NoError(t, err, "a failed order must not block re-ordering the offer")
		assert.NotNil(t, retry)
	})

	// List is scoped by prescription id because orders carry no user column —
	// ownership runs order -> prescription -> user, and the caller passes the id set
	// it already resolved for the authenticated user.
	t.Run("List returns only orders for the given prescriptions", func(t *testing.T) {
		mine, err := repo.Create(ctx, newDomainOrder(t, rxID, seedOffer(t, client, rxID, pharmID)))
		require.NoError(t, err)

		// A second user's order, reachable only through their own prescription.
		otherUserID := seedUser(t, client)
		otherRxID := seedPrescription(t, client, otherUserID)
		theirs, err := repo.Create(ctx,
			newDomainOrder(t, otherRxID, seedOffer(t, client, otherRxID, pharmID)))
		require.NoError(t, err)

		got, err := repo.List(ctx, []uuid.UUID{rxID})
		require.NoError(t, err)

		ids := make([]uuid.UUID, 0, len(got))
		for _, o := range got {
			ids = append(ids, o.ID)
		}
		assert.Contains(t, ids, mine.ID)
		assert.NotContains(t, ids, theirs.ID, "another user's order must not be reachable")
	})

	// The empty case is the one with teeth: an unfiltered query would return every
	// order in the table, so a user with no prescriptions must short-circuit to none.
	t.Run("List with no prescription IDs returns no orders", func(t *testing.T) {
		got, err := repo.List(ctx, []uuid.UUID{})
		require.NoError(t, err)
		assert.Empty(t, got)

		got, err = repo.List(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("List returns newest first and populates PlacedAt", func(t *testing.T) {
		sortRxID := seedPrescription(t, client, userID)
		created := make([]uuid.UUID, 0, 3)
		for range 3 {
			o, err := repo.Create(ctx,
				newDomainOrder(t, sortRxID, seedOffer(t, client, sortRxID, pharmID)))
			require.NoError(t, err)
			created = append(created, o.ID)
		}

		got, err := repo.List(ctx, []uuid.UUID{sortRxID})
		require.NoError(t, err)
		require.Len(t, got, 3)

		// Reverse insertion order — the ORDER BY created_at DESC the API contract
		// promises ("newest first"), not whatever order Postgres happens to return.
		assert.Equal(t, created[2], got[0].ID)
		assert.Equal(t, created[1], got[1].ID)
		assert.Equal(t, created[0], got[2].ID)

		// PlacedAt is the created_at column renamed at the domain boundary; if the
		// mapper drops it, every order reports the zero time.
		assert.False(t, got[0].PlacedAt.IsZero(), "PlacedAt must come back from created_at")
		assert.False(t, got[0].PlacedAt.Before(got[2].PlacedAt))
	})
}
