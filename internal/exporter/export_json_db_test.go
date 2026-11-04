//go:build db

package exporter_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/templates"
)

type withID struct {
	ID uuid.UUID `json:"id"`
}

type jsonAttribute struct {
	ID       uuid.UUID `json:"id"`
	Archived bool      `json:"archived"`
	Options  []withID  `json:"options"`
}

// datos is the part of datos.json the tests read.
type datos struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	Settings   struct {
		Currency string `json:"currency"`
		Timezone string `json:"timezone"`
	} `json:"settings"`
	Attributes   []jsonAttribute `json:"attributes"`
	Services     []withID        `json:"services"`
	Templates    []withID        `json:"templates"`
	Rules        []withID        `json:"rules"`
	Clients      []withID        `json:"clients"`
	Appointments []struct {
		ID       uuid.UUID `json:"id"`
		Done     bool      `json:"done"`
		Services []struct {
			ServiceID uuid.UUID `json:"service_id"`
		} `json:"services"`
	} `json:"appointments"`
	ReminderLog []struct {
		ID         uuid.UUID `json:"id"`
		ClientID   uuid.UUID `json:"client_id"`
		ClientName string    `json:"client_name"`
		Kind       string    `json:"kind"`
	} `json:"reminder_log"`
}

func (e env) datos(t *testing.T) datos {
	t.Helper()
	_, files := zipFiles(t, e.export(t))
	var d datos
	if err := json.Unmarshal(files["datos.json"], &d); err != nil {
		t.Fatalf("parse datos.json: %v", err)
	}
	return d
}

func idsOf(items []withID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestExportJSONRoundTripsIDs(t *testing.T) {
	e := newEnv(t, "USD")
	pedicure := e.service(t, "Pedicure", 30, nil)
	tpl, err := e.templates.Create(t.Context(), e.pool, templates.TemplateInput{Name: "Recordatorio", Body: "Hola {{client.first_name}}"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	lucia := e.client(t, "Lucía Rojas", nil, nil)
	ana := e.client(t, "Ana Rojas", nil, nil)
	starts := time.Date(2026, 10, 24, 13, 0, 0, 0, time.UTC)
	visit := e.book(t, lucia.ID, starts, pedicure)
	later := e.book(t, ana.ID, starts.Add(48*time.Hour), pedicure)
	ruleID := e.rule(t, tpl.ID)
	logID := e.logRow(t, ruleID, visit.ID, starts)
	e.clock.Set(time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))

	d := e.datos(t)
	if d.Version != 1 || !d.ExportedAt.Equal(e.clock.Now()) {
		t.Errorf("version %d, exported_at %s; want 1 and %s", d.Version, d.ExportedAt, e.clock.Now())
	}
	if d.Settings.Currency != "USD" || d.Settings.Timezone != "Europe/Madrid" {
		t.Errorf("settings = %+v", d.Settings)
	}
	gotClients := idsOf(d.Clients)
	if !sameIDs(gotClients, []uuid.UUID{lucia.ID, ana.ID}) {
		t.Errorf("client ids = %v, want %v and %v", gotClients, lucia.ID, ana.ID)
	}
	if got := idsOf(d.Services); !slices.Equal(got, []uuid.UUID{pedicure}) {
		t.Errorf("service ids = %v, want %v", got, pedicure)
	}
	if got := idsOf(d.Templates); !slices.Equal(got, []uuid.UUID{tpl.ID}) {
		t.Errorf("template ids = %v, want %v", got, tpl.ID)
	}
	if got := idsOf(d.Rules); !slices.Equal(got, []uuid.UUID{ruleID}) {
		t.Errorf("rule ids = %v, want %v", got, ruleID)
	}
	if len(d.Appointments) != 2 || !sameIDs([]uuid.UUID{d.Appointments[0].ID, d.Appointments[1].ID}, []uuid.UUID{visit.ID, later.ID}) {
		t.Fatalf("appointments = %+v, want %v and %v", d.Appointments, visit.ID, later.ID)
	}
	for _, a := range d.Appointments {
		wantDone := a.ID == visit.ID
		if a.Done != wantDone || len(a.Services) != 1 || a.Services[0].ServiceID != pedicure {
			t.Errorf("appointment %s = %+v; want done %v and one Pedicure line", a.ID, a, wantDone)
		}
	}
	if len(d.ReminderLog) != 1 {
		t.Fatalf("reminder_log = %+v, want one row", d.ReminderLog)
	}
	l := d.ReminderLog[0]
	if l.ID != logID || l.ClientID != lucia.ID || l.ClientName != "Lucía Rojas" || l.Kind != "before_appointment" {
		t.Errorf("reminder_log[0] = %+v; want %s for Lucía Rojas, before_appointment", l, logID)
	}
}

// sameIDs reports whether got holds exactly want, in any order (the export orders by id).
func sameIDs(got, want []uuid.UUID) bool {
	a, b := slices.Clone(got), slices.Clone(want)
	byBytes := func(x, y uuid.UUID) int { return slices.Compare(x[:], y[:]) }
	slices.SortFunc(a, byBytes)
	slices.SortFunc(b, byBytes)
	return slices.Equal(a, b)
}

func TestExportArchivedAttributeOnlyInJSON(t *testing.T) {
	e := newEnv(t, "USD")
	e.attribute(t, "Alergias", attributes.ScopeClient, attributes.TypeText)
	old := e.attribute(t, "Tono antiguo", attributes.ScopeClient, attributes.TypeSelect)
	optionID := e.option(t, old.ID, "Rosa")
	e.archiveAttribute(t, old.ID)

	rows := e.csvFile(t, "clientas.csv")
	if slices.Contains(rows[0], "Tono antiguo") {
		t.Errorf("CSV header %q has the archived attribute", rows[0])
	}
	d := e.datos(t)
	i := slices.IndexFunc(d.Attributes, func(a jsonAttribute) bool { return a.ID == old.ID })
	if i < 0 {
		t.Fatalf("datos.json attributes %+v miss the archived one", d.Attributes)
	}
	got := d.Attributes[i]
	if !got.Archived || len(got.Options) != 1 || got.Options[0].ID != optionID {
		t.Fatalf("archived attribute in JSON = %+v; want archived with its option %s", got, optionID)
	}
}
