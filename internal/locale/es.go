package locale

// spanish is the only locale in release 1. All day and month names live here.
var spanish = Locale{
	weekdays: [7]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"},
	months: [12]string{
		"enero", "febrero", "marzo", "abril", "mayo", "junio",
		"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
	},
	dayMonthFormat: "%d de %s",
	am:             "a. m.",
	pm:             "p. m.",
	decimalMark:    ",",
	listAnd:        "y",
}
