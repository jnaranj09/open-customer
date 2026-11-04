// Package media stores the photos of appointments and clients: two JPEG files on disk (the full
// image and a thumbnail) and one row that describes them. Files are written before the row, so
// there is never a row without its file.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/files"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Upload size limits per part.
const (
	MaxFullBytes  = 10 << 20
	MaxThumbBytes = 1 << 20
)

const contentType = "image/jpeg"

// jpegSignature is how every JPEG file starts.
var jpegSignature = []byte{0xFF, 0xD8, 0xFF}

// Options is what serve passes to New. MediaDir is the root of the stored files.
type Options struct {
	Clock    clock.Clock
	MediaDir string
}

// Module is the media module. Every method takes ctx and a db.DBTX first.
type Module struct {
	clock    clock.Clock
	mediaDir string
}

// New returns the media module.
func New(opts Options) *Module { return &Module{clock: opts.Clock, mediaDir: opts.MediaDir} }

// Media is one stored photo. Width, Height and Bytes describe the full image.
type Media struct {
	ID, ClientID  uuid.UUID
	AppointmentID uuid.NullUUID
	Width, Height int
	Bytes         int
	CreatedAt     time.Time
	Archived      bool
}

// MediaInput is one upload. Either ID may be unset, but not both. With an AppointmentID the
// client is the appointment's.
type MediaInput struct {
	ClientID, AppointmentID uuid.NullUUID
	Full, Thumb             []byte
}

// Save checks the upload, writes both files and then inserts the row. When the insert fails the
// files are removed. When the caller's commit fails after Save, the caller calls Discard.
func (m *Module) Save(ctx context.Context, q db.DBTX, in MediaInput) (Media, error) {
	if !bytes.HasPrefix(in.Full, jpegSignature) || !bytes.HasPrefix(in.Thumb, jpegSignature) {
		return Media{}, problem.UnsupportedMediaType{}
	}
	if len(in.Full) > MaxFullBytes || len(in.Thumb) > MaxThumbBytes {
		return Media{}, problem.PayloadTooLarge{}
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(in.Full))
	if err != nil {
		return Media{}, problem.UnsupportedMediaType{}
	}
	if _, err := jpeg.DecodeConfig(bytes.NewReader(in.Thumb)); err != nil {
		return Media{}, problem.UnsupportedMediaType{}
	}
	size, okSize := int32Of(len(in.Full))
	width, okWidth := int32Of(cfg.Width)
	height, okHeight := int32Of(cfg.Height)
	if !okSize || !okWidth || !okHeight {
		return Media{}, errors.New("image size does not fit the column type")
	}
	queries := sqlc.New(q)
	clientID, err := resolveOwner(ctx, queries, in)
	if err != nil {
		return Media{}, err
	}
	id, err := ids.New()
	if err != nil {
		return Media{}, fmt.Errorf("new media id: %w", err)
	}
	now := m.clock.Now()
	md := Media{
		ID: id, ClientID: clientID, AppointmentID: in.AppointmentID,
		Width: cfg.Width, Height: cfg.Height, Bytes: len(in.Full), CreatedAt: now,
	}
	key, thumbKey := keys(md)
	if err := m.writeFiles(key, thumbKey, in); err != nil {
		m.Discard(ctx, md)
		return Media{}, err
	}
	if err := queries.InsertMedia(ctx, sqlc.InsertMediaParams{
		ID: id, ClientID: clientID, AppointmentID: in.AppointmentID, StorageKey: key, ThumbKey: thumbKey,
		ContentType: contentType, Bytes: size, Width: width, Height: height,
		CreatedAt: now,
	}); err != nil {
		m.Discard(ctx, md)
		return Media{}, fmt.Errorf("insert media: %w", err)
	}
	return md, nil
}

// int32Of narrows a size or a dimension to the column type, and reports false when it does not
// fit. The upload limits and the JPEG header keep both inside the range, so false is a bug.
func int32Of(n int) (int32, bool) {
	if n < 0 || n > math.MaxInt32 {
		return 0, false
	}
	return int32(n), true
}

// writeFiles creates the folder and writes both files, synced.
func (m *Module) writeFiles(key, thumbKey string, in MediaInput) error {
	if err := files.MkdirAll(filepath.Join(m.mediaDir, filepath.FromSlash(path.Dir(key))), 0o750); err != nil {
		return fmt.Errorf("create media folder: %w", err)
	}
	if err := files.WriteSynced(filepath.Join(m.mediaDir, filepath.FromSlash(key)), in.Full); err != nil {
		return fmt.Errorf("write full image: %w", err)
	}
	if err := files.WriteSynced(filepath.Join(m.mediaDir, filepath.FromSlash(thumbKey)), in.Thumb); err != nil {
		return fmt.Errorf("write thumbnail: %w", err)
	}
	return nil
}

// resolveOwner resolves the client of an upload and checks the client is active.
func resolveOwner(ctx context.Context, queries *sqlc.Queries, in MediaInput) (uuid.UUID, error) {
	if !in.ClientID.Valid && !in.AppointmentID.Valid {
		return uuid.Nil, fieldProblem("client_id", problem.MsgMediaOwnerRequired)
	}
	clientID := in.ClientID.UUID
	if in.AppointmentID.Valid {
		apptClient, err := queries.GetAppointmentOwner(ctx, in.AppointmentID.UUID)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fieldProblem("appointment_id", problem.MsgAppointmentUnknown)
		}
		if err != nil {
			return uuid.Nil, fmt.Errorf("get appointment owner: %w", err)
		}
		if in.ClientID.Valid && in.ClientID.UUID != apptClient {
			return uuid.Nil, fieldProblem("client_id", problem.MsgMediaOwnerMismatch)
		}
		clientID = apptClient
	}
	archivedAt, err := queries.GetClientArchivedAt(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && archivedAt != nil) {
		return uuid.Nil, fieldProblem("client_id", problem.MsgClientUnavailable)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get client state: %w", err)
	}
	return clientID, nil
}

func fieldProblem(field, msg string) error {
	return problem.Validation{Errors: []problem.FieldError{{Field: field, Message: msg}}}
}

// keys returns the storage keys of a photo, relative to MediaDir: <yyyy>/<mm> is the UTC month of
// the upload.
func keys(md Media) (full, thumb string) {
	t := md.CreatedAt.UTC()
	base := fmt.Sprintf("%04d/%02d/%s", t.Year(), int(t.Month()), md.ID)
	return base + ".jpg", base + "_t.jpg"
}

// Discard removes both files of a saved photo. The handler calls it when the commit that should
// have stored the row fails. A failure is logged with a code and never the path.
func (m *Module) Discard(ctx context.Context, md Media) {
	root, err := os.OpenRoot(m.mediaDir)
	if err != nil {
		slog.ErrorContext(ctx, "could not remove the files of a discarded photo", "code", "media_discard_failed")
		return
	}
	defer root.Close()
	key, thumbKey := keys(md)
	for _, k := range []string{key, thumbKey} {
		if err := root.Remove(k); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.ErrorContext(ctx, "could not remove a file of a discarded photo", "code", "media_discard_failed")
		}
	}
}

// Open returns the full image or the thumbnail of one photo, archived or not. An unknown id is
// problem.NotFound. A row whose file is missing is an error that names no path.
func (m *Module) Open(ctx context.Context, q db.DBTX, id uuid.UUID, thumb bool) (io.ReadSeekCloser, Media, error) {
	row, err := sqlc.New(q).GetMedia(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, Media{}, problem.NotFound{}
	}
	if err != nil {
		return nil, Media{}, fmt.Errorf("get media: %w", err)
	}
	key := row.StorageKey
	if thumb {
		key = row.ThumbKey
	}
	root, err := os.OpenRoot(m.mediaDir)
	if err != nil {
		return nil, Media{}, missing(ctx, err)
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(key))
	if err != nil {
		return nil, Media{}, missing(ctx, err)
	}
	return f, mediaFrom(row), nil
}

// missing logs a file that cannot be opened (code, never the path) and returns an error without
// the path.
func missing(ctx context.Context, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		slog.ErrorContext(ctx, "a stored photo file is missing", "code", "media_file_missing")
		return errors.New("media file is missing")
	}
	slog.ErrorContext(ctx, "a stored photo file cannot be opened", "code", "media_file_unreadable")
	return errors.New("media file cannot be opened")
}

// ListForClient returns the client's active photos, newest first. An unknown client is
// problem.NotFound. The slice is never nil.
func (m *Module) ListForClient(ctx context.Context, q db.DBTX, clientID uuid.UUID) ([]Media, error) {
	queries := sqlc.New(q)
	if _, err := queries.GetClientArchivedAt(ctx, clientID); errors.Is(err, pgx.ErrNoRows) {
		return nil, problem.NotFound{}
	} else if err != nil {
		return nil, fmt.Errorf("get client state: %w", err)
	}
	rows, err := queries.ListMediaForClient(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	out := make([]Media, 0, len(rows))
	for _, r := range rows {
		out = append(out, mediaFrom(r))
	}
	return out, nil
}

// SetArchived hides or restores one photo. An unknown id is problem.NotFound.
func (m *Module) SetArchived(ctx context.Context, q db.DBTX, id uuid.UUID, archived bool) (Media, error) {
	var at *time.Time
	if archived {
		now := m.clock.Now()
		at = &now
	}
	row, err := sqlc.New(q).SetMediaArchived(ctx, sqlc.SetMediaArchivedParams{ID: id, ArchivedAt: at})
	if errors.Is(err, pgx.ErrNoRows) {
		return Media{}, problem.NotFound{}
	}
	if err != nil {
		return Media{}, fmt.Errorf("set media archived: %w", err)
	}
	return mediaFrom(row), nil
}

func mediaFrom(r sqlc.Medium) Media {
	return Media{
		ID: r.ID, ClientID: r.ClientID, AppointmentID: r.AppointmentID,
		Width: int(r.Width), Height: int(r.Height), Bytes: int(r.Bytes),
		CreatedAt: r.CreatedAt, Archived: r.ArchivedAt != nil,
	}
}
