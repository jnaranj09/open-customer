package dbtest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxMode is a transaction's isolation level and read-only flag, as Postgres reports them.
type TxMode struct {
	Isolation string // current_setting('transaction_isolation'), such as "repeatable read"
	ReadOnly  bool
}

// SnapshotMode is the mode of the snapshot readers: a read-only REPEATABLE READ transaction.
var SnapshotMode = TxMode{Isolation: "repeatable read", ReadOnly: true}

// TxModeRecorder begins real transactions on Pool and records the mode Postgres gives each one,
// so a test sees the options a caller really passed. It is safe for concurrent use.
type TxModeRecorder struct {
	Pool  *pgxpool.Pool
	mu    sync.Mutex
	modes []TxMode
}

// BeginTx begins the transaction and reads its mode inside it.
func (r *TxModeRecorder) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := r.Pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	var mode TxMode
	var readOnly string
	err = tx.QueryRow(ctx, `SELECT current_setting('transaction_isolation'), current_setting('transaction_read_only')`).
		Scan(&mode.Isolation, &readOnly)
	if err != nil {
		if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
			err = errors.Join(err, rbErr)
		}
		return nil, fmt.Errorf("read the transaction mode: %w", err)
	}
	mode.ReadOnly = readOnly == "on"
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modes = append(r.modes, mode)
	return tx, nil
}

// Modes returns the mode of every transaction begun so far, in order.
func (r *TxModeRecorder) Modes() []TxMode {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.modes)
}
