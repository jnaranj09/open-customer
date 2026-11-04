package reminders

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
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
