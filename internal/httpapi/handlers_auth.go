package httpapi

import (
	"net/http"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// GetSetupStatus has no session on purpose: the setup wizard asks before any user exists (see the
// spec description). It is a GET, so it reads through the pool.
func (s *Server) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	needed, err := s.auth.SetupNeeded(r.Context(), s.pool)
	if err != nil {
		problem.Write(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.SetupStatus{NeedsSetup: needed})
}
