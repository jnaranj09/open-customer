package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// setupTokenLifetime is how long a setup link works ("Auth, sessions and handover").
const setupTokenLifetime = 24 * time.Hour

// setupLockKey names the transaction-scoped advisory lock that NewSetupToken and CompleteSetup take
// first, so two of them never interleave: two token calls cannot leave two live tokens, and two
// setups cannot both create a user. The value is arbitrary; it only has to be used by nothing else.
const setupLockKey int64 = 0x6f635f7365747570 // "oc_setup"

// SetupSettings is the settings part of the setup body. auth defines it so it never names a
// settings type (design doc item 40). A nil PushHour means "absent"; settings fills in 8.
type SetupSettings struct {
	Name, Timezone, Locale, Currency, CountryCode, AccentColor string
	PushHour                                                   *int
}

// SettingsCreator validates s and inserts the settings row in q. cmd/open-customer passes a
// closure over settings.Create.
type SettingsCreator func(ctx context.Context, q db.DBTX, s SetupSettings) error

// Seeder writes the starter data for locale in q (starter.Seed from P4).
type Seeder func(ctx context.Context, q db.DBTX, locale string) error

// SetupInput is the POST /setup body.
type SetupInput struct {
	Token, Username, Password string
	Settings                  SetupSettings
}

// SetupNeeded reports whether first-run setup is still open: true until the first user exists.
func (a *Auth) SetupNeeded(ctx context.Context, q db.DBTX) (bool, error) {
	exists, err := sqlc.New(q).UserExists(ctx)
	if err != nil {
		return false, fmt.Errorf("check for a user: %w", err)
	}
	return !exists, nil
}

// NewSetupToken deletes every old setup token, stores a new one valid for setupTokenLifetime and
// returns it for the setup link. Only its hash is stored, so the link cannot be printed again. Once
// a user exists it is problem.SetupClosed, even though both callers check first (OP17). q must be
// a transaction: the setup lock it takes first lasts until that transaction ends, and on the pool
// it would end with the statement.
func (a *Auth) NewSetupToken(ctx context.Context, q db.DBTX) (string, error) {
	queries := sqlc.New(q)
	if err := lockSetup(ctx, queries); err != nil {
		return "", err
	}
	if err := refuseOnceUserExists(ctx, queries); err != nil {
		return "", err
	}
	if err := queries.DeleteSetupTokens(ctx); err != nil {
		return "", fmt.Errorf("delete the setup tokens: %w", err)
	}
	token, hash, err := newToken()
	if err != nil {
		return "", err
	}
	if err := queries.InsertSetupToken(ctx, sqlc.InsertSetupTokenParams{
		TokenHash: hash, ExpiresAt: a.clock.Now().Add(setupTokenLifetime),
	}); err != nil {
		return "", fmt.Errorf("insert a setup token: %w", err)
	}
	return token, nil
}

// CompleteSetup creates the settings row (through the settings creator), the owner user, the
// starter data (through the seeder) and her first session, and deletes every setup token (design
// doc item 40). q must be a transaction: the POST /setup handler opens it, so any error leaves
// nothing behind, and the setup lock taken first holds until it ends. It returns the new user and
// the session token for the cookie.
func (a *Auth) CompleteSetup(ctx context.Context, q db.DBTX, in SetupInput) (Me, string, error) {
	queries := sqlc.New(q)
	if err := lockSetup(ctx, queries); err != nil {
		return Me{}, "", err
	}
	if err := refuseOnceUserExists(ctx, queries); err != nil {
		return Me{}, "", err
	}
	if err := a.checkSetupToken(ctx, queries, in.Token); err != nil {
		return Me{}, "", err
	}
	username, err := normalizeUsername(in.Username)
	if err != nil {
		return Me{}, "", err
	}
	if err := validatePassword("password", in.Password); err != nil {
		return Me{}, "", err
	}
	if err := a.settingsCreator(ctx, q, in.Settings); err != nil {
		return Me{}, "", prefixSettingsFields(err)
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return Me{}, "", err
	}
	userID, err := ids.New()
	if err != nil {
		return Me{}, "", fmt.Errorf("new user id: %w", err)
	}
	// must_change_password keeps its column default, false: she chose this password.
	if err := queries.InsertUser(ctx, sqlc.InsertUserParams{
		ID: userID, Username: username, PasswordHash: hash, CreatedAt: a.clock.Now(),
	}); err != nil {
		return Me{}, "", fmt.Errorf("insert the owner user: %w", err)
	}
	if err := a.seeder(ctx, q, in.Settings.Locale); err != nil {
		return Me{}, "", fmt.Errorf("seed the starter data: %w", err)
	}
	if err := queries.DeleteSetupTokens(ctx); err != nil {
		return Me{}, "", fmt.Errorf("delete the setup tokens: %w", err)
	}
	token, err := a.createSession(ctx, q, userID)
	if err != nil {
		return Me{}, "", err
	}
	return Me{ID: userID, Username: username}, token, nil
}

// lockSetup waits for the setup lock (setupLockKey). A concurrent setup or token call that held it
// has committed by the time it returns, so the checks after it see that call's writes.
func lockSetup(ctx context.Context, queries *sqlc.Queries) error {
	if err := queries.LockSetup(ctx, setupLockKey); err != nil {
		return fmt.Errorf("take the setup lock: %w", err)
	}
	return nil
}

// refuseOnceUserExists is problem.SetupClosed once the first user exists.
func refuseOnceUserExists(ctx context.Context, queries *sqlc.Queries) error {
	exists, err := queries.UserExists(ctx)
	if err != nil {
		return fmt.Errorf("check for a user: %w", err)
	}
	if exists {
		return problem.SetupClosed{}
	}
	return nil
}

// checkSetupToken is problem.SetupTokenInvalid unless token parses, is stored, unused and not
// expired.
func (a *Auth) checkSetupToken(ctx context.Context, queries *sqlc.Queries, token string) error {
	hash, err := parseToken(token)
	if err != nil {
		return problem.SetupTokenInvalid{}
	}
	row, err := queries.GetSetupToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem.SetupTokenInvalid{}
	}
	if err != nil {
		return fmt.Errorf("load the setup token: %w", err)
	}
	// used_at is never set today (CompleteSetup deletes every token, OP12), but a used token is
	// still refused.
	if row.UsedAt != nil || !a.clock.Now().Before(row.ExpiresAt) {
		return problem.SetupTokenInvalid{}
	}
	return nil
}

// prefixSettingsFields names each field of a settings Validation as it appears in the setup body
// (settings.timezone); any other error passes through as is (the creator already wraps it).
func prefixSettingsFields(err error) error {
	var v problem.Validation
	if !errors.As(err, &v) {
		return err
	}
	fields := make([]problem.FieldError, len(v.Errors))
	for i, fe := range v.Errors {
		fields[i] = problem.FieldError{Field: "settings." + fe.Field, Message: fe.Message}
	}
	return problem.Validation{Errors: fields}
}
