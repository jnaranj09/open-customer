//go:build db

package httpapi

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/media"
)

const mediaPath = "/api/v1/media"

// mediaKeys are the keys of a Media.
var mediaKeys = []string{"appointment_id", "archived", "bytes", "client_id", "created_at", "height", "id", "width"}

// testJPEG is a made-up w×h JPEG.
func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = byte(i % 251)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return buf.Bytes()
}

// paddedJPEG is a JPEG of exactly size bytes: a valid JPEG followed by zero bytes.
func paddedJPEG(t *testing.T, size int) []byte {
	t.Helper()
	small := testJPEG(t, 8, 8)
	return append(small, make([]byte, size-len(small))...)
}

// mediaFixture is a signed-in app whose media folder is dir, with one client.
type mediaFixture struct {
	app    testApp
	pool   *pgxpool.Pool
	cookie *http.Cookie
	dir    string
	client gen.Client
}

func newMediaFixture(t *testing.T, beginner func(*pgxpool.Pool) db.Beginner) mediaFixture {
	t.Helper()
	_, pool, c := signedInOwner(t)
	cfg := testServerConfig{pool: pool, mediaDir: t.TempDir()}
	if beginner != nil {
		cfg.beginner = beginner(pool)
	}
	app := newTestApp(t, cfg)
	return mediaFixture{app: app, pool: pool, cookie: c, dir: cfg.mediaDir, client: createClient(t, app, c, clientBody)}
}

func (f mediaFixture) send(method, target, body string) *httptest.ResponseRecorder {
	return sendAs(f.app, f.cookie, method, target, body)
}

// upload sends parts as POST /media.
func (f mediaFixture) upload(t *testing.T, parts ...iconPart) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, nil, parts...)
	return f.uploadRaw(ct, body.String())
}

func (f mediaFixture) uploadRaw(contentType, body string) *httptest.ResponseRecorder {
	return send(f.app.handler, http.MethodPost, mediaPath, testOrigin, body, func(r *http.Request) {
		r.Header.Set("Content-Type", contentType)
		r.AddCookie(f.cookie)
	})
}

// validParts is a well-formed upload for the client: full 40×30, thumb 8×6.
func (f mediaFixture) validParts(t *testing.T, extra ...iconPart) []iconPart {
	t.Helper()
	parts := []iconPart{{"full", testJPEG(t, 40, 30)}, {"thumb", testJPEG(t, 8, 6)}}
	return append(parts, extra...)
}

func (f mediaFixture) clientPart() iconPart {
	return iconPart{"client_id", []byte(f.client.Id.String())}
}

// storedFiles lists the files (not folders) under the media folder.
func storedFiles(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			found = append(found, p)
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk media folder: %v", err)
	}
	return found
}

func (f mediaFixture) assertNothingStored(t *testing.T) {
	t.Helper()
	if files := storedFiles(t, f.dir); len(files) != 0 {
		t.Fatalf("%d files stored after a refused upload, want none", len(files))
	}
}

func (f mediaFixture) uploadOK(t *testing.T) gen.Media {
	t.Helper()
	return decodeInto[gen.Media](t, f.upload(t, f.validParts(t, f.clientPart())...), http.StatusCreated)
}

func clientMediaPath(id uuid.UUID) string { return clientPath(id) + "/media" }

func TestUploadMediaOK(t *testing.T) {
	f := newMediaFixture(t, nil)
	parts := f.validParts(t, f.clientPart())
	rec := f.upload(t, parts...)
	got := decodeInto[gen.Media](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), mediaKeys...)
	if !strings.Contains(rec.Body.String(), `"appointment_id":null`) {
		t.Fatalf("body %q lacks a null appointment_id", rec.Body.String())
	}
	if got.ClientId != f.client.Id || got.Width != 40 || got.Height != 30 || got.Bytes != len(parts[0].data) || got.Archived {
		t.Fatalf("media = %+v, want the client, 40x30, %d bytes, active", got, len(parts[0].data))
	}
	for _, tc := range []struct {
		name, target string
		want         []byte
	}{
		{"GET /media/{id} serves the full image", mediaPath + "/" + got.Id.String(), parts[0].data},
		{"GET /media/{id}/thumb serves the thumbnail", mediaPath + "/" + got.Id.String() + "/thumb", parts[1].data},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.send(http.MethodGet, tc.target, "")
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" ||
				rec.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" ||
				!bytes.Equal(rec.Body.Bytes(), tc.want) {
				t.Fatalf("got %d %q %q and %d bytes, want 200 image/jpeg, the cache header and %d bytes",
					rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"), rec.Body.Len(), len(tc.want))
			}
		})
	}
}

func TestUploadMediaWithAppointmentFillsClient(t *testing.T) {
	f := newMediaFixture(t, nil)
	booking := bookingFixture{app: f.app, cookie: f.cookie, client: f.client,
		service: createService(t, f.app, f.cookie, `{"name":"Acrílico","duration_min":90,"price_cents":2500,"color":"#AA3366"}`)}
	appt := booking.book(t, "2026-11-10T10:00:00Z")
	got := decodeInto[gen.Media](t, f.upload(t, f.validParts(t, iconPart{"appointment_id", []byte(appt.Id.String())})...), http.StatusCreated)
	if got.ClientId != f.client.Id || got.AppointmentId.MustGet() != appt.Id {
		t.Fatalf("media = %+v, want the appointment's client and the appointment", got)
	}
}

// TestUploadMediaCommitFailureRemovesFiles: a deferred trigger makes the commit fail after Save
// stored the files; the handler discards them.
func TestUploadMediaCommitFailureRemovesFiles(t *testing.T) {
	f := newMediaFixture(t, nil)
	for _, stmt := range []string{
		`CREATE FUNCTION fail_media_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'made-up commit failure'; END $$`,
		`CREATE CONSTRAINT TRIGGER media_fails_at_commit AFTER INSERT ON media DEFERRABLE INITIALLY DEFERRED
			FOR EACH ROW EXECUTE FUNCTION fail_media_commit()`,
	} {
		if _, err := f.pool.Exec(t.Context(), stmt); err != nil {
			t.Fatalf("set up the trigger: %v", err)
		}
	}
	rec := f.upload(t, f.validParts(t, f.clientPart())...)
	assertProblem(t, rec, http.StatusInternalServerError, ":internal")
	f.assertNothingStored(t)
}

func TestUploadMediaNotJPEGIs415(t *testing.T) {
	f := newMediaFixture(t, nil)
	parts := []iconPart{{"full", []byte("GIF89a made-up image")}, {"thumb", testJPEG(t, 8, 6)}, f.clientPart()}
	assertProblem(t, f.upload(t, parts...), http.StatusUnsupportedMediaType, ":unsupported-media-type")
	f.assertNothingStored(t)
}

func TestUploadMediaFullOver10MBIs413(t *testing.T) {
	f := newMediaFixture(t, nil)
	parts := []iconPart{{"full", paddedJPEG(t, media.MaxFullBytes+1)}, {"thumb", testJPEG(t, 8, 6)}, f.clientPart()}
	assertProblem(t, f.upload(t, parts...), http.StatusRequestEntityTooLarge, ":payload-too-large")
	f.assertNothingStored(t)
}

func TestUploadMediaThumbOver1MBIs413(t *testing.T) {
	f := newMediaFixture(t, nil)
	parts := []iconPart{{"full", testJPEG(t, 40, 30)}, {"thumb", paddedJPEG(t, media.MaxThumbBytes+1)}, f.clientPart()}
	assertProblem(t, f.upload(t, parts...), http.StatusRequestEntityTooLarge, ":payload-too-large")
	f.assertNothingStored(t)
}

func TestUploadMediaRequestOver12MBIs413(t *testing.T) {
	f := newMediaFixture(t, nil)
	parts := []iconPart{{"full", paddedJPEG(t, media.MaxFullBytes)}, {"thumb", paddedJPEG(t, media.MaxThumbBytes)}, {"client_id", bytes.Repeat([]byte("0"), 3<<20)}}
	assertProblem(t, f.upload(t, parts...), http.StatusRequestEntityTooLarge, ":payload-too-large")
	f.assertNothingStored(t)
}

func TestUploadMediaNoOwnerIs422(t *testing.T) {
	f := newMediaFixture(t, nil)
	errs := fieldErrors(t, f.upload(t, f.validParts(t)...))
	if len(errs) != 1 || errs[0].Field != "client_id" {
		t.Fatalf("errors = %+v, want one on client_id", errs)
	}
	f.assertNothingStored(t)
}

func TestUploadMediaMismatchIs422(t *testing.T) {
	f := newMediaFixture(t, nil)
	booking := bookingFixture{app: f.app, cookie: f.cookie, client: f.client,
		service: createService(t, f.app, f.cookie, `{"name":"Acrílico","duration_min":90,"price_cents":2500,"color":"#AA3366"}`)}
	appt := booking.book(t, "2026-11-10T10:00:00Z")
	other := createClient(t, f.app, f.cookie, `{"name":"Marta Solano"}`)
	parts := f.validParts(t, iconPart{"appointment_id", []byte(appt.Id.String())}, iconPart{"client_id", []byte(other.Id.String())})
	errs := fieldErrors(t, f.upload(t, parts...))
	if len(errs) != 1 || errs[0].Field != "client_id" {
		t.Fatalf("errors = %+v, want one on client_id", errs)
	}
	f.assertNothingStored(t)
}

func TestUploadMediaBadIdIs422(t *testing.T) {
	f := newMediaFixture(t, nil)
	for _, field := range []string{"client_id", "appointment_id"} {
		t.Run("a bad "+field+" is 422 on that field", func(t *testing.T) {
			errs := fieldErrors(t, f.upload(t, f.validParts(t, iconPart{field, []byte("not-a-uuid")})...))
			if len(errs) != 1 || errs[0].Field != field {
				t.Fatalf("errors = %+v, want one on %s", errs, field)
			}
			f.assertNothingStored(t)
		})
	}
}

func TestUploadMediaNotMultipartIs415(t *testing.T) {
	f := newMediaFixture(t, nil)
	assertProblem(t, f.uploadRaw("application/json", `{"full":"x"}`), http.StatusUnsupportedMediaType, ":unsupported-media-type")
	f.assertNothingStored(t)
}

func TestUploadMediaMissingPartIs422(t *testing.T) {
	f := newMediaFixture(t, nil)
	valid := f.validParts(t, f.clientPart())
	for _, tc := range []struct {
		name  string
		parts []iconPart
		field string
	}{
		{"a missing thumb is 422 on thumb", []iconPart{valid[0], valid[2]}, "thumb"},
		{"a missing full is 422 on full", []iconPart{valid[1], valid[2]}, "full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := fieldErrors(t, f.upload(t, tc.parts...))
			if len(errs) != 1 || errs[0].Field != tc.field {
				t.Fatalf("errors = %+v, want one on %s", errs, tc.field)
			}
			f.assertNothingStored(t)
		})
	}
}

func TestUploadMediaRepeatedOrUnknownPartIs422(t *testing.T) {
	f := newMediaFixture(t, nil)
	valid := f.validParts(t, f.clientPart())
	for _, tc := range []struct {
		name  string
		parts []iconPart
	}{
		{"a repeated part is 422", append(append([]iconPart{}, valid...), valid[1])},
		{"an unknown part is 422", append(append([]iconPart{}, valid...), iconPart{"extra", []byte("x")})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertProblem(t, f.upload(t, tc.parts...), http.StatusUnprocessableEntity, ":validation")
			f.assertNothingStored(t)
		})
	}
}

func TestGetMediaBadUUIDIs422(t *testing.T) {
	f := newMediaFixture(t, nil)
	for _, target := range []string{mediaPath + "/not-a-uuid", mediaPath + "/not-a-uuid/thumb"} {
		t.Run(target, func(t *testing.T) {
			assertProblem(t, f.send(http.MethodGet, target, ""), http.StatusUnprocessableEntity, ":validation")
		})
	}
}

func TestGetMediaUnknownIs404(t *testing.T) {
	f := newMediaFixture(t, nil)
	id := uuid.NewString()
	for _, target := range []string{mediaPath + "/" + id, mediaPath + "/" + id + "/thumb"} {
		t.Run(target, func(t *testing.T) {
			assertProblem(t, f.send(http.MethodGet, target, ""), http.StatusNotFound, ":not-found")
		})
	}
}

func TestGetMediaFileMissingIs500WithoutPath(t *testing.T) {
	f := newMediaFixture(t, nil)
	got := f.uploadOK(t)
	for _, p := range storedFiles(t, f.dir) {
		if err := os.Remove(p); err != nil {
			t.Fatalf("remove stored file: %v", err)
		}
	}
	rec := f.send(http.MethodGet, mediaPath+"/"+got.Id.String(), "")
	assertProblem(t, rec, http.StatusInternalServerError, ":internal")
	if strings.Contains(rec.Body.String(), f.dir) {
		t.Fatalf("body %q names the media folder", rec.Body.String())
	}
}

func TestMediaRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	id := uuid.NewString()
	for _, rt := range []struct{ method, target, body string }{
		{http.MethodPost, mediaPath, `{}`},
		{http.MethodGet, mediaPath + "/" + id, ""},
		{http.MethodGet, mediaPath + "/" + id + "/thumb", ""},
		{http.MethodPatch, mediaPath + "/" + id, `{"archived":true}`},
		{http.MethodGet, clientsPath + "/" + id + "/media", ""},
	} {
		t.Run(fmt.Sprintf("%s %s", rt.method, rt.target), func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}

func TestPatchMediaArchiveHidesFromList(t *testing.T) {
	f := newMediaFixture(t, nil)
	got := f.uploadOK(t)
	list := func() []gen.Media {
		return decodeInto[gen.MediaList](t, f.send(http.MethodGet, clientMediaPath(f.client.Id), ""), http.StatusOK).Items
	}
	if items := list(); len(items) != 1 || items[0].Id != got.Id {
		t.Fatalf("list = %+v, want the photo", items)
	}
	rec := f.send(http.MethodPatch, mediaPath+"/"+got.Id.String(), `{"archived":true}`)
	if archived := decodeInto[gen.Media](t, rec, http.StatusOK); !archived.Archived {
		t.Fatalf("media = %+v, want archived", archived)
	}
	rec = f.send(http.MethodGet, clientMediaPath(f.client.Id), "")
	if items := decodeInto[gen.MediaList](t, rec, http.StatusOK).Items; len(items) != 0 || !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("list = %q, want an empty items array", rec.Body.String())
	}
	if rec := f.send(http.MethodGet, mediaPath+"/"+got.Id.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("GET archived photo = %d, want 200", rec.Code)
	}
	rec = f.send(http.MethodPatch, mediaPath+"/"+got.Id.String(), `{"archived":false}`)
	if restored := decodeInto[gen.Media](t, rec, http.StatusOK); restored.Archived {
		t.Fatalf("media = %+v, want restored", restored)
	}
	if items := list(); len(items) != 1 {
		t.Fatalf("list = %+v, want the restored photo", items)
	}
}

func TestPatchMediaBadInput(t *testing.T) {
	f := newMediaFixture(t, nil)
	got := f.uploadOK(t)
	target := mediaPath + "/" + got.Id.String()
	for _, tc := range []struct {
		name, body string
	}{
		{"a missing archived is 422 on archived", `{}`},
		{"a null archived is 422 on archived", `{"archived":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := fieldErrors(t, f.send(http.MethodPatch, target, tc.body))
			if len(errs) != 1 || errs[0].Field != "archived" {
				t.Fatalf("errors = %+v, want one on archived", errs)
			}
		})
	}
	assertProblem(t, f.send(http.MethodPatch, mediaPath+"/"+uuid.NewString(), `{"archived":true}`), http.StatusNotFound, ":not-found")
}

func TestListClientMediaUnknownIs404(t *testing.T) {
	f := newMediaFixture(t, nil)
	assertProblem(t, f.send(http.MethodGet, clientMediaPath(uuid.New()), ""), http.StatusNotFound, ":not-found")
}
