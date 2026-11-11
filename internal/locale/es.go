package locale

// spanish is the only locale in release 1. All day and month names, and the labels of the fixed template variables, the sample preview texts, the daily push text and the export texts live here.
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
	sampleClient:   "Ana Pérez",
	sampleServices: []string{"Servicio de ejemplo"},
	pushOne:        "Tienes 1 mensaje para enviar hoy",
	pushMany:       "Tienes %d mensajes para enviar hoy",
	varLabels: map[string]string{
		"client.name":          "Nombre",
		"client.first_name":    "Primer nombre",
		"appointment.date":     "Fecha de la cita",
		"appointment.time":     "Hora de la cita",
		"appointment.services": "Servicios",
		"business.name":        "Nombre del negocio",
	},
	export: ExportText{
		ClientsFile:      "clientas.csv",
		AppointmentsFile: "citas.csv",
		ServicesFile:     "servicios.csv",
		ClientCols:       []string{"nombre", "telefono", "notas", "creada", "archivada"},
		AppointmentCols:  []string{"fecha", "hora", "clienta", "servicios", "total", "pagado", "estado", "notas"},
		ServiceCols:      []string{"nombre", "descripcion", "duracion_min", "precio", "archivado"},
		Status: map[string]string{
			"scheduled": "agendada",
			"confirmed": "confirmada",
			"cancelled": "cancelada",
			"no_show":   "no vino",
			"done":      "realizada",
		},
		Yes: "sí",
	},
}
