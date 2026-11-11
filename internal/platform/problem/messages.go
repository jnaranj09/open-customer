package problem

// Field messages for Validation errors[].message (design doc item 24, OP14). They are Spanish on
// purpose: the web app shows them unchanged next to the field.
const (
	// MsgUsernameInvalid is the username rule from "Session and request rules".
	MsgUsernameInvalid = "Usa de 3 a 32 letras, números, punto, guion o guion bajo"
	// MsgPasswordTooShort is the minimum password length (item 38).
	MsgPasswordTooShort = "La contraseña debe tener al menos 8 caracteres"
	// MsgCurrentPasswordWrong is a wrong current_password on POST /auth/password (item 42).
	MsgCurrentPasswordWrong = "La contraseña actual no es correcta"
	// MsgNullNotAllowed is a PATCH field sent as null that cannot be cleared (design doc item 32).
	MsgNullNotAllowed = "Este dato no se puede dejar vacío"
	// MsgIDInvalid is an id field that is not a valid id.
	MsgIDInvalid = "Identificador no válido"
	// MsgFieldRequired is a required field that is missing.
	MsgFieldRequired = "Este dato es obligatorio"
	// MsgClientErrorFieldInvalid is a client error field that is empty or over its limit.
	MsgClientErrorFieldInvalid = "Dato no válido"
)

// Field messages for the business settings (design doc item 44).
const (
	MsgNameInvalid     = "Escribe un nombre de 1 a 80 caracteres"
	MsgLocaleInvalid   = "Idioma no disponible"
	MsgColorInvalid    = "Usa un color con el formato #rrggbb"
	MsgPushHourInvalid = "Elige una hora entre 0 y 19"
	MsgTimezoneInvalid = "Zona horaria no válida"
	MsgCurrencyInvalid = "Moneda no válida"
	MsgCountryInvalid  = "País no válido"
)

// Field messages for attributes and their values (design doc "Attribute values"). The %s of the
// value messages is the attribute label, or the key for an unknown key.
const (
	MsgScopeInvalid       = "Elige dónde se usa este dato"
	MsgAttrTypeInvalid    = "Elige un tipo de dato"
	MsgAttrsInvalid       = "Los datos adicionales no son válidos"
	MsgAttrUnknown        = "El dato '%s' no existe"
	MsgAttrOtherScope     = "%s no se usa aquí"
	MsgAttrArchived       = "%s está archivado"
	MsgAttrRequired       = "%s es obligatorio"
	MsgAttrTextInvalid    = "%s: escribe un texto de hasta 500 caracteres"
	MsgAttrNumberInvalid  = "%s: escribe un número"
	MsgAttrDateInvalid    = "%s: elige una fecha"
	MsgAttrOptionInvalid  = "%s: elige una opción de la lista"
	MsgAttrOptionArchived = "%s: esa opción está archivada"
	MsgAttrOptionRepeated = "%s: una opción está repetida"
)

// Field messages for attribute definitions and their options (design doc item 45).
const (
	MsgOptionsNotAllowed = "Este tipo de dato no tiene opciones"
	MsgParentInvalid     = "La opción superior no es de este dato"
	MsgParentArchived    = "La opción superior está archivada"
	MsgAttributeArchived = "Este dato está archivado"
	MsgPositionInvalid   = "La posición no es válida"
)

// Field messages for services (design doc item 49).
const (
	MsgDurationInvalid = "Escribe una duración de 1 a 1440 minutos"
	MsgPriceInvalid    = "El precio no puede ser negativo"
)

// Field messages for a one-line text (text.Line; reminders-and-replies design, item 11).
const (
	MsgTextTooLong = "Es muy largo"
	MsgTextOneLine = "Escríbelo en una sola línea"
)

// Field messages for clients (design doc item 48).
const (
	MsgClientNameInvalid = "Escribe un nombre de 1 a 120 caracteres"
	MsgPhoneInvalid      = "Número de teléfono no válido"
	MsgNotesTooLong      = "Escribe hasta 2000 caracteres"
)

// Field messages for appointments (design doc item 50).
const (
	MsgStatusInvalid         = "Estado no válido"
	MsgServiceUnavailable    = "Un servicio no existe o está archivado"
	MsgTooManyServices       = "Elige hasta 20 servicios"
	MsgClientUnavailable     = "Esa clienta no existe o está archivada"
	MsgServicesOrEndRequired = "Elige al menos un servicio o la hora de fin"
	MsgEndBeforeStart        = "La hora de fin debe ser después del inicio"
	MsgPaidInvalid           = "El monto pagado no puede ser negativo"
	// MsgRangeInvalid is a list range whose "to" is before "from", or longer than the endpoint's
	// 31-day limit.
	MsgRangeInvalid = "Elige un rango válido de hasta 31 días"
	// MsgBootstrapRangeInvalid is a bootstrap window whose "to" is before "from", or longer than
	// 200 days.
	MsgBootstrapRangeInvalid = "Elige un rango válido de hasta 200 días"
)

// Field messages for media (design doc "Photos").
const (
	MsgMediaOwnerRequired = "Falta la clienta o la cita"
	MsgMediaOwnerMismatch = "La foto no es de esa clienta"
	MsgAppointmentUnknown = "Esa cita no existe"
)

// Field messages for message templates (design doc "Message templates"). MsgTemplateUnclosed takes
// the 1-based rune position of the bad "{{"; MsgTemplateUnknownVar takes the variable key only.
const (
	MsgTemplateUnclosed    = "Hay un {{ sin cerrar (posición %d)"
	MsgTemplateUnknownVar  = "La variable '%s' no existe"
	MsgTemplateBodyInvalid = "Escribe un mensaje de hasta 2000 caracteres"
)

// Field messages for reminder rules (design doc item 55).
const (
	MsgRuleKindInvalid       = "Elige un tipo de regla"
	MsgOffsetInvalid         = "Escribe un número de días de 0 a 365"
	MsgComeBackOffsetInvalid = "Escribe un número de días de 1 a 365"
	MsgServiceNotAllowed     = "Esta regla no lleva servicio"
	MsgGeneralNoReturn       = "Una regla para todos los servicios no lleva servicio para agendar"
	MsgTemplateUnavailable   = "Esa plantilla no existe o está archivada"
	// MsgRuleNotComeBack is a rule preview of a "before" rule (reminders-and-replies design).
	MsgRuleNotComeBack = "Esta regla no es de después de un servicio"
)

// Field messages for the reminder log (design doc item 55). MsgLogFilterInvalid is GET
// /reminders/log with neither or both of its filters.
const (
	MsgActionInvalid      = "Acción no válida"
	MsgRuleUnknown        = "Esa regla no existe"
	MsgAnchorInvalid      = "La hora de la cita no corresponde al recordatorio"
	MsgPhoneDigitsInvalid = "Número no válido"
	MsgLogFilterInvalid   = "Elige una clienta o una fecha"
)

// Field messages for push subscriptions (design doc item 57). The same text goes on `endpoint`,
// `keys.p256dh` and `keys.auth`: she cannot fix any of them, the browser made them.
const (
	MsgPushEndpointInvalid = "La suscripción no es válida"
)
