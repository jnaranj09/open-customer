package httpapi

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/reminders"
)

// CreateReminderRule adds a rule. The module parses the kind and checks the rest of the body.
func (s *Server) CreateReminderRule(w http.ResponseWriter, r *http.Request) {
	var body gen.RuleInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := reminders.RuleInput{
		Kind:            reminders.Kind(body.Kind),
		ServiceID:       nullUUID(body.ServiceId),
		ReturnServiceID: nullUUID(body.ReturnServiceId),
		OffsetDays:      body.OffsetDays,
		TemplateID:      body.TemplateId,
		IncludeOverdue:  body.IncludeOverdue != nil && *body.IncludeOverdue,
	}
	var out reminders.Rule
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.reminders.CreateRule(r.Context(), tx, in); err != nil {
			return fmt.Errorf("create reminder rule: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenRule(out))
}

// UpdateReminderRule changes the fields sent. Only return_service_id may be null.
func (s *Server) UpdateReminderRule(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.RulePatch
	if decodePatch(w, r, &body, "return_service_id") != nil {
		return
	}
	patch := reminders.RulePatch{
		OffsetDays: body.OffsetDays, TemplateID: body.TemplateId, ReturnServiceID: body.ReturnServiceId,
		Archived: body.Archived, IncludeOverdue: body.IncludeOverdue != nil && *body.IncludeOverdue,
	}
	var out reminders.Rule
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.reminders.UpdateRule(r.Context(), tx, id, patch); err != nil {
			return fmt.Errorf("update reminder rule: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenRule(out))
}

// toGenRule is the wire form of a rule.
func toGenRule(r reminders.Rule) gen.Rule {
	return gen.Rule{
		Id: r.ID, Kind: gen.RuleKind(r.Kind), ServiceId: nullableUUID(r.ServiceID),
		ReturnServiceId: nullableUUID(r.ReturnServiceID), OffsetDays: r.OffsetDays,
		TemplateId: r.TemplateID, AppliesFrom: nullval.Of(r.AppliesFrom), Archived: r.Archived,
	}
}

// nullUUID is a request id field as the module takes it: absent and null are both "none".
func nullUUID(n nullable.Nullable[uuid.UUID]) uuid.NullUUID {
	if v := nullval.Ptr(n); v != nil {
		return uuid.NullUUID{UUID: *v, Valid: true}
	}
	return uuid.NullUUID{}
}

// nullableUUID is a response id field: none is an explicit null, so the key is always present.
func nullableUUID(n uuid.NullUUID) nullable.Nullable[uuid.UUID] {
	if !n.Valid {
		return nullable.NewNullNullable[uuid.UUID]()
	}
	return nullable.NewNullableWithValue(n.UUID)
}
