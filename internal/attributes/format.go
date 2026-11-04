package attributes

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/locale"
)

// Format renders one stored value for a message template or a CSV cell. Empty or missing values
// give "". A stored value that does not fit its type, or an option that is not in def.Options, is
// left out and logged with a code, never with the value.
func Format(def Attribute, raw json.RawMessage, loc locale.Locale, target Target) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return ""
	}
	out, ok := formatValue(def, trimmed, loc, target)
	if !ok {
		slog.Warn("attribute value unreadable", "code", "attr_value_invalid", "attribute_id", def.ID)
	}
	return out
}

// formatValue returns false when the stored value, or part of it, cannot be read as def's type.
// The string holds whatever was readable.
func formatValue(def Attribute, raw json.RawMessage, loc locale.Locale, target Target) (string, bool) {
	switch def.Type {
	case TypeText:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", false
		}
		return s, true
	case TypeNumber:
		f, ok := parseNumber(raw)
		if !ok {
			return "", false
		}
		return loc.Number(f), true
	case TypeDate:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", false
		}
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			return "", false
		}
		switch target {
		case ForTemplate:
			return loc.DayMonth(d), true
		case ForCSV:
			return d.Format(time.DateOnly), true
		}
		return "", false
	case TypeSelect:
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			return "", false
		}
		return optionPath(def, id, target)
	case TypeMultiselect:
		return formatMultiselect(def, raw, loc, target)
	}
	return "", false
}

func formatMultiselect(def Attribute, raw json.RawMessage, loc locale.Locale, target Target) (string, bool) {
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return "", false
	}
	allOK := true
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		p, ok := optionPath(def, id, target)
		if !ok {
			allOK = false
			continue
		}
		paths = append(paths, p)
	}
	switch target {
	case ForTemplate:
		return loc.JoinList(paths), allOK
	case ForCSV:
		return strings.Join(paths, "; "), allOK
	}
	return "", false
}

// optionPath joins the labels from the root option down to the option with the given ID.
func optionPath(def Attribute, id string, target Target) (string, bool) {
	byID := make(map[uuid.UUID]Option, len(def.Options))
	for _, o := range def.Options {
		byID[o.ID] = o
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return "", false
	}
	cur, ok := byID[u]
	if !ok {
		return "", false
	}
	labels := []string{cur.Label}
	for steps := 0; cur.ParentID.Valid && steps < len(def.Options); steps++ {
		parent, found := byID[cur.ParentID.UUID]
		if !found {
			return "", false
		}
		labels = append(labels, parent.Label)
		cur = parent
	}
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
	switch target {
	case ForTemplate:
		return strings.Join(labels, " "), true
	case ForCSV:
		return strings.Join(labels, " / "), true
	}
	return "", false
}
