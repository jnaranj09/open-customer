package httpapi

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// readParts reads a multipart/form-data body into the bytes of each part. limits holds the part
// names that are accepted and, for each, how many bytes are read (one more than the largest
// accepted size, so the module can tell an oversize part). A body that is not form-data is 415; a
// repeated or unknown part, or broken multipart, is 422; a body over the request limit is 413.
// Which parts are required is the caller's check. Its errors are problems only (never a reader
// error, whose text can quote the body).
func readParts(r *http.Request, limits map[string]int64) (map[string][]byte, error) {
	// MultipartReader also takes multipart/mixed and other multipart types; only form-data is ours.
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/form-data" {
		return nil, problem.UnsupportedMediaType{}
	}
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, problem.Validation{}
	}
	parts := make(map[string][]byte, len(limits))
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, bodyProblem(err)
		}
		name := part.FormName()
		limit, known := limits[name]
		if _, seen := parts[name]; seen || !known {
			return nil, problem.Validation{}
		}
		data, err := io.ReadAll(io.LimitReader(part, limit))
		if err != nil {
			return nil, bodyProblem(err)
		}
		parts[name] = data
	}
	return parts, nil
}

// bodyProblem is payload-too-large for a body over the request limit and validation for any other
// multipart read error.
func bodyProblem(err error) error {
	if tooBig := new(http.MaxBytesError); errors.As(err, &tooBig) {
		return problem.PayloadTooLarge{}
	}
	return problem.Validation{}
}

// uploadRejectCode is the code= of a refused upload.
func uploadRejectCode(err error) string {
	switch {
	case errors.As(err, &problem.PayloadTooLarge{}):
		return "payload_too_large"
	case errors.As(err, &problem.UnsupportedMediaType{}):
		return "not_multipart"
	default:
		return "body_invalid"
	}
}

// withoutFileText returns err only when its text is known to be safe to log: the problems
// SetIcons, Icon and media.Save return, and a Postgres error (writeError redacts it). Any other
// error (a file error, or another error whose text is not known to be safe) is replaced by op and
// a fixed text, because problem.Write logs an internal error's text and a path must never reach
// the log.
func withoutFileText(op string, err error) error {
	var pgErr *pgconn.PgError
	if err == nil || errors.As(err, &problem.UnsupportedMediaType{}) || errors.As(err, &problem.PayloadTooLarge{}) ||
		errors.As(err, &problem.NotFound{}) || errors.As(err, &problem.Validation{}) || errors.As(err, &pgErr) {
		return err
	}
	return fmt.Errorf("%s: error text withheld (it can name a path)", op)
}
