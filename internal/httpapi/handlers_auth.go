package httpapi

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/settings"
)

// GetSetupStatus has no session on purpose: the setup wizard asks before any user exists (see the
// spec description). It is a GET, so it reads through the pool.
func (s *Server) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	needed, err := s.auth.SetupNeeded(r.Context(), s.pool)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.SetupStatus{NeedsSetup: needed})
}

// GetSetupOptions has no session on purpose: the setup wizard needs the country list before any
// user exists (see the spec description).
func (s *Server) GetSetupOptions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, gen.SetupOptions{Countries: settings.Countries()})
}

// CompleteSetup has no session on purpose: the setup token in the body authorizes it. It runs in
// a READ COMMITTED transaction (the setup lock CompleteSetup takes is transaction-scoped, and a
// second setup waiting on it then sees the first one's user and gets setup-closed, not a
// serialization failure). The cookie and the 201 are written only after the commit.
func (s *Server) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	var body gen.SetupInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := auth.SetupInput{
		Token: body.Token, Username: body.Username, Password: body.Password,
		Settings: auth.SetupSettings{
			Name: body.Settings.Name, Timezone: body.Settings.Timezone, Locale: body.Settings.Locale,
			Currency: body.Settings.Currency, CountryCode: body.Settings.CountryCode,
			AccentColor: body.Settings.AccentColor, PushHour: body.Settings.PushHour,
		},
	}
	var me auth.Me
	var token string
	err := db.WithTx(r.Context(), s.beginner, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		if me, token, err = s.auth.CompleteSetup(r.Context(), tx, in); err != nil {
			return fmt.Errorf("complete setup: %w", err)
		}
		return nil
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	http.SetCookie(w, auth.SessionCookie(token, s.secureCookies))
	writeJSON(w, http.StatusCreated, toMe(me))
}

// Login has no session on purpose: it is how a session is made. The rate limits are per client
// IP (from the client-IP middleware step) and per username. The cookie and the 200 are written
// only after the commit.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var body gen.LoginInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := auth.LoginInput{Username: body.Username, Password: body.Password}
	var me auth.Me
	var token string
	err := db.WithTx(r.Context(), s.beginner, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		if me, token, err = s.auth.Login(r.Context(), tx, in, clientIP(r.Context())); err != nil {
			return fmt.Errorf("login: %w", err)
		}
		return nil
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	http.SetCookie(w, auth.SessionCookie(token, s.secureCookies))
	writeJSON(w, http.StatusOK, toMe(me))
}

func toMe(me auth.Me) gen.Me {
	return gen.Me{Id: me.ID, Username: me.Username, MustChangePassword: me.MustChangePassword}
}
