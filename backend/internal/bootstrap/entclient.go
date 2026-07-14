// Package bootstrap holds composition-root helpers shared by the app's binaries
// (cmd/server, cmd/worker): database-client construction and startup seeding.
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/LoneWolfPR/MedMarket/backend/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	// Registers the pgx stdlib driver for sql.Open("pgx", ...); importing this
	// package pulls the driver into whichever binary uses NewEntClient.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// NewEntClient opens the database connection, fails fast if it is unreachable,
// and wraps it in an Ent client. The caller owns closing the returned client.
func NewEntClient(databaseURL string) (*ent.Client, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("could not initialize database connection: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database unreachable: %w", err)
	}
	drv := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(drv)), nil
}
