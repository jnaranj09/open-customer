//go:build db

package templates

import (
	"errors"
	"testing"

	"github.com/jnaranj09/open-customer/internal/attributes"
	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/dbtest"
	"github.com/jnaranj09/open-customer/internal/locale"
)

type dbOperation struct {
	name  string
	build func(t *testing.T) (db.DBTX, func(q db.DBTX) error)
}

var dbOperations = []dbOperation{
	{"List", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		f.create(t, "Uno", "hola")
		return f.pool, func(q db.DBTX) error { _, err := f.m.List(t.Context(), q); return err }
	}},
	{"Create", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		in := TemplateInput{Name: "Uno", Body: "{{client.first_name}}"}
		return f.pool, func(q db.DBTX) error { _, err := f.m.Create(t.Context(), q, in); return err }
	}},
	{"Update body", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		tpl := f.create(t, "Uno", "hola")
		p := TemplatePatch{Name: ptr("Dos"), Body: ptr("{{client.first_name}}"), Archived: &yes}
		return f.pool, func(q db.DBTX) error { _, err := f.m.Update(t.Context(), q, tpl.ID, p); return err }
	}},
	{"Update without body", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		tpl := f.create(t, "Uno", "hola")
		return f.pool, func(q db.DBTX) error {
			_, err := f.m.Update(t.Context(), q, tpl.ID, TemplatePatch{Name: ptr("Dos")})
			return err
		}
	}},
	{"Vars", func(t *testing.T) (db.DBTX, func(q db.DBTX) error) {
		t.Helper()
		f := newFixture(t)
		f.attribute(t, "Bebida", attributes.ScopeClient)
		es, err := locale.For("es")
		if err != nil {
			t.Fatalf("locale: %v", err)
		}
		return f.pool, func(q db.DBTX) error { _, err := f.m.Vars(t.Context(), q, es); return err }
	}},
}

// TestTemplatesDBErrors fails each database call of each operation in turn. The caller must get
// the database error back wrapped: never a success, and never a problem that would hide an outage.
func TestTemplatesDBErrors(t *testing.T) {
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
