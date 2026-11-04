// Package auth owns the users, sessions and setup tokens: first-run setup, login, sessions and
// password changes (design doc items 38–42). It imports no domain module: setup gets the settings
// creator and the seeder as functions from cmd/open-customer.
package auth

import (
	"errors"

	"github.com/jnaranj09/open-customer/internal/platform/clock"
)

// Options is what serve passes to New. Every field is required.
type Options struct {
	Clock clock.Clock
	// SettingsCreator validates and inserts the settings row during setup.
	SettingsCreator SettingsCreator
	// Seeder writes the starter data during setup.
	Seeder Seeder
}

// Auth is the auth module. Every method takes ctx and a db.DBTX first.
type Auth struct {
	clock           clock.Clock
	settingsCreator SettingsCreator
	seeder          Seeder
}

// New returns the auth module. A nil SettingsCreator or Seeder is an error, so setup can never
// run without them (fail closed).
func New(opts Options) (*Auth, error) {
	if opts.SettingsCreator == nil {
		return nil, errors.New("auth: the settings creator is nil")
	}
	if opts.Seeder == nil {
		return nil, errors.New("auth: the seeder is nil")
	}
	return &Auth{clock: opts.Clock, settingsCreator: opts.SettingsCreator, seeder: opts.Seeder}, nil
}
