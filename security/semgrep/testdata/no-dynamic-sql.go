package fixtures

import (
	"context"
	"fmt"
	"strings"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (int, error)
	Query(ctx context.Context, sql string, args ...any) (int, error)
	QueryRow(ctx context.Context, sql string, args ...any) int
}

const listClients = `SELECT id FROM clients WHERE archived = $1`

func buildsTheQuery(ctx context.Context, db DBTX, name string, cols []string) {
	// ruleid: no-dynamic-sql
	db.Exec(ctx, "DELETE FROM clients WHERE name = '"+name+"'")
	// ruleid: no-dynamic-sql
	db.Query(ctx, fmt.Sprintf("SELECT %s FROM clients", strings.Join(cols, ",")))
	// ruleid: no-dynamic-sql
	db.QueryRow(ctx, fmt.Sprintf("SELECT id FROM clients WHERE name = '%s'", name))
	q := "SELECT id FROM clients WHERE name = '" + name + "'"
	// ruleid: no-dynamic-sql
	db.Query(ctx, q)
	var b strings.Builder
	b.WriteString("SELECT id FROM clients WHERE name = '")
	b.WriteString(name)
	// ruleid: no-dynamic-sql
	db.Query(ctx, b.String())
}

func bindsTheValues(ctx context.Context, db DBTX, name string) {
	// ok: no-dynamic-sql
	db.Exec(ctx, "DELETE FROM clients WHERE name = $1", name)
	// ok: no-dynamic-sql
	db.Query(ctx, listClients, false)
	// ok: no-dynamic-sql
	db.QueryRow(ctx, `SELECT current_setting('transaction_isolation')`)
}

// A query text a caller hands in is that caller's string; the callers are checked where they are.
func passesAQueryThrough(ctx context.Context, db DBTX, sql string, args ...any) {
	// ok: no-dynamic-sql
	db.Exec(ctx, sql, args...)
}
