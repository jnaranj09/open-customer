// Package exporter writes all of the business's data as one ZIP (design doc "Export"): three
// Spanish CSV files and datos.json, read inside the caller's one read-only snapshot.
package exporter

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/currency"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/settings"
)

// Options is what serve passes to New.
type Options struct {
	Clock      clock.Clock
	Settings   *settings.Module
	Attributes *attributes.Module
}

// Exporter writes the export ZIP.
type Exporter struct {
	clock      clock.Clock
	settings   *settings.Module
	attributes *attributes.Module
}

// New returns the exporter.
func New(opts Options) *Exporter {
	return &Exporter{clock: opts.Clock, settings: opts.Settings, attributes: opts.Attributes}
}

// FileName is the download name of the ZIP for day, the business-local date of the export.
func FileName(day time.Time) string {
	return "open-customer-" + day.Format(time.DateOnly) + ".zip"
}

// bom makes Excel read the CSV files as UTF-8, so accents show correctly.
const bom = "\xEF\xBB\xBF"

// run is one export: the values every file needs, read once at the start.
type run struct {
	ctx      context.Context
	q        *sqlc.Queries
	settings settings.Settings
	loc      locale.Locale
	text     locale.ExportText
	tz       *time.Location
	scale    int // the currency's decimals
	attrs    []attributes.Attribute
	now      time.Time
}

// Write streams the ZIP to w. q is the caller's read-only REPEATABLE READ transaction, so every
// file is one snapshot. The entries are written in the order clientas.csv, citas.csv,
// servicios.csv, datos.json, and each table is paged twice (once for its CSV, once for the JSON),
// so no more than one page of 500 rows is held at a time. On an error the ZIP is left unclosed:
// with no central directory the download is broken, never a file that looks complete.
func (e *Exporter) Write(ctx context.Context, q db.DBTX, w io.Writer) error {
	x, err := e.start(ctx, q)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	zw := zip.NewWriter(w)
	entries := []struct {
		name  string
		write func(io.Writer) error
	}{
		{x.text.ClientsFile, x.writeClients},
		{x.text.AppointmentsFile, x.writeAppointments},
		{x.text.ServicesFile, x.writeServices},
		{jsonFile, x.writeJSON},
	}
	for _, en := range entries {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: en.name, Method: zip.Deflate, Modified: x.now})
		if err != nil {
			return fmt.Errorf("export: create %s: %w", en.name, err)
		}
		if err := en.write(f); err != nil {
			return fmt.Errorf("export: %s: %w", en.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("export: close zip: %w", err)
	}
	return nil
}

func (e *Exporter) start(ctx context.Context, q db.DBTX) (*run, error) {
	st, err := e.settings.Get(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	attrs, err := e.attributes.List(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list attributes: %w", err)
	}
	loc, err := locale.For(st.Locale)
	if err != nil {
		return nil, fmt.Errorf("settings locale: %w", err)
	}
	tz, err := time.LoadLocation(st.Timezone)
	if err != nil {
		return nil, fmt.Errorf("settings time zone: %w", err)
	}
	unit, err := currency.ParseISO(st.Currency)
	if err != nil {
		return nil, fmt.Errorf("settings currency: %w", err)
	}
	scale, _ := currency.Standard.Rounding(unit)
	return &run{
		ctx: ctx, q: sqlc.New(q), settings: st, loc: loc, text: loc.ExportText(), tz: tz,
		scale: scale, attrs: attrs, now: e.clock.Now(),
	}, nil
}

// eachPage calls load with the last id of the previous page (uuid.Nil first) and visit with each
// page, until load returns an empty page.
func eachPage[R any](load func(after uuid.UUID) ([]R, error), id func(R) uuid.UUID, visit func([]R) error) error {
	after := uuid.Nil
	for {
		page, err := load(after)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		if err := visit(page); err != nil {
			return err
		}
		after = id(page[len(page)-1])
	}
}

// The page loaders, one per table, and the id each table pages by.

func (x *run) clients(after uuid.UUID) ([]sqlc.Client, error) {
	rows, err := x.q.ExportClients(x.ctx, after)
	if err != nil {
		return nil, fmt.Errorf("export clients: %w", err)
	}
	return rows, nil
}

func (x *run) appointments(after uuid.UUID) ([]sqlc.ExportAppointmentsRow, error) {
	rows, err := x.q.ExportAppointments(x.ctx, after)
	if err != nil {
		return nil, fmt.Errorf("export appointments: %w", err)
	}
	return rows, nil
}

func (x *run) services(after uuid.UUID) ([]sqlc.Service, error) {
	rows, err := x.q.ExportServices(x.ctx, after)
	if err != nil {
		return nil, fmt.Errorf("export services: %w", err)
	}
	return rows, nil
}

func (x *run) templates(after uuid.UUID) ([]sqlc.ExportTemplatesRow, error) {
	rows, err := x.q.ExportTemplates(x.ctx, after)
	if err != nil {
		return nil, fmt.Errorf("export templates: %w", err)
	}
	return rows, nil
}

func (x *run) rules(after uuid.UUID) ([]sqlc.ExportRulesRow, error) {
	rows, err := x.q.ExportRules(x.ctx, after)
	if err != nil {
		return nil, fmt.Errorf("export rules: %w", err)
	}
	return rows, nil
}

func (x *run) reminderLog(after uuid.UUID) ([]sqlc.ExportLogRow, error) {
	rows, err := x.q.ExportLog(x.ctx, after)
	if err != nil {
		return nil, fmt.Errorf("export reminder log: %w", err)
	}
	return rows, nil
}

// lines returns the service lines of one page of appointments, by appointment, in position order.
func (x *run) lines(page []sqlc.ExportAppointmentsRow) (map[uuid.UUID][]sqlc.ExportAppointmentLinesRow, error) {
	ids := make([]uuid.UUID, 0, len(page))
	for _, r := range page {
		ids = append(ids, r.ID)
	}
	rows, err := x.q.ExportAppointmentLines(x.ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("export appointment lines: %w", err)
	}
	out := make(map[uuid.UUID][]sqlc.ExportAppointmentLinesRow, len(page))
	for _, r := range rows {
		out[r.AppointmentID] = append(out[r.AppointmentID], r)
	}
	return out, nil
}

func clientID(r sqlc.Client) uuid.UUID                     { return r.ID }
func appointmentID(r sqlc.ExportAppointmentsRow) uuid.UUID { return r.ID }
func serviceID(r sqlc.Service) uuid.UUID                   { return r.ID }

// csvFile writes one CSV entry: the BOM, the header, then the rows that rows writes, flushed a
// page at a time.
func csvFile(w io.Writer, header []string, rows func(cw *csv.Writer) error) error {
	if _, err := io.WriteString(w, bom); err != nil {
		return fmt.Errorf("write bom: %w", err)
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(neutralize(header)); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if err := rows(cw); err != nil {
		return err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("write rows: %w", err)
	}
	return nil
}

// writePage writes one page of CSV records and flushes them.
func writePage(cw *csv.Writer, records [][]string) error {
	for _, r := range records {
		neutralize(r)
	}
	if err := cw.WriteAll(records); err != nil {
		return fmt.Errorf("write rows: %w", err)
	}
	return nil
}

// formulaStart are the first characters that make a spreadsheet read a cell as a formula.
const formulaStart = "=+-@\t\r"

// neutralize puts ' before every cell of record that starts with a formulaStart character, so a
// spreadsheet shows the text instead of running it (threat model T42). It changes record in place
// and returns it. The E.164 telefono starts with + and gets the ' too.
func neutralize(record []string) []string {
	for i, cell := range record {
		if cell != "" && strings.ContainsRune(formulaStart, rune(cell[0])) {
			record[i] = "'" + cell
		}
	}
	return record
}

// columns returns the active attributes of scope in position order, and their column names: the
// label, with " (<key>)" added when another of them has the same label.
func (x *run) columns(scope attributes.Scope) ([]attributes.Attribute, []string) {
	var defs []attributes.Attribute
	count := map[string]int{}
	for _, a := range x.attrs {
		if a.Scope == scope && !a.Archived {
			defs = append(defs, a)
			count[a.Label]++
		}
	}
	names := make([]string, 0, len(defs))
	for _, a := range defs {
		name := a.Label
		if count[a.Label] > 1 {
			name += " (" + a.Key + ")"
		}
		names = append(names, name)
	}
	return defs, names
}

// attrCells formats the stored attrs object for the CSV columns defs.
func (x *run) attrCells(defs []attributes.Attribute, raw json.RawMessage) ([]string, error) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode attrs: %w", err)
	}
	cells := make([]string, 0, len(defs))
	for _, d := range defs {
		cells = append(cells, attributes.Format(d, values[d.Key], x.loc, attributes.ForCSV))
	}
	return cells, nil
}

func (x *run) yes(b bool) string {
	if b {
		return x.text.Yes
	}
	return ""
}

func (x *run) money(cents *int64) string {
	if cents == nil {
		return ""
	}
	return x.loc.Money(*cents, x.scale)
}

func (x *run) date(t time.Time) string { return t.In(x.tz).Format(time.DateOnly) }

func (x *run) writeClients(w io.Writer) error {
	defs, names := x.columns(attributes.ScopeClient)
	return csvFile(w, slices.Concat(x.text.ClientCols, names), func(cw *csv.Writer) error {
		return eachPage(x.clients, clientID, func(page []sqlc.Client) error {
			records := make([][]string, 0, len(page))
			for _, r := range page {
				cells, err := x.attrCells(defs, r.Attrs)
				if err != nil {
					return err
				}
				records = append(records, append([]string{
					r.Name, deref(r.PhoneE164), deref(r.Notes), x.date(r.CreatedAt), x.yes(r.ArchivedAt != nil),
				}, cells...))
			}
			return writePage(cw, records)
		})
	})
}

// errUnknownStatus is a stored appointment status with no estado word.
var errUnknownStatus = errors.New("unknown appointment status")

func (x *run) writeAppointments(w io.Writer) error {
	defs, names := x.columns(attributes.ScopeAppointment)
	return csvFile(w, slices.Concat(x.text.AppointmentCols, names), func(cw *csv.Writer) error {
		return eachPage(x.appointments, appointmentID, func(page []sqlc.ExportAppointmentsRow) error {
			lines, err := x.lines(page)
			if err != nil {
				return err
			}
			records := make([][]string, 0, len(page))
			for _, r := range page {
				state := r.Status
				if appointments.IsDone(appointments.Status(r.Status), r.EndsAt, x.now) {
					state = "done"
				}
				estado, ok := x.text.Status[state]
				if !ok {
					return errUnknownStatus
				}
				serviceNames := make([]string, 0, len(lines[r.ID]))
				for _, l := range lines[r.ID] {
					serviceNames = append(serviceNames, l.Name)
				}
				cells, err := x.attrCells(defs, r.Attrs)
				if err != nil {
					return err
				}
				start := r.StartsAt.In(x.tz)
				records = append(records, append([]string{
					start.Format(time.DateOnly), start.Format("15:04"), r.ClientName,
					strings.Join(serviceNames, "; "), x.loc.Money(r.TotalCents, x.scale), x.money(r.PaidCents),
					estado, deref(r.Notes),
				}, cells...))
			}
			return writePage(cw, records)
		})
	})
}

func (x *run) writeServices(w io.Writer) error {
	return csvFile(w, x.text.ServiceCols, func(cw *csv.Writer) error {
		return eachPage(x.services, serviceID, func(page []sqlc.Service) error {
			records := make([][]string, 0, len(page))
			for _, r := range page {
				records = append(records, []string{
					r.Name, deref(r.Description), strconv.Itoa(int(r.DurationMin)), x.money(r.PriceCents),
					x.yes(r.ArchivedAt != nil),
				})
			}
			return writePage(cw, records)
		})
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
