package reminders

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/templates"
)

// lookBackDays is how long an unhandled come-back item stays listed after its due day.
const lookBackDays = 60

// Mode says how an item is shown: a client message, or a prompt to add the return service to a
// booking the client already has.
type Mode string

// The two modes of a due item.
const (
	ModeMessage       Mode = "message"
	ModeBookingPrompt Mode = "booking_prompt"
)

// Prompt is the future booking a come-back item asks to extend with the return service.
type Prompt struct {
	AppointmentID     uuid.UUID
	StartsAt          time.Time
	Services          []string
	ReturnServiceID   uuid.UUID
	ReturnServiceName string
}

// DueItem is one reminder to send today. ID is "<rule>:<appt>:<anchor Unix seconds>" for a
// "before" item and "<rule>:<appt>" for a come-back item. AnchorAt is the appointment start for a
// "before" item and nil for a come-back item. DueDate is the business-local midnight of the due
// day. PhoneDigits and WaLink are nil when the client has no phone. Prompt is set only in
// ModeBookingPrompt; Text and WaLink are still set then, for "Enviar mensaje".
//
// The label fields (reminders-and-replies design, "New fields on each due item"): SourceStartsAt
// and SourceServices are the appointment the item comes from; General marks a general rule's item;
// OffsetDays is the rule's days; ShownUntil is the business-local midnight of the last day the item
// is listed; MessageNumber counts the come-back messages sent for the same appointment, + 1;
// NextDueDate is the due day of the next general rule for the same visit (general items only).
type DueItem struct {
	ID            string
	RuleID        uuid.UUID
	AppointmentID uuid.UUID
	AnchorAt      *time.Time
	Kind          Kind
	ClientID      uuid.UUID
	ClientName    string
	DueDate       time.Time
	Late          bool
	Text          string
	PhoneDigits   *string
	WaLink        *string
	Mode          Mode
	Prompt        *Prompt

	SourceStartsAt time.Time
	SourceServices []string
	General        bool
	OffsetDays     int
	ShownUntil     time.Time
	MessageNumber  int
	NextDueDate    *time.Time
}

// ruleRow is a rule as the loader reads it: an active rule, or an archived specific come-back rule
// (Archived; it never makes an item, but its HandoverFrom limits the general rule). ReturnServiceID
// is the effective return service (return_service_id, or service_id when it is empty) and is zero
// for a "before" rule and a general rule.
type ruleRow struct {
	ID                uuid.UUID
	Archived          bool
	HandoverFrom      *time.Time
	Kind              Kind
	ServiceID         uuid.NullUUID
	ReturnServiceID   uuid.UUID
	ReturnServiceName string
	OffsetDays        int
	AppliesFrom       *time.Time
	TemplateBody      string
	TemplateArchived  bool
	ServiceArchived   bool
	ReturnArchived    bool
}

// general reports whether r is a general rule: a come-back rule for every service.
func (r ruleRow) general() bool { return r.Kind == KindComeBack && !r.ServiceID.Valid }

// canSend reports whether r can make items: it, its template, its service and its effective return
// service are active. A general rule has no service or return service.
func (r ruleRow) canSend() bool {
	return !r.Archived && !r.TemplateArchived && !r.ServiceArchived && !r.ReturnArchived
}

// covers reports whether the specific come-back rule r is about visit a: a holds r's service.
func (r ruleRow) covers(a apptRow) bool {
	return r.Kind == KindComeBack && r.ServiceID.Valid && slices.Contains(a.ServiceIDs, r.ServiceID.UUID)
}

// apptRow is a candidate appointment with its client and service lines.
type apptRow struct {
	ID           uuid.UUID
	ClientID     uuid.UUID
	ClientName   string
	PhoneDigits  *string
	ClientAttrs  json.RawMessage
	StartsAt     time.Time
	EndsAt       time.Time
	CreatedAt    time.Time
	Status       appointments.Status
	Attrs        json.RawMessage
	ServiceIDs   []uuid.UUID
	ServiceNames []string
}

// laterAppt is an appointment (not cancelled or no-show) of a come-back candidate's client that
// starts at or after that client's earliest candidate starts_at.
type laterAppt struct {
	ID           uuid.UUID
	ClientID     uuid.UUID
	StartsAt     time.Time
	ServiceIDs   []uuid.UUID
	ServiceNames []string
}

// logKey is the identity of a handled item. AnchorAt is the "before" item's starts_at in UTC and
// zero for a come-back item, so editing a past visit never brings its come-back back.
type logKey struct {
	RuleID        uuid.UUID
	AppointmentID uuid.UUID
	AnchorAt      time.Time
}

// dueData is everything compute needs, read in one snapshot. SentByVisit counts the sent come-back
// log rows per appointment; GeneralSent marks an appointment with a sent general row.
type dueData struct {
	TZ       *time.Location
	Locale   locale.Locale
	Business string
	Attrs    []attributes.Attribute
	Rules    []ruleRow
	Before   []apptRow
	ComeBack []apptRow
	Later    []laterAppt
	Logged   map[logKey]bool

	SentByVisit map[uuid.UUID]int
	GeneralSent map[uuid.UUID]bool
}

// DueList returns the items due today or earlier ("Para enviar hoy"). It loads the rows in a few
// queries on q, then hands them to compute. The caller passes a read-only REPEATABLE READ
// transaction, so every query sees one snapshot. Before setup (no settings row) it is
// problem.NotFound.
func (m *Module) DueList(ctx context.Context, q db.DBTX, now time.Time) ([]DueItem, error) {
	d, err := m.loadDue(ctx, q, now)
	if err != nil {
		return nil, err
	}
	return compute(d, now), nil
}

// loadDue reads the settings, the attributes and the active rules, then the candidate
// appointments those rules can pair with, the later bookings of the come-back clients and the log
// rows of the candidates (design doc "Due-list computation", steps 1-2). It only narrows the rows;
// compute checks every rule.
func (m *Module) loadDue(ctx context.Context, q db.DBTX, now time.Time) (dueData, error) {
	queries := sqlc.New(q)
	b, err := loadBusiness(ctx, queries)
	if err != nil {
		return dueData{}, err
	}
	d := dueData{
		TZ: b.TZ, Locale: b.Locale, Business: b.Name, Logged: map[logKey]bool{},
		SentByVisit: map[uuid.UUID]int{}, GeneralSent: map[uuid.UUID]bool{},
	}
	if d.Attrs, err = m.attributes.List(ctx, q); err != nil {
		return dueData{}, fmt.Errorf("due attributes: %w", err)
	}
	rules, err := queries.ListRulesForDue(ctx)
	if err != nil {
		return dueData{}, fmt.Errorf("list due rules: %w", err)
	}
	maxBefore, maxComeBack := 0, 0
	for _, r := range rules {
		row := ruleRowFrom(r)
		d.Rules = append(d.Rules, row)
		if row.Archived {
			continue
		}
		switch row.Kind {
		case KindBefore:
			maxBefore = max(maxBefore, row.OffsetDays)
		case KindComeBack:
			maxComeBack = max(maxComeBack, row.OffsetDays)
		}
	}
	tz := b.TZ
	today := locale.LocalDay(now, tz)
	before, err := queries.ListBeforeCandidates(ctx, sqlc.ListBeforeCandidatesParams{
		Now: now, Until: locale.AddDays(today, maxBefore+1, tz),
	})
	if err != nil {
		return dueData{}, fmt.Errorf("list before candidates: %w", err)
	}
	comeBack, err := queries.ListComeBackCandidates(ctx, sqlc.ListComeBackCandidatesParams{
		Since: locale.AddDays(today, -(maxComeBack + lookBackDays), tz), Now: now,
	})
	if err != nil {
		return dueData{}, fmt.Errorf("list come-back candidates: %w", err)
	}
	for _, r := range before {
		d.Before = append(d.Before, apptRowFrom(r))
	}
	for _, r := range comeBack {
		d.ComeBack = append(d.ComeBack, apptRowFrom(sqlc.ListBeforeCandidatesRow(r)))
	}
	candidateIDs := make([]uuid.UUID, 0, len(d.Before)+len(d.ComeBack))
	for _, a := range slices.Concat(d.Before, d.ComeBack) {
		candidateIDs = append(candidateIDs, a.ID)
	}
	lines, err := linesByAppointment(ctx, queries, candidateIDs)
	if err != nil {
		return dueData{}, err
	}
	for _, list := range [][]apptRow{d.Before, d.ComeBack} {
		for i := range list {
			list[i].ServiceIDs, list[i].ServiceNames = lines[list[i].ID].ids, lines[list[i].ID].names
		}
	}
	if d.Later, err = laterAppointments(ctx, queries, d.ComeBack); err != nil {
		return dueData{}, err
	}
	keys, err := queries.ListLogKeys(ctx, candidateIDs)
	if err != nil {
		return dueData{}, fmt.Errorf("list log keys: %w", err)
	}
	for _, k := range keys {
		key := logKey{RuleID: k.RuleID, AppointmentID: k.AppointmentID}
		if k.AnchorAt != nil {
			// compute looks "before" rows up by StartsAt.UTC(); a time.Time map key compares the zone.
			key.AnchorAt = k.AnchorAt.UTC()
		}
		d.Logged[key] = true
		if k.Action == string(ActionSent) && Kind(k.Kind) == KindComeBack {
			d.SentByVisit[k.AppointmentID]++
			if k.General {
				d.GeneralSent[k.AppointmentID] = true
			}
		}
	}
	return d, nil
}

// business is the settings a rendered message needs.
type business struct {
	TZ     *time.Location
	Locale locale.Locale
	Name   string
}

// loadBusiness reads the business name, time zone and locale. Before setup (no settings row) it
// is problem.NotFound. The due list and the template preview share it.
func loadBusiness(ctx context.Context, queries *sqlc.Queries) (business, error) {
	s, err := queries.GetDueSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return business{}, problem.NotFound{}
	}
	if err != nil {
		return business{}, fmt.Errorf("get due settings: %w", err)
	}
	tz, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return business{}, fmt.Errorf("load business time zone: %w", err)
	}
	loc, err := locale.For(s.Locale)
	if err != nil {
		return business{}, fmt.Errorf("business locale: %w", err)
	}
	return business{TZ: tz, Locale: loc, Name: s.Name}, nil
}

// serviceLines are the service ids and names of one appointment, in line order.
type serviceLines struct {
	ids   []uuid.UUID
	names []string
}

// linesByAppointment reads the service lines of the appointments ids.
func linesByAppointment(ctx context.Context, queries *sqlc.Queries, ids []uuid.UUID) (map[uuid.UUID]serviceLines, error) {
	rows, err := queries.ListCandidateLines(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list candidate lines: %w", err)
	}
	out := make(map[uuid.UUID]serviceLines, len(ids))
	for _, r := range rows {
		l := out[r.AppointmentID]
		l.ids, l.names = append(l.ids, r.ServiceID), append(l.names, r.Name)
		out[r.AppointmentID] = l
	}
	return out, nil
}

// laterAppointments reads, for the clients of the come-back candidates, the appointments (not
// cancelled or no-show) that start at or after the earliest candidate starts_at, with their lines,
// so a booking at the same start as a visit reaches compute. compute keeps, per visit, the ones it
// needs: after the visit's ends_at for a specific rule, after its start for a general rule.
func laterAppointments(ctx context.Context, queries *sqlc.Queries, candidates []apptRow) ([]laterAppt, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	after := candidates[0].StartsAt
	clientIDs := make([]uuid.UUID, 0, len(candidates))
	for _, a := range candidates {
		after = minTime(after, a.StartsAt)
		if !slices.Contains(clientIDs, a.ClientID) {
			clientIDs = append(clientIDs, a.ClientID)
		}
	}
	rows, err := queries.ListLaterAppointments(ctx, sqlc.ListLaterAppointmentsParams{ClientIds: clientIDs, After: after})
	if err != nil {
		return nil, fmt.Errorf("list later appointments: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	lines, err := linesByAppointment(ctx, queries, ids)
	if err != nil {
		return nil, err
	}
	out := make([]laterAppt, 0, len(rows))
	for _, r := range rows {
		out = append(out, laterAppt{
			ID: r.ID, ClientID: r.ClientID, StartsAt: r.StartsAt,
			ServiceIDs: lines[r.ID].ids, ServiceNames: lines[r.ID].names,
		})
	}
	return out, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func ruleRowFrom(r sqlc.ListRulesForDueRow) ruleRow {
	row := ruleRow{
		ID: r.ID, Archived: r.ArchivedAt != nil, HandoverFrom: r.HandoverFrom, Kind: Kind(r.Kind), ServiceID: r.ServiceID, ReturnServiceID: r.ReturnServiceID.UUID,
		OffsetDays: int(r.OffsetDays), AppliesFrom: r.AppliesFrom, TemplateBody: r.TemplateBody,
		TemplateArchived: r.TemplateArchivedAt != nil, ServiceArchived: r.ServiceArchivedAt != nil,
		ReturnArchived: r.ReturnArchivedAt != nil,
	}
	if r.ReturnServiceName != nil {
		row.ReturnServiceName = *r.ReturnServiceName
	}
	return row
}

func apptRowFrom(r sqlc.ListBeforeCandidatesRow) apptRow {
	return apptRow{
		ID: r.ID, ClientID: r.ClientID, ClientName: r.ClientName, PhoneDigits: r.PhoneDigits,
		ClientAttrs: r.ClientAttrs, StartsAt: r.StartsAt, EndsAt: r.EndsAt, CreatedAt: r.CreatedAt,
		Status: appointments.Status(r.Status), Attrs: r.Attrs,
	}
}

// compute applies the rules table ("Reminder rules and the 'done' rule", and the
// reminders-and-replies design's "Due-list rules") to the loaded rows and returns the items due
// today or earlier, sorted by due day, client name, then ID. It is pure: the loader only narrows the
// rows, every rule is checked here.
func compute(d dueData, now time.Time) []DueItem {
	today := locale.LocalDay(now, d.TZ)
	var items []DueItem
	for _, r := range d.Rules {
		if !r.canSend() || r.general() {
			continue
		}
		switch r.Kind {
		case KindBefore:
			for _, a := range d.Before {
				if it, ok := beforeItem(d, r, a, now, today); ok {
					items = append(items, it)
				}
			}
		case KindComeBack:
			for _, a := range d.ComeBack {
				if it, ok := comeBackItem(d, r, a, now, today); ok {
					items = append(items, it)
				}
			}
		}
	}
	items = append(items, generalItems(d, now, today)...)
	slices.SortFunc(items, func(x, y DueItem) int {
		return cmp.Or(x.DueDate.Compare(y.DueDate), cmp.Compare(x.ClientName, y.ClientName), cmp.Compare(x.ID, y.ID))
	})
	return items
}

// beforeItem is the "before" item of rule r for appointment a: due offset_days before the start
// day, shown until the start, never for a cancelled or no-show appointment.
func beforeItem(d dueData, r ruleRow, a apptRow, now, today time.Time) (DueItem, bool) {
	if !active(a.Status) || !a.StartsAt.After(now) {
		return DueItem{}, false
	}
	due := locale.AddDays(locale.LocalDay(a.StartsAt, d.TZ), -r.OffsetDays, d.TZ)
	if !dueReached(due, today) || !allowedFrom(d, r, a, due) || d.Logged[logKey{RuleID: r.ID, AppointmentID: a.ID, AnchorAt: a.StartsAt.UTC()}] {
		return DueItem{}, false
	}
	it := newItem(d, r, a, due, today)
	it.ShownUntil = locale.LocalDay(a.StartsAt, d.TZ)
	it.MessageNumber = 1
	anchor := a.StartsAt
	it.ID += ":" + strconv.FormatInt(anchor.Unix(), 10)
	it.AnchorAt = &anchor
	return it, true
}

// comeBackItem is the come-back item of rule r for the done visit a: due offset_days after the
// end day, listed for 60 days, skipped while the client has a later appointment with the return
// service, and once a general reminder for the visit was sent. A future booking without the return
// service turns the item into a booking prompt.
func comeBackItem(d dueData, r ruleRow, a apptRow, now, today time.Time) (DueItem, bool) {
	if !r.covers(a) || !appointments.IsDone(a.Status, a.EndsAt, now) || d.GeneralSent[a.ID] {
		return DueItem{}, false
	}
	due := locale.AddDays(locale.LocalDay(a.EndsAt, d.TZ), r.OffsetDays, d.TZ)
	if !dueReached(due, today) || !allowedFrom(d, r, a, due) || due.Before(locale.AddDays(today, -lookBackDays, d.TZ)) {
		return DueItem{}, false
	}
	var next *laterAppt
	for i := range d.Later {
		l := &d.Later[i]
		if l.ClientID != a.ClientID || !l.StartsAt.After(a.EndsAt) {
			continue
		}
		if slices.Contains(l.ServiceIDs, r.ReturnServiceID) {
			return DueItem{}, false
		}
		// a is done, so ends_at <= now: a booking after now is also after ends_at, and the check
		// above means it lacks the return service.
		if l.StartsAt.After(now) && (next == nil || earlier(*l, *next)) {
			next = l
		}
	}
	if d.Logged[logKey{RuleID: r.ID, AppointmentID: a.ID}] {
		return DueItem{}, false
	}
	it := comeBackNew(d, r, a, due, today)
	if next != nil {
		it.Mode = ModeBookingPrompt
		it.Prompt = &Prompt{
			AppointmentID: next.ID, StartsAt: next.StartsAt, Services: next.ServiceNames,
			ReturnServiceID: r.ReturnServiceID, ReturnServiceName: r.ReturnServiceName,
		}
	}
	return it, true
}

// active reports whether an appointment is still going ahead.
func active(s appointments.Status) bool {
	switch s {
	case appointments.Scheduled, appointments.Confirmed:
		return true
	case appointments.Cancelled, appointments.NoShow:
		return false
	}
	return false
}

// generalItems is at most one general reminder per client (reminders-and-replies design,
// "Due-list rules"): for the client's latest done visit v, the latest due general rule that can
// send, unless a later appointment, a specific rule that can send for v, or a log row stops it.
func generalItems(d dueData, now, today time.Time) []DueItem {
	var general []ruleRow
	for _, r := range d.Rules {
		if r.general() && r.canSend() {
			general = append(general, r)
		}
	}
	if len(general) == 0 {
		return nil
	}
	var items []DueItem
	for _, v := range latestVisits(d.ComeBack, now) {
		if it, ok := generalItem(d, general, v, today); ok {
			items = append(items, it)
		}
	}
	return items
}

// latestVisits is each client's latest done candidate visit (the latest start; on a tie, the higher
// id), in the order the candidates came.
func latestVisits(candidates []apptRow, now time.Time) []apptRow {
	at := map[uuid.UUID]int{}
	var out []apptRow
	for _, a := range candidates {
		if !appointments.IsDone(a.Status, a.EndsAt, now) {
			continue
		}
		i, seen := at[a.ClientID]
		switch {
		case !seen:
			at[a.ClientID] = len(out)
			out = append(out, a)
		case after(a.StartsAt, a.ID, out[i].StartsAt, out[i].ID):
			out[i] = a
		}
	}
	return out
}

// generalItem is the general reminder for visit v, the client's latest done visit.
func generalItem(d dueData, general []ruleRow, v apptRow, today time.Time) (DueItem, bool) {
	for _, l := range d.Later {
		if l.ClientID == v.ClientID && after(l.StartsAt, l.ID, v.StartsAt, v.ID) {
			return DueItem{}, false // G1: any later appointment
		}
	}
	var cutoff *time.Time
	for _, r := range d.Rules {
		if !r.covers(v) {
			continue
		}
		if r.canSend() {
			return DueItem{}, false // G2: a specific rule that can send has the visit
		}
		if r.HandoverFrom != nil && (cutoff == nil || r.HandoverFrom.After(*cutoff)) {
			cutoff = r.HandoverFrom
		}
	}
	end := locale.LocalDay(v.EndsAt, d.TZ)
	oldest := locale.AddDays(today, -lookBackDays, d.TZ)
	from := func(g ruleRow, due time.Time) bool {
		return allowedFrom(d, g, v, due) && (cutoff == nil || !due.Before(locale.LocalDay(*cutoff, d.TZ)))
	}
	var pick *ruleRow
	var pickDue time.Time
	for i := range general {
		g := &general[i]
		due := locale.AddDays(end, g.OffsetDays, d.TZ)
		if !dueReached(due, today) || due.Before(oldest) || !from(*g, due) {
			continue
		}
		if pick == nil || laterPick(due, *g, pickDue, *pick) {
			pick, pickDue = g, due
		}
	}
	if pick == nil || d.Logged[logKey{RuleID: pick.ID, AppointmentID: v.ID}] {
		return DueItem{}, false // G3: a handled latest due hides the earlier ones
	}
	it := comeBackNew(d, *pick, v, pickDue, today)
	for _, g := range general {
		due := locale.AddDays(end, g.OffsetDays, d.TZ)
		if due.After(pickDue) && from(g, due) && (it.NextDueDate == nil || due.Before(*it.NextDueDate)) {
			it.NextDueDate = &due
		}
	}
	if it.NextDueDate != nil {
		if dayBefore := locale.AddDays(*it.NextDueDate, -1, d.TZ); dayBefore.Before(it.ShownUntil) {
			it.ShownUntil = dayBefore
		}
	}
	return it, true
}

// laterPick orders general candidates: the latest due, then the larger offset, then the higher id.
func laterPick(due time.Time, g ruleRow, pickDue time.Time, pick ruleRow) bool {
	return cmp.Or(
		due.Compare(pickDue), cmp.Compare(g.OffsetDays, pick.OffsetDays), slices.Compare(g.ID[:], pick.ID[:]),
	) > 0
}

// after reports whether the appointment (start s, id) comes after (t, other): a later start, or the
// same start and a higher id (ids are UUIDv7, so the later created).
func after(s time.Time, id uuid.UUID, t time.Time, other uuid.UUID) bool {
	return cmp.Or(s.Compare(t), slices.Compare(id[:], other[:])) > 0
}

// dueReached holds for a due day that has come.
func dueReached(due, today time.Time) bool { return !due.After(today) }

// allowedFrom holds for a due day that is not before the appointment existed and not before the
// rule's applies_from day.
func allowedFrom(d dueData, r ruleRow, a apptRow, due time.Time) bool {
	if due.Before(locale.LocalDay(a.CreatedAt, d.TZ)) {
		return false
	}
	return r.AppliesFrom == nil || !due.Before(locale.LocalDay(*r.AppliesFrom, d.TZ))
}

// earlier orders bookings by start, then ID, so the prompt's booking is fixed.
func earlier(x, y laterAppt) bool {
	return cmp.Or(x.StartsAt.Compare(y.StartsAt), slices.Compare(x.ID[:], y.ID[:])) < 0
}

// newItem renders the message of rule r for appointment a in ModeMessage.
func newItem(d dueData, r ruleRow, a apptRow, due, today time.Time) DueItem {
	data := buildData(
		clientView{Name: a.ClientName, Attrs: a.ClientAttrs},
		apptView{StartsAt: a.StartsAt, ServiceNames: a.ServiceNames, Attrs: a.Attrs},
		d.Attrs, d.Locale, d.TZ, d.Business,
	)
	text := templates.Render(r.TemplateBody, data)
	var link *string
	if a.PhoneDigits != nil {
		l := WhatsAppLink(*a.PhoneDigits, text)
		link = &l
	}
	return DueItem{
		ID: r.ID.String() + ":" + a.ID.String(), RuleID: r.ID, AppointmentID: a.ID, Kind: r.Kind,
		ClientID: a.ClientID, ClientName: a.ClientName, DueDate: due, Late: due.Before(today),
		Text: text, PhoneDigits: a.PhoneDigits, WaLink: link, Mode: ModeMessage,
		SourceStartsAt: a.StartsAt, SourceServices: a.ServiceNames, General: r.general(),
		OffsetDays: r.OffsetDays,
	}
}

// comeBackNew is newItem for a come-back rule: listed until 60 days after the due day, numbered
// after the come-back messages already sent for the visit.
func comeBackNew(d dueData, r ruleRow, a apptRow, due, today time.Time) DueItem {
	it := newItem(d, r, a, due, today)
	it.ShownUntil = locale.AddDays(due, lookBackDays, d.TZ)
	it.MessageNumber = d.SentByVisit[a.ID] + 1
	return it
}
