package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/media"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/logging"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	// maxMediaRequest is the POST /media body limit: a 10 MB image, a 1 MB thumbnail and the
	// multipart framing.
	maxMediaRequest = 12 << 20
	// maxMediaIDPart is how much of an ID part is read; a UUID is 36 bytes.
	maxMediaIDPart = 64
	// The multipart part names of POST /media (the spec's MediaUpload).
	mediaFullPart        = "full"
	mediaThumbPart       = "thumb"
	mediaClientIDPart    = "client_id"
	mediaAppointmentPart = "appointment_id"
)

// UploadMedia stores one photo. The body is at most 12 MB; media.Save checks the JPEG signature,
// the sizes and the owner. A body that is not multipart/form-data is 415, a part over its size is
// 413, and a missing, repeated or unknown part, a bad ID or a bad owner is 422. When the commit
// fails after Save wrote the files, the files are removed.
func (s *Server) UploadMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaRequest)
	in, err := readMediaParts(r)
	if err != nil {
		slog.WarnContext(r.Context(), "media upload rejected",
			"code", uploadRejectCode(err), "path", r.URL.Path, "request_id", logging.RequestID(r.Context()))
		problem.Write(w, r, err)
		return
	}
	var out media.Media
	saved := false
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.media.Save(r.Context(), tx, in); err != nil {
			return withoutFileText("save media", err)
		}
		saved = true
		return nil
	}) {
		if saved {
			s.media.Discard(r.Context(), out)
		}
		return
	}
	writeJSON(w, http.StatusCreated, toGenMedia(out))
}

// readMediaParts reads the parts of POST /media into a media.MediaInput. Every bad field is
// reported in one 422.
func readMediaParts(r *http.Request) (media.MediaInput, error) {
	parts, err := readParts(r, map[string]int64{
		mediaFullPart:        media.MaxFullBytes + 1,
		mediaThumbPart:       media.MaxThumbBytes + 1,
		mediaClientIDPart:    maxMediaIDPart,
		mediaAppointmentPart: maxMediaIDPart,
	})
	if err != nil {
		return media.MediaInput{}, err
	}
	var errs []problem.FieldError
	for _, name := range []string{mediaFullPart, mediaThumbPart} {
		if _, ok := parts[name]; !ok {
			errs = append(errs, problem.FieldError{Field: name, Message: problem.MsgFieldRequired})
		}
	}
	in := media.MediaInput{Full: parts[mediaFullPart], Thumb: parts[mediaThumbPart]}
	for _, owner := range []struct {
		name string
		dst  *uuid.NullUUID
	}{{mediaClientIDPart, &in.ClientID}, {mediaAppointmentPart, &in.AppointmentID}} {
		name, dst := owner.name, owner.dst
		raw, ok := parts[name]
		if !ok {
			continue
		}
		id, err := ids.Parse(string(raw))
		if err != nil {
			errs = append(errs, problem.FieldError{Field: name, Message: problem.MsgIDInvalid})
			continue
		}
		*dst = uuid.NullUUID{UUID: id, Valid: true}
	}
	if len(errs) > 0 {
		return media.MediaInput{}, problem.Validation{Errors: errs}
	}
	return in, nil
}

// GetMedia serves the full image of a photo, archived or not.
func (s *Server) GetMedia(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	s.serveMedia(w, r, id, false)
}

// GetMediaThumb serves the thumbnail of a photo, archived or not.
func (s *Server) GetMediaThumb(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	s.serveMedia(w, r, id, true)
}

// serveMedia sends the file with http.ServeContent. A media ID never changes content, so the
// browser may keep it for a year.
func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request, id uuid.UUID, thumb bool) {
	f, md, err := s.media.Open(r.Context(), s.pool, id, thumb)
	if err != nil {
		writeError(w, r, withoutFileText("open media", err))
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.WarnContext(r.Context(), "media file not closed", "code", "media_close_failed")
		}
	}()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "", md.CreatedAt, f)
}

// UpdateMedia archives or restores a photo. archived is required.
func (s *Server) UpdateMedia(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.MediaPatch
	if decodePatch(w, r, &body) != nil {
		return
	}
	if body.Archived == nil {
		problem.Write(w, r, problem.Validation{Errors: []problem.FieldError{
			{Field: "archived", Message: problem.MsgFieldRequired},
		}})
		return
	}
	var out media.Media
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.media.SetArchived(r.Context(), tx, id, *body.Archived); err != nil {
			return fmt.Errorf("archive media: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenMedia(out))
}

// ListClientMedia answers the client's active photos, newest first.
func (s *Server) ListClientMedia(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	list, err := s.media.ListForClient(r.Context(), s.pool, id)
	if err != nil {
		writeError(w, r, fmt.Errorf("list media: %w", err))
		return
	}
	items := make([]gen.Media, 0, len(list))
	for _, m := range list {
		items = append(items, toGenMedia(m))
	}
	writeJSON(w, http.StatusOK, gen.MediaList{Items: items})
}

func toGenMedia(m media.Media) gen.Media {
	var appointmentID *uuid.UUID
	if m.AppointmentID.Valid {
		appointmentID = &m.AppointmentID.UUID
	}
	return gen.Media{
		Id: m.ID, ClientId: m.ClientID, AppointmentId: nullval.Of(appointmentID),
		Width: m.Width, Height: m.Height, Bytes: m.Bytes, CreatedAt: m.CreatedAt, Archived: m.Archived,
	}
}
