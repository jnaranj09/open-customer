//go:build db

package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

// Quick replies on the wire (reminders-and-replies design, "Quick replies"; item 58). The module
// tests hold the rules; these hold the routes, the shapes and the bootstrap list.

const repliesPath = "/api/v1/quick-replies"

const replyBody = `{"title":"Dirección","body":"Calle 1\nSan José"}`

func createReply(t *testing.T, app testApp, c *http.Cookie, body string) gen.QuickReply {
	t.Helper()
	return decodeInto[gen.QuickReply](t, sendAs(app, c, http.MethodPost, repliesPath, body), http.StatusCreated)
}

func TestCreateQuickReplyOK(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, repliesPath, replyBody)
	got := decodeInto[gen.QuickReply](t, rec, http.StatusCreated)
	assertKeys(t, rec.Body.Bytes(), "archived", "body", "id", "position", "title")
	if got.Title != "Dirección" || got.Body != "Calle 1\nSan José" || got.Position != 0 || got.Archived {
		t.Fatalf("reply = %+v, want the title, the two-line body, first and active", got)
	}
}

func TestCreateQuickReplyBadIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPost, repliesPath, `{"title":"Uno\nDos","body":""}`)
	if errs := fieldErrors(t, rec); len(errs) != 2 || errs[0].Field != "title" || errs[1].Field != "body" {
		t.Fatalf("errors = %+v, want title and body", errs)
	}
}

func TestUpdateQuickReplyArchivesAndMoves(t *testing.T) {
	app, _, c := signedInOwner(t)
	r := createReply(t, app, c, replyBody)
	rec := sendAs(app, c, http.MethodPatch, repliesPath+"/"+r.Id.String(), `{"archived":true,"position":3}`)
	if got := decodeInto[gen.QuickReply](t, rec, http.StatusOK); !got.Archived || got.Position != 3 {
		t.Fatalf("reply = %+v, want archived at position 3", got)
	}
}

func TestUpdateQuickReplyNullIs422(t *testing.T) {
	app, _, c := signedInOwner(t)
	r := createReply(t, app, c, replyBody)
	rec := sendAs(app, c, http.MethodPatch, repliesPath+"/"+r.Id.String(), `{"title":null}`)
	if errs := fieldErrors(t, rec); len(errs) != 1 || errs[0].Field != "title" {
		t.Fatalf("errors = %+v, want one on title", errs)
	}
}

func TestUpdateQuickReplyUnknownIs404(t *testing.T) {
	app, _, c := signedInOwner(t)
	rec := sendAs(app, c, http.MethodPatch, repliesPath+"/"+uuid.NewString(), `{"archived":true}`)
	assertProblem(t, rec, http.StatusNotFound, ":not-found")
}

func TestListQuickRepliesAndBootstrapHoldArchivedOnes(t *testing.T) {
	app, _, c := signedInOwner(t)
	first := createReply(t, app, c, replyBody)
	second := createReply(t, app, c, `{"title":"Precios","body":"Uñas: 20"}`)
	sendAs(app, c, http.MethodPatch, repliesPath+"/"+first.Id.String(), `{"archived":true}`)

	list := decodeInto[gen.QuickReplyList](t, sendAs(app, c, http.MethodGet, repliesPath, ""), http.StatusOK)
	if len(list.Items) != 2 || list.Items[0].Id != first.Id || !list.Items[0].Archived || list.Items[1].Id != second.Id {
		t.Fatalf("list = %+v, want the archived first, then the second", list.Items)
	}
	boot := decodeInto[gen.Bootstrap](t, sendAs(app, c, http.MethodGet, bootstrapURL("2026-03-01", "2026-04-01"), ""), http.StatusOK)
	if len(boot.QuickReplies) != 2 || boot.QuickReplies[0].Id != first.Id {
		t.Fatalf("bootstrap quick_replies = %+v, want both, by position", boot.QuickReplies)
	}
}

func TestQuickReplyRoutesNeedSession(t *testing.T) {
	app, _, _ := signedInOwner(t)
	for _, rt := range []struct{ method, target, body string }{
		{http.MethodGet, repliesPath, ""},
		{http.MethodPost, repliesPath, replyBody},
		{http.MethodPatch, repliesPath + "/" + uuid.NewString(), `{"archived":true}`},
	} {
		t.Run(rt.method+" "+rt.target, func(t *testing.T) {
			assertUnauthorized(t, send(app.handler, rt.method, rt.target, testOrigin, rt.body, nil))
		})
	}
}
