//go:build db

package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

const clientsPath = "/api/v1/clients"

const clientBody = `{"name":"Lucía Rojas","phone":"612 345 678"}`

// clientKeys are the keys of a Client.
var clientKeys = []string{
	"archived", "attrs", "created_at", "id", "name", "notes", "phone_digits", "phone_e164", "updated_at",
}

func createClient(t *testing.T, app testApp, c *http.Cookie, body string) gen.Client {
	t.Helper()
	return decodeInto[gen.Client](t, sendAs(app, c, http.MethodPost, clientsPath, body), http.StatusCreated)
}

func clientPath(id uuid.UUID) string { return clientsPath + "/" + id.String() }

func TestCreateClientOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, clientsPath, clientBody)
	got := decodeInto[gen.Client](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), clientKeys...)
	body := rec.Body.String()
	for _, want := range []string{`"phone_e164":"+34612345678"`, `"phone_digits":"34612345678"`, `"notes":null`, `"attrs":{}`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q lacks %s", body, want)
		}
	}
	if got.Name != "Lucía Rojas" || got.Archived {
		t.Fatalf("client = %+v, want Lucía Rojas, active", got)
	}
}

func TestCreateClientDuplicatePhoneIs409(t *testing.T) {
	app, _, c := signedInOwner(t)
	first := createClient(t, app, c, clientBody)
	rec := sendAs(app, c, http.MethodPost, clientsPath, `{"name":"Marta Solano","phone":"+34 612-345-678"}`)
	assertProblem(t, rec, http.StatusConflict, ":duplicate-phone")
	var body struct {
		Title   string            `json:"title"`
		Clients []json.RawMessage `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body.Title != "Otra clienta tiene ese teléfono" || len(body.Clients) != 1 {
		t.Fatalf("title %q, %d clients, want the title and one client", body.Title, len(body.Clients))
	}
	assertKeys(t, body.Clients[0], clientKeys...)
	var listed gen.Client
	if err := json.Unmarshal(body.Clients[0], &listed); err != nil || listed.Id != first.Id {
		t.Fatalf("clients[0] = %+v, %v, want the first client", listed, err)
	}
}

func TestCreateClientAllowDuplicate(t *testing.T) {
	app, _, c := signedInOwner(t)
	createClient(t, app, c, clientBody)
	createClient(t, app, c, `{"name":"Marta Solano","phone":"612345678","allow_duplicate_phone":true}`)
}

func TestCreateClientBadPhoneIs422Field(t *testing.T) {
	app, _, c := signedInOwner(t)
	errs := fieldErrors(t, sendAs(app, c, http.MethodPost, clientsPath, `{"name":"Lucía Rojas","phone":"123"}`))
	if len(errs) != 1 || errs[0].Field != "phone" {
		t.Fatalf("errors = %+v, want one on phone", errs)
	}
}

func TestCreateClientUnknownAttrIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	errs := fieldErrors(t, sendAs(app, c, http.MethodPost, clientsPath, `{"name":"Lucía Rojas","attrs":{"nope":"x"}}`))
	if len(errs) != 1 || errs[0].Field != "attrs.nope" {
		t.Fatalf("errors = %+v, want one on attrs.nope", errs)
	}
}

func TestListClientsArchived(t *testing.T) {
	app, _, c := signedInOwner(t)
	active := createClient(t, app, c, clientBody)
	gone := createClient(t, app, c, `{"name":"Marta Solano"}`)
	sendAs(app, c, http.MethodPatch, clientPath(gone.Id), `{"archived":true}`)
	cases := []struct {
		query string
		want  uuid.UUID
	}{{"", active.Id}, {"?archived=false", active.Id}, {"?archived=true", gone.Id}}
	for _, tc := range cases {
		list := decodeInto[gen.ClientList](t, sendAs(app, c, http.MethodGet, clientsPath+tc.query, ""), http.StatusOK)
		if len(list.Items) != 1 || list.Items[0].Id != tc.want {
			t.Fatalf("list %q = %+v, want only %s", tc.query, list.Items, tc.want)
		}
	}
}

func TestListClientsEmptyIsArray(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodGet, clientsPath, "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"items":[]}` {
		t.Fatalf("got %d %q, want 200 {\"items\":[]}", rec.Code, rec.Body.String())
	}
}

func TestListClientsBadArchivedParamIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	assertProblem(t, sendAs(app, c, http.MethodGet, clientsPath+"?archived=talvez", ""), http.StatusUnprocessableEntity, ":validation")
}

func TestGetClientOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	made := createClient(t, app, c, clientBody)
	rec := sendAs(app, c, http.MethodGet, clientPath(made.Id), "")
	got := decodeInto[gen.Client](t, rec, http.StatusOK)
	assertKeys(t, rec.Body.Bytes(), clientKeys...)
	if got.Id != made.Id || got.Name != "Lucía Rojas" {
		t.Fatalf("client = %+v, want the created one", got)
	}
}

func TestGetClientBadUUIDIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	assertProblem(t, sendAs(app, c, http.MethodGet, clientsPath+"/not-a-uuid", ""), http.StatusUnprocessableEntity, ":validation")
}

func TestGetClientUnknownIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	assertProblem(t, sendAs(app, c, http.MethodGet, clientPath(uuid.New()), ""), http.StatusNotFound, ":not-found")
}

func TestPatchClientNullPhoneClears(t *testing.T) {
	app, _, c := signedInOwner(t)
	made := createClient(t, app, c, `{"name":"Lucía Rojas","phone":"612345678","notes":"hola"}`)
	rec := sendAs(app, c, http.MethodPatch, clientPath(made.Id), `{"phone":null,"notes":null}`)
	decodeInto[gen.Client](t, rec, http.StatusOK)
	for _, want := range []string{`"phone_e164":null`, `"phone_digits":null`, `"notes":null`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body %q lacks %s", rec.Body.String(), want)
		}
	}
}

func TestPatchClientNullNameIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	made := createClient(t, app, c, clientBody)
	errs := fieldErrors(t, sendAs(app, c, http.MethodPatch, clientPath(made.Id), `{"name":null}`))
	if len(errs) != 1 || errs[0].Field != "name" {
		t.Fatalf("errors = %+v, want one on name", errs)
	}
}

func TestPatchClientAttrNullRemovesKey(t *testing.T) {
	app, _, c := signedInOwner(t)
	attr := decodeInto[gen.Attribute](t, sendAs(app, c, http.MethodPost, attributesPath,
		`{"label":"Alergias","scope":"client","type":"text"}`), http.StatusCreated)
	made := createClient(t, app, c, `{"name":"Lucía Rojas","attrs":{"`+attr.Key+`":"látex"}}`)
	if !strings.Contains(string(made.Attrs), "látex") {
		t.Fatalf("attrs = %s, want the value stored", made.Attrs)
	}
	rec := sendAs(app, c, http.MethodPatch, clientPath(made.Id), `{"attrs":{"`+attr.Key+`":null}}`)
	got := decodeInto[gen.Client](t, rec, http.StatusOK)
	if strings.TrimSpace(string(got.Attrs)) != "{}" {
		t.Fatalf("attrs = %s, want {}", got.Attrs)
	}
}

func TestPatchClientUnknownIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	assertProblem(t, sendAs(app, c, http.MethodPatch, clientPath(uuid.New()), `{"archived":true}`), http.StatusNotFound, ":not-found")
}

func TestPatchClientDuplicatePhoneIs409(t *testing.T) {
	app, _, c := signedInOwner(t)
	createClient(t, app, c, clientBody)
	other := createClient(t, app, c, `{"name":"Marta Solano","phone":"699 111 222"}`)
	assertProblem(t, sendAs(app, c, http.MethodPatch, clientPath(other.Id), `{"phone":"612345678"}`), http.StatusConflict, ":duplicate-phone")
}

func TestPatchClientRestoreWithTakenPhoneIs409(t *testing.T) {
	app, _, c := signedInOwner(t)
	old := createClient(t, app, c, clientBody)
	sendAs(app, c, http.MethodPatch, clientPath(old.Id), `{"archived":true}`)
	createClient(t, app, c, `{"name":"Marta Solano","phone":"612345678"}`)
	assertProblem(t, sendAs(app, c, http.MethodPatch, clientPath(old.Id), `{"archived":false}`), http.StatusConflict, ":duplicate-phone")
	got := decodeInto[gen.Client](t, sendAs(app, c, http.MethodPatch, clientPath(old.Id),
		`{"archived":false,"allow_duplicate_phone":true}`), http.StatusOK)
	if got.Archived {
		t.Fatal("client still archived after the allowed restore")
	}
}

func TestClientRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	id := uuid.NewString()
	for _, rt := range []struct{ method, target, body string }{
		{http.MethodGet, clientsPath, ""},
		{http.MethodPost, clientsPath, clientBody},
		{http.MethodGet, clientsPath + "/" + id, ""},
		{http.MethodPatch, clientsPath + "/" + id, `{"archived":true}`},
	} {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}
