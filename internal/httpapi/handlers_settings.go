package httpapi

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/settings"
)

const (
	// maxIconRequest is the POST /settings/icon body limit: two parts of at most 1 MB each, plus
	// the multipart framing.
	maxIconRequest = 3 << 20
	// maxIconPart is how much of one part the handler reads; settings.SetIcons refuses a part
	// over 1 MB, so one byte more is enough to tell.
	maxIconPart = 1<<20 + 1
	// The multipart part names of POST /settings/icon (the spec's IconUpload).
	icon512Part = "icon512"
	icon180Part = "icon180"
	// The icon sizes settings.Icon serves.
	icon512Size = 512
	icon180Size = 180
)

// UpdateSettings changes the fields sent and answers the stored settings, after the commit. No
// field may be null.
func (s *Server) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var body gen.SettingsPatch
	if decodePatch(w, r, &body) != nil {
		return
	}
	patch := settings.SettingsPatch{
		Name: body.Name, Timezone: body.Timezone, AccentColor: body.AccentColor, PushHour: body.PushHour,
	}
	var out settings.Settings
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.settings.Update(r.Context(), tx, patch); err != nil {
			return fmt.Errorf("update settings: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, gen.Settings{
		Name: out.Name, Timezone: out.Timezone, Locale: out.Locale, Currency: out.Currency,
		CountryCode: out.CountryCode, AccentColor: out.AccentColor, PushHour: out.PushHour,
		HasCustomIcon: out.HasCustomIcon,
	})
}

// UploadIcon stores the two icon PNGs. The body is at most 3 MB and each part is read up to one
// byte past 1 MB; settings.SetIcons checks the PNG signature and the 1 MB limit. A body that is
// not multipart/form-data is 415; a missing, repeated or unknown part, or broken multipart, is 422.
func (s *Server) UploadIcon(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxIconRequest)
	icons, err := readIconParts(r)
	if err != nil {
		slog.WarnContext(r.Context(), "icon upload rejected",
			"code", uploadRejectCode(err), "path", r.URL.Path, "request_id", logging.RequestID(r.Context()))
		problem.Write(w, r, err)
		return
	}
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		err := s.settings.SetIcons(r.Context(), tx, icons[icon512Part], icons[icon180Part])
		return withoutFileText("set icons", err)
	}) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readIconParts reads exactly the parts icon512 and icon180. Its errors are problems only (never
// a reader error, whose text can quote the body).
func readIconParts(r *http.Request) (map[string][]byte, error) {
	icons, err := readParts(r, map[string]int64{icon512Part: maxIconPart, icon180Part: maxIconPart})
	if err != nil {
		return nil, err
	}
	if len(icons) != 2 {
		return nil, problem.Validation{}
	}
	return icons, nil
}

// manifest serves the web app manifest. It has no session on purpose: the browser fetches it
// without cookies, before sign-in and at install. no-cache makes the browser revalidate, so iOS
// reads the current name when she adds the app to the Home Screen.
func (s *Server) manifest(w http.ResponseWriter, r *http.Request) {
	b, err := s.settings.Manifest(r.Context(), s.pool)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(b); err != nil {
		slog.WarnContext(r.Context(), "response not written", "code", "response_write_failed")
	}
}

// icon512 and appleTouchIcon serve the app icons. They have no session on purpose: the manifest,
// the browser and iOS fetch them without cookies, also before setup (the default icon).
func (s *Server) icon512(w http.ResponseWriter, r *http.Request) { s.icon(w, r, icon512Size) }

func (s *Server) appleTouchIcon(w http.ResponseWriter, r *http.Request) { s.icon(w, r, icon180Size) }

// icon serves the PNG of size with no-cache and, for an upload, its time as Last-Modified (the
// default has a zero time, so no Last-Modified).
func (s *Server) icon(w http.ResponseWriter, r *http.Request, size int) {
	b, updatedAt, err := s.settings.Icon(r.Context(), s.pool, size)
	if err != nil {
		writeError(w, r, withoutFileText("read icon", err))
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", updatedAt, bytes.NewReader(b))
}
