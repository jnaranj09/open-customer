package locale

// spanish is the only locale in release 1. All day and month names, and the labels of the fixed template variables, live here.
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
	varLabels: map[string]string{
		"client.name":          "Nombre",
		"client.first_name":    "Primer nombre",
		"appointment.date":     "Fecha de la cita",
		"appointment.time":     "Hora de la cita",
		"appointment.services": "Servicios",
		"business.name":        "Nombre del negocio",
	},
}
