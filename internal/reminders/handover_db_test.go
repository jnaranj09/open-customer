//go:build db

package reminders

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// General rules and the handover (reminders-and-replies design, "Handover", item 25).

func (f fixture) general(t *testing.T) Rule {
	t.Helper()
	return f.createRule(t, RuleInput{Kind: KindComeBack, OffsetDays: 30, TemplateID: f.template(t, false)})
}

func (f fixture) update(t *testing.T, id uuid.UUID, p RulePatch) Rule {
	t.Helper()
	r, err := f.m.UpdateRule(t.Context(), f.pool, id, p)
	if err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	return r
}

func wantHandover(t *testing.T, got *time.Time, want *time.Time) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Fatalf("handover_from = %v, want NULL", got)
	case want != nil && (got == nil || !got.Equal(*want)):
		t.Fatalf("handover_from = %v, want %v", got, want)
	}
}

func TestCreateGeneralRule(t *testing.T) {
	f := newFixture(t)
	r := f.general(t)
	stored := f.stored(t, r.ID)
	if stored.Kind != KindComeBack || stored.ServiceID.Valid || stored.ReturnServiceID.Valid {
		t.Fatalf("rule = %+v, want a come_back rule with no service", stored)
	}
}

func TestUpdateGeneralRuleRefusesReturnService(t *testing.T) {
	f := newFixture(t)
	r := f.general(t)
	_, err := f.m.UpdateRule(t.Context(), f.pool, r.ID, RulePatch{ReturnServiceID: nullable.NewNullableWithValue(f.service(t, false))})
	got := fieldsOf(t, err)
	if len(got) != 1 || got[0].Field != "return_service_id" || got[0].Message != problem.MsgGeneralNoReturn {
		t.Fatalf("errors = %+v, want one on return_service_id with %q", got, problem.MsgGeneralNoReturn)
	}
}

func TestArchiveRuleFromTodayWritesNow(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	got := f.update(t, r.ID, RulePatch{Archived: ptr(true)})
	wantHandover(t, got.HandoverFrom, &start)
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, &start)
}

func TestArchiveRuleIncludeLateWritesNull(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	f.update(t, r.ID, RulePatch{Archived: ptr(true), IncludeOverdue: true})
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, nil)
}

func TestArchiveRuleThatCannotSendWritesNothing(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	f.archive(t, "message_templates", r.TemplateID)
	f.update(t, r.ID, RulePatch{Archived: ptr(true)})
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, nil)
}

func TestArchiveRuleThenTemplateKeepsTheRuleAnswer(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	f.update(t, r.ID, RulePatch{Archived: ptr(true)})
	f.clock.Advance(time.Hour)
	if err := f.m.SetHandover(t.Context(), f.pool, HandoverScope{TemplateID: nu(r.TemplateID)}, true); err != nil {
		t.Fatalf("SetHandover: %v", err)
	}
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, &start)
}

func TestRestoreRuleClearsHandover(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	f.update(t, r.ID, RulePatch{Archived: ptr(true)})
	got := f.update(t, r.ID, RulePatch{Archived: ptr(false)})
	wantHandover(t, got.HandoverFrom, nil)
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, nil)
}

func TestRestoreRuleWhileTemplateArchivedKeepsHandover(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	f.update(t, r.ID, RulePatch{Archived: ptr(true)})
	f.archive(t, "message_templates", r.TemplateID)
	f.update(t, r.ID, RulePatch{Archived: ptr(false)})
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, &start)
}

func TestSetHandoverByTemplateAndService(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope func(r Rule) HandoverScope
	}{
		{"template", func(r Rule) HandoverScope { return HandoverScope{TemplateID: nu(r.TemplateID)} }},
		{"service", func(r Rule) HandoverScope { return HandoverScope{ServiceID: r.ServiceID} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			r := f.comeBack(t, uuid.NullUUID{})
			other := f.comeBack(t, uuid.NullUUID{})
			if err := f.m.SetHandover(t.Context(), f.pool, tc.scope(r), false); err != nil {
				t.Fatalf("SetHandover: %v", err)
			}
			wantHandover(t, f.stored(t, r.ID).HandoverFrom, &start)
			wantHandover(t, f.stored(t, other.ID).HandoverFrom, nil)
		})
	}
}

func TestSetHandoverByReturnService(t *testing.T) {
	f := newFixture(t)
	ret := f.service(t, false)
	r := f.comeBack(t, nu(ret))
	if err := f.m.SetHandover(t.Context(), f.pool, HandoverScope{ServiceID: nu(ret)}, false); err != nil {
		t.Fatalf("SetHandover: %v", err)
	}
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, &start)
}

func TestSetHandoverOnArchivedTemplateWritesNothing(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	f.archive(t, "message_templates", r.TemplateID)
	if err := f.m.SetHandover(t.Context(), f.pool, HandoverScope{TemplateID: nu(r.TemplateID)}, false); err != nil {
		t.Fatalf("SetHandover: %v", err)
	}
	wantHandover(t, f.stored(t, r.ID).HandoverFrom, nil)
}

func TestRestartRulesByTemplate(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	if err := f.m.SetHandover(t.Context(), f.pool, HandoverScope{TemplateID: nu(r.TemplateID)}, false); err != nil {
		t.Fatalf("SetHandover: %v", err)
	}
	f.archive(t, "message_templates", r.TemplateID)
	f.clock.Advance(time.Hour)
	if err := f.m.RestartRules(t.Context(), f.pool, HandoverScope{TemplateID: nu(r.TemplateID)}, true); err != nil {
		t.Fatalf("RestartRules: %v", err)
	}
	got := f.stored(t, r.ID)
	wantHandover(t, got.HandoverFrom, nil)
	wantAppliesFrom(t, got.AppliesFrom, nil)
}

func TestRestartRulesByServiceSkipsRulesStillStopped(t *testing.T) {
	f := newFixture(t)
	r := f.comeBack(t, uuid.NullUUID{})
	stillStopped := f.createRule(t, RuleInput{
		Kind: KindComeBack, ServiceID: r.ServiceID, OffsetDays: 21, TemplateID: f.template(t, false),
	})
	f.archive(t, "services", r.ServiceID.UUID)
	f.archive(t, "message_templates", stillStopped.TemplateID)
	later := start.Add(time.Hour)
	f.clock.Set(later)
	if err := f.m.RestartRules(t.Context(), f.pool, HandoverScope{ServiceID: r.ServiceID}, false); err != nil {
		t.Fatalf("RestartRules: %v", err)
	}
	wantAppliesFrom(t, f.stored(t, r.ID).AppliesFrom, &later)
	wantAppliesFrom(t, f.stored(t, stillStopped.ID).AppliesFrom, &start)
}
