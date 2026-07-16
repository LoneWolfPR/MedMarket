//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/LoneWolfPR/MedMarket/backend/ent"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newTestClient spins up a throwaway Postgres, builds the Ent schema into it,
// and returns an Ent client wired to it. The container and client are torn down
// via t.Cleanup. Callers decide the sharing granularity: one client per
// top-level test where the table has no unique constraints to collide on, one
// per subtest where it does (see setupPharmacyRepo).
func newTestClient(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("medmarket_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Schema.Create(ctx))
	return client
}

// discardLogger is the logger every repo under test gets: the repos require a
// non-nil logger, but their log output is not the subject of these tests.
func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// The seed* helpers below insert FK parent rows directly through Ent rather than
// through their repositories. These rows exist only so a foreign key resolves —
// their field values are never read by the test that needs them — so going
// through the domain constructors and repos would drag unrelated layers into the
// test for no assertion. Each seeds unique values where the table demands them.

// seedUser inserts a user row and returns its id. prescriptions.user_id is a FK
// to users.id, so any test persisting a prescription needs a real user first.
func seedUser(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()

	u, err := client.User.Create().
		SetEmail("user-" + uuid.NewString() + "@example.com").
		SetPasswordHash(validHash).
		SetFirstName("Test").
		SetLastName("User").
		Save(context.Background())
	require.NoError(t, err)
	return u.ID
}

// seedPrescription inserts a prescription row owned by userID and returns its
// id. offers.prescription_id is a FK to prescriptions.id.
func seedPrescription(t *testing.T, client *ent.Client, userID uuid.UUID) uuid.UUID {
	t.Helper()

	rx, err := client.Prescription.Create().
		SetUserID(userID).
		SetDocumentKey("prescriptions/" + uuid.NewString() + ".pdf").
		SetPhysicianName("Dr. Smith").
		SetMedName("atorvastatin").
		SetMedStrengthValue("20").
		SetMedStrengthUnit("mg").
		SetQuantity(30).
		Save(context.Background())
	require.NoError(t, err)
	return rx.ID
}

// seedOffer inserts an offer row and returns its id. orders.offer_id is a FK to
// offers.id.
func seedOffer(t *testing.T, client *ent.Client, rxID, pharmID uuid.UUID) uuid.UUID {
	t.Helper()

	o, err := client.Offer.Create().
		SetPrescriptionID(rxID).
		SetPharmacyID(pharmID).
		SetPriceCents(1299).
		SetPharmacyItemID("RXD-ATO-20").
		SetExpiresAt(time.Now().Add(15 * time.Minute)).
		Save(context.Background())
	require.NoError(t, err)
	return o.ID
}

// seedPharmacy inserts a pharmacy row and returns its id. offers.pharmacy_id is
// a FK to pharmacies.id. The regulatory ids are unique columns, so they are
// uuid-derived rather than the checksum-valid fixtures used by the pharmacy
// repo's own tests — nothing here reads them back through the domain.
func seedPharmacy(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()

	unique := uuid.NewString()
	p, err := client.Pharmacy.Create().
		SetCode("pharm-" + unique).
		SetName("Test Pharmacy").
		SetAddressStreet1("1 Main St").
		SetAddressCity("Anytown").
		SetAddressState("CA").
		SetAddressZip("90001").
		SetContactPhone("5551234567").
		SetNpi("npi-" + unique).
		SetDea("dea-" + unique).
		SetNcpdp("ncpdp-" + unique).
		Save(context.Background())
	require.NoError(t, err)
	return p.ID
}
