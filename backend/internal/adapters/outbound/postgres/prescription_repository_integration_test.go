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
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/prescription"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupPrescriptionRepo spins up a throwaway Postgres, builds the Ent schema,
// and returns a repository wired to it. Prescriptions have no unique
// constraints, so one container is safely shared across the subtests.
func setupPrescriptionRepo(t *testing.T) *postgres.PrescriptionRepository {
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

	repo, err := postgres.NewPrescriptionRepository(postgres.NewPrescriptionRepositoryParams{
		Client: client,
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	return repo
}

// newDomainPrescription builds a valid domain prescription for persistence.
func newDomainPrescription(t *testing.T, userID uuid.UUID, docKey string) *prescription.Prescription {
	t.Helper()

	ms, err := shared.NewMedStrength("20", "mg")
	require.NoError(t, err)
	rx, err := prescription.NewPrescription(prescription.NewPrescriptionParams{
		UserID:        userID,
		DocumentKey:   docKey,
		PhysicianName: "Dr. Smith",
		MedName:       "atorvastatin",
		MedStrength:   ms,
		Qty:           30,
	})
	require.NoError(t, err)
	return rx
}

func TestPrescriptionRepository(t *testing.T) {
	repo := setupPrescriptionRepo(t)
	ctx := context.Background()

	t.Run("create assigns an ID and round-trips via GetByID", func(t *testing.T) {
		userID := uuid.New()
		created, err := repo.Create(ctx, newDomainPrescription(t, userID, "prescriptions/a/1.pdf"))
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, created.ID, "Create should surface the DB-assigned ID")

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, userID, got.UserID)
		assert.Equal(t, "prescriptions/a/1.pdf", got.DocumentKey)
		assert.Equal(t, "Dr. Smith", got.PhysicianName)
		assert.Equal(t, "atorvastatin", got.MedName)
		assert.Equal(t, "20", got.MedStrength.Value())
		assert.Equal(t, "mg", got.MedStrength.Unit())
		assert.Equal(t, 30, got.Qty)
	})

	t.Run("List returns only the given user's prescriptions", func(t *testing.T) {
		userA := uuid.New()
		userB := uuid.New()
		_, err := repo.Create(ctx, newDomainPrescription(t, userA, "prescriptions/a/1.pdf"))
		require.NoError(t, err)
		_, err = repo.Create(ctx, newDomainPrescription(t, userA, "prescriptions/a/2.pdf"))
		require.NoError(t, err)
		_, err = repo.Create(ctx, newDomainPrescription(t, userB, "prescriptions/b/1.pdf"))
		require.NoError(t, err)

		listA, err := repo.List(ctx, userA)
		require.NoError(t, err)
		assert.Len(t, listA, 2)
		for _, rx := range listA {
			assert.Equal(t, userA, rx.UserID)
		}
	})

	t.Run("List returns an empty slice for a user with no prescriptions", func(t *testing.T) {
		list, err := repo.List(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("GetByID returns ErrPrescriptionNotFound for an unknown ID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, uuid.New())
		require.ErrorIs(t, err, outbound.ErrPrescriptionNotFound)
		assert.Nil(t, got)
	})
}
