//go:build db

package push

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const subject = "mailto:dev@example.com"

var start = time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC)

type fixture struct {
	pool *pgxpool.Pool
	m    *Module
	user uuid.UUID
}

// newFixture returns the module on a fresh database with one user and the VAPID keys in place.
// client is the push service's client (nil: the module's default).
func newFixture(t *testing.T, client *http.Client) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	user, err := ids.New()
	if err != nil {
		t.Fatalf("user id: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO users (id, username, password_hash) VALUES ($1, 'ana', 'x')`, user); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	m := New(Options{Clock: clock.NewFake(start), Subject: subject, HTTPClient: client})
	if err := m.EnsureKeys(t.Context(), pool); err != nil {
		t.Fatalf("EnsureKeys: %v", err)
	}
	return fixture{pool: pool, m: m, user: user}
}

// newKeys returns the keys of a made-up browser subscription: an uncompressed P-256 public key
// and a 16-byte auth secret, both base64url without padding.
func newKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("subscription key: %v", err)
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatalf("auth secret: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(secret)
}

func (f fixture) subscribe(t *testing.T, endpoint string) uuid.UUID {
	t.Helper()
	p256dh, auth := newKeys(t)
	id, _, err := f.m.Subscribe(t.Context(), f.pool, f.user, Subscription{Endpoint: endpoint, P256dh: p256dh, Auth: auth})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	return id
}

func (f fixture) count(t *testing.T) int {
	t.Helper()
	n, err := f.m.CountSubscriptions(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("CountSubscriptions: %v", err)
	}
	return n
}

func (f fixture) storedKeys(t *testing.T) (public, private string) {
	t.Helper()
	err := f.pool.QueryRow(t.Context(),
		`SELECT (SELECT value FROM server_keys WHERE name = 'vapid_public'), (SELECT value FROM server_keys WHERE name = 'vapid_private')`).
		Scan(&public, &private)
	if err != nil {
		t.Fatalf("read server keys: %v", err)
	}
	return public, private
}

func TestEnsureKeysCreatesOnce(t *testing.T) {
	f := newFixture(t, nil)
	public, private := f.storedKeys(t)
	if err := f.m.EnsureKeys(t.Context(), f.pool); err != nil {
		t.Fatalf("second EnsureKeys: %v", err)
	}
	public2, private2 := f.storedKeys(t)
	if public2 != public || private2 != private {
		t.Error("the second EnsureKeys changed the stored key pair")
	}
}

func TestPublicKeyIsUncompressedP256(t *testing.T) {
	f := newFixture(t, nil)
	key, err := f.m.PublicKey(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("PublicKey is not raw base64url: %v", err)
	}
	if len(raw) != 65 || raw[0] != 0x04 {
		t.Errorf("PublicKey decodes to %d bytes starting with %#x, want 65 starting with 0x04", len(raw), raw[0])
	}
}

func TestSubscribeInsertThenUpdate(t *testing.T) {
	f := newFixture(t, nil)
	endpoint := "https://push.example.test/send/abc"
	p256dh, auth := newKeys(t)
	id, created, err := f.m.Subscribe(t.Context(), f.pool, f.user, Subscription{Endpoint: endpoint, P256dh: p256dh, Auth: auth})
	if err != nil || !created {
		t.Fatalf("first Subscribe = %v, created %v; want created", err, created)
	}
	p256dh2, auth2 := newKeys(t)
	id2, created2, err := f.m.Subscribe(t.Context(), f.pool, f.user, Subscription{Endpoint: endpoint, P256dh: p256dh2, Auth: auth2})
	if err != nil || created2 || id2 != id {
		t.Fatalf("second Subscribe = %v, %v, %v; want the same ID %v and created false", id2, created2, err, id)
	}
	var gotP256dh, gotAuth string
	if err := f.pool.QueryRow(t.Context(), `SELECT p256dh, auth FROM push_subscriptions WHERE id = $1`, id).Scan(&gotP256dh, &gotAuth); err != nil {
		t.Fatalf("read subscription: %v", err)
	}
	if gotP256dh != p256dh2 || gotAuth != auth2 {
		t.Error("the known endpoint did not get the new keys")
	}
	if n := f.count(t); n != 1 {
		t.Errorf("CountSubscriptions = %d, want 1", n)
	}
}

func TestSubscribeRejects(t *testing.T) {
	p256dh, auth := newKeys(t)
	short := func(n int) string { return base64.RawURLEncoding.EncodeToString(make([]byte, n)) }
	cases := []struct {
		name  string
		sub   Subscription
		field string
	}{
		{"http endpoint", Subscription{Endpoint: "http://push.example.test/x", P256dh: p256dh, Auth: auth}, "endpoint"},
		{"not a url", Subscription{Endpoint: "not a url", P256dh: p256dh, Auth: auth}, "endpoint"},
		{"p256dh of 64 bytes", Subscription{Endpoint: "https://push.example.test/x", P256dh: short(64), Auth: auth}, "keys.p256dh"},
		{"auth of 15 bytes", Subscription{Endpoint: "https://push.example.test/x", P256dh: p256dh, Auth: short(15)}, "keys.auth"},
	}
	f := newFixture(t, nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := f.m.Subscribe(t.Context(), f.pool, f.user, tc.sub)
			var v problem.Validation
			if !errors.As(err, &v) {
				t.Fatalf("err = %v, want problem.Validation", err)
			}
			want := []problem.FieldError{{Field: tc.field, Message: problem.MsgPushEndpointInvalid}}
			if !slices.Equal(v.Errors, want) {
				t.Errorf("errors = %v, want %v", v.Errors, want)
			}
		})
	}
	if n := f.count(t); n != 0 {
		t.Errorf("CountSubscriptions = %d after rejected subscriptions, want 0", n)
	}
}
