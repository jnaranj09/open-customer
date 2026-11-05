package templates

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/locale"
)

func sampleLocale(t *testing.T) locale.Locale {
	t.Helper()
	l, err := locale.For("es")
	if err != nil {
		t.Fatalf("For(es): %v", err)
	}
	return l
}

func madridZone(t *testing.T) *time.Location {
	t.Helper()
	tz, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	return tz
}

// sampleNow is 2026-10-25 00:30 in Madrid, still the 24th in UTC.
var sampleNow = time.Date(2026, 10, 24, 22, 30, 0, 0, time.UTC)

func opt(id string, pos int, parent bool, archived bool) attributes.Option {
	o := attributes.Option{ID: uuid.MustParse(id), Label: id, Position: pos, Archived: archived}
	if parent {
		o.ParentID = uuid.NullUUID{UUID: uuid.MustParse("00000000-0000-0000-0000-0000000000ff"), Valid: true}
	}
	return o
}

const (
	optA = "00000000-0000-0000-0000-00000000000a"
	optB = "00000000-0000-0000-0000-00000000000b"
	optC = "00000000-0000-0000-0000-00000000000c"
)

func compact(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("Compact(%s): %v", raw, err)
	}
	return buf.String()
}

func TestSamplePlaceholdersByType(t *testing.T) {
	rootOpts := []attributes.Option{opt(optB, 2, false, false), opt(optA, 1, false, false), opt(optC, 0, true, false)}
	tests := []struct {
		name string
		attr attributes.Attribute
		want string
	}{
		{"text", attributes.Attribute{Key: "bebida", Label: "Bebida favorita", Type: attributes.TypeText}, `{"bebida":"[Bebida favorita]"}`},
		{"number", attributes.Attribute{Key: "hijos", Type: attributes.TypeNumber}, `{"hijos":1}`},
		{"date", attributes.Attribute{Key: "cumple", Type: attributes.TypeDate}, `{"cumple":"2026-10-25"}`},
		{"select", attributes.Attribute{Key: "color", Type: attributes.TypeSelect, Options: rootOpts}, `{"color":"` + optA + `"}`},
		{"multiselect", attributes.Attribute{Key: "tonos", Type: attributes.TypeMultiselect, Options: rootOpts}, `{"tonos":["` + optA + `"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.attr.Scope = attributes.ScopeClient
			got := Sample(sampleLocale(t), []attributes.Attribute{tt.attr}, sampleNow, madridZone(t))
			if s := compact(t, got.ClientAttrs); s != tt.want {
				t.Errorf("ClientAttrs = %s, want %s", s, tt.want)
			}
		})
	}
}

func TestSampleSplitsAttributesByScope(t *testing.T) {
	attrs := []attributes.Attribute{
		{Key: "a", Label: "A", Scope: attributes.ScopeClient, Type: attributes.TypeText},
		{Key: "b", Label: "B", Scope: attributes.ScopeAppointment, Type: attributes.TypeText},
	}
	got := Sample(sampleLocale(t), attrs, sampleNow, madridZone(t))
	if c, a := compact(t, got.ClientAttrs), compact(t, got.AppointmentAttrs); c != `{"a":"[A]"}` || a != `{"b":"[B]"}` {
		t.Errorf("ClientAttrs = %s, AppointmentAttrs = %s", c, a)
	}
}

func TestSampleSkipsArchivedAttributeAndOption(t *testing.T) {
	attrs := []attributes.Attribute{
		{Key: "old", Label: "Old", Scope: attributes.ScopeClient, Type: attributes.TypeText, Archived: true},
		{Key: "skip", Scope: attributes.ScopeClient, Type: attributes.TypeSelect,
			Options: []attributes.Option{opt(optA, 1, false, true), opt(optB, 2, false, false)}},
		{Key: "none", Scope: attributes.ScopeClient, Type: attributes.TypeSelect,
			Options: []attributes.Option{opt(optA, 1, false, true)}},
	}
	got := Sample(sampleLocale(t), attrs, sampleNow, madridZone(t))
	if s, want := compact(t, got.ClientAttrs), `{"skip":"`+optB+`"}`; s != want {
		t.Errorf("ClientAttrs = %s, want %s", s, want)
	}
}

func TestSampleAppointmentTomorrowAt15Local(t *testing.T) {
	got := Sample(sampleLocale(t), nil, sampleNow, madridZone(t))
	want := time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC)
	if !got.StartsAt.Equal(want) {
		t.Errorf("StartsAt = %s, want %s", got.StartsAt.UTC(), want)
	}
}

func TestSampleNamesComeFromLocale(t *testing.T) {
	got := Sample(sampleLocale(t), nil, sampleNow, madridZone(t))
	if got.ClientName != "Ana Pérez" || !slices.Equal(got.ServiceNames, []string{"Servicio de ejemplo"}) {
		t.Errorf("ClientName = %q, ServiceNames = %q", got.ClientName, got.ServiceNames)
	}
}

func TestSampleEmptyAttributesAreEmptyObjects(t *testing.T) {
	got := Sample(sampleLocale(t), nil, sampleNow, madridZone(t))
	if c, a := compact(t, got.ClientAttrs), compact(t, got.AppointmentAttrs); c != "{}" || a != "{}" {
		t.Errorf("ClientAttrs = %s, AppointmentAttrs = %s, want {}", c, a)
	}
}
