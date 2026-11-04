// Package locale formats dates, times, numbers and lists for one language. Release 1 has only
// "es"; this is the one place to add another language. Day and month names live in es.go.
package locale

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	// The binary needs time zone data inside a distroless image, and so do tests.
	_ "time/tzdata"
)

// Locale holds the names and marks of one language.
type Locale struct {
	weekdays       [7]string // indexed by time.Weekday, Sunday first
	months         [12]string
	dayMonthFormat string // %d day, %s month name
	am, pm         string
	decimalMark    string
	listAnd        string
	varLabels      map[string]string // "ns.key" to the label of a fixed template variable
	sampleClient   string            // name of the sample client used in template previews
	sampleServices []string          // service names of the sample appointment
	pushOne        string            // daily push body for one due item
	pushMany       string            // daily push body for n due items, %d the count
}

// For returns the locale for lang. Only "es" exists; any other value is an error.
func For(lang string) (Locale, error) {
	if lang == "es" {
		return spanish, nil
	}
	return Locale{}, fmt.Errorf("locale: unsupported language")
}

// Date formats t as weekday, day and month, for example "sábado 24 de octubre", in t's own zone.
func (l Locale) Date(t time.Time) string {
	return l.weekdays[t.Weekday()] + " " + l.DayMonth(t)
}

// DayMonth formats t as day and month, for example "24 de octubre", in t's own zone.
func (l Locale) DayMonth(t time.Time) string {
	return fmt.Sprintf(l.dayMonthFormat, t.Day(), l.months[t.Month()-1])
}

// Time formats t on a 12-hour clock, for example "3:00 p. m.", in t's own zone.
func (l Locale) Time(t time.Time) string {
	mark := l.am
	if t.Hour() >= 12 {
		mark = l.pm
	}
	hour := t.Hour() % 12
	if hour == 0 {
		hour = 12
	}
	return fmt.Sprintf("%d:%02d %s", hour, t.Minute(), mark)
}

// Number formats f with the locale's decimal mark and no trailing zeros, for example "12,5".
func (l Locale) Number(f float64) string {
	return strings.Replace(strconv.FormatFloat(f, 'f', -1, 64), ".", l.decimalMark, 1)
}

// JoinList joins items as "A", "A y B" or "A, B y C".
func (l Locale) JoinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		last := len(items) - 1
		return strings.Join(items[:last], ", ") + " " + l.listAnd + " " + items[last]
	}
}

// VarLabel returns the label of a fixed template variable such as ("client", "first_name"), or ""
// when the pair is not a fixed variable.
func (l Locale) VarLabel(ns, key string) string { return l.varLabels[ns+"."+key] }

// SampleClientName is the name of the sample client that template previews use.
func (l Locale) SampleClientName() string { return l.sampleClient }

// SampleServiceNames lists the service names of the sample appointment that template previews use.
// The slice is a copy.
func (l Locale) SampleServiceNames() []string { return slices.Clone(l.sampleServices) }

// PushBody is the daily push text for n due items, for example "Tienes 3 mensajes para enviar
// hoy". One item has its own singular text.
func (l Locale) PushBody(n int) string {
	if n == 1 {
		return l.pushOne
	}
	return fmt.Sprintf(l.pushMany, n)
}

// LocalDay returns midnight of t's calendar day in loc.
func LocalDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// AddDays moves day by n calendar days in loc. It never adds 24-hour multiples, so it lands on
// local midnight across a DST change.
func AddDays(day time.Time, n int, loc *time.Location) time.Time {
	y, m, d := day.In(loc).Date()
	return time.Date(y, m, d+n, 0, 0, 0, 0, loc)
}
