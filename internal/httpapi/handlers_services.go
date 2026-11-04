package httpapi

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/nullval"
	"github.com/jnaranj09/open-customer/internal/services"
)

// CreateService adds a service. A missing or null price_cents means no price.
func (s *Server) CreateService(w http.ResponseWriter, r *http.Request) {
	var body gen.ServiceInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := services.ServiceInput{Name: body.Name, DurationMin: body.DurationMin, Color: body.Color}
	if price, err := body.PriceCents.Get(); err == nil {
		in.PriceCents = &price
	}
	var out services.Service
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.services.Create(r.Context(), tx, in); err != nil {
			return fmt.Errorf("create service: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenService(out))
}

// UpdateService changes the fields sent. Only price_cents may be null (it clears the price).
func (s *Server) UpdateService(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.ServicePatch
	if decodePatch(w, r, &body, "price_cents") != nil {
		return
	}
	patch := services.ServicePatch{
		Name: body.Name, DurationMin: body.DurationMin, PriceCents: body.PriceCents,
		Color: body.Color, Position: body.Position, Archived: body.Archived,
	}
	var out services.Service
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = s.services.Update(r.Context(), tx, id, patch); err != nil {
			return fmt.Errorf("update service: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenService(out))
}

func toGenService(v services.Service) gen.Service {
	return gen.Service{
		Id: v.ID, Name: v.Name, DurationMin: v.DurationMin, PriceCents: nullval.Of(v.PriceCents),
		Color: v.Color, Position: v.Position, Archived: v.Archived,
	}
}
