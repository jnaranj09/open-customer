//go:build linux || darwin || freebsd || dragonfly

package status

import (
	"context"
	"log/slog"
	"math"
	"math/big"

	"golang.org/x/sys/unix"
)

// mediaFreeBytes is the space an unprivileged process can still use on the media volume, or nil
// when it cannot be read. The Statfs field types differ per OS and architecture, so both fields
// go through toUint64.
func (s *Status) mediaFreeBytes(ctx context.Context) *int64 {
	var st unix.Statfs_t
	if err := unix.Statfs(s.mediaDir, &st); err != nil {
		slog.WarnContext(ctx, "media free space unavailable", "code", "status_statfs_failed")
		return nil
	}
	// big.Int keeps the product exact; an absurd value is capped.
	total := new(big.Int).Mul(new(big.Int).SetUint64(toUint64(st.Bavail)), new(big.Int).SetUint64(toUint64(st.Bsize)))
	n := int64(math.MaxInt64)
	if total.IsInt64() {
		n = total.Int64()
	}
	return &n
}

// toUint64 widens a Statfs field of any integer type; a negative count (FreeBSD reports one when
// the reserved blocks are in use) is no free space.
func toUint64[T ~int32 | ~int64 | ~uint32 | ~uint64](v T) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}
