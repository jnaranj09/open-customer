package httpapi

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/push"
)

// GetPushKey answers the VAPID public key. It has no session: the key is public.
func (s *Server) GetPushKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.push.PublicKey(r.Context(), s.pool)
	if err != nil {
		writeError(w, r, fmt.Errorf("get push key: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, gen.PushKey{PublicKey: key})
}

// SubscribePush stores the browser's PushSubscription for the signed-in user: 201 for a new
// endpoint, 200 with the same ID for a known one. expirationTime is accepted and ignored (O5).
func (s *Server) SubscribePush(w http.ResponseWriter, r *http.Request) {
	var body gen.PushSubscriptionInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	// Check looks the endpoint's host up in DNS, so it runs before the transaction opens.
	sub, err := s.push.Check(r.Context(), push.Subscription{Endpoint: body.Endpoint, P256dh: body.Keys.P256dh, Auth: body.Keys.Auth})
	if err != nil {
		writeError(w, r, fmt.Errorf("check push subscription: %w", err))
		return
	}
	userID := mustSignedIn(r.Context()).me.ID
	var out gen.PushSubscriptionId
	var created bool
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		id, isNew, err := s.push.Subscribe(r.Context(), tx, userID, sub)
		if err != nil {
			return fmt.Errorf("subscribe push: %w", err)
		}
		out, created = gen.PushSubscriptionId{Id: id}, isNew
		return nil
	}) {
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, out)
}
