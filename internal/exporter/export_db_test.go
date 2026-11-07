//go:build db

package exporter_test

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nyaruka/phonenumbers"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/exporter"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/services"
	"github.com/jnaranj09/open-customer/internal/settings"
	"github.com/jnaranj09/open-customer/internal/templates"
)

// env is a migrated database with business settings, the real modules to make rows with, and the
// exporter under test. The clock starts at 2026-10-20 12:00 UTC.
type env struct {
	pool      *pgxpool.Pool
	clock     *clock.Fake
	attrs     *attributes.Module
	clients   *clients.Module
	services  *services.Module
	appts     *appointments.Module
	templates *templates.Module
	exp       *exporter.Exporter
}

func newEnv(t *testing.T, currency string) env {
	t.Helper()
	pool := dbtest.Pool(t)
	fake := clock.NewFake(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	attrs := attributes.New(attributes.Options{Clock: fake})
	svcs := services.New(services.Options{Clock: fake})
	sets := settings.New(settings.Options{Clock: fake})
	if _, err := sets.Create(t.Context(), pool, settings.SettingsInput{
		Name: "Estudio Sol", Timezone: "Europe/Madrid", Locale: "es", Currency: currency, CountryCode: "ES",
		AccentColor: "#aa3366",
	}); err != nil {
		t.Fatalf("create settings: %v", err)
	}
	return env{
		pool: pool, clock: fake, attrs: attrs, services: svcs,
		clients:   clients.New(clients.Options{Clock: fake, Attributes: attrs}),
		appts:     appointments.New(appointments.Options{Clock: fake, Services: svcs, Attributes: attrs}),
		templates: templates.New(templates.Options{Clock: fake, Attributes: attrs}),
		exp:       exporter.New(exporter.Options{Clock: fake, Settings: sets, Attributes: attrs}),
	}
}

// snapshot is the read-only REPEATABLE READ transaction the export handler opens.
var snapshot = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}

// export runs Write inside the snapshot transaction and returns the ZIP bytes.
func (e env) export(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	err := db.WithTx(t.Context(), e.pool, snapshot, func(tx pgx.Tx) error {
		return e.exp.Write(t.Context(), tx, &buf)
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return buf.Bytes()
}

// zipFiles reads the ZIP back: the entry names in order, and each entry's content.
func zipFiles(t *testing.T, b []byte) ([]string, map[string][]byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	names := make([]string, 0, len(zr.File))
	files := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		names = append(names, f.Name)
		files[f.Name] = data
	}
	return names, files
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// csvFile exports and returns the rows of one CSV entry, header first, without the BOM.
func (e env) csvFile(t *testing.T, name string) [][]string {
	t.Helper()
	_, files := zipFiles(t, e.export(t))
	data, ok := files[name]
	if !ok {
		t.Fatalf("no %s in the zip", name)
	}
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, bom))).ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return rows
}

// column returns the cell of row under the header col.
func column(t *testing.T, rows [][]string, row int, col string) string {
	t.Helper()
	i := slices.Index(rows[0], col)
	if i < 0 {
		t.Fatalf("no column %q in %q", col, rows[0])
	}
	return rows[row][i]
}

func (e env) attribute(t *testing.T, label string, scope attributes.Scope, typ attributes.Type) attributes.Attribute {
	t.Helper()
	a, err := e.attrs.Create(t.Context(), e.pool, attributes.AttributeInput{Label: label, Scope: scope, Type: typ})
	if err != nil {
		t.Fatalf("create attribute %s: %v", label, err)
	}
	return a
}

func (e env) option(t *testing.T, attrID uuid.UUID, label string) uuid.UUID {
	t.Helper()
	o, err := e.attrs.AddOption(t.Context(), e.pool, attrID, attributes.OptionInput{Label: label})
	if err != nil {
		t.Fatalf("add option %s: %v", label, err)
	}
	return o.ID
}

func (e env) archiveAttribute(t *testing.T, id uuid.UUID) {
	t.Helper()
	yes := true
	if _, err := e.attrs.Update(t.Context(), e.pool, id, attributes.AttributePatch{Archived: &yes}); err != nil {
		t.Fatalf("archive attribute: %v", err)
	}
}

// client creates a client with the ES example phone and the given attrs (nil for none).
func (e env) client(t *testing.T, name string, notes *string, attrs json.RawMessage) clients.Client {
	t.Helper()
	phone := phonenumbers.Format(phonenumbers.GetExampleNumber("ES"), phonenumbers.E164)
	c, err := e.clients.Create(t.Context(), e.pool, clients.ClientInput{
		Name: name, Phone: &phone, Notes: notes, Attrs: attrs, AllowDuplicatePhone: true,
	}, "ES")
	if err != nil {
		t.Fatalf("create client %s: %v", name, err)
	}
	return c
}

func (e env) service(t *testing.T, name string, minutes int, price *int64) uuid.UUID {
	t.Helper()
	s, err := e.services.Create(t.Context(), e.pool, services.ServiceInput{Name: name, DurationMin: minutes, PriceCents: price, Color: "#aa3366"})
	if err != nil {
		t.Fatalf("create service %s: %v", name, err)
	}
	return s.ID
}

func (e env) book(t *testing.T, clientID uuid.UUID, starts time.Time, serviceIDs ...uuid.UUID) appointments.Appointment {
	t.Helper()
	a, err := e.appts.Create(t.Context(), e.pool, appointments.AppointmentInput{
		ClientID: clientID, StartsAt: starts, ServiceIDs: serviceIDs, AllowOverlap: true,
	})
	if err != nil {
		t.Fatalf("book appointment: %v", err)
	}
	return a
}

func ptr[T any](v T) *T { return &v }

func TestExportHasFourFilesInOrder(t *testing.T) {
	e := newEnv(t, "USD")
	names, _ := zipFiles(t, e.export(t))
	want := []string{"clientas.csv", "citas.csv", "servicios.csv", "datos.json"}
	if !slices.Equal(names, want) {
		t.Fatalf("entries = %q, want %q", names, want)
	}
}

func TestExportCSVHeadersAndBOM(t *testing.T) {
	e := newEnv(t, "USD")
	e.attribute(t, "Alergias", attributes.ScopeClient, attributes.TypeText)
	e.attribute(t, "Cumpleaños", attributes.ScopeClient, attributes.TypeDate)
	e.attribute(t, "Diseño", attributes.ScopeAppointment, attributes.TypeText)
	_, files := zipFiles(t, e.export(t))
	data := files["clientas.csv"]
	if !bytes.HasPrefix(data, bom) {
		t.Fatalf("clientas.csv starts with % x, want the UTF-8 BOM", data[:min(3, len(data))])
	}
	header, _, _ := bytes.Cut(bytes.TrimPrefix(data, bom), []byte("\n"))
	if want := "nombre,telefono,notas,creada,archivada,Alergias,Cumpleaños"; string(header) != want {
		t.Fatalf("header = %q, want %q", header, want)
	}
}

func TestExportAccentedNameAndCommaNote(t *testing.T) {
	e := newEnv(t, "USD")
	c := e.client(t, "Lucía Rojas", ptr("Uñas, cortas"), nil)
	rows := e.csvFile(t, "clientas.csv")
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want header + 1", len(rows))
	}
	if got := column(t, rows, 1, "nombre"); got != "Lucía Rojas" {
		t.Errorf("nombre = %q", got)
	}
	if got := column(t, rows, 1, "notas"); got != "Uñas, cortas" {
		t.Errorf("notas = %q", got)
	}
	if got := column(t, rows, 1, "telefono"); got != "'"+*c.PhoneE164 {
		t.Errorf("telefono = %q, want %q", got, "'"+*c.PhoneE164)
	}
	if got := column(t, rows, 1, "creada"); got != "2026-10-20" {
		t.Errorf("creada = %q, want 2026-10-20", got)
	}
	if got := column(t, rows, 1, "archivada"); got != "" {
		t.Errorf("archivada = %q, want empty", got)
	}
}

func TestExportMultiselectCell(t *testing.T) {
	e := newEnv(t, "USD")
	tonos := e.attribute(t, "Tonos", attributes.ScopeClient, attributes.TypeMultiselect)
	a, b, c := e.option(t, tonos.ID, "A"), e.option(t, tonos.ID, "B"), e.option(t, tonos.ID, "C")
	attrs, err := json.Marshal(map[string][]uuid.UUID{tonos.Key: {a, b, c}})
	if err != nil {
		t.Fatalf("marshal attrs: %v", err)
	}
	e.client(t, "Ana Rojas", nil, attrs)
	rows := e.csvFile(t, "clientas.csv")
	if got := column(t, rows, 1, "Tonos"); got != "A; B; C" {
		t.Fatalf("Tonos = %q, want %q", got, "A; B; C")
	}
}

func TestExportSameLabelGetsKey(t *testing.T) {
	e := newEnv(t, "USD")
	first := e.attribute(t, "Color", attributes.ScopeClient, attributes.TypeText)
	second := e.attribute(t, "Color", attributes.ScopeClient, attributes.TypeText)
	rows := e.csvFile(t, "clientas.csv")
	got := rows[0][len(rows[0])-2:]
	want := []string{"Color (" + first.Key + ")", "Color (" + second.Key + ")"}
	if !slices.Equal(got, want) || first.Key != "color" || second.Key != "color_2" {
		t.Fatalf("attribute columns = %q, want [Color (color) Color (color_2)]", got)
	}
}

func TestExportArchivedClientMarked(t *testing.T) {
	e := newEnv(t, "USD")
	c := e.client(t, "Marta Solano", nil, nil)
	if _, err := e.clients.Update(t.Context(), e.pool, c.ID, clients.ClientPatch{Archived: ptr(true)}, "ES"); err != nil {
		t.Fatalf("archive client: %v", err)
	}
	rows := e.csvFile(t, "clientas.csv")
	if got := column(t, rows, 1, "archivada"); got != "sí" {
		t.Fatalf("archivada = %q, want sí", got)
	}
}

func TestExportAppointmentRow(t *testing.T) {
	e := newEnv(t, "USD")
	acrilico := e.service(t, "Acrílico", 60, ptr(int64(1000000)))
	pedicure := e.service(t, "Pedicure", 30, ptr(int64(250000)))
	c := e.client(t, "Lucía Rojas", nil, nil)
	e.book(t, c.ID, time.Date(2026, 10, 24, 13, 0, 0, 0, time.UTC), acrilico, pedicure)
	e.clock.Set(time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))

	rows := e.csvFile(t, "citas.csv")
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want header + 1", len(rows))
	}
	if want := []string{"fecha", "hora", "clienta", "servicios", "total", "pagado", "estado", "notas"}; !slices.Equal(rows[0], want) {
		t.Errorf("header = %q, want %q", rows[0], want)
	}
	for col, want := range map[string]string{
		"fecha": "2026-10-24", "hora": "15:00", "clienta": "Lucía Rojas", "servicios": "Acrílico; Pedicure",
		"total": "12500,00", "pagado": "", "estado": "realizada", "notas": "",
	} {
		if got := column(t, rows, 1, col); got != want {
			t.Errorf("%s = %q, want %q", col, got, want)
		}
	}
}

func TestExportNoShowAppointment(t *testing.T) {
	e := newEnv(t, "USD")
	pedicure := e.service(t, "Pedicure", 30, nil)
	c := e.client(t, "Ana Rojas", nil, nil)
	a := e.book(t, c.ID, time.Date(2026, 10, 24, 13, 0, 0, 0, time.UTC), pedicure)
	e.clock.Set(time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))
	if _, err := e.appts.SetStatus(t.Context(), e.pool, a.ID, appointments.NoShow, true); err != nil {
		t.Fatalf("set no-show: %v", err)
	}
	rows := e.csvFile(t, "citas.csv")
	if got := column(t, rows, 1, "estado"); got != "no vino" {
		t.Fatalf("estado = %q, want no vino", got)
	}
}

func TestExportZeroDecimalCurrency(t *testing.T) {
	e := newEnv(t, "JPY")
	e.service(t, "Pedicure", 30, ptr(int64(12500)))
	rows := e.csvFile(t, "servicios.csv")
	if want := []string{"nombre", "duracion_min", "precio", "archivado"}; !slices.Equal(rows[0], want) {
		t.Errorf("header = %q, want %q", rows[0], want)
	}
	if got := rows[1]; !slices.Equal(got, []string{"Pedicure", "30", "12500", ""}) {
		t.Fatalf("service row = %q, want [Pedicure 30 12500 \"\"]", got)
	}
}

func TestExportPagesPast500(t *testing.T) {
	e := newEnv(t, "USD")
	err := db.WithTx(t.Context(), e.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		for range 501 {
			if _, err := e.clients.Create(t.Context(), tx, clients.ClientInput{Name: "Ana Rojas"}, "ES"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("create clients: %v", err)
	}
	_, files := zipFiles(t, e.export(t))
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(files["clientas.csv"], bom))).ReadAll()
	if err != nil {
		t.Fatalf("parse clientas.csv: %v", err)
	}
	var data struct {
		Clients []json.RawMessage `json:"clients"`
	}
	if err := json.Unmarshal(files["datos.json"], &data); err != nil {
		t.Fatalf("parse datos.json: %v", err)
	}
	if len(rows)-1 != 501 || len(data.Clients) != 501 {
		t.Fatalf("got %d CSV rows and %d JSON clients, want 501 each", len(rows)-1, len(data.Clients))
	}
}

func TestExportDBErrorIsReturned(t *testing.T) {
	e := newEnv(t, "USD")
	var calls int
	err := db.WithTx(t.Context(), e.pool, snapshot, func(tx pgx.Tx) error {
		return e.exp.Write(t.Context(), dbtest.FailingDB{DBTX: tx, FailAt: 2, Calls: &calls}, io.Discard)
	})
	if !errors.Is(err, dbtest.ErrInjected) {
		t.Fatalf("Write error = %v, want one wrapping the injected failure", err)
	}
}

// rule and logRow insert rows the reminders module owns, through its queries, because the
// exporter's tests may not import reminders (depguard reminders-only-from-orchestrators).
func (e env) rule(t *testing.T, templateID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if err := sqlc.New(e.pool).InsertRule(t.Context(), sqlc.InsertRuleParams{
		ID: id, Kind: "before_appointment", OffsetDays: 1, TemplateID: templateID, Now: e.clock.Now(),
	}); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	return id
}

func (e env) logRow(t *testing.T, ruleID, appointmentID uuid.UUID, anchor time.Time) uuid.UUID {
	t.Helper()
	id, err := sqlc.New(e.pool).InsertLog(t.Context(), sqlc.InsertLogParams{
		ID: uuid.Must(uuid.NewV7()), RuleID: ruleID, AppointmentID: appointmentID, AnchorAt: &anchor,
		Action: "sent", Body: ptr("Hola Lucía"), ActedAt: e.clock.Now(),
	})
	if err != nil {
		t.Fatalf("insert log: %v", err)
	}
	return id
}
