// Package reminders holds the reminder rules, the due list and the reminder log. A rule
// says when a message to a client is due; the owner sends it by hand.
package reminders

import (
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

// Kind is the kind of a reminder rule.
type Kind string

// The two kinds of rule.
const (
	KindBefore   Kind = "before_appointment"
	KindComeBack Kind = "come_back"
)

// ParseKind parses a kind from a request body. Anything else is a 422 on the field "kind".
func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindBefore, KindComeBack:
		return k, nil
	}
	return "", problem.Field("kind", problem.MsgRuleKindInvalid)
}

// Rule is one reminder rule. ReturnServiceID is set only for a come_back rule that names a service
// other than its own by choice; an empty one means the rule's own service. AppliesFrom is nil when
// overdue items are included.
type Rule struct {
	ID              uuid.UUID
	Kind            Kind
	ServiceID       uuid.NullUUID
	ReturnServiceID uuid.NullUUID
	OffsetDays      int
	TemplateID      uuid.UUID
	AppliesFrom     *time.Time
	Archived        bool
	CreatedAt       time.Time
}

// RuleInput is everything CreateRule needs. IncludeOverdue false means "only from now on".
type RuleInput struct {
	Kind            Kind
	ServiceID       uuid.NullUUID
	ReturnServiceID uuid.NullUUID
	OffsetDays      int
	TemplateID      uuid.UUID
	IncludeOverdue  bool
}

// RulePatch holds the fields UpdateRule changes. There is no kind and no service: a rule never
// changes either. ReturnServiceID distinguishes unspecified (kept), null (back to the rule's own
// service) and a value. IncludeOverdue is read only when the change asks (see UpdateRule).
type RulePatch struct {
	OffsetDays      *int
	TemplateID      *uuid.UUID
	ReturnServiceID nullable.Nullable[uuid.UUID]
	Archived        *bool
	IncludeOverdue  bool
}

// Options is what serve passes to New.
type Options struct {
	Clock      clock.Clock
	Attributes *attributes.Module
}

// Module is the reminders module. Every method takes ctx and a db.DBTX first.
type Module struct {
	clock      clock.Clock
	attributes *attributes.Module
}

// New returns the reminders module.
func New(opts Options) *Module {
	return &Module{clock: opts.Clock, attributes: opts.Attributes}
}
