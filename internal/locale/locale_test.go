package locale_test

import (
	"testing"
	"time"

	"github.com/jnaranj09/open-customer/internal/locale"
)

func spanish(t *testing.T) locale.Locale {
	t.Helper()
	l, err := locale.For("es")
	if err != nil {
		t.Fatalf("For(es): %v", err)
	}
	return l
}

func madrid(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	return loc
}

func TestSpanishDate(t *testing.T) {
	at := time.Date(2026, 10, 24, 15, 0, 0, 0, madrid(t))
	l := spanish(t)
	if got := l.Date(at); got != "sábado 24 de octubre" {
		t.Errorf("Date = %q", got)
	}
	if got := l.DayMonth(at); got != "24 de octubre" {
		t.Errorf("DayMonth = %q", got)
	}
}

func TestSpanishTime(t *testing.T) {
	loc := madrid(t)
	l := spanish(t)
	for name, c := range map[string]struct {
		hour, min int
		want      string
	}{
		"afternoon":    {15, 0, "3:00 p. m."},
		"morning":      {9, 5, "9:05 a. m."},
		"midnight":     {0, 0, "12:00 a. m."},
		"noon":         {12, 0, "12:00 p. m."},
		"minute-pad":   {23, 7, "11:07 p. m."},
		"hour-twelves": {12, 30, "12:30 p. m."},
	} {
		t.Run(name, func(t *testing.T) {
			at := time.Date(2026, 10, 24, c.hour, c.min, 0, 0, loc)
			if got := l.Time(at); got != c.want {
				t.Errorf("Time = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSpanishNumber(t *testing.T) {
	l := spanish(t)
	if got := l.Number(12.5); got != "12,5" {
		t.Errorf("Number(12.5) = %q", got)
	}
	if got := l.Number(3); got != "3" {
		t.Errorf("Number(3) = %q", got)
	}
}

func TestJoinList(t *testing.T) {
	l := spanish(t)
	for name, c := range map[string]struct {
		in   []string
		want string
	}{
		"none":  {nil, ""},
		"one":   {[]string{"A"}, "A"},
		"two":   {[]string{"A", "B"}, "A y B"},
		"three": {[]string{"A", "B", "C"}, "A, B y C"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := l.JoinList(c.in); got != c.want {
				t.Errorf("JoinList = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAddDaysAcrossDST(t *testing.T) {
	loc := madrid(t)
	day := locale.LocalDay(time.Date(2026, 10, 24, 15, 0, 0, 0, loc), loc)
	if want := time.Date(2026, 10, 24, 0, 0, 0, 0, loc); !day.Equal(want) {
		t.Fatalf("LocalDay = %s, want %s", day, want)
	}

	for name, c := range map[string]struct {
		from time.Time
		n    int
		want time.Time
	}{
		"forward-one":  {day, 1, time.Date(2026, 10, 25, 0, 0, 0, 0, loc)},
		"forward-two":  {day, 2, time.Date(2026, 10, 26, 0, 0, 0, 0, loc)},
		"back-one":     {time.Date(2026, 10, 26, 0, 0, 0, 0, loc), -1, time.Date(2026, 10, 25, 0, 0, 0, 0, loc)},
		"back-to-24th": {time.Date(2026, 10, 26, 0, 0, 0, 0, loc), -2, day},
	} {
		t.Run(name, func(t *testing.T) {
			got := locale.AddDays(c.from, c.n, loc)
			if !c.want.Equal(got) || got.Hour() != 0 {
				t.Errorf("AddDays = %s, want %s", got, c.want)
			}
		})
	}
}

func TestForUnknownLocale(t *testing.T) {
	_, err := locale.For("en")
	if err == nil {
		t.Fatal("For(en) returned no error")
	}
}

func TestVarLabelGivesEachFixedLabel(t *testing.T) {
	l := spanish(t)
	for name, c := range map[string]struct{ ns, key, want string }{
		"client name":          {"client", "name", "Nombre"},
		"client first name":    {"client", "first_name", "Primer nombre"},
		"appointment date":     {"appointment", "date", "Fecha de la cita"},
		"appointment time":     {"appointment", "time", "Hora de la cita"},
		"appointment services": {"appointment", "services", "Servicios"},
		"business name":        {"business", "name", "Nombre del negocio"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := l.VarLabel(c.ns, c.key); got != c.want {
				t.Errorf("VarLabel(%q, %q) = %q, want %q", c.ns, c.key, got, c.want)
			}
		})
	}
}

func TestVarLabelUnknownPairIsEmpty(t *testing.T) {
	l := spanish(t)
	for name, c := range map[string]struct{ ns, key string }{
		"unknown key":        {"client", "nope"},
		"key in wrong scope": {"business", "first_name"},
		"unknown namespace":  {"other", "name"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := l.VarLabel(c.ns, c.key); got != "" {
				t.Errorf("VarLabel(%q, %q) = %q, want empty", c.ns, c.key, got)
			}
		})
	}
}

func TestSampleTexts(t *testing.T) {
	l := spanish(t)
	if got := l.SampleClientName(); got != "Ana Pérez" {
		t.Errorf("SampleClientName() = %q, want %q", got, "Ana Pérez")
	}
	got := l.SampleServiceNames()
	if len(got) != 1 || got[0] != "Servicio de ejemplo" {
		t.Errorf("SampleServiceNames() = %q, want [Servicio de ejemplo]", got)
	}
}

func TestPushBody(t *testing.T) {
	l := spanish(t)
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"one item is singular", 1, "Tienes 1 mensaje para enviar hoy"},
		{"three items are plural", 3, "Tienes 3 mensajes para enviar hoy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := l.PushBody(tt.n); got != tt.want {
				t.Errorf("PushBody(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
