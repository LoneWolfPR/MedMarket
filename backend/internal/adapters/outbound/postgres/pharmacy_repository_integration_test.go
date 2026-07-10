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
	"github.com/LoneWolfPR/MedMarket/backend/internal/adapters/outbound/postgres"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/pharmacy"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Two independently valid pharmacies. Regulatory IDs carry real checksums (NPI
// Luhn, DEA check digit); newDomainPharmacy re-validates them via NewNPI/NewDEA,
// so a bad constant fails the test immediately rather than silently.
const (
	npiA, deaA, ncpdpA = "1234567893", "AF1234563", "1234567"
	npiB, deaB, ncpdpB = "1245319599", "BX7654329", "7654321"
)

// fixtureAddress is the canonical valid address used by newDomainPharmacy and
// asserted on the round-trip. street2 is populated so the optional column is
// exercised; the "empty street2" case clears it on its own copy.
var fixtureAddress = shared.Address{
	Street1: "1 Main St",
	Street2: "Suite 100",
	City:    "Anytown",
	State:   "CA",
	Zip:     "90001",
}

// setupPharmacyRepo spins up a throwaway Postgres, builds the Ent schema into
// it, and returns a repository wired to it. Unlike the user table (one unique
// column, easily varied per case), Pharmacy has three unique columns — a shared
// container would force globally-unique fixtures and cross-subtest ordering. So
// each subtest gets its own container and stays self-contained with the two
// canonical fixtures above.
func setupPharmacyRepo(t *testing.T) *postgres.PharmacyRepository {
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

	repo, err := postgres.NewPharmacyRepository(postgres.NewPharmacyRepositoryParams{
		Client: client,
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	return repo
}

// newDomainPharmacy builds a valid, fully-populated domain pharmacy for
// persistence.
func newDomainPharmacy(t *testing.T, code, name, npi, dea, ncpdp string) *pharmacy.Pharmacy {
	t.Helper()

	phone, err := shared.NewPhone("5551234567")
	require.NoError(t, err)
	npiVO, err := pharmacy.NewNPI(npi)
	require.NoError(t, err)
	deaVO, err := pharmacy.NewDEA(dea)
	require.NoError(t, err)
	ncpdpVO, err := pharmacy.NewNCPDP(ncpdp)
	require.NoError(t, err)

	p, err := pharmacy.NewPharmacy(pharmacy.NewPharmacyParams{
		Code:         code,
		Name:         name,
		Address:      fixtureAddress,
		ContactPhone: phone,
		NPINum:       npiVO,
		DEANum:       deaVO,
		NCPDPNum:     ncpdpVO,
	})
	require.NoError(t, err)
	return p
}

func TestPharmacyRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("create assigns an ID and round-trips via GetByID", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		created, err := repo.Create(ctx, newDomainPharmacy(t, "acme", "Acme Pharmacy", npiA, deaA, ncpdpA))
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, created.ID, "Create should surface the DB-assigned ID")

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "Acme Pharmacy", got.Name)
		assert.Equal(t, npiA, got.NPINum.String())
		assert.Equal(t, deaA, got.DEANum.String())
		assert.Equal(t, ncpdpA, got.NCPDPNum.String())
		assert.Equal(t, "5551234567", got.ContactPhone.String())
		assert.Equal(t, fixtureAddress, got.Address)
	})

	t.Run("List returns all created pharmacies", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		a, err := repo.Create(ctx, newDomainPharmacy(t, "acme", "Acme", npiA, deaA, ncpdpA))
		require.NoError(t, err)
		b, err := repo.Create(ctx, newDomainPharmacy(t, "bell", "Bell", npiB, deaB, ncpdpB))
		require.NoError(t, err)

		list, err := repo.List(ctx)
		require.NoError(t, err)
		require.Len(t, list, 2)

		ids := map[uuid.UUID]bool{}
		for _, p := range list {
			ids[p.ID] = true
		}
		assert.True(t, ids[a.ID], "List should contain the first pharmacy")
		assert.True(t, ids[b.ID], "List should contain the second pharmacy")
	})

	t.Run("List returns an empty slice when no pharmacies exist", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("GetByID returns ErrPharmacyNotFound for an unknown ID", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		got, err := repo.GetByID(ctx, uuid.New())

		require.ErrorIs(t, err, outbound.ErrPharmacyNotFound)
		assert.Nil(t, got)
	})

	t.Run("GetByCode round-trips a created pharmacy", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		created, err := repo.Create(ctx, newDomainPharmacy(t, "acme", "Acme", npiA, deaA, ncpdpA))
		require.NoError(t, err)

		got, err := repo.GetByCode(ctx, "acme")
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "acme", got.Code)
	})

	t.Run("GetByCode returns ErrPharmacyNotFound for an unknown code", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		got, err := repo.GetByCode(ctx, "does-not-exist")

		require.ErrorIs(t, err, outbound.ErrPharmacyNotFound)
		assert.Nil(t, got)
	})

	t.Run("duplicate regulatory ID returns ErrPharmacyExists", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		_, err := repo.Create(ctx, newDomainPharmacy(t, "acme", "Acme", npiA, deaA, ncpdpA))
		require.NoError(t, err)

		got, err := repo.Create(ctx, newDomainPharmacy(t, "acme-copy", "Acme Copy", npiA, deaA, ncpdpA))

		require.ErrorIs(t, err, outbound.ErrPharmacyExists)
		assert.Nil(t, got)
	})

	t.Run("empty street2 round-trips as a zero value", func(t *testing.T) {
		repo := setupPharmacyRepo(t)

		p := newDomainPharmacy(t, "acme", "Acme", npiA, deaA, ncpdpA)
		p.Address.Street2 = ""

		created, err := repo.Create(ctx, p)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Empty(t, got.Address.Street2)
	})
}
