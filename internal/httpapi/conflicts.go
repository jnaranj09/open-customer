package httpapi

import (
	"errors"

	"github.com/jnaranj09/open-customer/internal/appointments"
	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// conflictProblem turns a typed module conflict into the 409 problem with its extension list as
// gen types (design doc item 24: modules cannot import gen, so the module errors carry their own
// structs and the Go field names never reach the wire). Any other error comes back unchanged.
func conflictProblem(err error) error {
	var overlap appointments.OverlapError
	if errors.As(err, &overlap) {
		return problem.Conflict{Name: "overlap", Extra: map[string]any{"appointments": toGenAppointments(overlap.Appointments)}}
	}
	var dup clients.DuplicatePhoneError
	if errors.As(err, &dup) {
		list := make([]gen.Client, 0, len(dup.Clients))
		for _, c := range dup.Clients {
			list = append(list, toGenClient(c))
		}
		return problem.Conflict{Name: "duplicate-phone", Extra: map[string]any{"clients": list}}
	}
	return err
}
