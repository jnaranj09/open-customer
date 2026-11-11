//go:build db

package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
)

// The handover question on a template or service archive and restore (reminders-and-replies
// design, "Handover"; item 29).

// specificRule makes a template, a service and a specific come-back rule on them.
func specificRule(t *testing.T, app testApp, c *http.Cookie) (gen.Template, gen.Service, gen.Rule) {
	t.Helper()
	tpl := ruleTemplate(t, app, c)
	svc := createService(t, app, c, serviceBody)
	rule := createRule(t, app, c, fmt.Sprintf(`{"kind":"come_back","service_id":%q,"offset_days":21,"template_id":%q}`, svc.Id, tpl.Id))
	return tpl, svc, rule
}

func handoverOf(t *testing.T, pool *pgxpool.Pool, rule gen.Rule) *time.Time {
	t.Helper()
	var at *time.Time
	if err := pool.QueryRow(t.Context(), `SELECT handover_from FROM reminder_rules WHERE id = $1`, rule.Id).Scan(&at); err != nil {
		t.Fatalf("read handover_from: %v", err)
	}
	return at
}

func TestArchiveTemplateOrServiceWritesTheHandover(t *testing.T) {
	templatePath := func(tpl gen.Template, _ gen.Service) string { return templatesPath + "/" + tpl.Id.String() }
	servicePath := func(_ gen.Template, svc gen.Service) string { return servicesPath + "/" + svc.Id.String() }
	for _, tc := range []struct {
		name, body string
		path       func(gen.Template, gen.Service) string
		set        bool
	}{
		{"template from today", `{"archived":true}`, templatePath, true},
		{"template include late", `{"archived":true,"include_overdue":true}`, templatePath, false},
		{"service from today", `{"archived":true}`, servicePath, true},
		{"service include late", `{"archived":true,"include_overdue":true}`, servicePath, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, pool, c := signedInOwner(t)
			tpl, svc, rule := specificRule(t, app, c)
			path := tc.path(tpl, svc)
			if rec := sendAs(app, c, http.MethodPatch, path, tc.body); rec.Code != http.StatusOK {
				t.Fatalf("archive: %d %s", rec.Code, rec.Body)
			}
			if got := handoverOf(t, pool, rule) != nil; got != tc.set {
				t.Fatalf("handover_from set = %v, want %v", got, tc.set)
			}
		})
	}
}

func TestRestoreTemplateRestartsItsRules(t *testing.T) {
	app, pool, c := signedInOwner(t)
	tpl, _, rule := specificRule(t, app, c)
	path := templatesPath + "/" + tpl.Id.String()
	sendAs(app, c, http.MethodPatch, path, `{"archived":true}`)
	if rec := sendAs(app, c, http.MethodPatch, path, `{"archived":false,"include_overdue":true}`); rec.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	var appliesFrom *time.Time
	if err := pool.QueryRow(t.Context(), `SELECT applies_from FROM reminder_rules WHERE id = $1`, rule.Id).Scan(&appliesFrom); err != nil {
		t.Fatalf("read applies_from: %v", err)
	}
	if handoverOf(t, pool, rule) != nil || appliesFrom != nil {
		t.Fatalf("after the restore: handover_from %v, applies_from %v; want both NULL", handoverOf(t, pool, rule), appliesFrom)
	}
}

func TestArchiveWhoseCommitFailsLeavesNoHandover(t *testing.T) {
	app, pool, c := signedInOwner(t)
	tpl, svc, rule := specificRule(t, app, c)
	failing := newTestApp(t, testServerConfig{pool: pool, beginner: failingCommit{pool}})
	for _, path := range []string{templatesPath + "/" + tpl.Id.String(), servicesPath + "/" + svc.Id.String()} {
		rec := sendAs(failing, c, http.MethodPatch, path, `{"archived":true}`)
		if got := problemType(t, rec, http.StatusInternalServerError); !strings.HasSuffix(got, ":internal") {
			t.Fatalf("%s: type = %q, want internal", path, got)
		}
	}
	var archived int
	if err := pool.QueryRow(t.Context(),
		`SELECT (SELECT count(*) FROM message_templates WHERE archived_at IS NOT NULL) + (SELECT count(*) FROM services WHERE archived_at IS NOT NULL)`,
	).Scan(&archived); err != nil {
		t.Fatalf("count archived: %v", err)
	}
	if archived != 0 || handoverOf(t, pool, rule) != nil {
		t.Fatalf("after failed commits: %d archived, handover_from %v; want none", archived, handoverOf(t, pool, rule))
	}
}
