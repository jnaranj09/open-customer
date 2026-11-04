// Package settings owns the single business_settings row: validation, get, create and update
// (design doc item 44). It imports no other domain module.
package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nyaruka/phonenumbers"
	"golang.org/x/text/currency"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/color"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
)

const (
	defaultPushHour = 8
	maxPushHour     = 19
	maxNameLen      = 80
	supportedLocale = "es"
)

// Options is what serve passes to New.
type Options struct {
	Clock    clock.Clock
	MediaDir string
}

// Module is the settings module. Every method takes ctx and a db.DBTX first.
type Module struct {
	clock    clock.Clock
	mediaDir string
}

// New returns the settings module.
func New(opts Options) *Module {
	return &Module{clock: opts.Clock, mediaDir: opts.MediaDir}
}

// Settings is the business settings as stored. HasCustomIcon is false while the built-in icon is
// in use.
type Settings struct {
	Name, Timezone, Locale, Currency, CountryCode, AccentColor string
	PushHour                                                   int
	HasCustomIcon                                              bool
}

// SettingsInput is everything Create needs. A nil PushHour means the default.
type SettingsInput struct {
	Name, Timezone, Locale, Currency, CountryCode, AccentColor string
	PushHour                                                   *int
}

// SettingsPatch holds the fields Update changes; a nil field is left as it is.
type SettingsPatch struct {
	Name, Timezone, AccentColor *string
	PushHour                    *int
}

// validated is a SettingsInput after validation, with the push hour filled in.
type validated struct {
	Name, Timezone, Locale, Currency, CountryCode, AccentColor string
	PushHour                                                   int
}

// Countries returns the supported ISO 3166-1 alpha-2 region codes, sorted.
func Countries() []string {
	regions := phonenumbers.GetSupportedRegions()
	out := make([]string, 0, len(regions))
	for code := range regions {
		out = append(out, code)
	}
	slices.Sort(out)
	return out
}

// Get returns the settings, or problem.NotFound before setup has created them.
func (m *Module) Get(ctx context.Context, q db.DBTX) (Settings, error) {
	row, err := sqlc.New(q).GetSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, problem.NotFound{}
	}
	if err != nil {
		return Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return Settings{
		Name: row.Name, Timezone: row.Timezone, Locale: row.Locale, Currency: row.Currency,
		CountryCode: row.CountryCode, AccentColor: row.AccentColor, PushHour: int(row.PushHour),
		HasCustomIcon: row.IconUpdatedAt != nil,
	}, nil
}

// Create validates in and inserts the one settings row. A second call fails on the primary key.
func (m *Module) Create(ctx context.Context, q db.DBTX, in SettingsInput) (Settings, error) {
	v, err := validateInput(in)
	if err != nil {
		return Settings{}, err
	}
	if err := sqlc.New(q).InsertSettings(ctx, sqlc.InsertSettingsParams{
		Name: v.Name, Timezone: v.Timezone, Locale: v.Locale, Currency: v.Currency,
		CountryCode: v.CountryCode, AccentColor: v.AccentColor, PushHour: smallint(v.PushHour),
		Now: m.clock.Now(),
	}); err != nil {
		return Settings{}, fmt.Errorf("insert settings: %w", err)
	}
	return Settings{
		Name: v.Name, Timezone: v.Timezone, Locale: v.Locale, Currency: v.Currency,
		CountryCode: v.CountryCode, AccentColor: v.AccentColor, PushHour: v.PushHour,
	}, nil
}

// Update validates the given fields, changes only those and returns the stored settings. It is
// problem.NotFound before setup.
func (m *Module) Update(ctx context.Context, q db.DBTX, p SettingsPatch) (Settings, error) {
	p, err := validatePatch(p)
	if err != nil {
		return Settings{}, err
	}
	params := sqlc.UpdateSettingsParams{Name: p.Name, Timezone: p.Timezone, AccentColor: p.AccentColor, Now: m.clock.Now()}
	if p.PushHour != nil {
		hour := smallint(*p.PushHour)
		params.PushHour = &hour
	}
	if err := sqlc.New(q).UpdateSettings(ctx, params); err != nil {
		return Settings{}, fmt.Errorf("update settings: %w", err)
	}
	// Before setup the update touches no row and Get gives problem.NotFound.
	return m.Get(ctx, q)
}

func validateInput(in SettingsInput) (validated, error) {
	v := validated{
		Locale: in.Locale, CountryCode: in.CountryCode, PushHour: defaultPushHour,
	}
	var errs fieldErrors
	v.Name = errs.name(in.Name)
	v.Timezone = errs.timezone(in.Timezone)
	errs.locale(in.Locale)
	v.Currency = errs.currency(in.Currency)
	errs.country(in.CountryCode)
	v.AccentColor = errs.accent(in.AccentColor)
	if in.PushHour != nil {
		v.PushHour = errs.pushHour(*in.PushHour)
	}
	return v, errs.err()
}

func validatePatch(p SettingsPatch) (SettingsPatch, error) {
	var errs fieldErrors
	out := SettingsPatch{}
	if p.Name != nil {
		out.Name = ptr(errs.name(*p.Name))
	}
	if p.Timezone != nil {
		out.Timezone = ptr(errs.timezone(*p.Timezone))
	}
	if p.AccentColor != nil {
		out.AccentColor = ptr(errs.accent(*p.AccentColor))
	}
	if p.PushHour != nil {
		out.PushHour = ptr(errs.pushHour(*p.PushHour))
	}
	return out, errs.err()
}

// smallint converts a validated push hour to the column type; the clamp keeps it in range.
func smallint(h int) int16 { return int16(min(max(h, 0), maxPushHour)) }

func ptr[T any](v T) *T { return &v }

// fieldErrors collects every bad field, so one response names them all.
type fieldErrors []problem.FieldError

func (e *fieldErrors) add(field, msg string) {
	*e = append(*e, problem.FieldError{Field: field, Message: msg})
}

func (e fieldErrors) err() error {
	if len(e) == 0 {
		return nil
	}
	return problem.Validation{Errors: e}
}

func (e *fieldErrors) name(s string) string {
	s, ok := text.Trim(s, 1, maxNameLen)
	if !ok {
		e.add("name", problem.MsgNameInvalid)
	}
	return s
}

func (e *fieldErrors) timezone(s string) string {
	if s == "" || s == "Local" {
		e.add("timezone", problem.MsgTimezoneInvalid)
		return s
	}
	if _, err := time.LoadLocation(s); err != nil {
		e.add("timezone", problem.MsgTimezoneInvalid)
	}
	return s
}

func (e *fieldErrors) locale(s string) {
	if s != supportedLocale {
		e.add("locale", problem.MsgLocaleInvalid)
	}
}

// currency accepts an ISO 4217 code in any case and returns it upper case. XXX (no currency) and
// XTS (testing) parse but are not real currencies, so they are rejected.
func (e *fieldErrors) currency(s string) string {
	unit, err := currency.ParseISO(s)
	if err != nil || unit == currency.XXX || unit == currency.XTS {
		e.add("currency", problem.MsgCurrencyInvalid)
		return s
	}
	return unit.String()
}

func (e *fieldErrors) country(s string) {
	if _, ok := phonenumbers.GetSupportedRegions()[s]; !ok {
		e.add("country_code", problem.MsgCountryInvalid)
	}
}

func (e *fieldErrors) accent(s string) string {
	s, ok := color.Normalize(s)
	if !ok {
		e.add("accent_color", problem.MsgColorInvalid)
	}
	return s
}

func (e *fieldErrors) pushHour(h int) int {
	if h < 0 || h > maxPushHour {
		e.add("push_hour", problem.MsgPushHourInvalid)
	}
	return h
}
