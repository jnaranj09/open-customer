//go:build db

package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
)

func countKeys(ctx context.Context, t *testing.T, q db.DBTX) int {
	t.Helper()
	var n int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM server_keys`).Scan(&n); err != nil {
		t.Fatalf("count server_keys: %v", err)
	}
	return n
}

func insertKey(ctx context.Context, q db.DBTX, name string) error {
	_, err := q.Exec(ctx, `INSERT INTO server_keys (name, value) VALUES ($1, 'v')`, name)
	return err
}

func TestWithTxCommitsOnNil(t *testing.T) {
	pool := dbtest.Pool(t)
	err := db.WithTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return insertKey(t.Context(), tx, "a")
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if got := countKeys(t.Context(), t, pool); got != 1 {
		t.Fatalf("rows after commit = %d, want 1", got)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	pool := dbtest.Pool(t)
	boom := errors.New("boom")
	err := db.WithTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := insertKey(t.Context(), tx, "a"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTx error = %v, want it to wrap boom", err)
	}
	if got := countKeys(t.Context(), t, pool); got != 0 {
		t.Fatalf("rows after rollback = %d, want 0", got)
	}
}

func TestWithTxRollsBackOnPanic(t *testing.T) {
	pool := dbtest.Pool(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("WithTx swallowed the panic, want it re-raised")
			}
		}()
		_ = db.WithTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			if err := insertKey(t.Context(), tx, "a"); err != nil {
				return err
			}
			panic("handler bug")
		})
	}()
	if got := countKeys(t.Context(), t, pool); got != 0 {
		t.Fatalf("rows after panic = %d, want 0", got)
	}
}

func TestWithTxReadOnlyRejectsWrites(t *testing.T) {
	pool := dbtest.Pool(t)
	opts := pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}
	err := db.WithTx(t.Context(), pool, opts, func(tx pgx.Tx) error {
		return insertKey(t.Context(), tx, "a")
	})
	if err == nil {
		t.Fatal("WithTx read-only accepted a write")
	}
	if got := countKeys(t.Context(), t, pool); got != 0 {
		t.Fatalf("rows after rejected write = %d, want 0", got)
	}
}

func TestWithTxRepeatableReadSeesOneSnapshot(t *testing.T) {
	pool := dbtest.Pool(t)
	opts := pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}
	var first, second int
	err := db.WithTx(t.Context(), pool, opts, func(tx pgx.Tx) error {
		first = countKeys(t.Context(), t, tx)
		if err := insertKey(t.Context(), pool, "late"); err != nil {
			return err
		}
		second = countKeys(t.Context(), t, tx)
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if first != 0 || second != 0 {
		t.Fatalf("counts inside snapshot = %d then %d, want 0 and 0", first, second)
	}
	if got := countKeys(t.Context(), t, pool); got != 1 {
		t.Fatalf("rows after the transaction = %d, want 1", got)
	}
}

func TestWithTxCommitErrorIsReturned(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `CREATE TABLE deferred_unique (n int UNIQUE DEFERRABLE INITIALLY DEFERRED)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	err := db.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// Both inserts succeed; the unique check only fires at commit.
		_, err := tx.Exec(ctx, `INSERT INTO deferred_unique (n) VALUES (1), (1)`)
		return err
	})
	if err == nil {
		t.Fatal("WithTx returned nil although the commit failed")
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM deferred_unique`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("rows after failed commit = %d, want 0", n)
	}
}
