//go:build db

package attributes

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
)

type dbOperation struct {
	name  string
	build func(t *testing.T) (db.DBTX, func(q db.DBTX) error)
}

func ptr[T any](v T) *T { return &v }

var dbOperations = []dbOperation{
	{"List", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		f.create(t, "Zona", ScopeClient, TypeSelect)
		return f.pool, func(q db.DBTX) error { _, err := f.m.List(t.Context(), q); return err }
	}},
	{"Create", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		in := AttributeInput{Label: "Zona", Scope: ScopeClient, Type: TypeSelect}
		return f.pool, func(q db.DBTX) error { _, err := f.m.Create(t.Context(), q, in); return err }
	}},
	{"Update", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		a := f.create(t, "Zona", ScopeClient, TypeSelect)
		p := AttributePatch{Label: ptr("Barrio"), Archived: ptr(true)}
		return f.pool, func(q db.DBTX) error { _, err := f.m.Update(t.Context(), q, a.ID, p); return err }
	}},
	{"AddOption", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		a := f.create(t, "Zona", ScopeClient, TypeSelect)
		return f.pool, func(q db.DBTX) error {
			_, err := f.m.AddOption(t.Context(), q, a.ID, OptionInput{Label: "Norte"})
			return err
		}
	}},
	{"AddOption under a parent", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		a := f.create(t, "Zona", ScopeClient, TypeSelect)
		parent := f.option(t, a, "Norte", uuid.NullUUID{})
		return f.pool, func(q db.DBTX) error {
			_, err := f.m.AddOption(t.Context(), q, a.ID, OptionInput{Label: "Centro", ParentID: under(parent)})
			return err
		}
	}},
	{"UpdateOption", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		a := f.create(t, "Zona", ScopeClient, TypeSelect)
		o := f.option(t, a, "Norte", uuid.NullUUID{})
		p := OptionPatch{Label: ptr("Sur"), Archived: ptr(true)}
		return f.pool, func(q db.DBTX) error { _, err := f.m.UpdateOption(t.Context(), q, a.ID, o.ID, p); return err }
	}},
	{"Validate", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		f.create(t, "Edad", ScopeClient, TypeNumber)
		return f.pool, func(q db.DBTX) error {
			_, err := f.m.Validate(t.Context(), q, ScopeClient, nil, json.RawMessage(`{"edad":3}`), true)
			return err
		}
	}},
}

// TestDBErrorsAreWrapped fails each database call of each operation in turn. The caller must get
// the database error back wrapped: never a success, and never a problem that would hide an outage.
func TestDBErrorsAreWrapped(t *testing.T) {
	for _, op := range dbOperations {
		t.Run(op.name, func(t *testing.T) {
			steps := 0
			pool, run := op.build(t)
			if err := run(dbtest.FailingDB{DBTX: pool, Calls: &steps}); err != nil {
				t.Fatalf("clean run: %v", err)
			}
			if steps == 0 {
				t.Fatal("clean run made no database call")
			}
			for step := 1; step <= steps; step++ {
				calls := 0
				pool, run := op.build(t)
				err := run(dbtest.FailingDB{DBTX: pool, FailAt: step, Calls: &calls})
				if !errors.Is(err, dbtest.ErrInjected) {
					t.Fatalf("failure at step %d of %d: err = %v; want the database error wrapped", step, steps, err)
				}
			}
		})
	}
}
