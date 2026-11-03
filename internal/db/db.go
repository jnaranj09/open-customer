// Package db owns the Postgres connection pool and the migrations.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db/sqlc"
)

// DBTX is what every module function takes: the pool or a transaction. sqlc generates the
// interface in its own package; this alias is the name modules use.
type DBTX = sqlc.DBTX

// Open creates a pool for databaseURL and checks that the database answers.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}
