package exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/settings"
)

// The datos.json element types. Each one has the JSON keys of the API schema of the same name
// (design doc item 15), because the exporter may not import the generated API types.
// httpapi's TestExportJSONMatchesSchemas compares the json tag names and omitempty flags of each
// type with its generated twin, so a key change here or in the spec fails that test.

// Settings is the API Settings schema.
type Settings struct {
	Name          string `json:"name"`
	Timezone      string `json:"timezone"`
	Locale        string `json:"locale"`
	Currency      string `json:"currency"`
	CountryCode   string `json:"country_code"`
	AccentColor   string `json:"accent_color"`
	PushHour      int    `json:"push_hour"`
	HasCustomIcon bool   `json:"has_custom_icon"`
}

// Attribute is the API Attribute schema, with every option.
type Attribute struct {
	ID       uuid.UUID `json:"id"`
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Scope    string    `json:"scope"`
	Type     string    `json:"type"`
	Required bool      `json:"required"`
	Position int       `json:"position"`
	Archived bool      `json:"archived"`
	Options  []Option  `json:"options"`
}

// Option is the API Option schema.
type Option struct {
	ID          uuid.UUID     `json:"id"`
	AttributeID uuid.UUID     `json:"attribute_id"`
	ParentID    uuid.NullUUID `json:"parent_id"`
	Label       string        `json:"label"`
	Position    int           `json:"position"`
	Archived    bool          `json:"archived"`
}

// Service is the API Service schema.
type Service struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	DurationMin int       `json:"duration_min"`
	PriceCents  *int64    `json:"price_cents"`
	Color       string    `json:"color"`
	Position    int       `json:"position"`
	Archived    bool      `json:"archived"`
}

// Template is the API Template schema.
type Template struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Body     string    `json:"body"`
	Archived bool      `json:"archived"`
}

// Rule is the API Rule schema.
type Rule struct {
	ID              uuid.UUID     `json:"id"`
	Kind            string        `json:"kind"`
	ServiceID       uuid.NullUUID `json:"service_id"`
	ReturnServiceID uuid.NullUUID `json:"return_service_id"`
	OffsetDays      int           `json:"offset_days"`
	AppliesFrom     *time.Time    `json:"applies_from"`
	TemplateID      uuid.UUID     `json:"template_id"`
	Archived        bool          `json:"archived"`
	HandoverFrom    *time.Time    `json:"handover_from"`
}

// Client is the API Client schema.
type Client struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	PhoneE164   *string         `json:"phone_e164"`
	PhoneDigits *string         `json:"phone_digits"`
	Notes       *string         `json:"notes"`
	Attrs       json.RawMessage `json:"attrs"`
	Archived    bool            `json:"archived"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Appointment is the API Appointment schema; Done is computed with appointments.IsDone.
type Appointment struct {
	ID         uuid.UUID            `json:"id"`
	ClientID   uuid.UUID            `json:"client_id"`
	ClientName string               `json:"client_name"`
	StartsAt   time.Time            `json:"starts_at"`
	EndsAt     time.Time            `json:"ends_at"`
	Status     string               `json:"status"`
	Done       bool                 `json:"done"`
	TotalCents int64                `json:"total_cents"`
	PaidCents  *int64               `json:"paid_cents"`
	Attrs      json.RawMessage      `json:"attrs"`
	Notes      *string              `json:"notes"`
	Services   []AppointmentService `json:"services"`
	CreatedAt  time.Time            `json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
}

// AppointmentService is the API AppointmentService schema: one copied service line.
type AppointmentService struct {
	ServiceID   uuid.UUID `json:"service_id"`
	Name        string    `json:"name"`
	PriceCents  *int64    `json:"price_cents"`
	DurationMin int       `json:"duration_min"`
}

// LogItem is the API LogItem schema: one reminder_log row with its rule kind and client.
type LogItem struct {
	ID            uuid.UUID  `json:"id"`
	RuleID        uuid.UUID  `json:"rule_id"`
	AppointmentID uuid.UUID  `json:"appointment_id"`
	AnchorAt      *time.Time `json:"anchor_at,omitempty"`
	Action        string     `json:"action"`
	Body          *string    `json:"body,omitempty"`
	PhoneDigits   *string    `json:"phone_digits,omitempty"`
	ActedAt       time.Time  `json:"acted_at"`
	Kind          string     `json:"kind"`
	ClientID      uuid.UUID  `json:"client_id"`
	ClientName    string     `json:"client_name"`
}

// jsonFile is the name of the JSON entry. It is not Spanish, so it is not in locale.
const jsonFile = "datos.json"

// jsonVersion is the datos.json format version. 2: services gained description. 3: rules gained
// handover_from.
const jsonVersion = 3

// jsonStream writes datos.json piece by piece, so each array is written one page at a time.
type jsonStream struct{ w io.Writer }

func (s jsonStream) raw(text string) error {
	if _, err := io.WriteString(s.w, text); err != nil {
		return fmt.Errorf("write %s: %w", jsonFile, err)
	}
	return nil
}

func (s jsonStream) value(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode %s: %w", jsonFile, err)
	}
	if _, err := s.w.Write(b); err != nil {
		return fmt.Errorf("write %s: %w", jsonFile, err)
	}
	return nil
}

// field writes `,"name":` and then v. The names are fixed keys that need no escaping.
func (s jsonStream) field(name string, v any) error {
	if err := s.raw(`,"` + name + `":`); err != nil {
		return err
	}
	return s.value(v)
}

// pagedField writes `,"name":[...]`, one page of rows at a time. convert turns one page into its
// array elements.
func pagedField[R any](s jsonStream, name string, load func(after uuid.UUID) ([]R, error), id func(R) uuid.UUID, convert func([]R) ([]any, error)) error {
	if err := s.raw(`,"` + name + `":[`); err != nil {
		return err
	}
	first := true
	err := eachPage(load, id, func(page []R) error {
		elems, err := convert(page)
		if err != nil {
			return err
		}
		for _, v := range elems {
			if !first {
				if err := s.raw(","); err != nil {
					return err
				}
			}
			first = false
			if err := s.value(v); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return s.raw("]")
}

// each converts a page row by row.
func each[R, J any](f func(R) J) func([]R) ([]any, error) {
	return func(page []R) ([]any, error) {
		out := make([]any, 0, len(page))
		for _, r := range page {
			out = append(out, f(r))
		}
		return out, nil
	}
}

// writeJSON writes datos.json: the settings, every attribute, then each table a page at a time.
func (x *run) writeJSON(w io.Writer) error {
	s := jsonStream{w: w}
	if err := s.raw(`{"version":`); err != nil {
		return err
	}
	if err := s.value(jsonVersion); err != nil {
		return err
	}
	if err := s.field("exported_at", x.now.UTC()); err != nil {
		return err
	}
	if err := s.field("settings", settingsJSON(x.settings)); err != nil {
		return err
	}
	attrs := make([]Attribute, 0, len(x.attrs))
	for _, a := range x.attrs {
		attrs = append(attrs, attributeJSON(a))
	}
	if err := s.field("attributes", attrs); err != nil {
		return err
	}
	if err := pagedField(s, "services", x.services, serviceID, each(serviceJSON)); err != nil {
		return err
	}
	if err := pagedField(s, "templates", x.templates, func(r sqlc.ExportTemplatesRow) uuid.UUID { return r.ID }, each(templateJSON)); err != nil {
		return err
	}
	if err := pagedField(s, "rules", x.rules, func(r sqlc.ExportRulesRow) uuid.UUID { return r.ID }, each(ruleJSON)); err != nil {
		return err
	}
	if err := pagedField(s, "clients", x.clients, clientID, each(clientJSON)); err != nil {
		return err
	}
	if err := pagedField(s, "appointments", x.appointments, appointmentID, x.appointmentsJSON); err != nil {
		return err
	}
	if err := pagedField(s, "reminder_log", x.reminderLog, func(r sqlc.ExportLogRow) uuid.UUID { return r.ID }, each(logJSON)); err != nil {
		return err
	}
	return s.raw("}")
}

func (x *run) appointmentsJSON(page []sqlc.ExportAppointmentsRow) ([]any, error) {
	lines, err := x.lines(page)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(page))
	for _, r := range page {
		out = append(out, appointmentJSON(r, lines[r.ID], x.now))
	}
	return out, nil
}

func settingsJSON(st settings.Settings) Settings {
	return Settings{
		Name: st.Name, Timezone: st.Timezone, Locale: st.Locale, Currency: st.Currency,
		CountryCode: st.CountryCode, AccentColor: st.AccentColor, PushHour: st.PushHour,
		HasCustomIcon: st.HasCustomIcon,
	}
}

func attributeJSON(a attributes.Attribute) Attribute {
	opts := make([]Option, 0, len(a.Options))
	for _, o := range a.Options {
		opts = append(opts, Option{
			ID: o.ID, AttributeID: o.AttributeID, ParentID: o.ParentID, Label: o.Label,
			Position: o.Position, Archived: o.Archived,
		})
	}
	return Attribute{
		ID: a.ID, Key: a.Key, Label: a.Label, Scope: string(a.Scope), Type: string(a.Type),
		Required: a.Required, Position: a.Position, Archived: a.Archived, Options: opts,
	}
}

func serviceJSON(r sqlc.Service) Service {
	return Service{
		ID: r.ID, Name: r.Name, Description: r.Description, DurationMin: int(r.DurationMin),
		PriceCents: r.PriceCents, Color: r.Color, Position: int(r.Position), Archived: r.ArchivedAt != nil,
	}
}

func templateJSON(r sqlc.ExportTemplatesRow) Template {
	return Template{ID: r.ID, Name: r.Name, Body: r.Body, Archived: r.ArchivedAt != nil}
}

func ruleJSON(r sqlc.ExportRulesRow) Rule {
	return Rule{
		ID: r.ID, Kind: r.Kind, ServiceID: r.ServiceID, ReturnServiceID: r.ReturnServiceID,
		OffsetDays: int(r.OffsetDays), AppliesFrom: utcPtr(r.AppliesFrom), TemplateID: r.TemplateID,
		Archived: r.ArchivedAt != nil, HandoverFrom: utcPtr(r.HandoverFrom),
	}
}

func clientJSON(r sqlc.Client) Client {
	return Client{
		ID: r.ID, Name: r.Name, PhoneE164: r.PhoneE164, PhoneDigits: r.PhoneDigits, Notes: r.Notes,
		Attrs: r.Attrs, Archived: r.ArchivedAt != nil, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

func appointmentJSON(r sqlc.ExportAppointmentsRow, lines []sqlc.ExportAppointmentLinesRow, now time.Time) Appointment {
	svcs := make([]AppointmentService, 0, len(lines))
	for _, l := range lines {
		svcs = append(svcs, AppointmentService{
			ServiceID: l.ServiceID, Name: l.Name, PriceCents: l.PriceCents, DurationMin: int(l.DurationMin),
		})
	}
	return Appointment{
		ID: r.ID, ClientID: r.ClientID, ClientName: r.ClientName,
		StartsAt: r.StartsAt.UTC(), EndsAt: r.EndsAt.UTC(), Status: r.Status,
		Done:       appointments.IsDone(appointments.Status(r.Status), r.EndsAt, now),
		TotalCents: r.TotalCents, PaidCents: r.PaidCents, Attrs: r.Attrs, Notes: r.Notes, Services: svcs,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

func logJSON(r sqlc.ExportLogRow) LogItem {
	return LogItem{
		ID: r.ID, RuleID: r.RuleID, AppointmentID: r.AppointmentID, AnchorAt: utcPtr(r.AnchorAt),
		Action: r.Action, Body: r.Body, PhoneDigits: r.PhoneDigits, ActedAt: r.ActedAt.UTC(),
		Kind: r.Kind, ClientID: r.ClientID, ClientName: r.ClientName,
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
