package attributes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/locale"
)

func esLocale(t *testing.T) locale.Locale {
	t.Helper()
	loc, err := locale.For("es")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func treeOptions() (Attribute, [3]uuid.UUID) {
	ids := [3]uuid.UUID{
		uuid.MustParse("00000000-0000-7000-8000-000000000001"),
		uuid.MustParse("00000000-0000-7000-8000-000000000002"),
		uuid.MustParse("00000000-0000-7000-8000-000000000003"),
	}
	return Attribute{
		ID: uuid.MustParse("00000000-0000-7000-8000-0000000000c1"), Type: TypeSelect,
		Options: []Option{
			{ID: ids[0], Label: "Almendra", Archived: true},
			{ID: ids[1], Label: "Corta", ParentID: uuid.NullUUID{UUID: ids[0], Valid: true}},
			{ID: ids[2], Label: "Brillante", ParentID: uuid.NullUUID{UUID: ids[1], Valid: true}},
		},
	}, ids
}

func TestFormat(t *testing.T) {
	loc := esLocale(t)
	tree, ids := treeOptions()
	multi := Attribute{Type: TypeMultiselect, Options: []Option{
		{ID: ids[0], Label: "A"}, {ID: ids[1], Label: "B"}, {ID: ids[2], Label: "C"},
	}}
	tests := []struct {
		name         string
		def          Attribute
		raw          string
		wantTemplate string
		wantCSV      string
	}{
		{"text", Attribute{Type: TypeText}, `"Café negro"`, "Café negro", "Café negro"},
		{"number", Attribute{Type: TypeNumber}, `12.5`, "12,5", "12,5"},
		{"date", Attribute{Type: TypeDate}, `"2026-10-24"`, "24 de octubre", "2026-10-24"},
		{"three level select with archived parent", tree, fmt.Sprintf("%q", ids[2]), "Almendra Corta Brillante", "Almendra / Corta / Brillante"},
		{"multiselect of three", multi, fmt.Sprintf(`[%q,%q,%q]`, ids[0], ids[1], ids[2]), "A, B y C", "A; B; C"},
		{"empty raw", Attribute{Type: TypeText}, ``, "", ""},
		{"null raw", Attribute{Type: TypeText}, `null`, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.def, json.RawMessage(tt.raw), loc, ForTemplate); got != tt.wantTemplate {
				t.Errorf("template = %q, want %q", got, tt.wantTemplate)
			}
			if got := Format(tt.def, json.RawMessage(tt.raw), loc, ForCSV); got != tt.wantCSV {
				t.Errorf("csv = %q, want %q", got, tt.wantCSV)
			}
		})
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf
}

func TestFormatBadStoredValueLogsCode(t *testing.T) {
	logs := captureLog(t)
	def := Attribute{ID: uuid.MustParse("00000000-0000-7000-8000-0000000000d1"), Type: TypeText}
	if got := Format(def, json.RawMessage(`{"x":1}`), esLocale(t), ForTemplate); got != "" {
		t.Errorf("Format = %q, want empty", got)
	}
	out := logs.String()
	if !strings.Contains(out, "code=attr_value_invalid") || !strings.Contains(out, def.ID.String()) {
		t.Errorf("log = %q, want the code and the attribute ID", out)
	}
	if strings.Contains(out, `"x"`) || strings.Contains(out, "x:") || strings.Contains(out, "{") {
		t.Errorf("log = %q leaks the value", out)
	}
}

func TestFormatUnknownOptionIsLeftOut(t *testing.T) {
	logs := captureLog(t)
	_, ids := treeOptions()
	def := Attribute{Type: TypeMultiselect, Options: []Option{{ID: ids[0], Label: "A"}}}
	raw := fmt.Sprintf(`[%q,"00000000-0000-7000-8000-0000000000ee"]`, ids[0])
	if got := Format(def, json.RawMessage(raw), esLocale(t), ForTemplate); got != "A" {
		t.Errorf("Format = %q, want only the known option", got)
	}
	if !strings.Contains(logs.String(), "code=attr_value_invalid") {
		t.Errorf("log = %q, want the code", logs.String())
	}
}

func TestFormatOptionCycleStops(t *testing.T) {
	a := uuid.MustParse("00000000-0000-7000-8000-0000000000a1")
	b := uuid.MustParse("00000000-0000-7000-8000-0000000000b1")
	def := Attribute{Type: TypeSelect, Options: []Option{
		{ID: a, Label: "A", ParentID: uuid.NullUUID{UUID: b, Valid: true}},
		{ID: b, Label: "B", ParentID: uuid.NullUUID{UUID: a, Valid: true}},
	}}
	got := Format(def, json.RawMessage(fmt.Sprintf("%q", a)), esLocale(t), ForCSV)
	if got != "A / B / A" {
		t.Errorf("Format = %q, want the walk to stop after len(options) steps", got)
	}
}
