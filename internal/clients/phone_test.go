package clients

import (
	"errors"
	"testing"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

func TestNormalizePhone(t *testing.T) {
	for _, tc := range []struct {
		name, raw, region, e164, digits string
	}{
		{"local with dash", "8888-7777", "CR", "+50688887777", "50688887777"},
		{"spaced international", "+34 612 345 678", "ES", "+34612345678", "34612345678"},
		{"national spaced", "612 345 678", "ES", "+34612345678", "34612345678"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e164, digits, err := NormalizePhone(tc.raw, tc.region)
			if err != nil {
				t.Fatalf("NormalizePhone(%q, %q): %v", tc.raw, tc.region, err)
			}
			if e164 != tc.e164 || digits != tc.digits {
				t.Fatalf("got %q, %q, want %q, %q", e164, digits, tc.e164, tc.digits)
			}
		})
	}
}

func TestNormalizePhoneRejects(t *testing.T) {
	for _, raw := range []string{"123", "abc", "8888-77", ""} {
		t.Run(raw, func(t *testing.T) {
			_, _, err := NormalizePhone(raw, "CR")
			var v problem.Validation
			if !errors.As(err, &v) {
				t.Fatalf("err = %v, want problem.Validation", err)
			}
			if len(v.Errors) != 1 || v.Errors[0].Field != "phone" || v.Errors[0].Message != problem.MsgPhoneInvalid {
				t.Fatalf("errors = %+v, want one error on phone with MsgPhoneInvalid", v.Errors)
			}
		})
	}
}
