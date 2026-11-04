// Package auth owns the users, sessions and setup tokens: first-run setup, login, sessions and
// password changes (design doc items 38–42). It imports no domain module: setup gets the settings
// creator as a function from cmd/open-customer.
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
}

// Auth is the auth module. Every method takes ctx and a db.DBTX first.
type Auth struct {
	clock           clock.Clock
	settingsCreator SettingsCreator
	// limiter is the login rate limit state (item 41), whose per-username delay also covers the
	// current password check of ChangePassword; it has its own mutex.
	limiter *loginLimiter
	// verify is verifyPassword. It is a field so a test can count the calls Login makes: the OP17
	// timing guard (an unknown username still verifies, against dummyHash) has no other trace.
	verify func(stored, pw string) (bool, error)
}

// New returns the auth module. A nil SettingsCreator is an error, so setup can never run without
// it (fail closed).
func New(opts Options) (*Auth, error) {
	if opts.SettingsCreator == nil {
		return nil, errors.New("auth: the settings creator is nil")
	}
	return &Auth{
		clock: opts.Clock, settingsCreator: opts.SettingsCreator,
		limiter: newLoginLimiter(), verify: verifyPassword,
	}, nil
}
