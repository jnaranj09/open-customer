// Package problem holds the API's error values (one per row of the design doc's error table) and
// writes them as application/problem+json (RFC 9457).
package problem

import (
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jnaranj09/open-customer/internal/platform/logging"
)

// TypePrefix starts every problem `type`; the problem name follows it.
const TypePrefix = "urn:open-customer:problem:"

type row struct {
	status int
	title  string // shown to her as is
}

// rows is the error table from the design doc (item 15). The titles are Spanish on purpose: the
// web app shows them unchanged.
var rows = map[string]row{
	"validation":             {http.StatusUnprocessableEntity, "Revisa los datos"},
	"not-found":              {http.StatusNotFound, "No encontrado"},
	"unauthorized":           {http.StatusUnauthorized, "Inicia sesión de nuevo"},
	"invalid-credentials":    {http.StatusUnauthorized, "Usuario o contraseña incorrectos"},
	"must-change-password":   {http.StatusForbidden, "Debes cambiar tu contraseña"},
	"forbidden-origin":       {http.StatusForbidden, "Solicitud no permitida"},
	"setup-closed":           {http.StatusConflict, "La configuración ya se completó"},
	"setup-token-invalid":    {http.StatusGone, "El enlace de configuración ya no sirve"},
	"overlap":                {http.StatusConflict, "Ya hay una cita a esa hora"},
	"duplicate-phone":        {http.StatusConflict, "Otra clienta tiene ese teléfono"},
	"invalid-transition":     {http.StatusConflict, "Ese cambio de estado no es posible"},
	"payload-too-large":      {http.StatusRequestEntityTooLarge, "El archivo es muy grande"},
	"unsupported-media-type": {http.StatusUnsupportedMediaType, "Tipo de archivo no permitido"},
	"rate-limited":           {http.StatusTooManyRequests, "Demasiados intentos. Espera un momento"},
	"internal":               {http.StatusInternalServerError, "Algo salió mal"},
}

// Names returns the problem names of the error table, sorted. The httpapi drift test compares
// them with the Problem.type enum in api/openapi.yaml.
func Names() []string {
	return slices.Sorted(maps.Keys(rows))
}

// problemError is implemented by every value below.
type problemError interface {
	error
	problemName() string
	extensions() map[string]any
}

// FieldError is one entry of a validation problem's errors[]. Message is Spanish.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validation is a 422 with the bad fields.
type Validation struct{ Errors []FieldError }

// Field returns a Validation with the one bad field name and its message.
func Field(name, msg string) error {
	return Validation{Errors: []FieldError{{Field: name, Message: msg}}}
}

// CollectFields appends the field errors of err to errs when err is a Validation, and reports
// whether it was. A nil err or any other error is not, so the caller returns it instead.
func CollectFields(errs *[]FieldError, err error) bool {
	var v Validation
	if !errors.As(err, &v) {
		return false
	}
	*errs = append(*errs, v.Errors...)
	return true
}

// NotFound is a 404.
type NotFound struct{}

// Unauthorized is a 401: no valid session.
type Unauthorized struct{}

// InvalidCredentials is a 401 from the login form.
type InvalidCredentials struct{}

// MustChangePassword is a 403 until she sets a new password.
type MustChangePassword struct{}

// ForbiddenOrigin is a 403 from the Origin check.
type ForbiddenOrigin struct{}

// SetupClosed is a 409: a user already exists.
type SetupClosed struct{}

// SetupTokenInvalid is a 410: the setup link is used or expired.
type SetupTokenInvalid struct{}

// Conflict is a 409. Name is "overlap", "duplicate-phone" or "invalid-transition"; Extra holds the
// extension fields ("appointments" or "clients").
type Conflict struct {
	Name  string
	Extra map[string]any
}

// PayloadTooLarge is a 413.
type PayloadTooLarge struct{}

// UnsupportedMediaType is a 415.
type UnsupportedMediaType struct{}

// RateLimited is a 429; RetryAfter goes to the Retry-After header and retry_after_s.
type RateLimited struct{ RetryAfter time.Duration }

func (Validation) problemName() string           { return "validation" }
func (NotFound) problemName() string             { return "not-found" }
func (Unauthorized) problemName() string         { return "unauthorized" }
func (InvalidCredentials) problemName() string   { return "invalid-credentials" }
func (MustChangePassword) problemName() string   { return "must-change-password" }
func (ForbiddenOrigin) problemName() string      { return "forbidden-origin" }
func (SetupClosed) problemName() string          { return "setup-closed" }
func (SetupTokenInvalid) problemName() string    { return "setup-token-invalid" }
func (c Conflict) problemName() string           { return c.Name }
func (PayloadTooLarge) problemName() string      { return "payload-too-large" }
func (UnsupportedMediaType) problemName() string { return "unsupported-media-type" }
func (RateLimited) problemName() string          { return "rate-limited" }

func (v Validation) extensions() map[string]any {
	errs := v.Errors
	if errs == nil {
		errs = []FieldError{}
	}
	return map[string]any{"errors": errs}
}
func (NotFound) extensions() map[string]any             { return nil }
func (Unauthorized) extensions() map[string]any         { return nil }
func (InvalidCredentials) extensions() map[string]any   { return nil }
func (MustChangePassword) extensions() map[string]any   { return nil }
func (ForbiddenOrigin) extensions() map[string]any      { return nil }
func (SetupClosed) extensions() map[string]any          { return nil }
func (SetupTokenInvalid) extensions() map[string]any    { return nil }
func (c Conflict) extensions() map[string]any           { return c.Extra }
func (PayloadTooLarge) extensions() map[string]any      { return nil }
func (UnsupportedMediaType) extensions() map[string]any { return nil }
func (r RateLimited) extensions() map[string]any {
	return map[string]any{"retry_after_s": r.seconds()}
}

func (r RateLimited) seconds() int { return int(math.Ceil(max(r.RetryAfter, time.Second).Seconds())) }

func (v Validation) Error() string           { return "problem: " + v.problemName() }
func (e NotFound) Error() string             { return "problem: " + e.problemName() }
func (e Unauthorized) Error() string         { return "problem: " + e.problemName() }
func (e InvalidCredentials) Error() string   { return "problem: " + e.problemName() }
func (e MustChangePassword) Error() string   { return "problem: " + e.problemName() }
func (e ForbiddenOrigin) Error() string      { return "problem: " + e.problemName() }
func (e SetupClosed) Error() string          { return "problem: " + e.problemName() }
func (e SetupTokenInvalid) Error() string    { return "problem: " + e.problemName() }
func (c Conflict) Error() string             { return "problem: " + c.problemName() }
func (e PayloadTooLarge) Error() string      { return "problem: " + e.problemName() }
func (e UnsupportedMediaType) Error() string { return "problem: " + e.problemName() }
func (r RateLimited) Error() string          { return "problem: " + r.problemName() }

// Write writes err as problem JSON. An error that is not one of the values above (or a Conflict
// with an unknown Name) is "internal": the client gets only the generic title, and the log gets the
// error with the request ID.
func Write(w http.ResponseWriter, r *http.Request, err error) {
	name, ext := "internal", map[string]any(nil)
	var p problemError
	if errors.As(err, &p) {
		if _, ok := rows[p.problemName()]; ok {
			name, ext = p.problemName(), p.extensions()
		}
	}
	if name == "internal" {
		slog.ErrorContext(r.Context(), "internal error",
			"code", "internal", "request_id", logging.RequestID(r.Context()), "err", err)
	}
	var rl RateLimited
	if errors.As(err, &rl) && name == "rate-limited" {
		w.Header().Set("Retry-After", strconv.Itoa(rl.seconds()))
	}

	rw := rows[name]
	body := make(map[string]any, len(ext)+3)
	for k, v := range ext {
		body[k] = v
	}
	body["type"] = TypePrefix + name
	body["title"] = rw.title
	body["status"] = rw.status

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(rw.status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.WarnContext(r.Context(), "problem body not written",
			"code", "problem_write_failed", "request_id", logging.RequestID(r.Context()))
	}
}
