package clients

import (
	"strings"

	"github.com/nyaruka/phonenumbers"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// NormalizePhone parses raw with region as the default country and returns the number in E.164
// form and the same digits without the "+". A number that does not parse or is not valid for its
// country is a problem.Validation on "phone". The caller handles the empty string (no phone).
func NormalizePhone(raw, region string) (e164, digits string, err error) {
	num, perr := phonenumbers.Parse(raw, region)
	if perr != nil || !phonenumbers.IsValidNumber(num) {
		return "", "", problem.Field("phone", problem.MsgPhoneInvalid)
	}
	e164 = phonenumbers.Format(num, phonenumbers.E164)
	return e164, strings.TrimPrefix(e164, "+"), nil
}
