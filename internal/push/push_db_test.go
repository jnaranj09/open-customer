//go:build db

package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/netip"
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

// fakeResolver answers each host from the map; a host it does not hold is a lookup error.
type fakeResolver map[string][]netip.Addr

func (r fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	addrs, ok := r[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	return addrs, nil
}

func addrs(list ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(list))
	for _, a := range list {
		out = append(out, netip.MustParseAddr(a))
	}
	return out
}

// resolver is the DNS the fixture's module sees. 203.0.113.10 is a documentation address; the
// module treats it as public.
var resolver = fakeResolver{
	"push.example.test":        addrs("203.0.113.10"),
	"mixed.example.test":       addrs("192.168.1.1", "203.0.113.10"),
	"loopback.example.test":    addrs("127.0.0.1"),
	"loopback6.example.test":   addrs("::1"),
	"private10.example.test":   addrs("10.0.0.5"),
	"private172.example.test":  addrs("172.16.0.5"),
	"private192.example.test":  addrs("192.168.1.1"),
	"linklocal.example.test":   addrs("169.254.169.254"),
	"linklocal6.example.test":  addrs("fe80::1"),
	"ula.example.test":         addrs("fd00::1"),
	"cgnat.example.test":       addrs("100.64.0.1"),
	"unspecified.example.test": addrs("0.0.0.0"),
	"unspec6.example.test":     addrs("::"),
	"multicast.example.test":   addrs("239.1.1.1"),
	"multicast6.example.test":  addrs("ff0e::1"),
	"mapped.example.test":      addrs("::ffff:127.0.0.1"),
	"mapped10.example.test":    addrs("::ffff:10.0.0.5"),
	"mappedll.example.test":    addrs("::ffff:169.254.169.254"),
	"mappedcgnat.example.test": addrs("::ffff:100.64.0.1"),
	"empty.example.test":       nil,
}

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
	m := New(Options{Clock: clock.NewFake(start), Subject: subject, HTTPClient: client, Resolver: resolver})
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

// check is Check on a subscription the test expects to pass.
func (f fixture) check(t *testing.T, s Subscription) Checked {
	t.Helper()
	c, err := f.m.Check(t.Context(), s)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return c
}

// store writes a subscription row as it is, past Subscribe's checks: a row stored before a check
// existed, or one whose host now resolves elsewhere.
func (f fixture) store(t *testing.T, endpoint string) uuid.UUID {
	t.Helper()
	p256dh, auth := newKeys(t)
	id, err := ids.New()
	if err != nil {
		t.Fatalf("subscription id: %v", err)
	}
	_, err = f.pool.Exec(t.Context(), `INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, $4, $5)`,
		id, f.user, endpoint, p256dh, auth)
	if err != nil {
		t.Fatalf("insert subscription: %v", err)
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
	id, created, err := f.m.Subscribe(t.Context(), f.pool, f.user, f.check(t, Subscription{Endpoint: endpoint, P256dh: p256dh, Auth: auth}))
	if err != nil || !created {
		t.Fatalf("first Subscribe = %v, created %v; want created", err, created)
	}
	p256dh2, auth2 := newKeys(t)
	id2, created2, err := f.m.Subscribe(t.Context(), f.pool, f.user, f.check(t, Subscription{Endpoint: endpoint, P256dh: p256dh2, Auth: auth2}))
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

func TestSubscribeAcceptsAHostWithAPublicAddress(t *testing.T) {
	f := newFixture(t, nil)
	p256dh, auth := newKeys(t)
	_, created, err := f.m.Subscribe(t.Context(), f.pool, f.user, f.check(t, Subscription{Endpoint: "https://mixed.example.test/x", P256dh: p256dh, Auth: auth}))
	if err != nil || !created {
		t.Fatalf("Subscribe = %v, created %v; want created (the send-time dial refuses the private address)", err, created)
	}
}

func TestSubscribeRefusesAnUncheckedSubscription(t *testing.T) {
	f := newFixture(t, nil)
	if _, _, err := f.m.Subscribe(t.Context(), f.pool, f.user, Checked{}); !errors.Is(err, errUnchecked) {
		t.Fatalf("err = %v, want errUnchecked", err)
	}
	if n := f.count(t); n != 0 {
		t.Errorf("CountSubscriptions = %d, want 0", n)
	}
}

// TestSubscribeRejects: Check, the check every subscription passes before Subscribe stores it.
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
	for _, endpoint := range []string{
		"https://192.168.1.1/admin",
		"https://169.254.169.254/latest/meta-data/",
		"https://203.0.113.10/x",
		"https://[::1]:8080/api/v1/x",
		"https://[2001:db8::1]/x",
	} {
		cases = append(cases, struct {
			name  string
			sub   Subscription
			field string
		}{"ip literal " + endpoint, Subscription{Endpoint: endpoint, P256dh: p256dh, Auth: auth}, "endpoint"})
	}
	for _, host := range []string{
		"loopback", "loopback6", "private10", "private172", "private192", "linklocal", "linklocal6", "ula",
		"cgnat", "unspecified", "unspec6", "multicast", "multicast6", "mapped", "mapped10", "mappedll", "mappedcgnat",
		"empty", "unknown",
	} {
		cases = append(cases, struct {
			name  string
			sub   Subscription
			field string
		}{"host with no public address " + host, Subscription{Endpoint: "https://" + host + ".example.test/x", P256dh: p256dh, Auth: auth}, "endpoint"})
	}
	f := newFixture(t, nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.m.Check(t.Context(), tc.sub)
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
