// Package attributes holds the owner-defined extra fields of clients and appointments: their
// definitions, option trees, value rules and display formatting.
package attributes

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Scope says which record an attribute belongs to.
type Scope string

// The two scopes.
const (
	ScopeClient      Scope = "client"
	ScopeAppointment Scope = "appointment"
)

// Type is the kind of value an attribute holds.
type Type string

// The five value types.
const (
	TypeText        Type = "text"
	TypeNumber      Type = "number"
	TypeDate        Type = "date"
	TypeSelect      Type = "select"
	TypeMultiselect Type = "multiselect"
)

// Target is where a formatted value is going.
type Target int

// The two formatting targets.
const (
	ForTemplate Target = iota
	ForCSV
)

// Option is one node of a select attribute's option tree.
type Option struct {
	ID, AttributeID uuid.UUID
	ParentID        uuid.NullUUID
	Label           string
	Position        int
	Archived        bool
}

// Attribute is one definition with its options.
type Attribute struct {
	ID       uuid.UUID
	Key      string
	Label    string
	Scope    Scope
	Type     Type
	Required bool
	Position int
	Archived bool
	Options  []Option
}

// ParseScope reads a scope from a request body; anything else is a 422 on "scope".
func ParseScope(s string) (Scope, error) {
	switch Scope(s) {
	case ScopeClient, ScopeAppointment:
		return Scope(s), nil
	}
	return "", fieldError("scope", problem.MsgScopeInvalid)
}

// ParseType reads a type from a request body; anything else is a 422 on "type".
func ParseType(s string) (Type, error) {
	switch Type(s) {
	case TypeText, TypeNumber, TypeDate, TypeSelect, TypeMultiselect:
		return Type(s), nil
	}
	return "", fieldError("type", problem.MsgAttrTypeInvalid)
}

func fieldError(field, msg string) error {
	return problem.Validation{Errors: []problem.FieldError{{Field: field, Message: msg}}}
}

// msg fills a value message with the attribute label.
func msg(format, label string) string { return fmt.Sprintf(format, label) }
