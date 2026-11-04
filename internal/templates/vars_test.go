package templates

import (
	"testing"

	"github.com/jnaranj09/open-customer/internal/attributes"
)

func TestKnownAcceptsFixedVariables(t *testing.T) {
	for name, c := range map[string]struct{ ns, key string }{
		"client name":          {"client", "name"},
		"client first name":    {"client", "first_name"},
		"appointment date":     {"appointment", "date"},
		"appointment time":     {"appointment", "time"},
		"appointment services": {"appointment", "services"},
		"business name":        {"business", "name"},
	} {
		t.Run(name, func(t *testing.T) {
			if !known(c.ns, c.key, nil) {
				t.Errorf("known(%q, %q) = false, want true", c.ns, c.key)
			}
		})
	}
}

func TestKnownRejectsUnknownVariables(t *testing.T) {
	attrs := []attributes.Attribute{{Key: "bebida", Scope: attributes.ScopeClient}}
	for name, c := range map[string]struct{ ns, key string }{
		"fixed key in the wrong namespace":    {"business", "first_name"},
		"unknown key":                         {"client", "nope"},
		"unknown namespace":                   {"other", "name"},
		"client attribute under appointment":  {"appointment", "bebida"},
		"client attribute under business":     {"business", "bebida"},
		"attribute key with a namespace typo": {"clients", "bebida"},
	} {
		t.Run(name, func(t *testing.T) {
			if known(c.ns, c.key, attrs) {
				t.Errorf("known(%q, %q) = true, want false", c.ns, c.key)
			}
		})
	}
}

func TestKnownAcceptsAttributesOfTheMatchingScopeArchivedIncluded(t *testing.T) {
	attrs := []attributes.Attribute{
		{Key: "bebida", Scope: attributes.ScopeClient},
		{Key: "zona", Scope: attributes.ScopeClient, Archived: true},
		{Key: "sala", Scope: attributes.ScopeAppointment, Archived: true},
	}
	for name, c := range map[string]struct{ ns, key string }{
		"active client attribute":        {"client", "bebida"},
		"archived client attribute":      {"client", "zona"},
		"archived appointment attribute": {"appointment", "sala"},
	} {
		t.Run(name, func(t *testing.T) {
			if !known(c.ns, c.key, attrs) {
				t.Errorf("known(%q, %q) = false, want true", c.ns, c.key)
			}
		})
	}
}
