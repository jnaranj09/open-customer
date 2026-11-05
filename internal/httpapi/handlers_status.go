package httpapi

import (
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/status"
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

// GetStatus answers the status screen data. The pool is enough: it is a plain read.
func (s *Server) GetStatus(w http.ResponseWriter, r *http.Request) {
	rep, err := s.status.Status(r.Context(), s.pool)
	if err != nil {
		writeError(w, r, fmt.Errorf("get status: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, statusBody(rep))
}

// lastPushBody is the generated anonymous type of Status.push.last_push; the tags must match it.
type lastPushBody = struct {
	Attempted int                `json:"attempted"`
	Date      openapi_types.Date `json:"date"`
	Delivered int                `json:"delivered"`
}

// statusBody converts the report to its response type; a nil pointer is an explicit null.
func statusBody(rep status.Report) gen.Status {
	out := gen.Status{
		Version:          rep.Version,
		MigrationVersion: rep.MigrationVersion,
		HeartbeatAgeS:    nullval.Of(rep.HeartbeatAgeS),
		MediaFreeBytes:   nullval.Of(rep.MediaFreeBytes),
		Backup: gen.BackupStatus{
			CheckedAt:  nullTime(rep.Backup.CheckedAt),
			LastDumpAt: nullTime(rep.Backup.LastDumpAt),
			LastCopyAt: nullTime(rep.Backup.LastCopyAt),
			LastError:  nullval.Of(rep.Backup.LastError),
			Red:        rep.Backup.Red,
		},
		Push: gen.PushStatus{
			Subscriptions: rep.Push.Subscriptions,
			LastPush:      nullable.NewNullNullable[lastPushBody](),
			Red:           rep.Push.Red,
		},
	}
	if lp := rep.Push.LastPush; lp != nil {
		// The stored date is the business-local day the timer wrote with its own layout; a value
		// that does not parse is reported as no last push rather than failing the screen.
		if day, err := time.Parse(time.DateOnly, lp.Date); err == nil {
			out.Push.LastPush = nullable.NewNullableWithValue(lastPushBody{
				Attempted: lp.Attempted, Date: openapi_types.Date{Time: day}, Delivered: lp.Delivered,
			})
		}
	}
	return out
}

// LogClientError logs an error the PWA caught. It opens no transaction: it touches no table (O3).
// The rate limit is per session, keyed by the hex of the session's token hash.
func (s *Server) LogClientError(w http.ResponseWriter, r *http.Request) {
	var body gen.ClientErrorInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	key := hex.EncodeToString(mustSignedIn(r.Context()).session.TokenHash)
	err := s.status.LogClientError(r.Context(), key, status.ClientError{
		Message: body.Message, Stack: body.Stack, AppVersion: body.AppVersion, Route: body.Route,
	})
	if err != nil {
		writeError(w, r, fmt.Errorf("log client error: %w", err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
