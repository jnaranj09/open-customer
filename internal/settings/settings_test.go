package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func validInput() SettingsInput {
	return SettingsInput{
		Name: "Made Up Studio", Timezone: "Europe/Madrid", Locale: "es", Currency: "EUR",
		CountryCode: "ES", AccentColor: "#aa3366",
	}
}

func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("error = %v, want problem.Validation", err)
	}
	fields := make([]string, 0, len(v.Errors))
	for _, e := range v.Errors {
		if e.Message == "" {
			t.Errorf("field %q has an empty message", e.Field)
		}
		fields = append(fields, e.Field)
	}
	return fields
}

func TestValidateSettingsInput(t *testing.T) {
	bad := []struct {
		name   string
		mutate func(*SettingsInput)
		field  string
	}{
		{"empty name is rejected", func(i *SettingsInput) { i.Name = "  " }, "name"},
		{"81 character name is rejected", func(i *SettingsInput) { i.Name = strings.Repeat("a", 81) }, "name"},
		{"non-es locale is rejected", func(i *SettingsInput) { i.Locale = "en" }, "locale"},
		{"short accent color is rejected", func(i *SettingsInput) { i.AccentColor = "#ABC" }, "accent_color"},
		{"push hour 20 is rejected", func(i *SettingsInput) { h := 20; i.PushHour = &h }, "push_hour"},
		{"push hour -1 is rejected", func(i *SettingsInput) { h := -1; i.PushHour = &h }, "push_hour"},
		{"unknown time zone is rejected", func(i *SettingsInput) { i.Timezone = "Mars/Base" }, "timezone"},
		{"Local time zone is rejected", func(i *SettingsInput) { i.Timezone = "Local" }, "timezone"},
		{"empty time zone is rejected", func(i *SettingsInput) { i.Timezone = "" }, "timezone"},
		{"unknown currency is rejected", func(i *SettingsInput) { i.Currency = "XXX" }, "currency"},
		{"unknown country is rejected", func(i *SettingsInput) { i.CountryCode = "ZZ" }, "country_code"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			in := validInput()
			tc.mutate(&in)
			_, err := validateInput(in)
			if got := fieldsOf(t, err); !slices.Equal(got, []string{tc.field}) {
				t.Errorf("fields = %v, want [%s]", got, tc.field)
			}
		})
	}

	t.Run("accepted accent color is lowercased", func(t *testing.T) {
		in := validInput()
		in.AccentColor = "#AA3366"
		got, err := validateInput(in)
		if err != nil || got.AccentColor != "#aa3366" {
			t.Errorf("got %q, %v; want #aa3366, nil", got.AccentColor, err)
		}
	})
	t.Run("name is trimmed", func(t *testing.T) {
		in := validInput()
		in.Name = "  Studio "
		got, err := validateInput(in)
		if err != nil || got.Name != "Studio" {
			t.Errorf("got %q, %v; want Studio, nil", got.Name, err)
		}
	})
	t.Run("absent push hour defaults to 8", func(t *testing.T) {
		got, err := validateInput(validInput())
		if err != nil || got.PushHour != 8 {
			t.Errorf("got %d, %v; want 8, nil", got.PushHour, err)
		}
	})
	t.Run("every bad field is reported at once", func(t *testing.T) {
		_, err := validateInput(SettingsInput{PushHour: new(int)})
		want := []string{"name", "timezone", "locale", "currency", "country_code", "accent_color"}
		if got := fieldsOf(t, err); !slices.Equal(got, want) {
			t.Errorf("fields = %v, want %v", got, want)
		}
	})
}

func TestValidateSettingsPatch(t *testing.T) {
	t.Run("empty patch is valid", func(t *testing.T) {
		if _, err := validatePatch(SettingsPatch{}); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
	t.Run("every bad given field is reported at once", func(t *testing.T) {
		name, tz, accent, hour := "", "Mars/Base", "red", 20
		_, err := validatePatch(SettingsPatch{Name: &name, Timezone: &tz, AccentColor: &accent, PushHour: &hour})
		want := []string{"name", "timezone", "accent_color", "push_hour"}
		if got := fieldsOf(t, err); !slices.Equal(got, want) {
			t.Errorf("fields = %v, want %v", got, want)
		}
	})
}

func TestCountriesSortedAndNonEmpty(t *testing.T) {
	got := Countries()
	if len(got) == 0 {
		t.Fatal("Countries() is empty")
	}
	if !slices.IsSorted(got) {
		t.Errorf("Countries() is not sorted: %v", got[:5])
	}
	if !slices.Contains(got, "ES") {
		t.Error("Countries() lacks ES")
	}
}

func TestValidateSettingsInputUppercasesCurrency(t *testing.T) {
	in := validInput()
	in.Currency = "eur"
	got, err := validateInput(in)
	if err != nil || got.Currency != "EUR" {
		t.Errorf("got %q, %v; want EUR, nil", got.Currency, err)
	}
}
