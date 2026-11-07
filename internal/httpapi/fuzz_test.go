package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// quietLog keeps a fuzz run from printing one log line per input.
func quietLog(f *testing.F) {
	f.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.Cleanup(func() { slog.SetDefault(prev) })
}

// problemStatus asserts rec holds a 4xx problem and returns its status. A 5xx is the bug the fuzzers
// look for (the ZAP scan found U+0000 in a string as a 500).
func problemStatus(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("status = %d, want a 4xx (body %q)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	return rec.Code
}

// FuzzDecodeJSON drives decodeJSON and decodePatch, the one parse site of every JSON route (a route
// cannot be driven without a database: the session check comes first). A body is either decoded, and
// then holds no U+0000 and nothing was written, or refused with a 4xx problem. Never a panic or 5xx.
func FuzzDecodeJSON(f *testing.F) {
	quietLog(f)
	for _, seed := range []string{
		`{"name":"x","extra":1}`, `{"name":`, ``, `{"name":"x"}{}`, `{"count":"three"}`,
		`{"name":"a\u0000b"}`, `{"a\u0000b":1}`, `{"name":"a\\u0000b"}`, `{"name":"x","count":3}`,
		`null`, `[]`, `{"name":null}`, `{"name":"\ud800"}`, "{\"name\":\"a\x00b\"}", `{"count":1e999}`,
		strings.Repeat("[", 2000),
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/decode-test", bytes.NewReader(body))
		var v decodeTarget
		if err := decodeJSON(rec, req, &v); err != nil {
			problemStatus(t, rec)
		} else {
			if rec.Body.Len() != 0 {
				t.Fatalf("decodeJSON wrote %q and returned nil", rec.Body.String())
			}
			if strings.ContainsRune(v.Name, 0) {
				t.Fatalf("decoded name holds U+0000: %q", v.Name)
			}
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPatch, "/decode-test", bytes.NewReader(body))
		var p struct {
			Name  *string `json:"name"`
			Count *int    `json:"count"`
		}
		if err := decodePatch(rec, req, &p, "count"); err != nil {
			problemStatus(t, rec)
		} else if rec.Body.Len() != 0 {
			t.Fatalf("decodePatch wrote %q and returned nil", rec.Body.String())
		}
	})
}

// multipartSeed is a form-data body (boundary "b") holding the named parts.
func multipartSeed(parts ...[2]string) []byte {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// SetBoundary only fails for a bad boundary; "b" is fine.
	_ = mw.SetBoundary("b")
	for _, p := range parts {
		name, data := p[0], p[1]
		fw, err := mw.CreateFormFile(name, name+".bin")
		if err != nil {
			continue
		}
		_, _ = fw.Write([]byte(data))
	}
	_ = mw.Close()
	return buf.Bytes()
}

// fuzzUpload runs read on a request with the given Content-Type and body, bounded like the handler,
// and asserts the outcome is parts or a problem that problem.Write answers with a 4xx.
func fuzzUpload(t *testing.T, limit int64, contentType string, body []byte, read func(*http.Request) error) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload-test", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Body = http.MaxBytesReader(rec, req.Body, limit)
	err := read(req)
	if err == nil {
		return
	}
	// An error that is not a problem is written as a 500, which problemStatus refuses.
	problem.Write(rec, req, err)
	problemStatus(t, rec)
}

// FuzzMultipartUpload drives readIconParts (POST /settings/icon) and readMediaParts (POST /media),
// the multipart parse site before any database call. Any content type and body is either read or
// refused with a 4xx problem (415, 413 or 422). Never a panic or 5xx.
func FuzzMultipartUpload(f *testing.F) {
	quietLog(f)
	const form = "multipart/form-data; boundary=b"
	icons := multipartSeed([2]string{"icon512", "\x89PNG"}, [2]string{"icon180", "\x89PNG"})
	media := multipartSeed([2]string{"full", "\xff\xd8"}, [2]string{"thumb", "\xff\xd8"}, [2]string{"client_id", "not-a-uuid"})
	for _, s := range []struct {
		ct   string
		body []byte
	}{
		{form, icons}, {form, media}, {"application/json", []byte(`{}`)}, {"", nil},
		{"multipart/mixed; boundary=b", icons}, {"multipart/form-data", icons},
		{form, []byte("--b\r\nContent-Disposition: form-data; name=\"icon512\"\r\n\r\nx")},
		{form, multipartSeed([2]string{"icon512", "x"}, [2]string{"other", "y"})},
		{form, []byte("--b\r\nContent-Disposition: form-data; name=\"icon512\"\r\n\r\nx\r\n--b\r\nContent-Disposition: form-data; name=\"icon512\"\r\n\r\ny\r\n--b--\r\n")},
		{form, nil}, {"multipart/form-data; boundary=\x00", icons},
	} {
		f.Add(s.ct, s.body)
	}
	f.Fuzz(func(t *testing.T, contentType string, body []byte) {
		fuzzUpload(t, maxIconRequest, contentType, body, func(r *http.Request) error {
			_, err := readIconParts(r)
			return err
		})
		fuzzUpload(t, maxMediaRequest, contentType, body, func(r *http.Request) error {
			_, err := readMediaParts(r)
			return err
		})
	})
}
