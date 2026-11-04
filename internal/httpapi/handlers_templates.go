package httpapi

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/reminders"
	"github.com/jnaranj09/open-customer/internal/templates"
)

// CreateTemplate adds a template. The module runs the save check on the body.
func (s *Server) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var body gen.TemplateInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	var out templates.Template
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.templates.Create(r.Context(), tx, templates.TemplateInput{Name: body.Name, Body: body.Body}); err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenTemplate(out))
}

// UpdateTemplate changes the fields sent. No field may be null.
func (s *Server) UpdateTemplate(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.TemplatePatch
	if decodePatch(w, r, &body) != nil {
		return
	}
	patch := templates.TemplatePatch{Name: body.Name, Body: body.Body, Archived: body.Archived}
	var out templates.Template
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.templates.Update(r.Context(), tx, id, patch); err != nil {
			return fmt.Errorf("update template: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenTemplate(out))
}

func toGenTemplate(t templates.Template) gen.Template {
	return gen.Template{Id: t.ID, Name: t.Name, Body: t.Body, Archived: t.Archived}
}

// PreviewTemplate renders a body for the sample record or for a client. It runs in a write
// transaction like every non-GET handler that reads the database.
func (s *Server) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var body gen.TemplatePreviewInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	var clientID uuid.NullUUID
	if body.ClientId != nil {
		clientID = uuid.NullUUID{UUID: *body.ClientId, Valid: true}
	}
	var out reminders.Preview
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.reminders.Preview(r.Context(), tx, body.Body, clientID); err != nil {
			return fmt.Errorf("preview template: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, gen.TemplatePreview{Text: out.Text, UnknownVars: out.UnknownVars})
}
