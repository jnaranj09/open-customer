package templates

import (
	"context"
	"fmt"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/locale"
)

// Template variable namespaces.
const (
	nsClient      = "client"
	nsAppointment = "appointment"
	nsBusiness    = "business"
)

// TemplateVar is one variable the editor offers as a chip: its namespace, key and current label.
type TemplateVar struct{ NS, Key, Label string }

// fixedVars is the one table of built-in variables, in the order Vars lists them. It has two
// views: Vars lists it plus the active attributes, and known checks a save against it plus every
// attribute, archived ones included.
var fixedVars = []struct{ NS, Key string }{
	{nsClient, "name"},
	{nsClient, "first_name"},
	{nsAppointment, "date"},
	{nsAppointment, "time"},
	{nsAppointment, "services"},
	{nsBusiness, "name"},
}

// Vars lists the variables the editor offers, in this order: the fixed client variables, the
// active client attributes by position, the fixed appointment variables, the active appointment
// attributes, then the business name. Fixed labels come from loc; attribute labels are the current
// ones. Archived attributes are left out, though a save still accepts them (see known).
func (m *Module) Vars(ctx context.Context, q db.DBTX, loc locale.Locale) ([]TemplateVar, error) {
	attrs, err := m.attributes.List(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list attributes for template variables: %w", err)
	}
	var out []TemplateVar
	fixed := func(ns string) {
		for _, f := range fixedVars {
			if f.NS == ns {
				out = append(out, TemplateVar{NS: ns, Key: f.Key, Label: loc.VarLabel(ns, f.Key)})
			}
		}
	}
	active := func(scope attributes.Scope) {
		for _, a := range attrs {
			if a.Scope == scope && !a.Archived {
				out = append(out, TemplateVar{NS: string(scope), Key: a.Key, Label: a.Label})
			}
		}
	}
	fixed(nsClient)
	active(attributes.ScopeClient)
	fixed(nsAppointment)
	active(attributes.ScopeAppointment)
	fixed(nsBusiness)
	return out, nil
}

// known reports whether {{ns.key}} names a variable at save time: a fixed one, or any attribute of
// the matching scope, archived ones included, so a template that uses an archived attribute still
// saves.
func known(ns, key string, attrs []attributes.Attribute) bool {
	for _, f := range fixedVars {
		if f.NS == ns && f.Key == key {
			return true
		}
	}
	for _, a := range attrs {
		if string(a.Scope) == ns && a.Key == key {
			return true
		}
	}
	return false
}

// UnknownVars returns every variable of p that known rejects, one entry per occurrence, in body
// order. The save check and the preview share it.
func UnknownVars(p Parsed, attrs []attributes.Attribute) []VarRef {
	var out []VarRef
	for _, v := range p.Vars {
		if !known(v.NS, v.Key, attrs) {
			out = append(out, v)
		}
	}
	return out
}
