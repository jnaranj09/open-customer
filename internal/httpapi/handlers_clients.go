package httpapi

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/clients"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

// ListClients answers the active clients, or the archived ones with archived=true.
func (s *Server) ListClients(w http.ResponseWriter, r *http.Request, params gen.ListClientsParams) {
	archived := params.Archived != nil && *params.Archived
	list, err := s.clients.List(r.Context(), s.pool, archived)
	if err != nil {
		writeError(w, r, fmt.Errorf("list clients: %w", err))
		return
	}
	items := make([]gen.Client, 0, len(list))
	for _, c := range list {
		items = append(items, toGenClient(c))
	}
	writeJSON(w, http.StatusOK, gen.ClientList{Items: items})
}

// CreateClient adds a client. The business country code (read in the same transaction) is the
// default region of the phone.
func (s *Server) CreateClient(w http.ResponseWriter, r *http.Request) {
	var body gen.ClientInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := clients.ClientInput{Name: body.Name, Phone: body.Phone, Notes: body.Notes}
	if body.Attrs != nil {
		in.Attrs = *body.Attrs
	}
	in.AllowDuplicatePhone = body.AllowDuplicatePhone != nil && *body.AllowDuplicatePhone
	var out clients.Client
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		cfg, err := s.settings.Get(r.Context(), tx)
		if err != nil {
			return fmt.Errorf("get settings: %w", err)
		}
		if out, err = s.clients.Create(r.Context(), tx, in, cfg.CountryCode); err != nil {
			return fmt.Errorf("create client: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusCreated, toGenClient(out))
}

// GetClient answers one client, archived or not.
func (s *Server) GetClient(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	out, err := s.clients.Get(r.Context(), s.pool, id)
	if err != nil {
		writeError(w, r, fmt.Errorf("get client: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, toGenClient(out))
}

// UpdateClient changes the fields sent. Only phone and notes may be null.
func (s *Server) UpdateClient(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.ClientPatch
	if decodePatch(w, r, &body, "phone", "notes") != nil {
		return
	}
	patch := clients.ClientPatch{
		Name: body.Name, Phone: body.Phone, Notes: body.Notes, Archived: body.Archived,
		AllowDuplicatePhone: body.AllowDuplicatePhone != nil && *body.AllowDuplicatePhone,
	}
	if body.Attrs != nil {
		patch.Attrs = *body.Attrs
	}
	var out clients.Client
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		cfg, err := s.settings.Get(r.Context(), tx)
		if err != nil {
			return fmt.Errorf("get settings: %w", err)
		}
		if out, err = s.clients.Update(r.Context(), tx, id, patch, cfg.CountryCode); err != nil {
			return fmt.Errorf("update client: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, http.StatusOK, toGenClient(out))
}

func toGenClient(c clients.Client) gen.Client {
	return gen.Client{
		Id: c.ID, Name: c.Name, PhoneE164: nullableOf(c.PhoneE164), PhoneDigits: nullableOf(c.PhoneDigits),
		Notes: nullableOf(c.Notes), Attrs: c.Attrs, Archived: c.Archived, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}
