//go:build db

package httpapi

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/push"
)

const (
	pushKeyPath  = "/api/v1/push/key"
	pushSubsPath = "/api/v1/push/subscriptions"
)

// pushApp is a signed-in app whose VAPID keys exist, like serve's start leaves them.
func pushApp(t *testing.T) (testApp, *http.Cookie) {
	t.Helper()
	app, pool, c := signedInOwner(t)
	// The same call serve's startup makes, so the key route has a key to answer.
	module := push.New(push.Options{Clock: app.clock, Subject: "mailto:dev@example.com"})
	err := db.WithTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error { return module.EnsureKeys(t.Context(), tx) })
	if err != nil {
		t.Fatalf("ensure vapid keys: %v", err)
	}
	return app, c
}

// browserSubscription is the JSON a browser's PushSubscription serializes to, with made-up keys.
func browserSubscription(endpoint string) string {
	p256dh := base64.RawURLEncoding.EncodeToString(append([]byte{4}, make([]byte, 64)...))
	auth := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	return fmt.Sprintf(`{"endpoint":%q,"expirationTime":null,"keys":{"p256dh":%q,"auth":%q}}`, endpoint, p256dh, auth)
}

func TestGetPushKeyNoSession(t *testing.T) {
	app, _ := pushApp(t)
	got := decodeInto[gen.PushKey](t, send(app.handler, http.MethodGet, pushKeyPath, testOrigin, "", nil), http.StatusOK)
	if len(got.PublicKey) != 87 {
		t.Fatalf("public_key = %d characters, want 87 (65 bytes, raw base64url)", len(got.PublicKey))
	}
}

func TestSubscribePushAcceptsBrowserJSON(t *testing.T) {
	app, c := pushApp(t)
	body := browserSubscription("https://push.example.com/abc")
	first := decodeInto[gen.PushSubscriptionId](t, sendAs(app, c, http.MethodPost, pushSubsPath, body), http.StatusCreated)
	second := decodeInto[gen.PushSubscriptionId](t, sendAs(app, c, http.MethodPost, pushSubsPath, body), http.StatusOK)
	if first.Id != second.Id {
		t.Fatalf("second id = %s, want the first %s", second.Id, first.Id)
	}
}

func TestSubscribePushHttpEndpointIs422(t *testing.T) {
	app, c := pushApp(t)
	rec := sendAs(app, c, http.MethodPost, pushSubsPath, browserSubscription("http://push.example.com/abc"))
	got := fieldErrors(t, rec)
	if len(got) != 1 || got[0].Field != "endpoint" {
		t.Fatalf("field errors = %+v, want one on endpoint", got)
	}
}

func TestSubscribePushMissingKeysIs422(t *testing.T) {
	app, c := pushApp(t)
	assertProblem(t, sendAs(app, c, http.MethodPost, pushSubsPath, `{"endpoint":"https://push.example.com/abc"}`),
		http.StatusUnprocessableEntity, ":validation")
}

func TestSubscribePushNeedsSession(t *testing.T) {
	app, _ := pushApp(t)
	assertUnauthorized(t, send(app.handler, http.MethodPost, pushSubsPath, testOrigin, browserSubscription("https://push.example.com/abc"), nil))
}
