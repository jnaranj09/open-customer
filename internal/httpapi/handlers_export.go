package httpapi

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/exporter"
	"github.com/jnaranj09/open-customer/internal/locale"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
)

// countingWriter counts the bytes that reached the client, so the handler knows whether the
// response has started.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	if err != nil {
		return n, fmt.Errorf("write response: %w", err)
	}
	return n, nil
}

// ExportData streams the ZIP of every record, read in one snapshot. An error before the first byte
// reached the client is a normal problem response. After it the status is sent and cannot change,
// so the handler logs and aborts the connection: the download fails instead of ending as a file
// that looks complete (D19).
func (s *Server) ExportData(w http.ResponseWriter, r *http.Request) {
	out := &countingWriter{w: w}
	err := s.snapshot(r, func(tx pgx.Tx) error {
		loc, err := s.businessZone(r.Context(), tx)
		if err != nil {
			return err
		}
		h := w.Header()
		h.Set("Content-Type", "application/zip")
		h.Set("Content-Disposition", `attachment; filename="`+exporter.FileName(locale.LocalDay(s.clock.Now(), loc))+`"`)
		return s.exporter.Write(r.Context(), tx, out)
	})
	if err == nil {
		return
	}
	if out.n == 0 {
		w.Header().Del("Content-Disposition")
		writeError(w, r, fmt.Errorf("export data: %w", err))
		return
	}
	slog.ErrorContext(r.Context(), "export aborted after the stream started",
		"code", "export_aborted", "request_id", logging.RequestID(r.Context()))
	panic(http.ErrAbortHandler)
}
