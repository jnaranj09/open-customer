package templates

import "testing"

var renderData = Data{
	Client:      map[string]string{"first_name": "Lucía", "bebida": ""},
	Appointment: map[string]string{"time": "10:30"},
	Business:    map[string]string{"name": "Uñas Ana"},
}

func TestRenderValueAndFallback(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"value", "Hola {{client.first_name}}", "Hola Lucía"},
		{"value ignores fallback", `{{client.first_name | "amiga"}}`, "Lucía"},
		{"all namespaces", "{{business.name}} {{appointment.time}}", "Uñas Ana 10:30"},
		{"empty value with fallback", `{{client.bebida | "un café"}}`, "un café"},
		{"empty value without fallback", "[{{client.bebida}}]", "[]"},
		{"missing key with fallback", `{{client.nada | "x"}}`, "x"},
		{"missing key without fallback", "[{{client.nada}}]", "[]"},
		{"empty fallback", `[{{client.nada | ""}}]`, "[]"},
		{"spaces inside", "{{  client.first_name  }}", "Lucía"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Render(tt.body, renderData); got != tt.want {
				t.Errorf("Render(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestRenderUnknownNamespaceIsEmpty(t *testing.T) {
	if got := Render(`[{{foo.name}}]`, renderData); got != "[]" {
		t.Errorf("got %q, want []", got)
	}
	if got := Render(`{{foo.name | "x"}}`, renderData); got != "x" {
		t.Errorf("got %q, want the fallback", got)
	}
}

func TestRenderNilMaps(t *testing.T) {
	if got := Render(`{{client.name | "x"}}`, Data{}); got != "x" {
		t.Errorf("got %q, want x", got)
	}
}

func TestRenderOddText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"lone braces and emoji fallback", "}} { 💅 {{client.x | \"🌸\"}}", "}} { 💅 🌸"},
		{"bad braces stay literal", "a {{ hola }} b", "a {{ hola }} b"},
		{"unclosed stays literal", "Hola {{client.first_name", "Hola {{client.first_name"},
		{"valid token after bad braces", "{{ x {{client.first_name}}", "{{ x Lucía"},
		{"scan resumes two characters on", "{{{client.first_name}}", "{{{client.first_name}}"},
		{"value is not rescanned", "{{client.raw}}", "{{business.name}}"},
	}
	data := Data{Client: map[string]string{"first_name": "Lucía", "raw": "{{business.name}}"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Render(tt.body, data); got != tt.want {
				t.Errorf("Render(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}
