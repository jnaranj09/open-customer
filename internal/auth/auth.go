// Package auth owns the users, sessions and setup tokens: first-run setup, login, sessions and
// password changes (design doc items 38–42). It imports no domain module.
package auth

import "github.com/jnaranj09/open-customer/internal/platform/clock"

// Options is what serve passes to New.
type Options struct {
	Clock clock.Clock
}

// Auth is the auth module. Every method takes ctx and a db.DBTX first.
type Auth struct {
	clock clock.Clock
}

// New returns the auth module.
func New(opts Options) *Auth {
	return &Auth{clock: opts.Clock}
}
