// Package db owns the Postgres connection pool and the migrations.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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

// Beginner begins a transaction: the pool (*pgxpool.Pool) in the app, or a test fake (a Commit or
// query that fails, or dbtest.TxModeRecorder).
type Beginner interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// WithTx runs fn in a transaction that pool begins with opts. It commits when fn returns nil and
// rolls back when fn returns an error or panics (the panic is re-raised). A commit error is
// returned: the caller must never report a failed commit as saved.
func WithTx(ctx context.Context, pool Beginner, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	// The rollback must still reach the server when ctx is already cancelled.
	rollback := func() error { return tx.Rollback(context.WithoutCancel(ctx)) }
	defer func() {
		if p := recover(); p != nil {
			// Re-raise the panic; a failed rollback is named in the panic value.
			if rbErr := rollback(); rbErr != nil {
				panic(fmt.Errorf("%v (rollback failed: %w)", p, rbErr))
			}
			panic(p)
		}
	}()
	if fnErr := fn(tx); fnErr != nil {
		if rbErr := rollback(); rbErr != nil {
			return fmt.Errorf("db: rollback: %w", errors.Join(rbErr, fnErr))
		}
		return fmt.Errorf("db: tx: %w", fnErr)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}
