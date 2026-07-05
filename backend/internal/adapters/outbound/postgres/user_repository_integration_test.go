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
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/user"
	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const validHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// setupRepo spins up a throwaway Postgres, builds the Ent schema into it, and
// returns a repository wired to it. One container is shared across the subtests
// of a single top-level test; t.Cleanup tears everything down.
func setupRepo(t *testing.T) *postgres.UserRepository {
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

	repo, err := postgres.NewUserRepository(postgres.NewUserRepositoryParams{
		Client: client,
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)
	return repo
}

// newDomainUser builds a valid, fully-populated domain user for persistence.
func newDomainUser(t *testing.T, email string) *user.User {
	t.Helper()

	e, err := user.NewEmail(email)
	require.NoError(t, err)
	h, err := user.NewPasswordHash(validHash)
	require.NoError(t, err)
	p, err := user.NewPhone("5551234567")
	require.NoError(t, err)
	u, err := user.NewUser(user.NewUserParams{
		Email:        e,
		PasswordHash: h,
		FirstName:    "Jane",
		LastName:     "Doe",
		Phone:        p,
		Address:      shared.Address{Street1: "1 Main St", City: "Anytown", State: "CA", Zip: "90001"},
	})
	require.NoError(t, err)
	return u
}

func TestUserRepository(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	t.Run("create assigns an ID and round-trips via GetByID", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainUser(t, "byid@example.com"))
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, created.ID, "Create should surface the DB-assigned ID")

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "byid@example.com", got.Email.String())
		assert.Equal(t, "Jane", got.FirstName)
		assert.Equal(t, "Doe", got.LastName)
		assert.Equal(t, validHash, got.PasswordHash.String())
		assert.Equal(t, "5551234567", got.Phone.String())
		assert.Equal(t, shared.Address{Street1: "1 Main St", City: "Anytown", State: "CA", Zip: "90001"}, got.Address)
	})

	t.Run("GetByEmail finds the created user", func(t *testing.T) {
		created, err := repo.Create(ctx, newDomainUser(t, "byemail@example.com"))
		require.NoError(t, err)

		got, err := repo.GetByEmail(ctx, created.Email)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "byemail@example.com", got.Email.String())
	})

	t.Run("GetByID returns ErrUserNotFound for an unknown ID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, uuid.New())

		require.ErrorIs(t, err, outbound.ErrUserNotFound)
		assert.Nil(t, got)
	})

	t.Run("GetByEmail returns ErrUserNotFound for an unknown email", func(t *testing.T) {
		email, err := user.NewEmail("nobody@example.com")
		require.NoError(t, err)

		got, err := repo.GetByEmail(ctx, email)

		require.ErrorIs(t, err, outbound.ErrUserNotFound)
		assert.Nil(t, got)
	})

	t.Run("duplicate email returns ErrEmailTaken", func(t *testing.T) {
		_, err := repo.Create(ctx, newDomainUser(t, "dupe@example.com"))
		require.NoError(t, err)

		got, err := repo.Create(ctx, newDomainUser(t, "dupe@example.com"))

		require.ErrorIs(t, err, outbound.ErrEmailTaken)
		assert.Nil(t, got)
	})

	t.Run("empty optional fields round-trip as zero values", func(t *testing.T) {
		e, err := user.NewEmail("minimal@example.com")
		require.NoError(t, err)
		h, err := user.NewPasswordHash(validHash)
		require.NoError(t, err)
		minimal, err := user.NewUser(user.NewUserParams{
			Email: e, PasswordHash: h, FirstName: "No", LastName: "Extras",
		})
		require.NoError(t, err)

		created, err := repo.Create(ctx, minimal)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, got.Phone.IsZero())
		assert.Equal(t, shared.Address{}, got.Address)
	})
}
