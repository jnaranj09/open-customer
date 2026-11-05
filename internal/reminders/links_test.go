package reminders

import "testing"

func TestWhatsAppLinkEncoding(t *testing.T) {
	got := WhatsAppLink("50688887777", "Hola Ana, ¿mañana?\n🌸 1+1")
	want := "https://wa.me/50688887777?text=Hola%20Ana%2C%20%C2%BFma%C3%B1ana%3F%0A%F0%9F%8C%B8%201%2B1"
	if got != want {
		t.Fatalf("link = %q, want %q", got, want)
	}
}
