package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// maxAppointmentRangeDays is the longest GET /appointments range: to is at most this many days
// after from (design doc item 32). Bootstrap reads a longer window through the module directly.
const maxAppointmentRangeDays = 31

// ListAppointments answers the appointments of every status that start on the local days from to
// to, both inclusive, read in the business time zone.
func (s *Server) ListAppointments(w http.ResponseWriter, r *http.Request, params gen.ListAppointmentsParams) {
	// A Date is midnight UTC, so the difference is a whole number of calendar days.
	if params.To.Sub(params.From.Time) > maxAppointmentRangeDays*24*time.Hour {
		writeError(w, r, problem.Validation{Errors: []problem.FieldError{{Field: "to", Message: problem.MsgRangeInvalid}}})
		return
	}
	cfg, err := s.settings.Get(r.Context(), s.pool)
	if err != nil {
		writeError(w, r, fmt.Errorf("get settings: %w", err))
		return
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		writeError(w, r, fmt.Errorf("load the business time zone: %w", err))
		return
	}
	list, err := s.appointments.ListRange(r.Context(), s.pool, params.From.Time, params.To.Time, loc)
	if err != nil {
		writeError(w, r, fmt.Errorf("list appointments: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, gen.AppointmentList{Items: toGenAppointments(list)})
}

// ListClientAppointments answers one client's appointments, newest first.
func (s *Server) ListClientAppointments(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	list, err := s.appointments.ListForClient(r.Context(), s.pool, id)
	if err != nil {
		writeError(w, r, fmt.Errorf("list client appointments: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, gen.AppointmentList{Items: toGenAppointments(list)})
}

// CreateAppointment books an appointment. An overlap is a 409 unless allow_overlap is true.
func (s *Server) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	var body gen.AppointmentInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := appointments.AppointmentInput{
		ClientID: body.ClientId, StartsAt: body.StartsAt, EndsAt: body.EndsAt, Notes: body.Notes,
		AllowOverlap: body.AllowOverlap != nil && *body.AllowOverlap,
	}
	if body.ServiceIds != nil {
		in.ServiceIDs = *body.ServiceIds
	}
	if body.Attrs != nil {
		in.Attrs = *body.Attrs
	}
	var out appointments.Appointment
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.appointments.Create(r.Context(), tx, in); err != nil {
			return fmt.Errorf("create appointment: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenAppointment(out))
}

// GetAppointment answers one appointment.
func (s *Server) GetAppointment(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	out, err := s.appointments.Get(r.Context(), s.pool, id)
	if err != nil {
		writeError(w, r, fmt.Errorf("get appointment: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, toGenAppointment(out))
}

// UpdateAppointment changes the fields sent. Only notes and paid_cents may be null.
func (s *Server) UpdateAppointment(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.AppointmentPatch
	if decodePatch(w, r, &body, "notes", "paid_cents") != nil {
		return
	}
	patch := appointments.AppointmentPatch{
		StartsAt: body.StartsAt, EndsAt: body.EndsAt, ServiceIDs: body.ServiceIds,
		Notes: body.Notes, PaidCents: body.PaidCents,
		AllowOverlap: body.AllowOverlap != nil && *body.AllowOverlap,
	}
	if body.Attrs != nil {
		patch.Attrs = *body.Attrs
	}
	var out appointments.Appointment
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.appointments.Update(r.Context(), tx, id, patch); err != nil {
			return fmt.Errorf("update appointment: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenAppointment(out))
}

// SetAppointmentStatus moves an appointment to another status. An impossible change is a 409
// invalid-transition; a change to scheduled runs the overlap check unless allow_overlap is true.
func (s *Server) SetAppointmentStatus(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.AppointmentStatusChange
	if decodeJSON(w, r, &body) != nil {
		return
	}
	to, err := appointments.ParseStatus(string(body.Status))
	if err != nil {
		writeError(w, r, err)
		return
	}
	var out appointments.Appointment
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.appointments.SetStatus(r.Context(), tx, id, to, body.AllowOverlap != nil && *body.AllowOverlap); err != nil {
			return fmt.Errorf("set appointment status: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenAppointment(out))
}

func toGenAppointments(list []appointments.Appointment) []gen.Appointment {
	items := make([]gen.Appointment, 0, len(list))
	for _, a := range list {
		items = append(items, toGenAppointment(a))
	}
	return items
}

func toGenAppointment(a appointments.Appointment) gen.Appointment {
	lines := make([]gen.AppointmentService, 0, len(a.Services))
	for _, l := range a.Services {
		lines = append(lines, gen.AppointmentService{
			ServiceId: l.ServiceID, Name: l.Name, PriceCents: nullval.Of(l.PriceCents), DurationMin: l.DurationMin,
		})
	}
	return gen.Appointment{
		Id: a.ID, ClientId: a.ClientID, ClientName: a.ClientName,
		StartsAt: a.StartsAt.UTC(), EndsAt: a.EndsAt.UTC(), Status: gen.AppointmentStatus(a.Status),
		Done: a.Done, TotalCents: a.TotalCents, PaidCents: nullval.Of(a.PaidCents),
		Attrs: a.Attrs, Notes: nullval.Of(a.Notes), Services: lines,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}
