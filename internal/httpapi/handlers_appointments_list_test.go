//go:build db

package httpapi

import (
	"net/http"
	"testing"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// listPath is GET /appointments for the local days from to to.
func listPath(from, to string) string { return appointmentsPath + "?from=" + from + "&to=" + to }

func TestListAppointmentsBadDateIs422(t *testing.T) {
	f := newBookingFixture(t)
	assertProblem(t, f.send(http.MethodGet, listPath("2026-13-01", "2026-11-02"), ""), http.StatusUnprocessableEntity, ":validation")
}

func TestListAppointmentsMissingToIs422(t *testing.T) {
	f := newBookingFixture(t)
	assertProblem(t, f.send(http.MethodGet, appointmentsPath+"?from=2026-11-01", ""), http.StatusUnprocessableEntity, ":validation")
}

func TestListAppointmentsOver31DaysIs422(t *testing.T) {
	f := newBookingFixture(t)
	errs := fieldErrors(t, f.send(http.MethodGet, listPath("2026-11-01", "2026-12-03"), ""))
	if len(errs) != 1 || errs[0].Field != "to" || errs[0].Message != problem.MsgRangeInvalid {
		t.Fatalf("errors = %+v, want one on to with the range message", errs)
	}
}

func TestListAppointmentsAt31DaysIs200(t *testing.T) {
	f := newBookingFixture(t)
	decodeInto[gen.AppointmentList](t, f.send(http.MethodGet, listPath("2026-11-01", "2026-12-02"), ""), http.StatusOK)
}

func TestListAppointmentsToBeforeFromIs422(t *testing.T) {
	f := newBookingFixture(t)
	errs := fieldErrors(t, f.send(http.MethodGet, listPath("2026-11-05", "2026-11-04"), ""))
	if len(errs) != 1 || errs[0].Field != "to" || errs[0].Message != problem.MsgRangeInvalid {
		t.Fatalf("errors = %+v, want one on to with the range message", errs)
	}
}

func TestListAppointmentsEmptyIsArray(t *testing.T) {
	f := newBookingFixture(t)
	list := decodeInto[gen.AppointmentList](t, f.send(http.MethodGet, listPath("2026-11-01", "2026-11-02"), ""), http.StatusOK)
	if list.Items == nil || len(list.Items) != 0 {
		t.Fatalf("items = %#v, want an empty non-nil slice", list.Items)
	}
}

// The setup zone is Europe/Madrid (UTC+1 in November): 22:30Z is 23:30 on the 10th and 23:30Z is
// 00:30 on the 11th.
func TestListAppointmentsLocalDay(t *testing.T) {
	f := newBookingFixture(t)
	late := decodeInto[gen.Appointment](t, f.send(http.MethodPost, appointmentsPath,
		f.bookBody("2026-11-10T22:30:00Z", `,"ends_at":"2026-11-10T23:00:00Z"`)), http.StatusCreated)
	next := decodeInto[gen.Appointment](t, f.send(http.MethodPost, appointmentsPath,
		f.bookBody("2026-11-10T23:30:00Z", `,"ends_at":"2026-11-10T23:45:00Z"`)), http.StatusCreated)
	tenth := decodeInto[gen.AppointmentList](t, f.send(http.MethodGet, listPath("2026-11-10", "2026-11-10"), ""), http.StatusOK)
	if len(tenth.Items) != 1 || tenth.Items[0].Id != late.Id {
		t.Fatalf("local 10th = %+v, want only the 23:30 local appointment", tenth.Items)
	}
	eleventh := decodeInto[gen.AppointmentList](t, f.send(http.MethodGet, listPath("2026-11-11", "2026-11-11"), ""), http.StatusOK)
	if len(eleventh.Items) != 1 || eleventh.Items[0].Id != next.Id {
		t.Fatalf("local 11th = %+v, want only the 00:30 local appointment", eleventh.Items)
	}
}

func TestListAppointmentsIncludesEveryStatus(t *testing.T) {
	f := newBookingFixture(t)
	made := f.book(t, "2026-11-10T10:00:00Z")
	f.send(http.MethodPost, statusPath(made.Id), `{"status":"cancelled"}`)
	list := decodeInto[gen.AppointmentList](t, f.send(http.MethodGet, listPath("2026-11-10", "2026-11-10"), ""), http.StatusOK)
	if len(list.Items) != 1 || list.Items[0].Status != gen.AppointmentStatusCancelled {
		t.Fatalf("items = %+v, want the cancelled appointment", list.Items)
	}
}
