package archive

import (
	"testing"
	"time"
)

var (
	first = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	now   = first.Add(time.Hour)
)

func yes() *bool { v := true; return &v }
func no() *bool  { v := false; return &v }

func TestAtKeepsStoredWhenNotSent(t *testing.T) {
	if got := At(nil, nil, now); got != nil {
		t.Fatalf("active, not sent = %v, want nil", got)
	}
	if got := At(nil, &first, now); got == nil || !got.Equal(first) {
		t.Fatalf("archived, not sent = %v, want %v", got, first)
	}
}

func TestAtArchivesNow(t *testing.T) {
	if got := At(yes(), nil, now); got == nil || !got.Equal(now) {
		t.Fatalf("archive = %v, want %v", got, now)
	}
}

func TestAtKeepsFirstArchiveTime(t *testing.T) {
	if got := At(yes(), &first, now); got == nil || !got.Equal(first) {
		t.Fatalf("archive again = %v, want %v", got, first)
	}
}

func TestAtRestores(t *testing.T) {
	if got := At(no(), &first, now); got != nil {
		t.Fatalf("restore = %v, want nil", got)
	}
	if got := At(no(), nil, now); got != nil {
		t.Fatalf("restore an active one = %v, want nil", got)
	}
}
