package httpapi

import (
	"errors"
	"testing"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func TestConflictProblemLeavesOtherErrorsAlone(t *testing.T) {
	other := errors.New("boom")
	if got := conflictProblem(other); got != other {
		t.Fatalf("got %v, want the same error back", got)
	}
}

func TestConflictProblemDuplicatePhoneListsClients(t *testing.T) {
	got := conflictProblem(clients.DuplicatePhoneError{Clients: []clients.Client{{Name: "Lucía Rojas"}}})
	var c problem.Conflict
	if !errors.As(got, &c) || c.Name != "duplicate-phone" || len(c.Extra) != 1 {
		t.Fatalf("got %#v, want a duplicate-phone conflict with one extension", got)
	}
}

func TestConflictProblemOverlapListsAppointments(t *testing.T) {
	got := conflictProblem(appointments.OverlapError{Appointments: []appointments.Appointment{{ClientName: "Lucía Rojas"}}})
	var c problem.Conflict
	if !errors.As(got, &c) || c.Name != "overlap" || len(c.Extra) != 1 {
		t.Fatalf("got %#v, want an overlap conflict with one extension", got)
	}
}
