package reminders

import (
	"net/url"
	"strings"
)

// WhatsAppLink returns the chat link for digits with text prefilled. It is the only place that
// builds the link. QueryEscape writes a space as "+", which WhatsApp may show as a literal plus,
// so every "+" it wrote is replaced by "%20" afterwards; a literal plus in text is already "%2B".
func WhatsAppLink(digits, text string) string {
	return "https://wa.me/" + digits + "?text=" + strings.ReplaceAll(url.QueryEscape(text), "+", "%20")
}
