//go:build db

package media

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var start = time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

type fixture struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	dir   string
	m     *Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := clock.NewFake(start)
	dir := t.TempDir()
	return fixture{pool: dbtest.Pool(t), clock: fake, dir: dir, m: New(Options{Clock: fake, MediaDir: dir})}
}

func (f fixture) client(t *testing.T, name string, archived bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	q := `INSERT INTO clients (id, name) VALUES ($1, $2)`
	if archived {
		q = `INSERT INTO clients (id, name, archived_at) VALUES ($1, $2, now())`
	}
	if _, err := f.pool.Exec(t.Context(), q, id, name); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	return id
}

func (f fixture) appointment(t *testing.T, clientID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := f.pool.Exec(t.Context(),
		`INSERT INTO appointments (id, client_id, starts_at, ends_at) VALUES ($1, $2, $3, $4)`,
		id, clientID, start.Add(time.Hour), start.Add(2*time.Hour)); err != nil {
		t.Fatalf("insert appointment: %v", err)
	}
	return id
}

func (f fixture) input(t *testing.T, client, appt uuid.NullUUID) MediaInput {
	t.Helper()
	return MediaInput{ClientID: client, AppointmentID: appt, Full: testJPEG(t, 64, 48), Thumb: testJPEG(t, 16, 12)}
}

func owner(id uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: id, Valid: true} }

func wantField(t *testing.T, err error, field, msg string) {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v; want Validation", err)
	}
	if len(v.Errors) != 1 || v.Errors[0].Field != field || v.Errors[0].Message != msg {
		t.Fatalf("errors = %+v; want %s %q", v.Errors, field, msg)
	}
}

func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSaveFillsClientFromAppointment(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Lucía Rojas", false)
	a := f.appointment(t, c)
	md, err := f.m.Save(t.Context(), f.pool, f.input(t, uuid.NullUUID{}, owner(a)))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if md.ClientID != c || md.AppointmentID != owner(a) {
		t.Errorf("owner = %v / %v; want %v / %v", md.ClientID, md.AppointmentID, c, a)
	}
}

func TestSaveUnknownAppointmentIs422(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Save(t.Context(), f.pool, f.input(t, uuid.NullUUID{}, owner(uuid.New())))
	wantField(t, err, "appointment_id", problem.MsgAppointmentUnknown)
}

func TestSaveMismatchedClientIs422(t *testing.T) {
	f := newFixture(t)
	a := f.appointment(t, f.client(t, "Lucía Rojas", false))
	other := f.client(t, "Marta Solano", false)
	_, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(other), owner(a)))
	wantField(t, err, "client_id", problem.MsgMediaOwnerMismatch)
}

func TestSaveWithoutOwnerIs422(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Save(t.Context(), f.pool, f.input(t, uuid.NullUUID{}, uuid.NullUUID{}))
	wantField(t, err, "client_id", problem.MsgMediaOwnerRequired)
}

func TestSaveArchivedOrUnknownClientIs422(t *testing.T) {
	f := newFixture(t)
	archived := f.client(t, "Valeria Mora", true)
	for name, id := range map[string]uuid.UUID{"archived": archived, "unknown": uuid.New()} {
		t.Run(name, func(t *testing.T) {
			_, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(id), uuid.NullUUID{}))
			wantField(t, err, "client_id", problem.MsgClientUnavailable)
		})
	}
}

func TestSaveWritesNothingForAnInvalidOwner(t *testing.T) {
	f := newFixture(t)
	if _, err := f.m.Save(t.Context(), f.pool, f.input(t, uuid.NullUUID{}, uuid.NullUUID{})); err == nil {
		t.Fatal("Save returned nil")
	}
	if got := listFiles(t, f.dir); len(got) != 0 {
		t.Errorf("files = %v; want none", got)
	}
}

func TestSaveInsertFailureRemovesFiles(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Lucía Rojas", false)
	// A clean run counts the calls; the insert is the last one.
	calls := 0
	if _, err := f.m.Save(t.Context(), dbtest.FailingDB{DBTX: f.pool, Calls: &calls}, f.input(t, owner(c), uuid.NullUUID{})); err != nil {
		t.Fatalf("clean Save: %v", err)
	}
	for _, p := range listFiles(t, f.dir) {
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	_, err := f.m.Save(t.Context(), dbtest.FailingDB{DBTX: f.pool, FailAt: calls, Calls: &n}, f.input(t, owner(c), uuid.NullUUID{}))
	if !errors.Is(err, dbtest.ErrInjected) {
		t.Fatalf("err = %v; want ErrInjected", err)
	}
	if got := listFiles(t, f.dir); len(got) != 0 {
		t.Errorf("files left after a failed insert: %d", len(got))
	}
}

func TestDiscardRemovesFiles(t *testing.T) {
	f := newFixture(t)
	md, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(f.client(t, "Lucía Rojas", false)), uuid.NullUUID{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := listFiles(t, f.dir); len(got) != 2 {
		t.Fatalf("files after Save = %d; want 2", len(got))
	}
	f.m.Discard(t.Context(), md)
	if got := listFiles(t, f.dir); len(got) != 0 {
		t.Errorf("files after Discard = %v; want none", got)
	}
}

func TestDiscardLogsNoPath(t *testing.T) {
	f := newFixture(t)
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	// The media folder is gone, so removing the files fails.
	gone := filepath.Join(f.dir, "gone")
	New(Options{Clock: f.clock, MediaDir: gone}).Discard(t.Context(), Media{ID: uuid.New(), CreatedAt: start})
	out := logs.String()
	if !strings.Contains(out, "media_discard_failed") {
		t.Errorf("log = %q; want code media_discard_failed", out)
	}
	if strings.Contains(out, gone) || strings.Contains(out, ".jpg") {
		t.Errorf("log names a path: %q", out)
	}
}

func TestSaveWritesFilesBeforeRow(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Lucía Rojas", false)
	in := f.input(t, owner(c), uuid.NullUUID{})
	md, err := f.m.Save(t.Context(), f.pool, in)
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(f.dir, "2026", "03", md.ID.String()+".jpg")
	thumb := filepath.Join(f.dir, "2026", "03", md.ID.String()+"_t.jpg")
	root, err := os.OpenRoot(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for path, want := range map[string][]byte{full: in.Full, thumb: in.Thumb} {
		rel, err := filepath.Rel(f.dir, path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := root.ReadFile(rel)
		if err != nil {
			t.Fatalf("read stored file: %v", err)
		}
		if string(got) != string(want) {
			t.Errorf("stored file differs from the upload")
		}
	}
	if md.Width != 64 || md.Height != 48 || md.Bytes != len(in.Full) || !md.CreatedAt.Equal(start) {
		t.Errorf("media = %+v; want 64x48, %d bytes, created %v", md, len(in.Full), start)
	}
	var key, thumbKey, ctype string
	if err := f.pool.QueryRow(t.Context(), `SELECT storage_key, thumb_key, content_type FROM media WHERE id = $1`, md.ID).Scan(&key, &thumbKey, &ctype); err != nil {
		t.Fatal(err)
	}
	if key != "2026/03/"+md.ID.String()+".jpg" || thumbKey != "2026/03/"+md.ID.String()+"_t.jpg" || ctype != "image/jpeg" {
		t.Errorf("row = %q %q %q", key, thumbKey, ctype)
	}
}

func TestSaveUsesUTCFolders(t *testing.T) {
	f := newFixture(t)
	// 01:30 on 1 Jan at UTC+3 is still December in UTC, whatever the host zone.
	f.clock.Set(time.Date(2027, 1, 1, 1, 30, 0, 0, time.FixedZone("x", 3*3600)))
	md, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(f.client(t, "Lucía Rojas", false)), uuid.NullUUID{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "2026", "12", md.ID.String()+".jpg")); err != nil {
		t.Errorf("file not under the UTC folder: %v", err)
	}
}

func TestOpenFullAndThumb(t *testing.T) {
	f := newFixture(t)
	in := f.input(t, owner(f.client(t, "Lucía Rojas", false)), uuid.NullUUID{})
	md, err := f.m.Save(t.Context(), f.pool, in)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		thumb bool
		want  []byte
	}{"full": {false, in.Full}, "thumb": {true, in.Thumb}} {
		t.Run(name, func(t *testing.T) {
			rc, got, err := f.m.Open(t.Context(), f.pool, md.ID, tc.thumb)
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != string(tc.want) || got.ID != md.ID {
				t.Errorf("Open(%s) returned the wrong file or media", name)
			}
		})
	}
}

func TestOpenUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, _, err := f.m.Open(t.Context(), f.pool, uuid.New(), false)
	if !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("err = %v; want NotFound", err)
	}
}

func TestOpenMissingFileLogsNoPath(t *testing.T) {
	f := newFixture(t)
	md, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(f.client(t, "Lucía Rojas", false)), uuid.NullUUID{}))
	if err != nil {
		t.Fatal(err)
	}
	f.m.Discard(t.Context(), md)
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	_, _, err = f.m.Open(t.Context(), f.pool, md.ID, false)
	if err == nil {
		t.Fatal("Open of a missing file returned nil")
	}
	if strings.Contains(err.Error(), f.dir) || strings.Contains(err.Error(), ".jpg") {
		t.Errorf("error names a path: %v", err)
	}
	out := logs.String()
	if !strings.Contains(out, "media_file_missing") || strings.Contains(out, f.dir) || strings.Contains(out, ".jpg") {
		t.Errorf("log = %q; want code media_file_missing and no path", out)
	}
}

func TestListForClientNewestFirstHidesArchived(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Lucía Rojas", false)
	var saved []Media
	for range 3 {
		f.clock.Advance(time.Minute)
		md, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(c), uuid.NullUUID{}))
		if err != nil {
			t.Fatal(err)
		}
		saved = append(saved, md)
	}
	if _, err := f.m.SetArchived(t.Context(), f.pool, saved[1].ID, true); err != nil {
		t.Fatal(err)
	}
	got, err := f.m.ListForClient(t.Context(), f.pool, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != saved[2].ID || got[1].ID != saved[0].ID {
		t.Errorf("list = %+v; want newest first without the archived one", got)
	}
}

func TestListForClientEmptyIsNonNil(t *testing.T) {
	f := newFixture(t)
	got, err := f.m.ListForClient(t.Context(), f.pool, f.client(t, "Lucía Rojas", false))
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("list = %v, %v; want empty non-nil", got, err)
	}
}

func TestListForClientUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.ListForClient(t.Context(), f.pool, uuid.New())
	if !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("err = %v; want NotFound", err)
	}
}

func TestListForArchivedClientStillLists(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "Valeria Mora", false)
	if _, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(c), uuid.NullUUID{})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE clients SET archived_at = now() WHERE id = $1`, c); err != nil {
		t.Fatal(err)
	}
	got, err := f.m.ListForClient(t.Context(), f.pool, c)
	if err != nil || len(got) != 1 {
		t.Errorf("list = %v, %v; want the photo of an archived client", got, err)
	}
}

func TestSetArchivedStillOpens(t *testing.T) {
	f := newFixture(t)
	md, err := f.m.Save(t.Context(), f.pool, f.input(t, owner(f.client(t, "Lucía Rojas", false)), uuid.NullUUID{}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.m.SetArchived(t.Context(), f.pool, md.ID, true)
	if err != nil || !got.Archived {
		t.Fatalf("SetArchived = %+v, %v; want archived", got, err)
	}
	rc, opened, err := f.m.Open(t.Context(), f.pool, md.ID, false)
	if err != nil {
		t.Fatalf("Open of archived media: %v", err)
	}
	defer rc.Close()
	if !opened.Archived {
		t.Error("opened media is not marked archived")
	}
	back, err := f.m.SetArchived(t.Context(), f.pool, md.ID, false)
	if err != nil || back.Archived {
		t.Errorf("restore = %+v, %v; want active", back, err)
	}
}

func TestSetArchivedUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.SetArchived(t.Context(), f.pool, uuid.New(), true)
	if !errors.As(err, new(problem.NotFound)) {
		t.Fatalf("err = %v; want NotFound", err)
	}
}

func TestSaveUnwritableMediaDirLeaksNoPath(t *testing.T) {
	f := newFixture(t)
	locked := filepath.Join(f.dir, "locked-dir")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	m := New(Options{Clock: f.clock, MediaDir: locked})
	_, err := m.Save(t.Context(), f.pool, f.input(t, owner(f.client(t, "Lucía Rojas", false)), uuid.NullUUID{}))
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v; want fs.ErrPermission", err)
	}
	for name, text := range map[string]string{"error": err.Error(), "log": logs.String()} {
		if strings.Contains(text, "locked-dir") || strings.Contains(text, ".jpg") {
			t.Errorf("%s names a path: %q", name, text)
		}
	}
}
