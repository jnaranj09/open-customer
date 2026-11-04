//go:build db

package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const servicesPath = "/api/v1/services"

const serviceBody = `{"name":"Acrílico","duration_min":90,"color":"#AA3366"}`

func createService(t *testing.T, app testApp, c *http.Cookie, body string) gen.Service {
	t.Helper()
	return decodeInto[gen.Service](t, sendAs(app, c, http.MethodPost, servicesPath, body), http.StatusCreated)
}

func TestCreateServiceOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, servicesPath, serviceBody)
	s := decodeInto[gen.Service](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), "archived", "color", "duration_min", "id", "name", "position", "price_cents")
	if s.Name != "Acrílico" || s.DurationMin != 90 || s.Color != "#aa3366" || s.Position != 0 || s.Archived {
		t.Fatalf("service = %+v, want Acrílico, 90, #aa3366, position 0, active", s)
	}
	if !strings.Contains(rec.Body.String(), `"price_cents":null`) {
		t.Fatalf("body %q lacks \"price_cents\":null", rec.Body.String())
	}
}

func TestCreateServiceWithPrice(t *testing.T) {
	app, _, c := signedInOwner(t)
	s := createService(t, app, c, `{"name":"Relleno","duration_min":45,"price_cents":15000,"color":"#112233"}`)
	got, err := s.PriceCents.Get()
	if err != nil || got != 15000 {
		t.Fatalf("price = %d, %v, want 15000", got, err)
	}
}

func TestCreateServiceBadFieldsIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, servicesPath, `{"name":"","duration_min":0,"price_cents":-1,"color":"red"}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 4 {
		t.Fatalf("errors = %+v, want four", errs)
	}
}

func TestCreateServiceUnknownFieldIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, servicesPath, `{"name":"X","duration_min":10,"color":"#000000","position":3}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchServiceNullNameIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	s := createService(t, app, c, serviceBody)
	rec := sendAs(app, c, http.MethodPatch, servicesPath+"/"+s.Id.String(), `{"name":null}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "name" {
		t.Fatalf("errors = %+v, want one on name", errs)
	}
}

func TestPatchServiceNullPriceClears(t *testing.T) {
	app, _, c := signedInOwner(t)
	s := createService(t, app, c, `{"name":"Relleno","duration_min":45,"price_cents":15000,"color":"#112233"}`)
	rec := sendAs(app, c, http.MethodPatch, servicesPath+"/"+s.Id.String(), `{"price_cents":null}`)
	got := decodeInto[gen.Service](t, rec, http.StatusOK)
	if !got.PriceCents.IsNull() || !strings.Contains(rec.Body.String(), `"price_cents":null`) {
		t.Fatalf("body %q, want price_cents null", rec.Body.String())
	}
	if got.Name != "Relleno" || got.DurationMin != 45 {
		t.Fatalf("service = %+v, want the other fields unchanged", got)
	}
}

func TestPatchServiceArchive(t *testing.T) {
	app, _, c := signedInOwner(t)
	s := createService(t, app, c, serviceBody)
	got := decodeInto[gen.Service](t, sendAs(app, c, http.MethodPatch, servicesPath+"/"+s.Id.String(), `{"archived":true,"position":4}`), http.StatusOK)
	if !got.Archived || got.Position != 4 || got.Id != s.Id {
		t.Fatalf("service = %+v, want archived at position 4", got)
	}
}

func TestPatchServiceBadPositionIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	s := createService(t, app, c, serviceBody)
	rec := sendAs(app, c, http.MethodPatch, servicesPath+"/"+s.Id.String(), `{"position":-1}`)
	errs := fieldErrors(t, rec)
	if len(errs) != 1 || errs[0].Field != "position" {
		t.Fatalf("errors = %+v, want one on position", errs)
	}
}

func TestPatchServiceBadUUIDIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, servicesPath+"/not-a-uuid", `{"archived":true}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity, ":validation")
}

func TestPatchServiceUnknownIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, servicesPath+"/"+uuid.NewString(), `{"archived":true}`)
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

func serviceRoutes() []struct{ method, target, body string } {
	id := uuid.NewString()
	return []struct{ method, target, body string }{
		{http.MethodPost, servicesPath, serviceBody},
		{http.MethodPatch, servicesPath + "/" + id, `{"archived":true}`},
	}
}

func TestServiceRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	for _, rt := range serviceRoutes() {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}

func TestServiceRoutesMustChangeIs403(t *testing.T) {
	app, pool, _ := signedInOwner(t)
	temp, err := app.auth.ResetPassword(t.Context(), pool, "ana")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	c := login(t, app, temp)
	for _, rt := range serviceRoutes() {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertProblem(t, sendAs(app, c, rt.method, rt.target, rt.body), http.StatusForbidden, ":must-change-password")
		})
	}
}
