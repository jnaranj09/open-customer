package problem

// Field messages for Validation errors[].message (design doc item 24, OP14). They are Spanish on
// purpose: the web app shows them unchanged next to the field.
const (
	// MsgUsernameInvalid is the username rule from "Session and request rules".
	MsgUsernameInvalid = "Usa de 3 a 32 letras, números, punto, guion o guion bajo"
	// MsgPasswordTooShort is the minimum password length (item 38).
	MsgPasswordTooShort = "La contraseña debe tener al menos 8 caracteres"
)
