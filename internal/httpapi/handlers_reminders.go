package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/reminders"
)

// CreateReminderRule adds a rule. The module parses the kind and checks the rest of the body.
func (s *Server) CreateReminderRule(w http.ResponseWriter, r *http.Request) {
	var body gen.RuleInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := reminders.RuleInput{
		Kind:            reminders.Kind(body.Kind),
		ServiceID:       nullUUID(body.ServiceId),
		ReturnServiceID: nullUUID(body.ReturnServiceId),
		OffsetDays:      body.OffsetDays,
		TemplateID:      body.TemplateId,
		IncludeOverdue:  body.IncludeOverdue != nil && *body.IncludeOverdue,
	}
	var out reminders.Rule
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.reminders.CreateRule(r.Context(), tx, in); err != nil {
			return fmt.Errorf("create reminder rule: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenRule(out))
}

// UpdateReminderRule changes the fields sent. Only return_service_id may be null.
func (s *Server) UpdateReminderRule(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.RulePatch
	if decodePatch(w, r, &body, "return_service_id") != nil {
		return
	}
	patch := reminders.RulePatch{
		OffsetDays: body.OffsetDays, TemplateID: body.TemplateId, ReturnServiceID: body.ReturnServiceId,
		Archived: body.Archived, IncludeOverdue: body.IncludeOverdue != nil && *body.IncludeOverdue,
	}
	var out reminders.Rule
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.reminders.UpdateRule(r.Context(), tx, id, patch); err != nil {
			return fmt.Errorf("update reminder rule: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenRule(out))
}

// toGenRule is the wire form of a rule.
func toGenRule(r reminders.Rule) gen.Rule {
	return gen.Rule{
		Id: r.ID, Kind: gen.RuleKind(r.Kind), ServiceId: nullableUUID(r.ServiceID),
		ReturnServiceId: nullableUUID(r.ReturnServiceID), OffsetDays: r.OffsetDays,
		TemplateId: r.TemplateID, AppliesFrom: nullval.Of(r.AppliesFrom), Archived: r.Archived,
		HandoverFrom: nullTime(r.HandoverFrom),
	}
}

// handover runs update, the archive or restore of a template or service, inside the caller's
// transaction, after the handover question of that change (reminders-and-replies design,
// "Handover"). Without archived in the patch it only runs update; an include_overdue left out means
// "from today".
func (s *Server) handover(ctx context.Context, tx pgx.Tx, archived, includeOverdue *bool, scope reminders.HandoverScope, update func() error) error {
	if archived != nil {
		include := includeOverdue != nil && *includeOverdue
		answer := s.reminders.RestartRules
		if *archived {
			answer = s.reminders.SetHandover
		}
		if err := answer(ctx, tx, scope, include); err != nil {
			return fmt.Errorf("handover: %w", err)
		}
	}
	return update()
}

// nullUUID is a request id field as the module takes it: absent and null are both "none".
func nullUUID(n nullable.Nullable[uuid.UUID]) uuid.NullUUID {
	if v := nullval.Ptr(n); v != nil {
		return uuid.NullUUID{UUID: *v, Valid: true}
	}
	return uuid.NullUUID{}
}

// nullableUUID is a response id field: none is an explicit null, so the key is always present.
func nullableUUID(n uuid.NullUUID) nullable.Nullable[uuid.UUID] {
	if !n.Valid {
		return nullable.NewNullNullable[uuid.UUID]()
	}
	return nullable.NewNullableWithValue(n.UUID)
}

// ListDueReminders answers the items due today or earlier, read in one snapshot.
func (s *Server) ListDueReminders(w http.ResponseWriter, r *http.Request) {
	var items []reminders.DueItem
	if err := s.snapshot(r, func(tx pgx.Tx) error {
		var err error
		if items, err = s.reminders.DueList(r.Context(), tx, s.clock.Now()); err != nil {
			return fmt.Errorf("list due reminders: %w", err)
		}
		return nil
	}); err != nil {
		writeError(w, r, err)
		return
	}
	out := make([]gen.DueItem, 0, len(items))
	for _, it := range items {
		out = append(out, toGenDueItem(it))
	}
	writeJSON(w, http.StatusOK, gen.DueList{Items: out})
}

// RecordReminder logs what the owner did with a due item: 201 for a new row, 200 with the stored
// row when the item was already logged.
func (s *Server) RecordReminder(w http.ResponseWriter, r *http.Request) {
	var body gen.LogEntry
	if decodeJSON(w, r, &body) != nil {
		return
	}
	action, err := reminders.ParseAction(string(body.Action))
	if err != nil {
		writeError(w, r, err)
		return
	}
	entry := reminders.LogEntry{
		RuleID: body.RuleId, AppointmentID: body.AppointmentId, AnchorAt: nullval.Ptr(body.AnchorAt),
		Action: action, Body: nullval.Ptr(body.Body), PhoneDigits: nullval.Ptr(body.PhoneDigits),
		ActedAt: body.ActedAt,
	}
	var out reminders.LogItem
	var created bool
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, created, err = s.reminders.Record(r.Context(), tx, entry); err != nil {
			return fmt.Errorf("record reminder: %w", err)
		}
		return nil
	}) {
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, toGenLogItem(out))
}

// ListReminderLog answers the log of one client or of one local day. Exactly one filter is needed.
func (s *Server) ListReminderLog(w http.ResponseWriter, r *http.Request, params gen.ListReminderLogParams) {
	if (params.ClientId == nil) == (params.Date == nil) {
		writeError(w, r, problem.Field("client_id", problem.MsgLogFilterInvalid))
		return
	}
	var items []reminders.LogItem
	var err error
	if params.ClientId != nil {
		items, err = s.reminders.LogForClient(r.Context(), s.pool, *params.ClientId)
	} else {
		items, err = s.logForDate(r, params.Date.Time)
	}
	if err != nil {
		writeError(w, r, fmt.Errorf("list reminder log: %w", err))
		return
	}
	out := make([]gen.LogItem, 0, len(items))
	for _, it := range items {
		out = append(out, toGenLogItem(it))
	}
	writeJSON(w, http.StatusOK, gen.LogList{Items: out})
}

// logForDate reads the rows acted on during the business-local day of date (a Date is midnight UTC
// of the calendar day asked for).
func (s *Server) logForDate(r *http.Request, date time.Time) ([]reminders.LogItem, error) {
	loc, err := s.businessZone(r.Context(), s.pool)
	if err != nil {
		return nil, err
	}
	y, m, d := date.Date()
	items, err := s.reminders.LogForDate(r.Context(), s.pool, time.Date(y, m, d, 0, 0, 0, 0, loc), loc)
	if err != nil {
		return nil, fmt.Errorf("log for date: %w", err)
	}
	return items, nil
}

// toGenDueItem is the wire form of a due item. due_date is the calendar day of the business-local
// midnight DueDate holds.
func toGenDueItem(it reminders.DueItem) gen.DueItem {
	out := gen.DueItem{
		Id: it.ID, RuleId: it.RuleID, AppointmentId: it.AppointmentID, AnchorAt: nullTime(it.AnchorAt),
		Kind: gen.RuleKind(it.Kind), ClientId: it.ClientID, ClientName: it.ClientName,
		DueDate: dateOf(it.DueDate), Late: it.Late, Text: it.Text,
		PhoneDigits: nullval.Of(it.PhoneDigits), WaLink: nullval.Of(it.WaLink),
		Mode: gen.DueItemMode(it.Mode), Prompt: nullable.NewNullNullable[gen.BookingPrompt](),
		SourceStartsAt: it.SourceStartsAt.UTC(), SourceServices: it.SourceServices, General: it.General,
		OffsetDays: it.OffsetDays, ShownUntil: dateOf(it.ShownUntil), MessageNumber: it.MessageNumber,
		NextDueDate: nullable.NewNullNullable[openapi_types.Date](),
	}
	if out.SourceServices == nil {
		out.SourceServices = []string{}
	}
	if it.NextDueDate != nil {
		out.NextDueDate = nullable.NewNullableWithValue(dateOf(*it.NextDueDate))
	}
	if p := it.Prompt; p != nil {
		out.Prompt = nullable.NewNullableWithValue(gen.BookingPrompt{
			AppointmentId: p.AppointmentID, StartsAt: p.StartsAt.UTC(), Services: p.Services,
			ReturnServiceId: p.ReturnServiceID, ReturnServiceName: p.ReturnServiceName,
		})
	}
	return out
}

// dateOf is the wire date of a business-local midnight: its calendar day, whatever the zone.
func dateOf(day time.Time) openapi_types.Date {
	y, m, d := day.Date()
	return openapi_types.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

// toGenLogItem is the wire form of a log row.
func toGenLogItem(it reminders.LogItem) gen.LogItem {
	return gen.LogItem{
		Id: it.ID, RuleId: it.RuleID, AppointmentId: it.AppointmentID, AnchorAt: nullTime(it.AnchorAt),
		Kind: gen.RuleKind(it.Kind), ClientId: it.ClientID, ClientName: it.ClientName,
		Action: gen.LogAction(it.Action), Body: nullval.Of(it.Body), PhoneDigits: nullval.Of(it.PhoneDigits),
		ActedAt: it.ActedAt.UTC(),
	}
}

// nullTime is a response time field in UTC; nil is an explicit null.
func nullTime(t *time.Time) nullable.Nullable[time.Time] {
	if t == nil {
		return nullable.NewNullNullable[time.Time]()
	}
	return nullable.NewNullableWithValue(t.UTC())
}
