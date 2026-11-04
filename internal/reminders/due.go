package reminders

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/locale"
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
}

// ruleRow is an active rule as the loader reads it. ReturnServiceID is the effective return
// service (return_service_id, or service_id when it is empty) and is zero for a "before" rule.
type ruleRow struct {
	ID                uuid.UUID
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
// starts after that client's earliest candidate ends_at.
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

// dueData is everything compute needs, read in one snapshot.
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
}

// compute applies the rules table ("Reminder rules and the 'done' rule") to the loaded rows and
// returns the items due today or earlier, sorted by due day, client name, then ID. It is pure:
// the loader only narrows the rows, every rule is checked here.
func compute(d dueData, now time.Time) []DueItem {
	today := locale.LocalDay(now, d.TZ)
	var items []DueItem
	for _, r := range d.Rules {
		if r.TemplateArchived || r.ServiceArchived {
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
			if r.ReturnArchived {
				continue
			}
			for _, a := range d.ComeBack {
				if it, ok := comeBackItem(d, r, a, now, today); ok {
					items = append(items, it)
				}
			}
		}
	}
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
	if !allowed(d, r, a, due, today) || d.Logged[logKey{RuleID: r.ID, AppointmentID: a.ID, AnchorAt: a.StartsAt.UTC()}] {
		return DueItem{}, false
	}
	it := newItem(d, r, a, due, today)
	anchor := a.StartsAt
	it.ID += ":" + strconv.FormatInt(anchor.Unix(), 10)
	it.AnchorAt = &anchor
	return it, true
}

// comeBackItem is the come-back item of rule r for the done visit a: due offset_days after the
// end day, listed for 60 days, skipped while the client has a later appointment with the return
// service. A future booking without it turns the item into a booking prompt.
func comeBackItem(d dueData, r ruleRow, a apptRow, now, today time.Time) (DueItem, bool) {
	if !r.ServiceID.Valid || !slices.Contains(a.ServiceIDs, r.ServiceID.UUID) || !appointments.IsDone(a.Status, a.EndsAt, now) {
		return DueItem{}, false
	}
	due := locale.AddDays(locale.LocalDay(a.EndsAt, d.TZ), r.OffsetDays, d.TZ)
	if !allowed(d, r, a, due, today) || due.Before(locale.AddDays(today, -lookBackDays, d.TZ)) {
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
	it := newItem(d, r, a, due, today)
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

// allowed holds for a due day that has come, is not before the appointment existed, and is not
// before the rule's applies_from day.
func allowed(d dueData, r ruleRow, a apptRow, due, today time.Time) bool {
	if due.After(today) || due.Before(locale.LocalDay(a.CreatedAt, d.TZ)) {
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
	}
}
