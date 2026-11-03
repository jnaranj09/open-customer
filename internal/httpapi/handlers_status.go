package httpapi

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/jnaranj09/open-customer/internal/platform/logging"
)

// healthz has no session: the container health check and the tunnel probe have none. It touches
// no database.
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeProbe(w, r, s.status.Health())
}

// readyz has no session, for the same callers as healthz. It is ready when the database answers
// and every migration is applied.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	writeProbe(w, r, s.status.Ready(r.Context(), s.pool))
}

// writeProbe answers a probe in plain text: 200 "ok", or 503 "not ready".
func writeProbe(w http.ResponseWriter, r *http.Request, ok bool) {
	code, body := http.StatusOK, "ok"
	if !ok {
		code, body = http.StatusServiceUnavailable, "not ready"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	if _, err := io.WriteString(w, body); err != nil {
		slog.WarnContext(r.Context(), "response not written",
			"code", "response_write_failed", "request_id", logging.RequestID(r.Context()))
	}
}
