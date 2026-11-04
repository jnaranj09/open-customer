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
)

// Field messages for the business settings (design doc item 44).
const (
	MsgNameInvalid        = "Escribe un nombre de 1 a 80 caracteres"
	MsgLocaleInvalid      = "Idioma no disponible"
	MsgAccentColorInvalid = "Usa un color con el formato #rrggbb"
	MsgPushHourInvalid    = "Elige una hora entre 0 y 19"
	MsgTimezoneInvalid    = "Zona horaria no válida"
	MsgCurrencyInvalid    = "Moneda no válida"
	MsgCountryInvalid     = "País no válido"
)
