package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/auth"
	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/settings"
)

// GetSetupStatus has no session on purpose: the setup wizard asks before any user exists (see the
// spec description). It is a GET, so it reads through the pool. After setup it adds the accent
// color, which is public anyway (the manifest's theme_color), for the login screen.
func (s *Server) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	needed, err := s.auth.SetupNeeded(r.Context(), s.pool)
	if err != nil {
		writeError(w, r, err)
		return
	}
	status := gen.SetupStatus{NeedsSetup: needed}
	if !needed {
		set, err := s.settings.Get(r.Context(), s.pool)
		switch {
		case err == nil:
			status.AccentColor = &set.AccentColor
		case errors.As(err, &problem.NotFound{}):
			// A user with no settings row: setup writes both in one transaction, so only a test
			// fixture gets here. The answer simply has no color.
		default:
			writeError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, status)
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
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if me, token, err = s.auth.CompleteSetup(r.Context(), tx, in); err != nil {
			return fmt.Errorf("complete setup: %w", err)
		}
		return nil
	}) {
		return
	}
	http.SetCookie(w, auth.SessionCookie(token, s.secureCookies))
	writeJSON(w, http.StatusCreated, toMe(me))
}

// Login has no session on purpose: it is how a session is made. The rate limits are per client
// IP (from the client-IP middleware step) and per username, checked before the transaction, so
// a 429 opens none. The cookie and the 200 are written only after the commit.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var body gen.LoginInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	attempt, err := s.auth.BeginLogin(auth.LoginInput{Username: body.Username, Password: body.Password}, clientIP(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	var me auth.Me
	var token string
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if me, token, err = s.auth.Login(r.Context(), tx, attempt); err != nil {
			return fmt.Errorf("login: %w", err)
		}
		return nil
	}) {
		return
	}
	http.SetCookie(w, auth.SessionCookie(token, s.secureCookies))
	writeJSON(w, http.StatusOK, toMe(me))
}

// GetMe answers the signed-in user from the session step 6 loaded. It is allowed while
// must_change_password is set (the spec's x-oc-allowed-while-must-change-password).
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toMe(mustSignedIn(r.Context()).me))
}

// Logout deletes the session step 6 loaded and expires the cookie, only after the commit. Step 7
// has already refused a request without a session; like GetMe and ChangePassword it does no 401
// of its own.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	session := mustSignedIn(r.Context()).session
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		if err := s.auth.Logout(r.Context(), tx, session); err != nil {
			return fmt.Errorf("logout: %w", err)
		}
		return nil
	}) {
		return
	}
	http.SetCookie(w, auth.ExpiredCookie(s.secureCookies))
	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword changes the signed-in user's password, keeping this session and closing the
// others. A wrong current password is 422 on current_password and counts toward the username's
// login delay (429 when the next try comes too early, checked before the transaction, so it opens
// none).
func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var body gen.PasswordChange
	if decodeJSON(w, r, &body) != nil {
		return
	}
	cur := mustSignedIn(r.Context())
	attempt, err := s.auth.BeginPasswordChange(cur.me)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		if err := s.auth.ChangePassword(r.Context(), tx, cur.session, attempt, body.CurrentPassword, body.NewPassword); err != nil {
			return fmt.Errorf("change the password: %w", err)
		}
		return nil
	}) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toMe(me auth.Me) gen.Me {
	return gen.Me{Id: me.ID, Username: me.Username, MustChangePassword: me.MustChangePassword}
}
