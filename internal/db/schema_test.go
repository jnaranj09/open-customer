//go:build db

package db_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
)

// SQLSTATE codes the schema tests expect.
const (
	checkViolation  = "23514"
	uniqueViolation = "23505"
)

// Made-up ids; the schema sets no default, so tests give their own.
const (
	clientID  = "00000000-0000-0000-0000-000000000001"
	serviceID = "00000000-0000-0000-0000-000000000002"
	apptID    = "00000000-0000-0000-0000-000000000003"
	tmplID    = "00000000-0000-0000-0000-000000000004"
	ruleID    = "00000000-0000-0000-0000-000000000005"
	userID    = "00000000-0000-0000-0000-000000000006"
)

const (
	insertClient   = `INSERT INTO clients (id, name) VALUES ('` + clientID + `', 'Test Client')`
	insertService  = `INSERT INTO services (id, name, duration_min, color, position) VALUES ('` + serviceID + `', 'Cut', 30, 'blue', 1)`
	insertAppt     = `INSERT INTO appointments (id, client_id, starts_at, ends_at) VALUES ('` + apptID + `', '` + clientID + `', '2026-03-01 10:00+00', '2026-03-01 11:00+00')`
	insertTemplate = `INSERT INTO message_templates (id, name, body) VALUES ('` + tmplID + `', 'T', 'Hello')`
	insertUser     = `INSERT INTO users (id, username, password_hash) VALUES ('` + userID + `', 'tester', 'hash')`
)

func TestSchemaRejectsInvalidRows(t *testing.T) {
	cases := []struct {
		name  string
		setup []string
		stmt  string
		code  string
	}{
		{
			name: "business_settings id 2",
			stmt: `INSERT INTO business_settings (id, name, timezone, locale, currency, country_code, accent_color)
				VALUES (2, 'Shop', 'UTC', 'xx', 'XXX', 'XX', '#000000')`,
			code: checkViolation,
		},
		{
			name: "push_hour 20",
			stmt: `INSERT INTO business_settings (id, name, timezone, locale, currency, country_code, accent_color, push_hour)
				VALUES (1, 'Shop', 'UTC', 'xx', 'XXX', 'XX', '#000000', 20)`,
			code: checkViolation,
		},
		{
			name: "client attrs is an array",
			stmt: `INSERT INTO clients (id, name, attrs) VALUES ('` + clientID + `', 'Test Client', '[]')`,
			code: checkViolation,
		},
		{
			name:  "appointment ends_at equals starts_at",
			setup: []string{insertClient},
			stmt: `INSERT INTO appointments (id, client_id, starts_at, ends_at)
				VALUES ('` + apptID + `', '` + clientID + `', '2026-03-01 10:00+00', '2026-03-01 10:00+00')`,
			code: checkViolation,
		},
		{
			name: "service duration_min 0",
			stmt: `INSERT INTO services (id, name, duration_min, color, position)
				VALUES ('` + serviceID + `', 'Cut', 0, 'blue', 1)`,
			code: checkViolation,
		},
		{
			name: "service negative price_cents",
			stmt: `INSERT INTO services (id, name, duration_min, price_cents, color, position)
				VALUES ('` + serviceID + `', 'Cut', 30, -1, 'blue', 1)`,
			code: checkViolation,
		},
		{
			name: "service description of 201 characters",
			stmt: `INSERT INTO services (id, name, duration_min, color, position, description)
				VALUES ('` + serviceID + `', 'Cut', 30, 'blue', 1, repeat('a', 201))`,
			code: checkViolation,
		},
		{
			name: "service empty description",
			stmt: `INSERT INTO services (id, name, duration_min, color, position, description)
				VALUES ('` + serviceID + `', 'Cut', 30, 'blue', 1, '')`,
			code: checkViolation,
		},
		{
			name:  "come_back rule with offset_days 0",
			setup: []string{insertService, insertTemplate},
			stmt: `INSERT INTO reminder_rules (id, kind, service_id, offset_days, template_id)
				VALUES ('` + ruleID + `', 'come_back', '` + serviceID + `', 0, '` + tmplID + `')`,
			code: checkViolation,
		},
		{
			name:  "before_appointment rule with a service_id",
			setup: []string{insertService, insertTemplate},
			stmt: `INSERT INTO reminder_rules (id, kind, service_id, offset_days, template_id)
				VALUES ('` + ruleID + `', 'before_appointment', '` + serviceID + `', 1, '` + tmplID + `')`,
			code: checkViolation,
		},
		{
			name:  "before_appointment rule with a return_service_id",
			setup: []string{insertService, insertTemplate},
			stmt: `INSERT INTO reminder_rules (id, kind, offset_days, template_id, return_service_id)
				VALUES ('` + ruleID + `', 'before_appointment', 1, '` + tmplID + `', '` + serviceID + `')`,
			code: checkViolation,
		},
		{
			name:  "appointment status outside the four values",
			setup: []string{insertClient},
			stmt: `INSERT INTO appointments (id, client_id, starts_at, ends_at, status)
				VALUES ('` + apptID + `', '` + clientID + `', '2026-03-01 10:00+00', '2026-03-01 11:00+00', 'done')`,
			code: checkViolation,
		},
		{
			name: "two come_back log rows with null anchor_at for one rule and appointment",
			setup: []string{
				insertClient, insertService, insertTemplate, insertAppt,
				`INSERT INTO reminder_rules (id, kind, service_id, offset_days, template_id)
					VALUES ('` + ruleID + `', 'come_back', '` + serviceID + `', 30, '` + tmplID + `')`,
				`INSERT INTO reminder_log (id, rule_id, appointment_id, anchor_at, action, acted_at)
					VALUES ('00000000-0000-0000-0000-0000000000a1', '` + ruleID + `', '` + apptID + `', NULL, 'sent', now())`,
			},
			stmt: `INSERT INTO reminder_log (id, rule_id, appointment_id, anchor_at, action, acted_at)
				VALUES ('00000000-0000-0000-0000-0000000000a2', '` + ruleID + `', '` + apptID + `', NULL, 'skipped', now())`,
			code: uniqueViolation,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is rejected", func(t *testing.T) {
			pool := dbtest.Pool(t)
			for _, s := range tc.setup {
				if _, err := pool.Exec(t.Context(), s); err != nil {
					t.Fatalf("setup %q: %v", s, err)
				}
			}
			_, err := pool.Exec(t.Context(), tc.stmt)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("insert error = %v, want a Postgres constraint error", err)
			}
			if pgErr.Code != tc.code {
				t.Fatalf("SQLSTATE = %s (%s), want %s", pgErr.Code, pgErr.ConstraintName, tc.code)
			}
		})
	}
}

func TestSchemaDeletingUserCascadesToSessions(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := t.Context()
	for _, s := range []string{
		insertUser,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ('\x01', '` + userID + `', now() + interval '1 day')`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("setup %q: %v", s, err)
		}
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = '`+userID+`'`); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if n != 0 {
		t.Fatalf("sessions after deleting the user = %d, want 0", n)
	}
}
