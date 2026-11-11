package reminders

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/templates"
)

// Preview is a template body rendered for one record. UnknownVars holds "ns.key" once each, in
// body order.
type Preview struct {
	Text        string
	UnknownVars []string
}

// Preview renders body for the client clientID and its latest appointment that is not cancelled or
// no-show, or for the built-in sample when clientID is not set. A client with no such appointment
// gets the sample appointment. An archived client is allowed; an unknown one is a 422 on
// "client_id". The render is lenient: a malformed "{{" shows as text and an unknown variable as
// its fallback, so the owner sees the body while typing. The data goes through buildData, as in
// the due list.
func (m *Module) Preview(ctx context.Context, q db.DBTX, body string, clientID uuid.NullUUID) (Preview, error) {
	queries := sqlc.New(q)
	b, err := loadBusiness(ctx, queries)
	if err != nil {
		return Preview{}, err
	}
	attrs, err := m.attributes.List(ctx, q)
	if err != nil {
		return Preview{}, fmt.Errorf("preview attributes: %w", err)
	}
	sample := templates.Sample(b.Locale, attrs, m.clock.Now(), b.TZ)
	client := clientView{Name: sample.ClientName, Attrs: sample.ClientAttrs}
	appt := apptView{StartsAt: sample.StartsAt, ServiceNames: sample.ServiceNames, Attrs: sample.AppointmentAttrs}
	if clientID.Valid {
		if client, appt, err = previewRecord(ctx, queries, clientID.UUID, appt); err != nil {
			return Preview{}, err
		}
	}
	// The partial Parsed is used on purpose: a malformed token must not hide the variables around it.
	parsed, _ := templates.Parse(body)
	return Preview{
		Text:        templates.Render(body, buildData(client, appt, attrs, b.Locale, b.TZ, b.Name)),
		UnknownVars: unknownNames(templates.UnknownVars(parsed, attrs)),
	}, nil
}

// previewRecord reads the client and its latest active appointment. With no appointment it keeps
// fallback.
func previewRecord(ctx context.Context, queries *sqlc.Queries, id uuid.UUID, fallback apptView) (clientView, apptView, error) {
	c, err := queries.GetPreviewClient(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return clientView{}, apptView{}, problem.Field("client_id", problem.MsgClientUnavailable)
	}
	if err != nil {
		return clientView{}, apptView{}, fmt.Errorf("get preview client: %w", err)
	}
	client := clientView{Name: c.Name, Attrs: c.Attrs}
	a, err := queries.GetLatestAppointmentForPreview(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return client, fallback, nil
	}
	if err != nil {
		return clientView{}, apptView{}, fmt.Errorf("get preview appointment: %w", err)
	}
	lines, err := linesByAppointment(ctx, queries, []uuid.UUID{a.ID})
	if err != nil {
		return clientView{}, apptView{}, err
	}
	return client, apptView{StartsAt: a.StartsAt, ServiceNames: lines[a.ID].names, Attrs: a.Attrs}, nil
}

// unknownNames is the "ns.key" of each reference, once each, in order.
func unknownNames(refs []templates.VarRef) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, r := range refs {
		name := r.NS + "." + r.Key
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// RuleDraft is a come-back rule as the rule form holds it. RuleID is the saved rule it edits; none
// for a new rule. No ServiceID makes a general rule.
type RuleDraft struct {
	RuleID          uuid.NullUUID
	ServiceID       uuid.NullUUID
	ReturnServiceID uuid.NullUUID
	OffsetDays      int
	TemplateID      uuid.UUID
}

// RulePreviewResult is what a draft does (reminders-and-replies design, "Look-ahead pass"):
// TodayCount clients have a reminder of it due exactly today; Next is its earliest reminder after
// today, or nil.
type RulePreviewResult struct {
	TodayCount int
	Next       *NextClient
}

// NextClient is the client of a draft's next reminder and its due day (business-local midnight).
type NextClient struct {
	ClientName string
	DueDate    time.Time
}

// RulePreview computes the due list and the look-ahead with draft in place of the saved rule (or
// added, for a new rule), and writes nothing. The draft gets the checks of a new come-back rule; an
// unknown RuleID is problem.NotFound and a "before" one a 422 on "rule_id". The draft has no
// applies_from, so late items count the same whatever the include-overdue answer; they are not in
// TodayCount anyway.
func (m *Module) RulePreview(ctx context.Context, q db.DBTX, now time.Time, draft RuleDraft) (RulePreviewResult, error) {
	queries := sqlc.New(q)
	id := uuid.Max // a new rule's id: later than every saved one, as a new UUIDv7 is
	if draft.RuleID.Valid {
		kind, err := queries.GetRuleKind(ctx, draft.RuleID.UUID)
		if errors.Is(err, pgx.ErrNoRows) {
			return RulePreviewResult{}, problem.NotFound{}
		}
		if err != nil {
			return RulePreviewResult{}, fmt.Errorf("get rule kind: %w", err)
		}
		if Kind(kind) != KindComeBack {
			return RulePreviewResult{}, problem.Field("rule_id", problem.MsgRuleNotComeBack)
		}
		id = draft.RuleID.UUID
	}
	offset, err := checkRuleInput(ctx, queries, RuleInput{
		Kind: KindComeBack, ServiceID: draft.ServiceID, ReturnServiceID: draft.ReturnServiceID,
		OffsetDays: draft.OffsetDays, TemplateID: draft.TemplateID,
	})
	if err != nil {
		return RulePreviewResult{}, err
	}
	d, err := m.loadDue(ctx, q, now, loadOpts{extraComeBack: int(offset)})
	if err != nil {
		return RulePreviewResult{}, err
	}
	row := ruleRow{ID: id, Kind: KindComeBack, ServiceID: draft.ServiceID, OffsetDays: int(offset)}
	if draft.ServiceID.Valid {
		row.ReturnServiceID = cmp.Or(draft.ReturnServiceID.UUID, draft.ServiceID.UUID)
	}
	d.Rules = slices.DeleteFunc(d.Rules, func(r ruleRow) bool { return r.ID == id })
	d.Rules = append(d.Rules, row)

	var out RulePreviewResult
	today := locale.LocalDay(now, d.TZ)
	clients := map[uuid.UUID]bool{}
	for _, it := range compute(d, now) {
		if it.RuleID == id && it.DueDate.Equal(today) && !clients[it.ClientID] {
			clients[it.ClientID] = true
			out.TodayCount++
		}
	}
	for _, it := range upcoming(d, now, locale.AddDays(today, maxOffsetDays, d.TZ)) {
		if it.RuleID == id {
			out.Next = &NextClient{ClientName: it.ClientName, DueDate: it.DueDate}
			break
		}
	}
	return out, nil
}
