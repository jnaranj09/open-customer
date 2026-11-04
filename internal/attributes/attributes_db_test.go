//go:build db

package attributes

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/position"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

var start = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

type fixture struct {
	pool  *pgxpool.Pool
	clock *clock.Fake
	m     *Module
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := clock.NewFake(start)
	return fixture{pool: dbtest.Pool(t), clock: fake, m: New(Options{Clock: fake})}
}

func (f fixture) create(t *testing.T, label string, scope Scope, typ Type) Attribute {
	t.Helper()
	a, err := f.m.Create(t.Context(), f.pool, AttributeInput{Label: label, Scope: scope, Type: typ})
	if err != nil {
		t.Fatalf("Create %q: %v", label, err)
	}
	return a
}

func (f fixture) option(t *testing.T, attr Attribute, label string, parent uuid.NullUUID) Option {
	t.Helper()
	o, err := f.m.AddOption(t.Context(), f.pool, attr.ID, OptionInput{Label: label, ParentID: parent})
	if err != nil {
		t.Fatalf("AddOption %q: %v", label, err)
	}
	return o
}

func under(o Option) uuid.NullUUID { return uuid.NullUUID{UUID: o.ID, Valid: true} }

func fieldsOf(t *testing.T, err error) []problem.FieldError {
	t.Helper()
	var v problem.Validation
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want problem.Validation", err)
	}
	return v.Errors
}

func wantField(t *testing.T, err error, field, msg string) {
	t.Helper()
	got := fieldsOf(t, err)
	if len(got) != 1 || got[0].Field != field || got[0].Message != msg {
		t.Errorf("errors = %+v, want one on %q with %q", got, field, msg)
	}
}

func TestCreateGeneratesKeyAndPosition(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Bebida favorita", ScopeClient, TypeText)
	b := f.create(t, "Bebida favorita", ScopeClient, TypeText)
	other := f.create(t, "Alergias", ScopeAppointment, TypeText)
	if a.Key != "bebida_favorita" || b.Key != "bebida_favorita_2" {
		t.Errorf("keys = %q, %q, want bebida_favorita and bebida_favorita_2", a.Key, b.Key)
	}
	if a.Position != 0 || b.Position != 1 || other.Position != 0 {
		t.Errorf("positions = %d, %d, %d, want 0, 1 (client) and 0 (appointment)", a.Position, b.Position, other.Position)
	}
}

func TestCreateKeyAvoidsArchivedKeysOfOtherScope(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Color", ScopeClient, TypeText)
	archived := true
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Archived: &archived}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	b := f.create(t, "Color", ScopeAppointment, TypeText)
	if b.Key != "color_2" {
		t.Errorf("key = %q, want color_2", b.Key)
	}
}

func TestCreateReturnsStoredFields(t *testing.T) {
	f := newFixture(t)
	got, err := f.m.Create(t.Context(), f.pool, AttributeInput{
		Label: "  Talla  ", Scope: ScopeClient, Type: TypeNumber, Required: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if got.Label != "Talla" || !got.Required || got.Type != TypeNumber || got.Archived {
		t.Errorf("created = %+v", got)
	}
	if got.ID == uuid.Nil || got.ID != list[0].ID || list[0].Label != "Talla" || !list[0].Required {
		t.Errorf("created %+v does not match stored %+v", got, list[0])
	}
}

func TestCreateRejectsBadInput(t *testing.T) {
	tests := []struct {
		name  string
		in    AttributeInput
		field string
		msg   string
	}{
		{"empty label", AttributeInput{Label: "  ", Scope: ScopeClient, Type: TypeText}, "label", problem.MsgNameInvalid},
		{"81-rune label", AttributeInput{Label: strings.Repeat("ñ", 81), Scope: ScopeClient, Type: TypeText}, "label", problem.MsgNameInvalid},
		{"unknown scope", AttributeInput{Label: "Talla", Scope: "x", Type: TypeText}, "scope", problem.MsgScopeInvalid},
		{"unknown type", AttributeInput{Label: "Talla", Scope: ScopeClient, Type: "x"}, "type", problem.MsgAttrTypeInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.m.Create(t.Context(), f.pool, tc.in)
			wantField(t, err, tc.field, tc.msg)
			list, lerr := f.m.List(t.Context(), f.pool)
			if lerr != nil || len(list) != 0 {
				t.Errorf("List = %+v, %v, want nothing written", list, lerr)
			}
		})
	}
}

func TestCreateRecordsClockTime(t *testing.T) {
	f := newFixture(t)
	f.clock.Set(start.Add(36 * time.Hour))
	a := f.create(t, "Talla", ScopeClient, TypeText)
	var at time.Time
	if err := f.pool.QueryRow(t.Context(), `SELECT created_at FROM attribute_definitions WHERE id = $1`, a.ID).Scan(&at); err != nil {
		t.Fatalf("read created_at: %v", err)
	}
	if want := start.Add(36 * time.Hour); !at.Equal(want) {
		t.Errorf("created_at = %v, want %v", at, want)
	}
}

func TestCreateAfterMaxPositionIs422(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Talla", ScopeClient, TypeText)
	top := position.Max
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Position: &top}); err != nil {
		t.Fatalf("Update to the largest allowed position: %v", err)
	}
	_, err := f.m.Create(t.Context(), f.pool, AttributeInput{Label: "Edad", Scope: ScopeClient, Type: TypeText})
	wantField(t, err, "position", problem.MsgPositionInvalid)
}

func TestCreateAtLastFreePositionStoresIt(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Talla", ScopeClient, TypeText)
	below := position.Max - 1
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Position: &below}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	b := f.create(t, "Edad", ScopeClient, TypeText)
	var stored int
	if err := f.pool.QueryRow(t.Context(), `SELECT position FROM attribute_definitions WHERE id = $1`, b.ID).Scan(&stored); err != nil {
		t.Fatalf("read position: %v", err)
	}
	if b.Position != position.Max || stored != position.Max {
		t.Errorf("returned %d, stored %d, want both %d", b.Position, stored, position.Max)
	}
}

func TestAddOptionAfterMaxPositionIs422(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	o := f.option(t, a, "Norte", uuid.NullUUID{})
	top := position.Max
	if _, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, o.ID, OptionPatch{Position: &top}); err != nil {
		t.Fatalf("UpdateOption to the largest allowed position: %v", err)
	}
	_, err := f.m.AddOption(t.Context(), f.pool, a.ID, OptionInput{Label: "Sur"})
	wantField(t, err, "position", problem.MsgPositionInvalid)
}

func TestCreateAcceptsEightyRuneLabel(t *testing.T) {
	f := newFixture(t)
	label := strings.Repeat("ñ", 80)
	if got := f.create(t, label, ScopeClient, TypeText); got.Label != label {
		t.Errorf("label = %q", got.Label)
	}
}

func TestUpdateCannotChangeKey(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Bebida favorita", ScopeClient, TypeText)
	label := "Café preferido"
	got, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Label: &label})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Key != "bebida_favorita" || got.Label != label || got.Scope != ScopeClient || got.Type != TypeText {
		t.Errorf("updated = %+v", got)
	}
}

func TestUpdateChangesOnlyGivenFields(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Talla", ScopeClient, TypeNumber)
	required, pos := true, 4
	got, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Required: &required, Position: &pos})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Label != "Talla" || !got.Required || got.Position != 4 || got.Archived {
		t.Errorf("updated = %+v", got)
	}
}

func TestUpdateKeepsFieldsNotInThePatch(t *testing.T) {
	f := newFixture(t)
	a, err := f.m.Create(t.Context(), f.pool, AttributeInput{Label: "Talla", Scope: ScopeClient, Type: TypeNumber, Required: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pos := 3
	got, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Position: &pos})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	for _, d := range []Attribute{got, list[0]} {
		if !d.Required || d.Label != "Talla" || d.Position != 3 {
			t.Errorf("attribute = %+v, want Required, label Talla, position 3", d)
		}
	}
}

func TestUpdateRejectsBadPatch(t *testing.T) {
	bad := " "
	neg, big := -1, math.MaxInt32
	tests := []struct {
		name  string
		patch AttributePatch
		field string
		msg   string
	}{
		{"empty label", AttributePatch{Label: &bad}, "label", problem.MsgNameInvalid},
		{"negative position", AttributePatch{Position: &neg}, "position", problem.MsgPositionInvalid},
		{"position above the limit", AttributePatch{Position: &big}, "position", problem.MsgPositionInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			a := f.create(t, "Talla", ScopeClient, TypeText)
			_, err := f.m.Update(t.Context(), f.pool, a.ID, tc.patch)
			wantField(t, err, tc.field, tc.msg)
		})
	}
}

func TestArchiveAndRestore(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Talla", ScopeClient, TypeText)
	yes, no := true, false
	got, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Archived: &yes})
	if err != nil || !got.Archived {
		t.Fatalf("archive: %+v, %v", got, err)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil || len(list) != 1 || !list[0].Archived {
		t.Fatalf("List after archive = %+v, %v, want the archived attribute still listed", list, err)
	}
	got, err = f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Archived: &no})
	if err != nil || got.Archived {
		t.Fatalf("restore: %+v, %v", got, err)
	}
}

func TestArchiveKeepsTheFirstArchiveTime(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Talla", ScopeClient, TypeText)
	yes := true
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Archived: &yes}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	f.clock.Advance(time.Hour)
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Archived: &yes}); err != nil {
		t.Fatalf("archive again: %v", err)
	}
	var at time.Time
	if err := f.pool.QueryRow(t.Context(), `SELECT archived_at FROM attribute_definitions WHERE id = $1`, a.ID).Scan(&at); err != nil {
		t.Fatalf("read archived_at: %v", err)
	}
	if !at.Equal(start) {
		t.Errorf("archived_at = %v, want %v", at, start)
	}
}

func TestUpdateUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	label := "Talla"
	_, err := f.m.Update(t.Context(), f.pool, uuid.New(), AttributePatch{Label: &label})
	if !errors.As(err, &problem.NotFound{}) {
		t.Errorf("err = %v, want NotFound", err)
	}
}

func TestOptionTree(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	top := f.option(t, a, "Norte", uuid.NullUUID{})
	mid := f.option(t, a, "Centro", under(top))
	leaf := f.option(t, a, "Plaza", under(mid))
	if mid.ParentID != under(top) || leaf.ParentID != under(mid) || mid.AttributeID != a.ID {
		t.Errorf("tree = %+v, %+v", mid, leaf)
	}
	value := json.RawMessage(`{"zona":"` + mid.ID.String() + `"}`)
	got, err := f.m.Validate(t.Context(), f.pool, ScopeClient, nil, value, true)
	if err != nil {
		t.Fatalf("Validate a middle-level pick: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("merged = %s, want %s", got, value)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil || len(list) != 1 || len(list[0].Options) != 3 {
		t.Errorf("List = %+v, %v, want one attribute with three options", list, err)
	}
}

func TestOptionForeignParentRejected(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	b := f.create(t, "Barrio", ScopeClient, TypeSelect)
	foreign := f.option(t, b, "Centro", uuid.NullUUID{})
	for name, parent := range map[string]uuid.NullUUID{
		"option of another attribute": under(foreign),
		"unknown option":              {UUID: uuid.New(), Valid: true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.m.AddOption(t.Context(), f.pool, a.ID, OptionInput{Label: "Norte", ParentID: parent})
			wantField(t, err, "parent_id", problem.MsgParentInvalid)
		})
	}
}

func TestOptionOnTypeWithoutOptionsRejected(t *testing.T) {
	for _, typ := range []Type{TypeText, TypeNumber, TypeDate} {
		t.Run(string(typ), func(t *testing.T) {
			f := newFixture(t)
			a := f.create(t, "Notas", ScopeClient, typ)
			_, err := f.m.AddOption(t.Context(), f.pool, a.ID, OptionInput{Label: "Norte"})
			wantField(t, err, "type", problem.MsgOptionsNotAllowed)
		})
	}
}

func TestUpdateReturnsOptions(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	o := f.option(t, a, "Norte", uuid.NullUUID{})
	label := "Área"
	got, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Label: &label})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(got.Options) != 1 || got.Options[0].ID != o.ID {
		t.Errorf("options = %+v, want the one option", got.Options)
	}
}

func TestAttributesWithoutOptionsHaveEmptySlice(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Notas", ScopeClient, TypeText)
	label := "Notas 2"
	updated, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Label: &label})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if updated.Options == nil || list[0].Options == nil {
		t.Errorf("Update options nil = %v, List options nil = %v, want empty non-nil slices", updated.Options == nil, list[0].Options == nil)
	}
}

func TestUpdateOptionArchiveKeepsTheFirstArchiveTime(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	o := f.option(t, a, "Norte", uuid.NullUUID{})
	yes := true
	if _, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, o.ID, OptionPatch{Archived: &yes}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	f.clock.Advance(time.Hour)
	if _, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, o.ID, OptionPatch{Archived: &yes}); err != nil {
		t.Fatalf("archive again: %v", err)
	}
	var at time.Time
	if err := f.pool.QueryRow(t.Context(), `SELECT archived_at FROM attribute_options WHERE id = $1`, o.ID).Scan(&at); err != nil {
		t.Fatalf("read archived_at: %v", err)
	}
	if !at.Equal(start) {
		t.Errorf("archived_at = %v, want %v", at, start)
	}
}

func TestOptionOnMultiselectAccepted(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Gustos", ScopeAppointment, TypeMultiselect)
	if o := f.option(t, a, "Dulce", uuid.NullUUID{}); o.Label != "Dulce" || o.Archived {
		t.Errorf("option = %+v", o)
	}
}

func TestOptionOnArchivedAttributeRejected(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	yes := true
	if _, err := f.m.Update(t.Context(), f.pool, a.ID, AttributePatch{Archived: &yes}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := f.m.AddOption(t.Context(), f.pool, a.ID, OptionInput{Label: "Norte"})
	wantField(t, err, "attribute", problem.MsgAttributeArchived)
}

func TestOptionUnderArchivedParentRejected(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	parent := f.option(t, a, "Norte", uuid.NullUUID{})
	yes := true
	if _, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, parent.ID, OptionPatch{Archived: &yes}); err != nil {
		t.Fatalf("archive option: %v", err)
	}
	_, err := f.m.AddOption(t.Context(), f.pool, a.ID, OptionInput{Label: "Centro", ParentID: under(parent)})
	wantField(t, err, "parent_id", problem.MsgParentArchived)
}

func TestAddOptionBadInput(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	_, err := f.m.AddOption(t.Context(), f.pool, a.ID, OptionInput{Label: " "})
	wantField(t, err, "label", problem.MsgNameInvalid)
}

func TestAddOptionUnknownAttributeIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.AddOption(t.Context(), f.pool, uuid.New(), OptionInput{Label: "Norte"})
	if !errors.As(err, &problem.NotFound{}) {
		t.Errorf("err = %v, want NotFound", err)
	}
}

func TestUpdateOptionChangesFieldsAndKeepsParent(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	top := f.option(t, a, "Norte", uuid.NullUUID{})
	child := f.option(t, a, "Centro", under(top))
	label, pos, yes := "Plaza", 3, true
	got, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, child.ID, OptionPatch{Label: &label, Position: &pos, Archived: &yes})
	if err != nil {
		t.Fatalf("UpdateOption: %v", err)
	}
	if got.Label != "Plaza" || got.Position != 3 || !got.Archived || got.ParentID != under(top) {
		t.Errorf("option = %+v", got)
	}
	no := false
	got, err = f.m.UpdateOption(t.Context(), f.pool, a.ID, child.ID, OptionPatch{Archived: &no})
	if err != nil || got.Archived || got.Label != "Plaza" {
		t.Errorf("restore = %+v, %v", got, err)
	}
}

func TestUpdateOptionRejectsBadPatch(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	o := f.option(t, a, "Norte", uuid.NullUUID{})
	bad, neg := "", -2
	_, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, o.ID, OptionPatch{Label: &bad})
	wantField(t, err, "label", problem.MsgNameInvalid)
	_, err = f.m.UpdateOption(t.Context(), f.pool, a.ID, o.ID, OptionPatch{Position: &neg})
	wantField(t, err, "position", problem.MsgPositionInvalid)
	big := math.MaxInt32
	_, err = f.m.UpdateOption(t.Context(), f.pool, a.ID, o.ID, OptionPatch{Position: &big})
	wantField(t, err, "position", problem.MsgPositionInvalid)
}

func TestUpdateOptionOfOtherAttributeIsNotFound(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	b := f.create(t, "Barrio", ScopeClient, TypeSelect)
	o := f.option(t, b, "Centro", uuid.NullUUID{})
	label := "Norte"
	for name, id := range map[string]uuid.UUID{"other attribute's option": o.ID, "unknown option": uuid.New()} {
		t.Run(name, func(t *testing.T) {
			_, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, id, OptionPatch{Label: &label})
			if !errors.As(err, &problem.NotFound{}) {
				t.Errorf("err = %v, want NotFound", err)
			}
		})
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, def := range list {
		for _, opt := range def.Options {
			if opt.Label != "Centro" {
				t.Errorf("option label = %q, want it untouched", opt.Label)
			}
		}
	}
}

func TestOptionPositionsPerParent(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	n := f.option(t, a, "Norte", uuid.NullUUID{})
	s := f.option(t, a, "Sur", uuid.NullUUID{})
	nChild0 := f.option(t, a, "N0", under(n))
	sChild0 := f.option(t, a, "S0", under(s))
	nChild1 := f.option(t, a, "N1", under(n))
	got := []int{n.Position, s.Position, nChild0.Position, sChild0.Position, nChild1.Position}
	want := []int{0, 1, 0, 0, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("positions = %v, want %v", got, want)
		}
	}
}

func TestValidateUsesStoredDefinitions(t *testing.T) {
	f := newFixture(t)
	f.create(t, "Edad", ScopeClient, TypeNumber)
	_, err := f.m.Validate(t.Context(), f.pool, ScopeClient, nil, json.RawMessage(`{"edad":"abc"}`), true)
	got := fieldsOf(t, err)
	if len(got) != 1 || got[0].Field != "attrs.edad" {
		t.Errorf("errors = %+v, want one on attrs.edad", got)
	}
	merged, err := f.m.Validate(t.Context(), f.pool, ScopeClient, json.RawMessage(`{"edad":30}`), json.RawMessage(`{"edad":31}`), false)
	if err != nil || string(merged) != `{"edad":31}` {
		t.Errorf("good value: %s, %v", merged, err)
	}
}

func TestValidateRejectsKeyOfOtherScope(t *testing.T) {
	f := newFixture(t)
	f.create(t, "Edad", ScopeAppointment, TypeNumber)
	_, err := f.m.Validate(t.Context(), f.pool, ScopeClient, nil, json.RawMessage(`{"edad":3}`), true)
	if got := fieldsOf(t, err); len(got) != 1 || got[0].Field != "attrs.edad" {
		t.Errorf("errors = %+v", got)
	}
}

func TestListOrder(t *testing.T) {
	f := newFixture(t)
	uno := f.create(t, "Uno", ScopeClient, TypeSelect)
	dos := f.create(t, "Dos", ScopeClient, TypeText)
	tres := f.create(t, "Tres", ScopeAppointment, TypeText)
	pos := 5
	if _, err := f.m.Update(t.Context(), f.pool, uno.ID, AttributePatch{Position: &pos}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	a := f.option(t, uno, "A", uuid.NullUUID{})
	b := f.option(t, uno, "B", uuid.NullUUID{})
	two := 2
	if _, err := f.m.UpdateOption(t.Context(), f.pool, uno.ID, a.ID, OptionPatch{Position: &two}); err != nil {
		t.Fatalf("UpdateOption: %v", err)
	}
	list, err := f.m.List(t.Context(), f.pool)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var keys []string
	for _, d := range list {
		keys = append(keys, d.Key)
	}
	if want := []string{tres.Key, dos.Key, uno.Key}; strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", keys, want)
	}
	opts := list[2].Options
	if len(opts) != 2 || opts[0].ID != b.ID || opts[1].ID != a.ID {
		t.Errorf("options = %+v, want B (position 1) before A (position 2)", opts)
	}
	if len(list[0].Options) != 0 {
		t.Errorf("attribute without options has %+v", list[0].Options)
	}
}

func TestUpdateUnknownWithBadFieldIsNotFound(t *testing.T) {
	f := newFixture(t)
	label := " "
	_, err := f.m.Update(t.Context(), f.pool, uuid.New(), AttributePatch{Label: &label})
	if !errors.As(err, &problem.NotFound{}) {
		t.Errorf("err = %v, want NotFound", err)
	}
}

func TestUpdateOptionUnknownWithBadFieldIsNotFound(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, "Zona", ScopeClient, TypeSelect)
	label := " "
	_, err := f.m.UpdateOption(t.Context(), f.pool, a.ID, uuid.New(), OptionPatch{Label: &label})
	if !errors.As(err, &problem.NotFound{}) {
		t.Errorf("err = %v, want NotFound", err)
	}
}
