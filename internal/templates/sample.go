package templates

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/locale"
)

// SampleRecord is the built-in client and appointment that template previews use when no real
// client is picked. It is never stored. Attribute values are raw, in the stored JSON shape: the
// caller formats them (D4).
type SampleRecord struct {
	ClientName       string
	ClientAttrs      json.RawMessage
	AppointmentAttrs json.RawMessage
	StartsAt         time.Time
	ServiceNames     []string
}

// Sample builds the preview record. Only active attributes get a value, keyed by Key and split by
// scope: text shows its label in brackets, number 1, date today's local day, select and
// multiselect the first active root option (by position, then ID). A select with no such option
// gets no key. The appointment starts tomorrow at 15:00 in tz. The sample has no phone.
func Sample(loc locale.Locale, attrs []attributes.Attribute, now time.Time, tz *time.Location) SampleRecord {
	client := map[string]any{}
	appointment := map[string]any{}
	today := locale.LocalDay(now, tz)
	for _, a := range attrs {
		if a.Archived {
			continue
		}
		v, ok := sampleValue(a, today)
		if !ok {
			continue
		}
		if a.Scope == attributes.ScopeClient {
			client[a.Key] = v
		} else {
			appointment[a.Key] = v
		}
	}
	y, m, d := today.Date()
	return SampleRecord{
		ClientName:       loc.SampleClientName(),
		ClientAttrs:      marshalSample(client),
		AppointmentAttrs: marshalSample(appointment),
		StartsAt:         time.Date(y, m, d+1, 15, 0, 0, 0, tz),
		ServiceNames:     loc.SampleServiceNames(),
	}
}

func sampleValue(a attributes.Attribute, today time.Time) (any, bool) {
	switch a.Type {
	case attributes.TypeText:
		return "[" + a.Label + "]", true
	case attributes.TypeNumber:
		return 1, true
	case attributes.TypeDate:
		return today.Format(time.DateOnly), true
	case attributes.TypeSelect:
		id, ok := firstRootOption(a.Options)
		return id, ok
	case attributes.TypeMultiselect:
		id, ok := firstRootOption(a.Options)
		return []string{id}, ok
	}
	return nil, false
}

// firstRootOption returns the ID of the active option with no parent that is first by position,
// then by ID.
func firstRootOption(opts []attributes.Option) (string, bool) {
	var roots []attributes.Option
	for _, o := range opts {
		if !o.Archived && !o.ParentID.Valid {
			roots = append(roots, o)
		}
	}
	if len(roots) == 0 {
		return "", false
	}
	first := slices.MinFunc(roots, func(x, y attributes.Option) int {
		return cmp.Or(cmp.Compare(x.Position, y.Position), slices.Compare(x.ID[:], y.ID[:]))
	})
	return first.ID.String(), true
}

// marshalSample encodes a map of strings, numbers and string lists, which cannot fail.
func marshalSample(m map[string]any) json.RawMessage {
	b, err := json.Marshal(m)
	if err != nil {
		panic("templates: sample attributes are not JSON: " + err.Error())
	}
	return b
}
