package reminders

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/locale"
)

func testLocale(t *testing.T) locale.Locale {
	t.Helper()
	l, err := locale.For("es")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func testZone(t *testing.T) *time.Location {
	t.Helper()
	tz, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatal(err)
	}
	return tz
}

func TestFirstName(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"several words", "  Ana María Rojas ", "Ana"},
		{"one word", "Lucía", "Lucía"},
		{"blank", "   ", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstName(tc.in); got != tc.want {
				t.Fatalf("firstName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBuildDataFixedValues(t *testing.T) {
	a := apptView{
		StartsAt:     time.Date(2026, 10, 24, 13, 0, 0, 0, time.UTC),
		ServiceNames: []string{"Acrílico", "Pedicure", "Cejas"},
	}
	d := buildData(clientView{Name: "Ana Rojas"}, a, nil, testLocale(t), testZone(t), "Estudio Sol")

	got := map[string]string{
		"client.name":         d.Client["name"],
		"client.first_name":   d.Client["first_name"],
		"appointment.date":    d.Appointment["date"],
		"appointment.time":    d.Appointment["time"],
		"appointment.service": d.Appointment["services"],
		"business.name":       d.Business["name"],
	}
	want := map[string]string{
		"client.name":         "Ana Rojas",
		"client.first_name":   "Ana",
		"appointment.date":    "sábado 24 de octubre",
		"appointment.time":    "3:00 p. m.",
		"appointment.service": "Acrílico, Pedicure y Cejas",
		"business.name":       "Estudio Sol",
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
}

func TestBuildDataArchivedAttributeAndOptionUseLastLabel(t *testing.T) {
	attrID, optID := uuid.New(), uuid.New()
	def := attributes.Attribute{
		ID: attrID, Key: "skin", Label: "Piel", Scope: attributes.ScopeClient,
		Type: attributes.TypeSelect, Archived: true,
		Options: []attributes.Option{{ID: optID, AttributeID: attrID, Label: "Seca", Archived: true}},
	}
	raw := json.RawMessage(`{"skin":"` + optID.String() + `"}`)
	d := buildData(clientView{Name: "Ana Rojas", Attrs: raw}, apptView{}, []attributes.Attribute{def}, testLocale(t), testZone(t), "")
	if got := d.Client["skin"]; got != "Seca" {
		t.Fatalf("client.skin = %q, want %q", got, "Seca")
	}
}

func TestBuildDataMissingValueIsEmpty(t *testing.T) {
	def := attributes.Attribute{ID: uuid.New(), Key: "allergy", Label: "Alergia", Scope: attributes.ScopeAppointment, Type: attributes.TypeText}
	d := buildData(clientView{Name: "Ana Rojas"}, apptView{}, []attributes.Attribute{def}, testLocale(t), testZone(t), "")
	v, ok := d.Appointment["allergy"]
	if !ok || v != "" {
		t.Fatalf("appointment.allergy = %q (present %v), want present and empty", v, ok)
	}
}
