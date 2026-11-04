//go:build db

package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const (
	settingsPath = "/api/v1/settings"
	iconPath     = "/api/v1/settings/icon"
	// The app-shell routes outside /api/v1 (design doc "Branding and the manifest").
	manifestPath       = "/manifest.webmanifest"
	icon512Path        = "/icons/512.png"
	appleTouchIconPath = "/apple-touch-icon.png"
)

// patchSettings sends body as PATCH /settings with the public Origin and c (none when nil).
func patchSettings(h http.Handler, body string, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, settingsPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// iconPart is one multipart part: its form name and its bytes.
type iconPart struct {
	name string
	data []byte
}

// multipartBody encodes parts as multipart/form-data, after preamble (bytes before the first
// boundary, which the reader skips), and returns the body and its Content-Type.
func multipartBody(t *testing.T, preamble []byte, parts ...iconPart) (*bytes.Buffer, string) {
	t.Helper()
	body := bytes.NewBuffer(preamble)
	mw := multipart.NewWriter(body)
	for _, p := range parts {
		fw, err := mw.CreateFormFile(p.name, p.name+".png")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := fw.Write(p.data); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return body, mw.FormDataContentType()
}

// uploadIcon sends body as POST /settings/icon with contentType, the public Origin and c.
func uploadIcon(h http.Handler, contentType string, body io.Reader, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, iconPath, body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", testOrigin)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// uploadParts uploads parts as multipart/form-data with c.
func uploadParts(t *testing.T, h http.Handler, c *http.Cookie, parts ...iconPart) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, nil, parts...)
	return uploadIcon(h, ct, body, c)
}

// testPNG is a made-up size×size PNG.
func testPNG(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i := range img.Pix {
		img.Pix[i] = byte(i % 251)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}

// validIcons is a well-formed upload: a 512 px and a 180 px PNG.
func validIcons(t *testing.T) []iconPart {
	t.Helper()
	return []iconPart{{"icon512", testPNG(t, 512)}, {"icon180", testPNG(t, 180)}}
}

// get sends a GET without a cookie and asserts a 200.
func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := serve(h, http.MethodGet, target)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %q, want 200", target, rec.Code, rec.Body.String())
	}
	return rec
}

// assertIconUnchanged asserts both icon routes still serve before512 and before180.
func assertIconUnchanged(t *testing.T, h http.Handler, before512, before180 []byte) {
	t.Helper()
	if !bytes.Equal(get(t, h, icon512Path).Body.Bytes(), before512) ||
		!bytes.Equal(get(t, h, appleTouchIconPath).Body.Bytes(), before180) {
		t.Fatal("an icon route serves new bytes after a refused upload, want nothing written")
	}
}

// storedSettings reads the settings row through a fresh pool query, outside any handler
// transaction, so it sees only committed data.
func storedSettings(t *testing.T, pool *pgxpool.Pool) sqlc.GetSettingsRow {
	t.Helper()
	row, err := sqlc.New(pool).GetSettings(t.Context())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	return row
}

func assertProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, suffix string) {
	t.Helper()
	if got := problemType(t, rec, status); !strings.HasSuffix(got, suffix) {
		t.Fatalf("type = %q, want %s", got, suffix)
	}
}

func TestPatchSettingsOK(t *testing.T) {
	app, pool, c := signedInOwner(t)
	rec := patchSettings(app.handler, `{"name":"Estudio Nuevo"}`, c)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("got %d %q %q, want 200 application/json", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	var keys map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	want := []string{"accent_color", "country_code", "currency", "has_custom_icon", "locale", "name", "push_hour", "timezone"}
	if got := slices.Sorted(maps.Keys(keys)); !slices.Equal(got, want) {
		t.Fatalf("keys = %v, want exactly %v", got, want)
	}
	var s gen.Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode Settings: %v", err)
	}
	wantSettings := gen.Settings{
		Name: "Estudio Nuevo", Timezone: "Europe/Madrid", Locale: "es", Currency: "EUR",
		CountryCode: "ES", AccentColor: "#aa3366", PushHour: 8, HasCustomIcon: false,
	}
	if s != wantSettings {
		t.Fatalf("Settings = %+v, want %+v", s, wantSettings)
	}
	if stored := storedSettings(t, pool); stored.Name != "Estudio Nuevo" {
		t.Fatalf("stored name = %q after the 200, want Estudio Nuevo (committed)", stored.Name)
	}
}

func TestPatchSettingsCurrencyIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	assertProblem(t, patchSettings(app.handler, `{"currency":"USD"}`, c), http.StatusUnprocessableEntity, ":validation")
}

func TestPatchSettingsBadAccentIs422Field(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := patchSettings(app.handler, `{"accent_color":"red"}`, c)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
	if fields := fieldNames(t, rec); !slices.Equal(fields, []string{"accent_color"}) {
		t.Fatalf("errors[].field = %v, want [accent_color]", fields)
	}
}

func TestPatchSettingsBadInput(t *testing.T) {
	app, pool, c := signedInOwner(t)
	before := storedSettings(t, pool)
	for _, tc := range []struct {
		name, body string
		status     int
		suffix     string
		fields     []string
	}{
		{"a malformed body is 422", `{"name":`, http.StatusUnprocessableEntity, ":validation", nil},
		{"an unknown field is 422", `{"name":"Estudio","color":"#000000"}`, http.StatusUnprocessableEntity, ":validation", nil},
		{"a field fixed at setup (country_code) is 422", `{"country_code":"PT"}`, http.StatusUnprocessableEntity, ":validation", nil},
		{"a field fixed at setup (locale) is 422", `{"locale":"es"}`, http.StatusUnprocessableEntity, ":validation", nil},
		{"an oversized body is 413", `{"name":"` + strings.Repeat("a", maxJSONBody) + `"}`,
			http.StatusRequestEntityTooLarge, ":payload-too-large", nil},
		{"a bare null body is 422", `null`, http.StatusUnprocessableEntity, ":validation", nil},
		{"an array body is 422", `[]`, http.StatusUnprocessableEntity, ":validation", nil},
		{"a null name is 422 on name", `{"name":null}`, http.StatusUnprocessableEntity, ":validation", []string{"name"}},
		{"a null push_hour is 422 on push_hour", `{"push_hour":null}`, http.StatusUnprocessableEntity, ":validation", []string{"push_hour"}},
		{"a null next to a valid field is 422 and the valid field is not written", `{"name":null,"push_hour":9}`,
			http.StatusUnprocessableEntity, ":validation", []string{"name"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := patchSettings(app.handler, tc.body, c)
			assertProblem(t, rec, tc.status, tc.suffix)
			if fields := fieldNames(t, rec); !slices.Equal(fields, tc.fields) {
				t.Fatalf("errors[].field = %v, want %v", fields, tc.fields)
			}
			if after := storedSettings(t, pool); after != before {
				t.Fatalf("stored settings changed to %+v, want %+v (nothing written)", after, before)
			}
		})
	}
}

func TestPatchSettingsWithoutSessionIs401(t *testing.T) {
	app, _, _ := signedInOwner(t)
	assertUnauthorized(t, patchSettings(app.handler, `{"name":"Estudio Nuevo"}`, nil))
}

// TestPatchSettingsMustChangeIs403 sends the real PATCH /settings through the full handler with a
// session whose user must change her password.
func TestPatchSettingsMustChangeIs403(t *testing.T) {
	app, pool, _ := signedInOwner(t)
	temp, err := app.auth.ResetPassword(t.Context(), pool, "ana")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	c := login(t, app, temp)
	assertProblem(t, patchSettings(app.handler, `{"name":"Estudio Nuevo"}`, c), http.StatusForbidden, ":must-change-password")
}

func TestUploadIconOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	manifestBefore := get(t, app.handler, manifestPath).Body.String()
	icons := validIcons(t)
	assertNoContent(t, uploadParts(t, app.handler, c, icons...))

	for _, tc := range []struct {
		name, path string
		want       []byte
	}{
		{"/icons/512.png serves the uploaded 512 px icon", icon512Path, icons[0].data},
		{"/apple-touch-icon.png serves the uploaded 180 px icon", appleTouchIconPath, icons[1].data},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, app.handler, tc.path)
			if !bytes.Equal(rec.Body.Bytes(), tc.want) {
				t.Fatalf("body is %d bytes, want the %d uploaded bytes", rec.Body.Len(), len(tc.want))
			}
			if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); ct != "image/png" || cc != "no-cache" {
				t.Fatalf("Content-Type %q, Cache-Control %q; want image/png, no-cache", ct, cc)
			}
			if lm := rec.Header().Get("Last-Modified"); lm != testStart.Format(http.TimeFormat) {
				t.Fatalf("Last-Modified = %q, want the upload time %q", lm, testStart.Format(http.TimeFormat))
			}
		})
	}
	if after := get(t, app.handler, manifestPath).Body.String(); after != manifestBefore {
		t.Fatalf("manifest changed after the upload:\n%s\nwant\n%s", after, manifestBefore)
	}
}

func TestUploadIconNotPNGIs415(t *testing.T) {
	app, _, c := signedInOwner(t)
	before512, before180 := get(t, app.handler, icon512Path).Body.Bytes(), get(t, app.handler, appleTouchIconPath).Body.Bytes()
	rec := uploadParts(t, app.handler, c, iconPart{"icon512", []byte("GIF89a made-up image")}, iconPart{"icon180", testPNG(t, 180)})
	assertProblem(t, rec, http.StatusUnsupportedMediaType, ":unsupported-media-type")
	assertIconUnchanged(t, app.handler, before512, before180)
}

func TestUploadIconPartOver1MBIs413(t *testing.T) {
	app, _, c := signedInOwner(t)
	before512, before180 := get(t, app.handler, icon512Path).Body.Bytes(), get(t, app.handler, appleTouchIconPath).Body.Bytes()
	big := append(testPNG(t, 512), make([]byte, 1<<20)...)
	rec := uploadParts(t, app.handler, c, iconPart{"icon512", big}, iconPart{"icon180", testPNG(t, 180)})
	assertProblem(t, rec, http.StatusRequestEntityTooLarge, ":payload-too-large")
	assertIconUnchanged(t, app.handler, before512, before180)
}

// TestUploadIconRequestOver3MBIs413: every part is valid and small, but a preamble pushes the
// request past the 3 MB cap, so only the request limit can refuse it.
func TestUploadIconRequestOver3MBIs413(t *testing.T) {
	app, _, c := signedInOwner(t)
	before512, before180 := get(t, app.handler, icon512Path).Body.Bytes(), get(t, app.handler, appleTouchIconPath).Body.Bytes()
	body, ct := multipartBody(t, bytes.Repeat([]byte("made-up preamble line\r\n"), (3<<20)/23+1), validIcons(t)...)
	assertProblem(t, uploadIcon(app.handler, ct, body, c), http.StatusRequestEntityTooLarge, ":payload-too-large")
	assertIconUnchanged(t, app.handler, before512, before180)
}

func TestUploadIconMissingPartIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	before512, before180 := get(t, app.handler, icon512Path).Body.Bytes(), get(t, app.handler, appleTouchIconPath).Body.Bytes()
	icons := validIcons(t)
	for _, tc := range []struct {
		name  string
		parts []iconPart
	}{
		{"a missing icon180 is 422", icons[:1]},
		{"a missing icon512 is 422", icons[1:]},
		{"an unknown part name is 422", append(slices.Clone(icons), iconPart{"icon64", testPNG(t, 64)})},
		{"a repeated part is 422", append(slices.Clone(icons), icons[0])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertProblem(t, uploadParts(t, app.handler, c, tc.parts...), http.StatusUnprocessableEntity, ":validation")
		})
	}
	assertIconUnchanged(t, app.handler, before512, before180)
}

func TestUploadIconNotMultipartIs415(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := uploadIcon(app.handler, "application/json", strings.NewReader(`{"icon512":"x"}`), c)
	assertProblem(t, rec, http.StatusUnsupportedMediaType, ":unsupported-media-type")
}

// TestUploadIconOtherMultipartIs415: well-formed parts sent as multipart/mixed (or with no
// Content-Type) are refused; only multipart/form-data is accepted.
func TestUploadIconOtherMultipartIs415(t *testing.T) {
	app, _, c := signedInOwner(t)
	before512, before180 := get(t, app.handler, icon512Path).Body.Bytes(), get(t, app.handler, appleTouchIconPath).Body.Bytes()
	for _, tc := range []struct {
		name        string
		contentType func(formData string) string
	}{
		{"multipart/mixed is 415", func(ct string) string { return strings.Replace(ct, "multipart/form-data", "multipart/mixed", 1) }},
		{"no Content-Type is 415", func(string) string { return "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, ct := multipartBody(t, nil, validIcons(t)...)
			assertProblem(t, uploadIcon(app.handler, tc.contentType(ct), body, c), http.StatusUnsupportedMediaType, ":unsupported-media-type")
		})
	}
	assertIconUnchanged(t, app.handler, before512, before180)
}

func TestUploadIconMalformedMultipartIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	for _, tc := range []struct{ name, contentType, body string }{
		{"multipart without a boundary is 422", "multipart/form-data", "--x\r\n"},
		{"a body that never reaches its boundary is 422", "multipart/form-data; boundary=made-up", "no parts here"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertProblem(t, uploadIcon(app.handler, tc.contentType, strings.NewReader(tc.body), c),
				http.StatusUnprocessableEntity, ":validation")
		})
	}
}

func TestUploadIconWithoutSessionIs401(t *testing.T) {
	app, _, _ := signedInOwner(t)
	body, ct := multipartBody(t, nil, validIcons(t)...)
	assertUnauthorized(t, uploadIcon(app.handler, ct, body, nil))
}

// TestUploadIconFileErrorLogsNoPath: a media dir that cannot be created is a 500, and neither the
// response nor the log names the path.
func TestUploadIconFileErrorLogsNoPath(t *testing.T) {
	_, pool, c := signedInOwner(t)
	mediaDir := filepath.Join(t.TempDir(), "made-up-media")
	if err := os.WriteFile(mediaDir, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	app := newTestApp(t, testServerConfig{pool: pool, mediaDir: mediaDir})
	log := captureDefaultLog(t)
	rec := uploadParts(t, app.handler, c, validIcons(t)...)
	assertProblem(t, rec, http.StatusInternalServerError, ":internal")
	if strings.Contains(log.String(), "made-up-media") || strings.Contains(rec.Body.String(), "made-up-media") {
		t.Fatalf("the path reached the log or the response:\nlog %s\nbody %s", log, rec.Body)
	}
	if !strings.Contains(log.String(), `"code":"internal"`) {
		t.Fatalf("log = %s, want an internal code= line", log)
	}
}

// TestIconReadErrorLogsNoPath: an uploaded icon that cannot be read is a 500 whose log does not
// name the path.
func TestIconReadErrorLogsNoPath(t *testing.T) {
	_, pool, c := signedInOwner(t)
	mediaDir := filepath.Join(t.TempDir(), "made-up-media")
	app := newTestApp(t, testServerConfig{pool: pool, mediaDir: mediaDir})
	assertNoContent(t, uploadParts(t, app.handler, c, validIcons(t)...))
	icon := filepath.Join(mediaDir, "branding", "icon-512.png")
	if err := os.Remove(icon); err != nil {
		t.Fatalf("remove icon: %v", err)
	}
	if err := os.Mkdir(icon, 0o750); err != nil {
		t.Fatalf("replace the icon with a directory: %v", err)
	}
	log := captureDefaultLog(t)
	assertProblem(t, serve(app.handler, http.MethodGet, icon512Path), http.StatusInternalServerError, ":internal")
	if strings.Contains(log.String(), "made-up-media") {
		t.Fatalf("the path reached the log: %s", log)
	}
}

func TestManifestRouteNoSessionNoCache(t *testing.T) {
	app, _, _ := signedInOwner(t)
	rec := get(t, app.handler, manifestPath)
	if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); ct != "application/manifest+json" || cc != "no-cache" {
		t.Fatalf("Content-Type %q, Cache-Control %q; want application/manifest+json, no-cache", ct, cc)
	}
	var m struct {
		Name       string `json:"name"`
		ThemeColor string `json:"theme_color"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil || m.Name != "Estudio Prueba" || m.ThemeColor != "#aa3366" {
		t.Fatalf("manifest = %q (%v), want name Estudio Prueba and theme_color #aa3366", rec.Body.String(), err)
	}
}

func TestManifestDBErrorIsInternal(t *testing.T) {
	pool := dbtest.Pool(t)
	app := newTestApp(t, testServerConfig{pool: pool})
	pool.Close()
	assertProblem(t, serve(app.handler, http.MethodGet, manifestPath), http.StatusInternalServerError, ":internal")
}

func TestAppleTouchIconIs180Default(t *testing.T) {
	app := newTestApp(t, testServerConfig{pool: dbtest.Pool(t)})
	for _, tc := range []struct {
		name, path string
		size       int
	}{
		{"/apple-touch-icon.png is the 180 px default before any upload", appleTouchIconPath, 180},
		{"/icons/512.png is the 512 px default before any upload", icon512Path, 512},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, app.handler, tc.path)
			if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" || rec.Header().Get("Last-Modified") != "" {
				t.Fatalf("Cache-Control %q, Last-Modified %q; want no-cache and none", cc, rec.Header().Get("Last-Modified"))
			}
			cfg, err := png.DecodeConfig(rec.Body)
			if err != nil || cfg.Width != tc.size || cfg.Height != tc.size {
				t.Fatalf("PNG %dx%d (%v), want %dx%d", cfg.Width, cfg.Height, err, tc.size, tc.size)
			}
		})
	}
}
