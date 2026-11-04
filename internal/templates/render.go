package templates

import "strings"

// Data is the values a body renders with, one map per namespace. A nil map has no values.
type Data struct{ Client, Appointment, Business map[string]string }

// Render fills a body with data. It never fails: a variable with no value (missing key, empty
// value or unknown namespace) becomes its fallback, or "". A "{{" that starts no valid variable
// is copied as text. Values are not scanned again.
func Render(body string, data Data) string {
	var b strings.Builder
	for _, s := range scan(body) {
		switch {
		case s.ref != nil:
			b.WriteString(value(*s.ref, data))
		case s.bad > 0:
			b.WriteString("{{")
		default:
			b.WriteString(s.lit)
		}
	}
	return b.String()
}

func value(ref VarRef, data Data) string {
	var m map[string]string
	switch ref.NS {
	case "client":
		m = data.Client
	case "appointment":
		m = data.Appointment
	case "business":
		m = data.Business
	}
	if v := m[ref.Key]; v != "" {
		return v
	}
	if ref.Fallback != nil {
		return *ref.Fallback
	}
	return ""
}
