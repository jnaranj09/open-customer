// Package attributes holds the owner-defined extra fields of clients and appointments: their
// definitions, option trees, value rules and display formatting.
package attributes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
	"github.com/jnaranj09/open-customer/internal/platform/text"
)

const maxLabelLen = 80

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

// Options is what serve passes to New.
type Options struct{ Clock clock.Clock }

// Module is the attributes module. Every method takes ctx and a db.DBTX first.
type Module struct{ clock clock.Clock }

// New returns the attributes module.
func New(opts Options) *Module { return &Module{clock: opts.Clock} }

// AttributeInput is everything Create needs.
type AttributeInput struct {
	Label    string
	Scope    Scope
	Type     Type
	Required bool
}

// AttributePatch holds the fields Update changes; a nil field is left as it is. The key, scope
// and type cannot change.
type AttributePatch struct {
	Label    *string
	Required *bool
	Position *int
	Archived *bool
}

// OptionInput is everything AddOption needs. A valid ParentID puts the option under another
// option of the same attribute.
type OptionInput struct {
	Label    string
	ParentID uuid.NullUUID
}

// OptionPatch holds the fields UpdateOption changes; a nil field is left as it is. The parent
// never changes.
type OptionPatch struct {
	Label    *string
	Position *int
	Archived *bool
}

// List returns every attribute, archived ones included, each with all its options. Attributes are
// ordered by scope, position and id; options by position and id.
func (m *Module) List(ctx context.Context, q db.DBTX) ([]Attribute, error) {
	queries := sqlc.New(q)
	rows, err := queries.ListAttributeDefinitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list attribute definitions: %w", err)
	}
	opts, err := queries.ListAttributeOptions(ctx, uuid.NullUUID{})
	if err != nil {
		return nil, fmt.Errorf("list attribute options: %w", err)
	}
	byAttr := make(map[uuid.UUID][]Option, len(rows))
	for _, o := range opts {
		byAttr[o.AttributeID] = append(byAttr[o.AttributeID], optionFrom(o))
	}
	out := make([]Attribute, 0, len(rows))
	for _, r := range rows {
		out = append(out, Attribute{
			ID: r.ID, Key: r.Key, Label: r.Label, Scope: Scope(r.Scope), Type: Type(r.Type),
			Required: r.Required, Position: int(r.Position), Archived: r.ArchivedAt != nil,
			Options: optionsOrEmpty(byAttr[r.ID]),
		})
	}
	return out, nil
}

// Create validates in, generates the key and places the attribute last in its scope.
func (m *Module) Create(ctx context.Context, q db.DBTX, in AttributeInput) (Attribute, error) {
	var errs []problem.FieldError
	label, ok := text.Trim(in.Label, 1, maxLabelLen)
	if !ok {
		errs = append(errs, problem.FieldError{Field: "label", Message: problem.MsgNameInvalid})
	}
	if _, err := ParseScope(string(in.Scope)); err != nil {
		errs = append(errs, problem.FieldError{Field: "scope", Message: problem.MsgScopeInvalid})
	}
	if _, err := ParseType(string(in.Type)); err != nil {
		errs = append(errs, problem.FieldError{Field: "type", Message: problem.MsgAttrTypeInvalid})
	}
	if len(errs) > 0 {
		return Attribute{}, problem.Validation{Errors: errs}
	}
	id, err := ids.New()
	if err != nil {
		return Attribute{}, fmt.Errorf("new attribute id: %w", err)
	}
	queries := sqlc.New(q)
	keys, err := queries.ListAttributeKeys(ctx)
	if err != nil {
		return Attribute{}, fmt.Errorf("list attribute keys: %w", err)
	}
	taken := make(map[string]bool, len(keys))
	for _, k := range keys {
		taken[k] = true
	}
	last, err := queries.MaxAttributePosition(ctx, string(in.Scope))
	if err != nil {
		return Attribute{}, fmt.Errorf("max attribute position: %w", err)
	}
	a := Attribute{
		ID: id, Key: GenerateKey(label, taken), Label: label, Scope: in.Scope, Type: in.Type,
		Required: in.Required, Position: int(last + 1),
	}
	if err := queries.InsertAttribute(ctx, sqlc.InsertAttributeParams{
		ID: a.ID, Key: a.Key, Label: a.Label, Scope: string(a.Scope), Type: string(a.Type),
		Required: a.Required, Position: last + 1, Now: m.clock.Now(),
	}); err != nil {
		return Attribute{}, fmt.Errorf("insert attribute: %w", err)
	}
	return a, nil
}

// Update changes the given fields of one attribute and returns it with its options. An unknown id
// is problem.NotFound. Archiving keeps the time of the first archive.
func (m *Module) Update(ctx context.Context, q db.DBTX, id uuid.UUID, p AttributePatch) (Attribute, error) {
	var errs []problem.FieldError
	label := ""
	if p.Label != nil {
		var ok bool
		if label, ok = text.Trim(*p.Label, 1, maxLabelLen); !ok {
			errs = append(errs, problem.FieldError{Field: "label", Message: problem.MsgNameInvalid})
		}
	}
	var pos int32
	if p.Position != nil {
		var ok bool
		if pos, ok = position(*p.Position); !ok {
			errs = append(errs, problem.FieldError{Field: "position", Message: problem.MsgPositionInvalid})
		}
	}
	if len(errs) > 0 {
		return Attribute{}, problem.Validation{Errors: errs}
	}
	queries := sqlc.New(q)
	row, err := queries.GetAttribute(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attribute{}, problem.NotFound{}
	}
	if err != nil {
		return Attribute{}, fmt.Errorf("get attribute: %w", err)
	}
	params := sqlc.UpdateAttributeParams{
		ID: id, Label: row.Label, Required: row.Required, Position: row.Position, ArchivedAt: row.ArchivedAt,
	}
	if p.Label != nil {
		params.Label = label
	}
	if p.Required != nil {
		params.Required = *p.Required
	}
	if p.Position != nil {
		params.Position = pos
	}
	params.ArchivedAt = archivedAt(p.Archived, row.ArchivedAt, m.clock.Now())
	if err := queries.UpdateAttribute(ctx, params); err != nil {
		return Attribute{}, fmt.Errorf("update attribute: %w", err)
	}
	opts, err := queries.ListAttributeOptions(ctx, uuid.NullUUID{UUID: id, Valid: true})
	if err != nil {
		return Attribute{}, fmt.Errorf("list attribute options: %w", err)
	}
	a := Attribute{
		ID: id, Key: row.Key, Label: params.Label, Scope: Scope(row.Scope), Type: Type(row.Type),
		Required: params.Required, Position: int(params.Position), Archived: params.ArchivedAt != nil,
		Options: make([]Option, 0, len(opts)),
	}
	for _, o := range opts {
		a.Options = append(a.Options, optionFrom(o))
	}
	return a, nil
}

// AddOption adds an option to a select or multiselect attribute, last among its siblings. An
// unknown attribute is problem.NotFound.
func (m *Module) AddOption(ctx context.Context, q db.DBTX, attrID uuid.UUID, in OptionInput) (Option, error) {
	label, ok := text.Trim(in.Label, 1, maxLabelLen)
	if !ok {
		return Option{}, fieldError("label", problem.MsgNameInvalid)
	}
	queries := sqlc.New(q)
	attr, err := queries.GetAttribute(ctx, attrID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Option{}, problem.NotFound{}
	}
	if err != nil {
		return Option{}, fmt.Errorf("get attribute: %w", err)
	}
	switch Type(attr.Type) {
	case TypeSelect, TypeMultiselect:
	case TypeText, TypeNumber, TypeDate:
		return Option{}, fieldError("type", problem.MsgOptionsNotAllowed)
	}
	if attr.ArchivedAt != nil {
		return Option{}, fieldError("attribute", problem.MsgAttributeArchived)
	}
	if in.ParentID.Valid {
		parent, err := queries.GetAttributeOption(ctx, in.ParentID.UUID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Option{}, fmt.Errorf("get parent option: %w", err)
		}
		switch {
		case errors.Is(err, pgx.ErrNoRows) || parent.AttributeID != attrID:
			return Option{}, fieldError("parent_id", problem.MsgParentInvalid)
		case parent.ArchivedAt != nil:
			return Option{}, fieldError("parent_id", problem.MsgParentArchived)
		}
	}
	id, err := ids.New()
	if err != nil {
		return Option{}, fmt.Errorf("new option id: %w", err)
	}
	last, err := queries.MaxOptionPosition(ctx, sqlc.MaxOptionPositionParams{AttributeID: attrID, ParentID: in.ParentID})
	if err != nil {
		return Option{}, fmt.Errorf("max option position: %w", err)
	}
	next := last + 1
	o := Option{ID: id, AttributeID: attrID, ParentID: in.ParentID, Label: label, Position: int(next)}
	if err := queries.InsertAttributeOption(ctx, sqlc.InsertAttributeOptionParams{
		ID: o.ID, AttributeID: attrID, ParentID: in.ParentID, Label: label, Position: next,
	}); err != nil {
		return Option{}, fmt.Errorf("insert attribute option: %w", err)
	}
	return o, nil
}

// UpdateOption changes the given fields of one option. An option that is not one of attrID is
// problem.NotFound.
func (m *Module) UpdateOption(ctx context.Context, q db.DBTX, attrID, optionID uuid.UUID, p OptionPatch) (Option, error) {
	var errs []problem.FieldError
	label := ""
	if p.Label != nil {
		var ok bool
		if label, ok = text.Trim(*p.Label, 1, maxLabelLen); !ok {
			errs = append(errs, problem.FieldError{Field: "label", Message: problem.MsgNameInvalid})
		}
	}
	var pos int32
	if p.Position != nil {
		var ok bool
		if pos, ok = position(*p.Position); !ok {
			errs = append(errs, problem.FieldError{Field: "position", Message: problem.MsgPositionInvalid})
		}
	}
	if len(errs) > 0 {
		return Option{}, problem.Validation{Errors: errs}
	}
	queries := sqlc.New(q)
	row, err := queries.GetAttributeOption(ctx, optionID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.AttributeID != attrID) {
		return Option{}, problem.NotFound{}
	}
	if err != nil {
		return Option{}, fmt.Errorf("get attribute option: %w", err)
	}
	o := optionFrom(row)
	storedPos := row.Position
	if p.Label != nil {
		o.Label = label
	}
	if p.Position != nil {
		o.Position = int(pos)
		storedPos = pos
	}
	at := archivedAt(p.Archived, row.ArchivedAt, m.clock.Now())
	o.Archived = at != nil
	if err := queries.UpdateAttributeOption(ctx, sqlc.UpdateAttributeOptionParams{
		ID: optionID, Label: o.Label, Position: storedPos, ArchivedAt: at,
	}); err != nil {
		return Option{}, fmt.Errorf("update attribute option: %w", err)
	}
	return o, nil
}

// Validate checks the attrs of one write against the stored definitions and returns the merged
// object (see validateValues). Bad values are a problem.Validation with one error per attrs.<key>.
func (m *Module) Validate(ctx context.Context, q db.DBTX, scope Scope, stored, request json.RawMessage, isCreate bool) (json.RawMessage, error) {
	defs, err := m.List(ctx, q)
	if err != nil {
		return nil, err
	}
	merged, errs := validateValues(defs, scope, stored, request, isCreate)
	if len(errs) > 0 {
		return nil, problem.Validation{Errors: errs}
	}
	return merged, nil
}

// archivedAt is the archive time after a patch: unchanged when archived is nil, the stored time
// (or now, when none) for true, and none for false.
func archivedAt(archived *bool, stored *time.Time, now time.Time) *time.Time {
	switch {
	case archived == nil:
		return stored
	case !*archived:
		return nil
	case stored != nil:
		return stored
	}
	return &now
}

// maxPosition leaves room for the "last + 1" of Create and AddOption.
const maxPosition = math.MaxInt32 - 1

// position checks a requested position and converts it to the column type.
func position(n int) (int32, bool) {
	if n < 0 || n > maxPosition {
		return 0, false
	}
	return int32(n), true
}

// optionsOrEmpty gives an attribute without options an empty slice, so its JSON is [] and never null.
func optionsOrEmpty(o []Option) []Option {
	if o == nil {
		return []Option{}
	}
	return o
}

func optionFrom(o sqlc.AttributeOption) Option {
	return Option{
		ID: o.ID, AttributeID: o.AttributeID, ParentID: o.ParentID, Label: o.Label,
		Position: int(o.Position), Archived: o.ArchivedAt != nil,
	}
}
