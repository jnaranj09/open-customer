package reminders

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/templates"
)

// clientView is the part of a client that a message template can use.
type clientView struct {
	Name  string
	Attrs json.RawMessage
}

// apptView is the part of an appointment that a message template can use.
type apptView struct {
	StartsAt     time.Time
	ServiceNames []string
	Attrs        json.RawMessage
}

// firstName is the first word of name, or "" when name is blank.
func firstName(name string) string {
	f := strings.Fields(name)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// buildData makes the values a template can name. Attribute values go through attributes.Format,
// so a sample record and a real client are formatted the same way. Archived attributes are
// included. A missing or unreadable value is "": Render applies the fallback.
func buildData(c clientView, a apptView, attrs []attributes.Attribute, loc locale.Locale, tz *time.Location, business string) templates.Data {
	start := a.StartsAt.In(tz)
	client := map[string]string{"name": c.Name, "first_name": firstName(c.Name)}
	appt := map[string]string{
		"date":     loc.Date(start),
		"time":     loc.Time(start),
		"services": loc.JoinList(a.ServiceNames),
	}
	clientRaw, apptRaw := rawValues(c.Attrs), rawValues(a.Attrs)
	for _, def := range attrs {
		switch def.Scope {
		case attributes.ScopeClient:
			client[def.Key] = attributes.Format(def, clientRaw[def.Key], loc, attributes.ForTemplate)
		case attributes.ScopeAppointment:
			appt[def.Key] = attributes.Format(def, apptRaw[def.Key], loc, attributes.ForTemplate)
		}
	}
	return templates.Data{Client: client, Appointment: appt, Business: map[string]string{"name": business}}
}

// rawValues reads the stored attribute object. Anything that is not an object gives no values.
func rawValues(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		slog.Warn("attribute values unreadable", "code", "attr_values_invalid")
		return nil
	}
	return m
}
