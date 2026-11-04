//go:build db

package push

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
)

var payload = Payload{Title: "Estudio Ana", Body: "Tienes 2 mensajes para enviar hoy", Count: 2, URL: "/enviar"}

// fakeService is a TLS push service that answers every request with status and keeps the headers
// of the last one.
type fakeService struct {
	*httptest.Server
	mu   sync.Mutex
	last http.Header
}

func newFakeService(t *testing.T, status int) *fakeService {
	t.Helper()
	s := &fakeService{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.last = r.Header.Clone()
		s.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *fakeService) lastHeader() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf
}

func (f fixture) sendAll(t *testing.T) SendResult {
	t.Helper()
	got, err := f.m.SendAll(t.Context(), f.pool, payload)
	if err != nil {
		t.Fatalf("SendAll: %v", err)
	}
	return got
}

func TestSendAllDelivers(t *testing.T) {
	svc := newFakeService(t, http.StatusCreated)
	f := newFixture(t, svc.Client())
	f.subscribe(t, svc.URL+"/send/1")
	if got := f.sendAll(t); got != (SendResult{Attempted: 1, Delivered: 1}) {
		t.Errorf("SendAll = %+v, want {1 1}", got)
	}
	h := svc.lastHeader()
	want := map[string]string{"TTL": "28800", "Urgency": "normal", "Content-Encoding": "aes128gcm"}
	for name, value := range want {
		if got := h.Get(name); got != value {
			t.Errorf("header %s = %q, want %q", name, got, value)
		}
	}
}

func TestVAPIDSubjectHasOneMailto(t *testing.T) {
	svc := newFakeService(t, http.StatusCreated)
	f := newFixture(t, svc.Client())
	f.subscribe(t, svc.URL+"/send/1")
	f.sendAll(t)
	auth := svc.lastHeader().Get("Authorization")
	token, _, ok := strings.Cut(strings.TrimPrefix(auth, "vapid t="), ",")
	parts := strings.Split(token, ".")
	if !strings.HasPrefix(auth, "vapid t=") || !ok || len(parts) != 3 {
		t.Fatalf("Authorization = %q, want vapid t=<jwt>, k=<key>", auth)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("JWT payload is not base64url: %v", err)
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("JWT payload is not JSON: %v", err)
	}
	if claims.Sub != subject {
		t.Errorf("JWT sub = %q, want %q", claims.Sub, subject)
	}
}

func TestSendAllGoneDeletesSubscription(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			svc := newFakeService(t, status)
			f := newFixture(t, svc.Client())
			f.subscribe(t, svc.URL+"/send/1")
			if got := f.sendAll(t); got != (SendResult{Attempted: 1, Delivered: 0}) {
				t.Errorf("SendAll = %+v, want {1 0}", got)
			}
			if n := f.count(t); n != 0 {
				t.Errorf("CountSubscriptions = %d, want 0", n)
			}
		})
	}
}

func TestSendAllServerErrorKeepsSubscription(t *testing.T) {
	logs := captureLog(t)
	svc := newFakeService(t, http.StatusInternalServerError)
	f := newFixture(t, svc.Client())
	f.subscribe(t, svc.URL+"/send/1")
	if got := f.sendAll(t); got != (SendResult{Attempted: 1, Delivered: 0}) {
		t.Errorf("SendAll = %+v, want {1 0}", got)
	}
	if n := f.count(t); n != 1 {
		t.Errorf("CountSubscriptions = %d, want 1", n)
	}
	out := logs.String()
	if !strings.Contains(out, "code=push_send_failed") || !strings.Contains(out, "status=500") {
		t.Errorf("log = %q, want the code and the status", out)
	}
	if strings.Contains(out, svc.URL) || strings.Contains(out, "/send/1") {
		t.Errorf("log = %q leaks the endpoint", out)
	}
}

func TestSendAllNetworkErrorCountsNotDelivered(t *testing.T) {
	logs := captureLog(t)
	svc := newFakeService(t, http.StatusCreated)
	f := newFixture(t, svc.Client())
	f.subscribe(t, svc.URL+"/send/1")
	svc.Close()
	if got := f.sendAll(t); got != (SendResult{Attempted: 1, Delivered: 0}) {
		t.Errorf("SendAll = %+v, want {1 0}", got)
	}
	if n := f.count(t); n != 1 {
		t.Errorf("CountSubscriptions = %d, want 1", n)
	}
	out := logs.String()
	if !strings.Contains(out, "code=push_send_failed") || strings.Contains(out, svc.URL) {
		t.Errorf("log = %q, want the code and no endpoint", out)
	}
}

func TestSendAllDBErrorIsReturned(t *testing.T) {
	svc := newFakeService(t, http.StatusCreated)
	f := newFixture(t, svc.Client())
	f.subscribe(t, svc.URL+"/send/1")
	calls := 0
	_, err := f.m.SendAll(t.Context(), dbtest.FailingDB{DBTX: f.pool, FailAt: 1, Calls: &calls}, payload)
	if !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("err = %v, want the database error wrapped", err)
	}
	if svc.lastHeader() != nil {
		t.Error("SendAll sent a push although the subscription list failed to load")
	}
}
