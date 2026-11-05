//go:build !linux && !darwin && !freebsd && !dragonfly

package status

import "context"

// mediaFreeBytes reports nil (unknown) where unix.Statfs is not available, such as Windows. The
// app runs on Linux; this file only keeps `go build` working on every GOOS (design doc item 2).
func (s *Status) mediaFreeBytes(_ context.Context) *int64 {
	return nil
}
