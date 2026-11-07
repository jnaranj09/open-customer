package fixtures

import (
	"context"
	"net/http"
)

type pool struct{}

type clientsModule struct{}

func (clientsModule) List(ctx context.Context, db *pool) error   { return nil }
func (clientsModule) Create(ctx context.Context, db any) error { return nil }

type Server struct {
	pool    *pool
	clients clientsModule
}

func (s *Server) inWriteTx(w http.ResponseWriter, r *http.Request, fn func(tx any) error) bool {
	return fn(nil) == nil
}

// A read handler (GET) may read from the pool.
func (s *Server) ListClients(w http.ResponseWriter, r *http.Request) {
	// ok: non-get-handler-in-withtx
	s.clients.List(r.Context(), s.pool)
}

func (s *Server) GetClient(w http.ResponseWriter, r *http.Request) {
	// ok: non-get-handler-in-withtx
	s.clients.List(r.Context(), s.pool)
}

// A non-GET handler runs its module call in the transaction.
func (s *Server) CreateClient(w http.ResponseWriter, r *http.Request) {
	// ok: non-get-handler-in-withtx
	s.inWriteTx(w, r, func(tx any) error {
		return s.clients.Create(r.Context(), tx)
	})
}

// A non-GET handler that goes around the transaction, in any verb the API uses.
func (s *Server) UpdateClient(w http.ResponseWriter, r *http.Request) {
	// ruleid: non-get-handler-in-withtx
	s.clients.Create(r.Context(), s.pool)
}

func (s *Server) SetAppointmentStatus(w http.ResponseWriter, r *http.Request) {
	// ruleid: non-get-handler-in-withtx
	s.clients.List(r.Context(), s.pool)
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	// ruleid: non-get-handler-in-withtx
	s.clients.List(r.Context(), s.pool)
}

func (s *Server) UploadMedia(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength == 0 {
		// ruleid: non-get-handler-in-withtx
		s.clients.Create(r.Context(), s.pool)
	}
}

func (s *Server) LogClientError(w http.ResponseWriter, r *http.Request) {
	// ruleid: non-get-handler-in-withtx
	s.clients.List(r.Context(), s.pool)
}
