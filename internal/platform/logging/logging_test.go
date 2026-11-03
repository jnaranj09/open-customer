package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareLogsOneLinePerRequest(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	var seenID string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		seenID = RequestID(r.Context())
		w.WriteHeader(http.StatusTeapot)
	})
	h := Middleware(New(&buf, slog.LevelInfo), mux)

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	req.Header.Set("CF-Ray", "8f00example-MIA")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log is not one JSON line: %v (%q)", err, buf.String())
	}
	want := map[string]any{
		"request_id": seenID,
		"cf_ray":     "8f00example-MIA",
		"route":      "GET /items/{id}",
		"status":     float64(http.StatusTeapot),
	}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("log %s = %v, want %v", k, line[k], v)
		}
	}
	if len(seenID) != 32 {
		t.Errorf("request ID = %q, want 32 hex characters", seenID)
	}
}

func TestRequestIDOutsideARequestIsEmpty(t *testing.T) {
	t.Parallel()
	if got := RequestID(t.Context()); got != "" {
		t.Fatalf("RequestID = %q, want empty", got)
	}
}
