//go:build db

package db_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
)

// releaseOneMigrations is the embedded migrations up to 00002, the schema release 1 shipped.
func releaseOneMigrations(t *testing.T) fs.FS {
	t.Helper()
	all, err := db.EmbeddedMigrations()
	if err != nil {
		t.Fatalf("embedded migrations: %v", err)
	}
	out := fstest.MapFS{}
	for _, name := range []string{"00001_init.sql", "00002_reminder_return_service.sql"} {
		data, err := fs.ReadFile(all, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		out[name] = &fstest.MapFile{Data: data}
	}
	return out
}

// TestMigrateFromRelease1Data upgrades a database that holds release-1 rows (reminders-and-replies
// design, item 28): the rows stay as they were, no rule gets a handover_from, a general rule can be
// inserted, and a "before" rule with a service is still refused.
func TestMigrateFromRelease1Data(t *testing.T) {
	c := dbtest.Empty(t)
	pool, err := db.Open(t.Context(), c.URL)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	opts := db.MigrateOptions{AllowWithoutBackup: true}
	if err := db.MigrateFS(t.Context(), pool, releaseOneMigrations(t), opts); err != nil {
		t.Fatalf("migrate to release 1: %v", err)
	}
	for _, stmt := range []string{
		insertClient, insertService, insertTemplate, insertAppt,
		`INSERT INTO reminder_rules (id, kind, offset_days, template_id) VALUES ('00000000-0000-0000-0000-0000000000b1', 'before_appointment', 1, '` + tmplID + `')`,
		`INSERT INTO reminder_rules (id, kind, service_id, offset_days, template_id) VALUES ('` + ruleID + `', 'come_back', '` + serviceID + `', 21, '` + tmplID + `')`,
		`INSERT INTO reminder_rules (id, kind, service_id, offset_days, template_id, archived_at) VALUES ('00000000-0000-0000-0000-0000000000b2', 'come_back', '` + serviceID + `', 30, '` + tmplID + `', now())`,
		`INSERT INTO reminder_log (id, rule_id, appointment_id, action, acted_at) VALUES ('00000000-0000-0000-0000-0000000000c1', '` + ruleID + `', '` + apptID + `', 'sent', now())`,
	} {
		if _, err := pool.Exec(t.Context(), stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
	const snapshot = `SELECT string_agg(concat_ws('|', id, kind, service_id, return_service_id, offset_days, template_id, archived_at IS NOT NULL), ',' ORDER BY id) FROM reminder_rules`
	var before string
	if err := pool.QueryRow(t.Context(), snapshot).Scan(&before); err != nil {
		t.Fatalf("rules before: %v", err)
	}

	if err := db.Migrate(t.Context(), pool, opts); err != nil {
		t.Fatalf("migrate to latest: %v", err)
	}

	var after string
	if err := pool.QueryRow(t.Context(), snapshot).Scan(&after); err != nil {
		t.Fatalf("rules after: %v", err)
	}
	if after != before {
		t.Errorf("rules changed:\n before %s\n after  %s", before, after)
	}
	var handovers, logs int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FILTER (WHERE handover_from IS NOT NULL) FROM reminder_rules`).Scan(&handovers); err != nil || handovers != 0 {
		t.Errorf("rules with handover_from = %d, %v; want 0", handovers, err)
	}
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM reminder_log`).Scan(&logs); err != nil || logs != 1 {
		t.Errorf("log rows = %d, %v; want 1", logs, err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO reminder_rules (id, kind, offset_days, template_id) VALUES ('00000000-0000-0000-0000-0000000000b3', 'come_back', 30, '`+tmplID+`')`); err != nil {
		t.Errorf("insert a general rule: %v", err)
	}
	_, err = pool.Exec(t.Context(), `INSERT INTO reminder_rules (id, kind, service_id, offset_days, template_id) VALUES ('00000000-0000-0000-0000-0000000000b4', 'before_appointment', '`+serviceID+`', 1, '`+tmplID+`')`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != checkViolation || !strings.Contains(pgErr.ConstraintName, "service") {
		t.Errorf("a before rule with a service: err = %v, want a service CHECK violation", err)
	}
}
