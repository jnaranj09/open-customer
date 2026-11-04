// Package archive holds the one rule for the archived_at column of the catalog tables.
package archive

import "time"

// At is the archive time after a patch: unchanged when archived is nil, the stored time (or now,
// when none) for true, and none for false. Archiving twice keeps the first time.
func At(archived *bool, stored *time.Time, now time.Time) *time.Time {
	switch {
	case archived == nil:
		return stored
	case !*archived:
		return nil
	case stored != nil:
		return stored
	}
	return &now
}
