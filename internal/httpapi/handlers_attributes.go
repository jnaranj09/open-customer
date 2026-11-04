package httpapi

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
)

// CreateAttribute adds an attribute. The scope and type strings are parsed by the module, so an
// unknown one is a 422 on its field.
func (s *Server) CreateAttribute(w http.ResponseWriter, r *http.Request) {
	var body gen.AttributeInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := attributes.AttributeInput{
		Label: body.Label, Scope: attributes.Scope(body.Scope), Type: attributes.Type(body.Type),
	}
	if body.Required != nil {
		in.Required = *body.Required
	}
	var out attributes.Attribute
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.attributes.Create(r.Context(), tx, in); err != nil {
			return fmt.Errorf("create attribute: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenAttribute(out))
}

// UpdateAttribute changes the fields sent. No field may be null.
func (s *Server) UpdateAttribute(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.AttributePatch
	if decodePatch(w, r, &body) != nil {
		return
	}
	patch := attributes.AttributePatch{
		Label: body.Label, Required: body.Required, Position: body.Position, Archived: body.Archived,
	}
	var out attributes.Attribute
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.attributes.Update(r.Context(), tx, id, patch); err != nil {
			return fmt.Errorf("update attribute: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenAttribute(out))
}

// CreateOption adds an option to the select attribute id.
func (s *Server) CreateOption(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.OptionInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := attributes.OptionInput{Label: body.Label}
	if body.ParentId != nil {
		in.ParentID = uuid.NullUUID{UUID: *body.ParentId, Valid: true}
	}
	var out attributes.Option
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.attributes.AddOption(r.Context(), tx, id, in); err != nil {
			return fmt.Errorf("add attribute option: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenOption(out))
}

// UpdateOption changes the fields sent of one option of the attribute id. No field may be null.
func (s *Server) UpdateOption(w http.ResponseWriter, r *http.Request, id, optionID openapi_types.UUID) {
	var body gen.OptionPatch
	if decodePatch(w, r, &body) != nil {
		return
	}
	patch := attributes.OptionPatch{Label: body.Label, Position: body.Position, Archived: body.Archived}
	var out attributes.Option
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.attributes.UpdateOption(r.Context(), tx, id, optionID, patch); err != nil {
			return fmt.Errorf("update attribute option: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenOption(out))
}

// toGenAttribute converts an attribute with its options; the options list is never null.
func toGenAttribute(a attributes.Attribute) gen.Attribute {
	opts := make([]gen.Option, 0, len(a.Options))
	for _, o := range a.Options {
		opts = append(opts, toGenOption(o))
	}
	return gen.Attribute{
		Id: a.ID, Key: a.Key, Label: a.Label, Scope: gen.AttributeScope(a.Scope), Type: gen.AttributeType(a.Type),
		Required: a.Required, Position: a.Position, Archived: a.Archived, Options: opts,
	}
}

func toGenOption(o attributes.Option) gen.Option {
	var parent *uuid.UUID
	if o.ParentID.Valid {
		parent = &o.ParentID.UUID
	}
	return gen.Option{
		Id: o.ID, AttributeId: o.AttributeID, ParentId: nullval.Of(parent), Label: o.Label,
		Position: o.Position, Archived: o.Archived,
	}
}
