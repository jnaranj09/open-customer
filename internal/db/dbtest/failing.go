package dbtest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jnaranj09/open-customer/internal/db"
)

// ErrInjected is the failure FailingDB returns from its chosen call.
var ErrInjected = errors.New("made-up database failure")

// FailingDB is the real database, except that call number FailAt (1-based; 0 never fails) returns
// ErrInjected instead of running. Calls counts every call, so a clean run tells how many steps an
// operation has.
type FailingDB struct {
	db.DBTX
	FailAt int
	Calls  *int
}

func (f FailingDB) fail() bool {
	*f.Calls++
	return *f.Calls == f.FailAt
}

// Exec runs the call on the real database unless it is the chosen one.
func (f FailingDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.fail() {
		return pgconn.CommandTag{}, ErrInjected
	}
	tag, err := f.DBTX.Exec(ctx, sql, args...)
	if err != nil {
		return tag, fmt.Errorf("exec: %w", err)
	}
	return tag, nil
}

// Query runs the call on the real database unless it is the chosen one.
func (f FailingDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.fail() {
		return nil, ErrInjected
	}
	rows, err := f.DBTX.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return rows, nil
}

// QueryRow runs the call on the real database unless it is the chosen one.
func (f FailingDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.fail() {
		return failedRow{}
	}
	return f.DBTX.QueryRow(ctx, sql, args...)
}

type failedRow struct{}

func (failedRow) Scan(...any) error { return ErrInjected }
