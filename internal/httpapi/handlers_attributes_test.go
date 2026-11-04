//go:build db

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const attributesPath = "/api/v1/attributes"

// sendAs sends body with method to target as the owner c, with the public Origin.
func sendAs(app testApp, c *http.Cookie, method, target, body string) *httptest.ResponseRecorder {
	return send(app.handler, method, target, testOrigin, body, withCookie(c))
}

// createSelectAttribute creates a select attribute and returns it.
func createSelectAttribute(t *testing.T, app testApp, c *http.Cookie) gen.Attribute {
	t.Helper()
	rec := sendAs(app, c, http.MethodPost, attributesPath, `{"label":"Bebida favorita","scope":"client","type":"select"}`)
	return decodeInto[gen.Attribute](t, rec, http.StatusCreated)
}

func optionsPath(attrID uuid.UUID) string { return attributesPath + "/" + attrID.String() + "/options" }

func TestCreateAttributeOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, attributesPath, `{"label":"Bebida favorita","scope":"client","type":"select"}`)
	a := decodeInto[gen.Attribute](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), "archived", "id", "key", "label", "options", "position", "required", "scope", "type")
	if a.Key != "bebida_favorita" || a.Label != "Bebida favorita" || string(a.Scope) != "client" || string(a.Type) != "select" {
		t.Fatalf("attribute = %+v, want bebida_favorita, client, select", a)
	}
	if a.Options == nil || len(a.Options) != 0 {
		t.Fatalf("options = %#v, want an empty list", a.Options)
	}
	if !strings.Contains(rec.Body.String(), `"options":[]`) {
		t.Fatalf("body %q lacks \"options\":[]", rec.Body.String())
	}
}

func TestCreateAttributeBadTypeIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, attributesPath, `{"label":"Foto","scope":"client","type":"image"}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "type" {
		t.Fatalf("errors = %+v, want one on type", errs)
	}
}

func TestCreateAttributeUnknownFieldIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, attributesPath, `{"label":"X","scope":"client","type":"text","key":"x"}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchAttributeArchive(t *testing.T) {
	app, _, c := signedInOwner(t)
	a := createSelectAttribute(t, app, c)
	rec := sendAs(app, c, http.MethodPatch, attributesPath+"/"+a.Id.String(), `{"archived":true}`)
	got := decodeInto[gen.Attribute](t, rec, http.StatusOK)
	if !got.Archived || got.Id != a.Id || got.Label != a.Label {
		t.Fatalf("attribute = %+v, want archived with the same id and label", got)
	}
}

func TestPatchAttributeNullLabelIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	a := createSelectAttribute(t, app, c)
	rec := sendAs(app, c, http.MethodPatch, attributesPath+"/"+a.Id.String(), `{"label":null}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "label" {
		t.Fatalf("errors = %+v, want one on label", errs)
	}
}

func TestPatchAttributeBadUUIDIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, attributesPath+"/not-a-uuid", `{"archived":true}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchAttributeUnknownIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, attributesPath+"/"+uuid.NewString(), `{"archived":true}`)
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

func TestCreateOptionWithParent(t *testing.T) {
	app, _, c := signedInOwner(t)
	a := createSelectAttribute(t, app, c)
	rootRec := sendAs(app, c, http.MethodPost, optionsPath(a.Id), `{"label":"Café"}`)
	root := decodeInto[gen.Option](t, rootRec, http.StatusCreated)
	assertKeys(t, rootRec.Body.Bytes(), "archived", "attribute_id", "id", "label", "parent_id", "position")
	if !strings.Contains(rootRec.Body.String(), `"parent_id":null`) {
		t.Fatalf("root option body %q lacks \"parent_id\":null", rootRec.Body.String())
	}
	childRec := sendAs(app, c, http.MethodPost, optionsPath(a.Id), `{"label":"Con leche","parent_id":"`+root.Id.String()+`"}`)
	child := decodeInto[gen.Option](t, childRec, http.StatusCreated)
	assertKeys(t, childRec.Body.Bytes(), "archived", "attribute_id", "id", "label", "parent_id", "position")
	parent, err := child.ParentId.Get()
	if err != nil || parent != root.Id || child.AttributeId != a.Id || child.Label != "Con leche" {
		t.Fatalf("child = %+v (parent %v, %v), want under %s", child, parent, err, root.Id)
	}
}

func TestCreateOptionForeignParentIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	a := createSelectAttribute(t, app, c)
	other := createSelectAttribute(t, app, c)
	foreign := decodeInto[gen.Option](t, sendAs(app, c, http.MethodPost, optionsPath(other.Id), `{"label":"Té"}`), http.StatusCreated)
	rec := sendAs(app, c, http.MethodPost, optionsPath(a.Id), `{"label":"Con leche","parent_id":"`+foreign.Id.String()+`"}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "parent_id" {
		t.Fatalf("errors = %+v, want one on parent_id", errs)
	}
}

func TestPatchOptionParentIdIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	a := createSelectAttribute(t, app, c)
	o := decodeInto[gen.Option](t, sendAs(app, c, http.MethodPost, optionsPath(a.Id), `{"label":"Café"}`), http.StatusCreated)
	rec := sendAs(app, c, http.MethodPatch, optionsPath(a.Id)+"/"+o.Id.String(), `{"parent_id":"`+uuid.NewString()+`"}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchOptionOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	a := createSelectAttribute(t, app, c)
	o := decodeInto[gen.Option](t, sendAs(app, c, http.MethodPost, optionsPath(a.Id), `{"label":"Café"}`), http.StatusCreated)
	rec := sendAs(app, c, http.MethodPatch, optionsPath(a.Id)+"/"+o.Id.String(), `{"label":"Cortado","position":3,"archived":true}`)
	got := decodeInto[gen.Option](t, rec, http.StatusOK)
	if got.Id != o.Id || got.Label != "Cortado" || got.Position != 3 || !got.Archived {
		t.Fatalf("option = %+v, want Cortado at 3, archived", got)
	}
}

func TestPatchOptionOfAnotherAttributeIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	a := createSelectAttribute(t, app, c)
	other := createSelectAttribute(t, app, c)
	o := decodeInto[gen.Option](t, sendAs(app, c, http.MethodPost, optionsPath(other.Id), `{"label":"Té"}`), http.StatusCreated)
	rec := sendAs(app, c, http.MethodPatch, optionsPath(a.Id)+"/"+o.Id.String(), `{"label":"Cortado"}`)
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

// attributeRoutes are the four attribute routes, with a body each would accept.
func attributeRoutes() []struct{ method, target, body string } {
	id := uuid.NewString()
	return []struct{ method, target, body string }{
		{http.MethodPost, attributesPath, `{"label":"X","scope":"client","type":"text"}`},
		{http.MethodPatch, attributesPath + "/" + id, `{"archived":true}`},
		{http.MethodPost, attributesPath + "/" + id + "/options", `{"label":"X"}`},
		{http.MethodPatch, attributesPath + "/" + id + "/options/" + id, `{"label":"X"}`},
	}
}

func TestAttributeRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	for _, rt := range attributeRoutes() {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}

func TestAttributeRoutesMustChangeIs403(t *testing.T) {
	app, pool, _ := signedInOwner(t)
	temp, err := app.auth.ResetPassword(t.Context(), pool, "ana")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	c := login(t, app, temp)
	for _, rt := range attributeRoutes() {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertProblem(t, sendAs(app, c, rt.method, rt.target, rt.body), http.StatusForbidden, ":must-change-password")
		})
	}
}
