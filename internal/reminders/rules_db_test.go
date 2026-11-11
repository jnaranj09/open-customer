//go:build db

package reminders

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var start = time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC)

type fixture struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	m     *Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := clock.NewFake(start)
	return fixture{pool: dbtest.Pool(t), clock: fake, m: New(Options{Clock: fake})}
}

func ptr[T any](v T) *T { return &v }

func nu(id uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: id, Valid: true} }

func newID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := ids.New()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}
	return id
}

// template inserts a template and returns its id; archived makes it an archived one.
func (f fixture) template(t *testing.T, archived bool) uuid.UUID {
	t.Helper()
	id := newID(t)
	if _, err := f.pool.Exec(t.Context(),
		`INSERT INTO message_templates (id, name, body) VALUES ($1, 'Plantilla', 'Hola')`, id); err != nil {
		t.Fatalf("insert template: %v", err)
	}
	if archived {
		f.archive(t, "message_templates", id)
	}
	return id
}

func (f fixture) service(t *testing.T, archived bool) uuid.UUID {
	t.Helper()
	id := newID(t)
	if _, err := f.pool.Exec(t.Context(),
		`INSERT INTO services (id, name, duration_min, color, position) VALUES ($1, 'Acrílico', 60, '#aa3366', 1)`, id); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if archived {
		f.archive(t, "services", id)
	}
	return id
}

func (f fixture) archive(t *testing.T, table string, id uuid.UUID) {
	t.Helper()
	if _, err := f.pool.Exec(t.Context(), `UPDATE `+table+` SET archived_at = now() WHERE id = $1`, id); err != nil {
		t.Fatalf("archive %s: %v", table, err)
	}
}

func (f fixture) before(t *testing.T, includeOverdue bool) Rule {
	t.Helper()
	return f.createRule(t, RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: f.template(t, false), IncludeOverdue: includeOverdue})
}

// comeBack creates a come_back rule on a new service, returning the rule.
func (f fixture) comeBack(t *testing.T, returnService uuid.NullUUID) Rule {
	t.Helper()
	return f.createRule(t, RuleInput{
		Kind: KindComeBack, ServiceID: nu(f.service(t, false)), ReturnServiceID: returnService,
		OffsetDays: 21, TemplateID: f.template(t, false),
	})
}

func (f fixture) createRule(t *testing.T, in RuleInput) Rule {
	t.Helper()
	r, err := f.m.CreateRule(t.Context(), f.pool, in)
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	return r
}

func (f fixture) stored(t *testing.T, id uuid.UUID) Rule {
	t.Helper()
	rules, err := f.m.ListRules(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	for _, r := range rules {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %s not listed", id)
	return Rule{}
}

func fieldsOf(t *testing.T, err error) []problem.FieldError {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want problem.Validation", err)
	}
	return v.Errors
}

func wantAppliesFrom(t *testing.T, got *time.Time, want *time.Time) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Fatalf("applies_from = %v, want NULL", got)
	case want != nil && (got == nil || !got.Equal(*want)):
		t.Fatalf("applies_from = %v, want %v", got, want)
	}
}

func TestParseKindRejectsUnknown(t *testing.T) {
	_, err := ParseKind("weekly")
	got := fieldsOf(t, err)
	if len(got) != 1 || got[0].Field != "kind" || got[0].Message != problem.MsgRuleKindInvalid {
		t.Fatalf("errors = %+v, want one on kind", got)
	}
}

func TestCreateBeforeRuleSetsAppliesFromNow(t *testing.T) {
	f := newFixture(t)
	r := f.before(t, false)
	wantAppliesFrom(t, r.AppliesFrom, &start)
	wantAppliesFrom(t, f.stored(t, r.ID).AppliesFrom, &start)
}

func TestCreateIncludeOverdueLeavesAppliesFromNull(t *testing.T) {
	f := newFixture(t)
	r := f.before(t, true)
	wantAppliesFrom(t, r.AppliesFrom, nil)
	wantAppliesFrom(t, f.stored(t, r.ID).AppliesFrom, nil)
}

func TestCreateRuleStoresCreationTimeFromClock(t *testing.T) {
	f := newFixture(t)
	r := f.before(t, false)
	if got := f.stored(t, r.ID).CreatedAt; !got.Equal(start) {
		t.Fatalf("created_at = %v, want %v", got, start)
	}
}

func TestCreateRuleRejects(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, f fixture) RuleInput
		field string
		msg   string
	}{
		{"before with service_id", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindBefore, ServiceID: nu(f.service(t, false)), OffsetDays: 1, TemplateID: f.template(t, false)}
		}, "service_id", problem.MsgServiceNotAllowed},
		{"before with return_service_id", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindBefore, ReturnServiceID: nu(f.service(t, false)), OffsetDays: 1, TemplateID: f.template(t, false)}
		}, "return_service_id", problem.MsgServiceNotAllowed},
		{"general with return_service_id", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindComeBack, ReturnServiceID: nu(f.service(t, false)), OffsetDays: 21, TemplateID: f.template(t, false)}
		}, "return_service_id", problem.MsgGeneralNoReturn},
		{"come_back offset 0", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindComeBack, ServiceID: nu(f.service(t, false)), OffsetDays: 0, TemplateID: f.template(t, false)}
		}, "offset_days", problem.MsgComeBackOffsetInvalid},
		{"come_back offset 366", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindComeBack, ServiceID: nu(f.service(t, false)), OffsetDays: 366, TemplateID: f.template(t, false)}
		}, "offset_days", problem.MsgComeBackOffsetInvalid},
		{"before offset -1", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindBefore, OffsetDays: -1, TemplateID: f.template(t, false)}
		}, "offset_days", problem.MsgOffsetInvalid},
		{"before offset 366", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindBefore, OffsetDays: 366, TemplateID: f.template(t, false)}
		}, "offset_days", problem.MsgOffsetInvalid},
		{"unknown template", func(t *testing.T, _ fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: newID(t)}
		}, "template_id", problem.MsgTemplateUnavailable},
		{"archived template", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: f.template(t, true)}
		}, "template_id", problem.MsgTemplateUnavailable},
		{"archived service", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{Kind: KindComeBack, ServiceID: nu(f.service(t, true)), OffsetDays: 21, TemplateID: f.template(t, false)}
		}, "service_id", problem.MsgServiceUnavailable},
		{"archived return service", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{
				Kind: KindComeBack, ServiceID: nu(f.service(t, false)), ReturnServiceID: nu(f.service(t, true)),
				OffsetDays: 21, TemplateID: f.template(t, false),
			}
		}, "return_service_id", problem.MsgServiceUnavailable},
		{"unknown return service", func(t *testing.T, f fixture) RuleInput {
			t.Helper()
			return RuleInput{
				Kind: KindComeBack, ServiceID: nu(f.service(t, false)), ReturnServiceID: nu(newID(t)),
				OffsetDays: 21, TemplateID: f.template(t, false),
			}
		}, "return_service_id", problem.MsgServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.m.CreateRule(t.Context(), f.pool, tc.build(t, f))
			got := fieldsOf(t, err)
			if len(got) != 1 || got[0].Field != tc.field || got[0].Message != tc.msg {
				t.Fatalf("errors = %+v, want one on %q with %q", got, tc.field, tc.msg)
			}
		})
	}
}

func TestCreateRuleRejectsUnknownKind(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.CreateRule(t.Context(), f.pool, RuleInput{Kind: "weekly", OffsetDays: 1, TemplateID: f.template(t, false)})
	got := fieldsOf(t, err)
	if len(got) != 1 || got[0].Field != "kind" {
		t.Fatalf("errors = %+v, want one on kind", got)
	}
}

func TestCreateRuleCollectsEveryError(t *testing.T) {
	f := newFixture(t)
	in := RuleInput{Kind: KindBefore, ServiceID: nu(f.service(t, false)), OffsetDays: -1, TemplateID: f.template(t, false)}
	_, err := f.m.CreateRule(t.Context(), f.pool, in)
	got := fieldsOf(t, err)
	if len(got) != 2 {
		t.Fatalf("errors = %+v, want two", got)
	}
	fields := map[string]bool{got[0].Field: true, got[1].Field: true}
	if !fields["service_id"] || !fields["offset_days"] {
		t.Fatalf("errors = %+v, want service_id and offset_days", got)
	}
}

func TestCreateRuleKeepsExplicitReturnService(t *testing.T) {
	f := newFixture(t)
	ret := f.service(t, false)
	r := f.comeBack(t, nu(ret))
	if got := f.stored(t, r.ID).ReturnServiceID; got != nu(ret) {
		t.Fatalf("return_service_id = %v, want %v", got, nu(ret))
	}
}

func TestUpdateRuleAsks(t *testing.T) {
	later := start.Add(48 * time.Hour)
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, f fixture) (Rule, RulePatch)
	}{
		{"new offset", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			return f.before(t, true), RulePatch{OffsetDays: ptr(3)}
		}},
		{"new return service", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			return f.comeBack(t, uuid.NullUUID{}), RulePatch{ReturnServiceID: nullable.NewNullableWithValue(f.service(t, false))}
		}},
		{"restored rule", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			r := f.before(t, true)
			f.archive(t, "reminder_rules", r.ID)
			return r, RulePatch{Archived: ptr(false)}
		}},
		{"new template while the stored one is archived", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			r := f.before(t, true)
			f.archive(t, "message_templates", r.TemplateID)
			return r, RulePatch{TemplateID: ptr(f.template(t, false))}
		}},
		{"new return service while the stored one is archived", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			r := f.comeBack(t, nu(f.service(t, false)))
			f.archive(t, "services", r.ReturnServiceID.UUID)
			return r, RulePatch{ReturnServiceID: nullable.NewNullableWithValue(f.service(t, false))}
		}},
	} {
		for _, overdue := range []bool{false, true} {
			name := tc.name + " only from now on"
			want := &later
			if overdue {
				name, want = tc.name+" include overdue", nil
			}
			t.Run(name, func(t *testing.T) {
				f := newFixture(t)
				r, p := tc.build(t, f)
				f.clock.Set(later)
				p.IncludeOverdue = overdue
				got, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, p)
				if err != nil {
					t.Fatalf("UpdateRule: %v", err)
				}
				wantAppliesFrom(t, got.AppliesFrom, want)
				wantAppliesFrom(t, f.stored(t, r.ID).AppliesFrom, want)
			})
		}
	}
}

func TestUpdateRuleDoesNotAsk(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, f fixture) (Rule, RulePatch)
	}{
		{"template change while the old template is active", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			return f.before(t, false), RulePatch{TemplateID: ptr(f.template(t, false))}
		}},
		{"archiving", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			return f.before(t, false), RulePatch{Archived: ptr(true)}
		}},
		{"same offset", func(t *testing.T, f fixture) (Rule, RulePatch) {
			t.Helper()
			return f.before(t, false), RulePatch{OffsetDays: ptr(1)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			r, p := tc.build(t, f)
			f.clock.Set(start.Add(48 * time.Hour))
			p.IncludeOverdue = true
			got, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, p)
			if err != nil {
				t.Fatalf("UpdateRule: %v", err)
			}
			wantAppliesFrom(t, got.AppliesFrom, &start)
			wantAppliesFrom(t, f.stored(t, r.ID).AppliesFrom, &start)
		})
	}
}

func TestUpdateRuleArchiveStoresClockTime(t *testing.T) {
	f := newFixture(t)
	r := f.before(t, false)
	got, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, RulePatch{Archived: ptr(true)})
	if err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if !got.Archived || !f.stored(t, r.ID).Archived {
		t.Fatal("rule is not archived")
	}
}

func TestUpdateReturnServiceToOwnServiceDoesNotAsk(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	f.clock.Set(start.Add(48 * time.Hour))
	for _, set := range []nullable.Nullable[uuid.UUID]{
		nullable.NewNullableWithValue(r.ServiceID.UUID),
		nullable.NewNullNullable[uuid.UUID](),
	} {
		got, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, RulePatch{ReturnServiceID: set, IncludeOverdue: true})
		if err != nil {
			t.Fatalf("UpdateRule: %v", err)
		}
		wantAppliesFrom(t, got.AppliesFrom, &start)
	}
}

func TestUpdateReturnServiceNullClearsIt(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, nu(f.service(t, false)))
	got, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, RulePatch{ReturnServiceID: nullable.NewNullNullable[uuid.UUID]()})
	if err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if got.ReturnServiceID.Valid || f.stored(t, r.ID).ReturnServiceID.Valid {
		t.Fatalf("return_service_id = %v, want NULL", got.ReturnServiceID)
	}
}

func TestUpdateRuleRejectsArchivedTemplateAndReturnService(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	_, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, RulePatch{
		TemplateID:      ptr(f.template(t, true)),
		ReturnServiceID: nullable.NewNullableWithValue(f.service(t, true)),
		OffsetDays:      ptr(0),
	})
	got := fieldsOf(t, err)
	want := []problem.FieldError{
		{Field: "offset_days", Message: problem.MsgComeBackOffsetInvalid},
		{Field: "template_id", Message: problem.MsgTemplateUnavailable},
		{Field: "return_service_id", Message: problem.MsgServiceUnavailable},
	}
	if len(got) != len(want) {
		t.Fatalf("errors = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("errors = %+v, want %+v", got, want)
		}
	}
}

func TestUpdateBeforeRuleReturnServiceIs422(t *testing.T) {
	f := newFixture(t)
	r := f.before(t, false)
	_, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, RulePatch{ReturnServiceID: nullable.NewNullableWithValue(f.service(t, false))})
	got := fieldsOf(t, err)
	if len(got) != 1 || got[0].Field != "return_service_id" || got[0].Message != problem.MsgServiceNotAllowed {
		t.Fatalf("errors = %+v, want return_service_id not allowed", got)
	}
}

func TestUpdateUnknownRuleIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.UpdateRule(t.Context(), f.pool, newID(t), RulePatch{OffsetDays: ptr(-5)})
	if !errors.As(err, &problem.NotFound{}) {
		t.Fatalf("err = %v, want problem.NotFound before the field check", err)
	}
}

func TestListRulesIncludesArchived(t *testing.T) {
	f := newFixture(t)
	first := f.before(t, false)
	f.clock.Set(start.Add(time.Hour))
	second := f.before(t, false)
	if _, err := f.m.UpdateRule(t.Context(), f.pool, first.ID, RulePatch{Archived: ptr(true)}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	rules, err := f.m.ListRules(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 2 || rules[0].ID != first.ID || !rules[0].Archived || rules[1].ID != second.ID || rules[1].Archived {
		t.Fatalf("rules = %+v, want the archived first rule then the active second one", rules)
	}
}

func TestListRulesIsNotNilWhenEmpty(t *testing.T) {
	f := newFixture(t)
	rules, err := f.m.ListRules(t.Context(), f.pool)
	if err != nil || rules == nil || len(rules) != 0 {
		t.Fatalf("rules = %v, err = %v, want an empty non-nil slice", rules, err)
	}
}

type dbOperation struct {
	name  string
	build func(t *testing.T) (db.DBTX, func(q db.DBTX) error)
}

var dbOperations = []dbOperation{
	{"ListRules", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		f.before(t, false)
		return f.pool, func(q db.DBTX) error { _, err := f.m.ListRules(t.Context(), q); return err }
	}},
	{"CreateRule before", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		in := RuleInput{Kind: KindBefore, OffsetDays: 1, TemplateID: f.template(t, false)}
		return f.pool, func(q db.DBTX) error { _, err := f.m.CreateRule(t.Context(), q, in); return err }
	}},
	{"CreateRule come_back", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		in := RuleInput{
			Kind: KindComeBack, ServiceID: nu(f.service(t, false)), ReturnServiceID: nu(f.service(t, false)),
			OffsetDays: 21, TemplateID: f.template(t, false),
		}
		return f.pool, func(q db.DBTX) error { _, err := f.m.CreateRule(t.Context(), q, in); return err }
	}},
	{"UpdateRule everything", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		r := f.comeBack(t, nu(f.service(t, false)))
		p := RulePatch{
			OffsetDays: ptr(30), TemplateID: ptr(f.template(t, false)),
			ReturnServiceID: nullable.NewNullableWithValue(f.service(t, false)), Archived: ptr(true),
		}
		return f.pool, func(q db.DBTX) error { _, err := f.m.UpdateRule(t.Context(), q, r.ID, p); return err }
	}},
	{"UpdateRule offset only", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		r := f.before(t, false)
		return f.pool, func(q db.DBTX) error {
			_, err := f.m.UpdateRule(t.Context(), q, r.ID, RulePatch{OffsetDays: ptr(2)})
			return err
		}
	}},
}

// TestRulesDBErrors fails each database call of each operation in turn. The caller must get the
// database error back wrapped: never a success, and never a problem that would hide an outage.
func TestRulesDBErrors(t *testing.T) {
	for _, op := range dbOperations {
		t.Run(op.name, func(t *testing.T) { sweepDBErrors(t, op.build) })
	}
}

// sweepDBErrors runs the operation build returns once cleanly to count its database calls, then
// once per call with that call failing (each on fresh rows from build). Every failure must reach
// the caller as the injected error, wrapped.
func sweepDBErrors(t *testing.T, build func(t *testing.T) (db.DBTX, func(q db.DBTX) error)) {
	t.Helper()
	steps := 0
	pool, run := build(t)
	if err := run(dbtest.FailingDB{DBTX: pool, Calls: &steps}); err != nil {
		t.Fatalf("clean run: %v", err)
	}
	if steps == 0 {
		t.Fatal("clean run made no database call")
	}
	for step := 1; step <= steps; step++ {
		calls := 0
		pool, run := build(t)
		err := run(dbtest.FailingDB{DBTX: pool, FailAt: step, Calls: &calls})
		if !errors.Is(err, dbtest.ErrInjected) {
			t.Fatalf("failure at step %d of %d: err = %v; want the database error wrapped", step, steps, err)
		}
	}
}
