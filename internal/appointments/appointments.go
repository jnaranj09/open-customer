// Package appointments holds appointments: their status rules, the service lines copied onto them
// and, in later files, the stored operations. Done is computed on read, never stored.
package appointments

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status is where an appointment stands. Done is not a status: see IsDone.
type Status string

// The four statuses.
const (
	Scheduled Status = "scheduled"
	Confirmed Status = "confirmed"
	Cancelled Status = "cancelled"
	NoShow    Status = "no_show"
)

// Line is one service copied onto an appointment at save time. PriceCents is nil when the service
// had no price.
type Line struct {
	ServiceID   uuid.UUID
	Name        string
	PriceCents  *int64
	DurationMin int
}

// Appointment is one booking. Done and TotalCents are computed, not stored.
type Appointment struct {
	ID, ClientID uuid.UUID
	ClientName   string
	StartsAt     time.Time
	EndsAt       time.Time
	Status       Status
	Done         bool
	TotalCents   int64
	PaidCents    *int64
	Attrs        json.RawMessage
	Notes        *string
	Services     []Line
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
