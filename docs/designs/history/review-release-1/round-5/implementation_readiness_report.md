# Implementation Readiness Report: Stage 9 (round 5)

Target: `docs/designs/release-1.md`, DetailedRange lines 1219–2349.

Reviewers (fresh contexts, direct dispatch): `egm-implementation-readiness` (implementation-readiness-standard), `egm-codebase-details` (no code on disk), `egm-style` (no standard).

Verdict: no Blockers. 33 Majors and 59 Nits before merging duplicates. P4, P6 and P7 are buildable as written. P0, P1, P2, P3 and P5 each need a few lines added.

Overlaps:
- IR-M4 = CD-M1 (Reactivar).
- IR-M2 = CD-M3 (setup types).
- CD-M2 = Stage 5 M1 (transactions).
- ST-M8 = CD-M5 = IR-N7 (bootstrap sources).
- IR-M1 = ST-M10 = CD-N5 (unknown /api/).
- ST-M12 = IR-N3 = CD-N4 (413).
- ST-N4 = CD-N1 = comprehension #3 (item 17).
- ST-M9 = CD-N18 (wizard fields).
- ST-M7 = IR-N14 (multipart file names).
- ST-N5 = IR-N4 = CD-N19 (curl Origin).
- ST-N10 = Stage 5 N3.
- ST-N18 = CD-N16.

---

### Implementation readiness (Stage 9): 0 Blocker, 7 Major, 14 Nit. Verdict: ready with fixes.

P4, P6 and P7 can be built as written. P0, P1, P2, P3 and P5 each have one open design choice.

Majors:
- IR-M1 [P1] (683, 1734, 1785): an unknown or wrong-method /api/v1 request falls through the Go 1.22 mux to static.go, so it gets index.html with a 200. Fix: a catch-all /api/ handler returning 404 not-found (and 405 or 404 for a wrong method); tests in item 37.
- IR-M2 [P2] (335, 1743, 1829): the setup callback types use settings.SettingsInput, which auth may not import; the tx type is unstated. Fix: auth defines SetupSettings, SettingsCreator and Seeder (q db.DBTX); cmd converts.
- IR-M3 [P3] (329–339 vs 617, 650, 1914, 1938): the module read rules forbid what appointments (client_name, archived client) and media (client_id from the appointment) need. Fix: read-only exceptions for appointments.sql (clients) and media.sql (appointments, clients). The handler passes a *time.Location to ListRange.
- IR-M4 [P3/P5] (642, 649, 1561, 1747, 1924): Reactivar cannot override an overlap. Fix: add allow_overlap to the POST /status body; SetStatus(id, status, allowOverlap); the UI shows the overlap sheet.
- IR-M5 [P0] (1228, 1295): check-pins "any postgres: tag in the repo" would match the docs. Fix: name the files to scan; never scan docs/*.md.
- IR-M6 [P3] (477, 1947–1957, 1966–1975): item 54 tests things templates cannot do (formatting is in attributes, locale, reminders) and contradicts line 477. Fix: move those cases to items 46, 61 and 56.
- IR-M7 [P5] (2119–2212): P5 screens leave product choices open: the WeekView start day, whether cancelled/no-show appear on Hoy, the same service twice in ServicePicker, the end-time field, navigation after save, the client edit form and archiving a photo, the option-tree editor, confirming Omitir, and the drafts key/save/clear rules. Fix: a short per-screen table.

Nits:
1 [P2] settings validation: locale es, accent_color regex, push_hour optional (default 8), name rules.
2 [P2] setup error order: check user exists (409) before the token (410).
3 [P1] decodeJSON: MaxBytesError → 413.
4 [P1/P2] Origin: HEAD/OPTIONS like GET; trailing slash; the curl exit check needs -H Origin.
5 [P1] query files are added in each module's phase.
6 [P1] FK targets that are not obvious; nullability of server_keys / scheduler_state.
7 [P4] Bootstrap methods missing: templates.Vars, status.BackupRed.
8 [P4] "desconocido" has no field: unreadable file → null times + red.
9 [P3] attribute edge cases: [] multiselect, null on create, errors[].field = attrs.<key>.
10 [P3] service_ids [] without ends_at → 422, not a CHECK 500.
11 [P4] export: column order by position, datos.json arrays use API schemas, an error mid-stream closes the connection.
12 [P4] 60-day drop inclusive?; the name sort key.
13 [P0/P1] CI has no pnpm install step; schema.d.ts is generated in P1.
14 small defaults: main.version "dev"; build output bin/open-customer; how make e2e drops the DB; the item 37 db tag; default icon content; restore unpack dir (/backup-status/work/restore); unknown rule/appointment on log POST → 422; item 33 multipart file names.

By phase: P0 after M5; P1 after M1; P2 after M2; P3 after M3, M4 and M6; P4 yes; P5 not yet (M7); P6 yes; P7 yes.

### Technical accuracy (Stage 9): 0 Blocker, 9 Major, 21 Nit

Majors:
- CD-M1 (1561, 1747, 1924): Reactivar cannot send allow_overlap (same as IR-M4).
- CD-M2 (337, 1677, 1738–1755): multi-statement writes have no transaction. Includes ChangePassword, ResetPassword, NewSetupToken, attributes.Create. DBTX has no Begin. Fix: handlers wrap in db.WithTx, or a db.Beginner interface.
- CD-M3 (1743, 1829, 335): the settings creator type pulls settings.SettingsInput into auth (depguard). Fix: auth.SetupSettings; give the tx type (same as IR-M2).
- CD-M4 (1742 vs 1821): ChangePassword(userID, …) cannot keep the current session, and must clear must_change_password. Fix: ChangePassword(session, current, new).
- CD-M5 (1544 vs 1761, 1749–1753): Bootstrap template_vars, backup_red and server_time have no source (same as ST-M8).
- CD-M6 (1356, 728, item 81): a Compose healthcheck as a plain string runs CMD-SHELL, and distroless has no shell. Fix: test: ["CMD", "/usr/local/bin/open-customer", "healthcheck"].
- CD-M7 (2064–2065): the web package is missing vite, @sveltejs/vite-plugin-svelte and typescript.
- CD-M8 (1224, 1229, 1234, 1295, 1351): `go mod tidy` drops a toolchain line equal to the go line (fairly sure; not confirmed for the pinned version). Fix: go 1.N.0 + toolchain go1.N.P, or check-pins falls back to the go line.
- CD-M9 (2174): Intl.DisplayNames needs {type}, and has no time-zone type. Fix: currency names only; time zones by IANA ID or a DateTimeFormat longGeneric label.

Nits:
1 item 17 is missing from P1/P2.
2 make dev lacks web-stub.
3 sqlc pointer go_type object form (not confirmed).
4 MaxBytesError → 413 first; the same in the multipart readers.
5 static: route / to the index handler; FileServerFS lists directories; the catch-all also catches unknown /api/v1 and wrong methods (404 problem JSON); _app/version.json no-cache.
6 the std-http handler shape is (w, r, id, params), not (w, r, params).
7 webpush-go double mailto: (not confirmed); test the JWT sub.
8 backup image: STOPSIGNAL SIGINT from the postgres base, and bash PID 1 ignores signals. Add a trap + stop_signal: SIGTERM.
9 anonymous volume from the inherited VOLUME on each `compose run` (not confirmed).
10 golangci-lint v2: exclusions do not stop package loading; pass $(GO_PKGS) (not confirmed).
11 GET /setup/options has no module method: settings.Countries().
12 push.Subscribe has no userID.
13 LogClientError has no session key for its rate limit.
14 Authenticate gives no signal for the sliding cookie re-send.
15 no status methods for /healthz and /readyz.
16 item 52 is missing the 1 MB thumb limit.
17 the shorthand "body text NULL" vs message_templates.body NOT NULL.
18 the wizard does not send locale or push_hour.
19 the P2 curl check needs -H Origin; P2 exit check lacks make test.
20 Dependabot docker-compose exists (GA 2025, to the reviewer's knowledge; exact name not confirmed).
21 CI setup steps are missing (setup-go/node, pnpm install, playwright install).

Checked and accurate (summary): oapi-codegen std-http options, ServerInterface/HandlerWithOptions/StdHTTPServerOptions, binding errors to ErrorHandlerFunc; ServeFile/ServeFileFS redirect /index.html; FileServerFS (Go 1.22); ServeContent, MaxBytesReader, MultipartReader; embed all:build; goose Provider/fs.Sub/GetDBVersion/HasPending; pgx OpenDBFromPool and read-only REPEATABLE READ; the sqlc options and overrides, :many, cgo parser; NULLS NOT DISTINCT; restic --host, forget --group-by, global --no-lock, cat config; jq fromdateiso8601 form; the PG18 image paths; dropdb --force; rslave; profiles; distroless PATH; $service-worker exports; kit.version.name; Tailwind @theme inline; Intl.supportedValuesOf iOS 15.4+; DisplayNames region; Badging API; webpush TTL/Urgency; the Playwright features; the Actions inputs; Corepack; NewV7; x/text/currency; phonenumbers; url.QueryEscape '+'; items 1–87 all phased; cross-references; the link table covers all 7 links.

### Style (Stage 9): 0 Blocker, 17 Major, 24 Nit

Majors:
- ST-M1 (1223): the "four files" list for a new repo path is incomplete (Go imports, .golangci.yml, compose). Fix: go mod edit + replace across .go files.
- ST-M2 (1227 vs 1239): the golangci-lint version is in two places and unchecked. Fix: check-pins compares them, or CI reads it from the Makefile.
- ST-M3 (1293, 1251, 1258 vs item 66): schema.d.ts is generated from P1 but item 66 is in P5. Fix: P1 lists 66 (schema.d.ts only).
- ST-M4 (1261): the P4 exit check's "seeded data" has no seeder (seed.ts is P6). Fix: a db-tagged Go test seeds it.
- ST-M5 (1417): "Phase 3 gate" vs build phase P3. Fix: "product Phase 3 gate".
- ST-M6 (1669 vs 1768–1776, 1835): who writes 429. The middleware has no rate-limit step. Fix: auth.Login / status return RateLimited; handlers write it. Item 41: which routes the IP limiter covers.
- ST-M7 (1762): handler files misnamed for multipart. Export has no multipart; settings icon is in handlers_settings.go.
- ST-M8 (1761 vs 1544, 1749–1753): Bootstrap template_vars[], backup_red and server_time have no source method. Fix: templates.Vars(), status.BackupRed(), the Clock.
- ST-M9 (1552, 2173): the wizard does not ask for locale or push_hour, which POST /setup needs. Fix: send locale "es" and a default push_hour.
- ST-M10 (1785): unknown /api/ paths are not specified. Fix: 404 not-found problem JSON.
- ST-M11 (1743, 1788, 2078, 2086): no file is named for the manifest/icon routes; icon paths given only partly.
- ST-M12 (1736 vs 1669): an oversized JSON body is either payload-too-large or validation. Fix: MaxBytesError → 413.
- ST-M13 (1985): wa.me vs whatsapp:// choice is left open. Fix: wa.me default; switch only if the iPhone test fails.
- ST-M14 (2186, 2221 vs 1541): the offline filter uses starts_at, which DueItem lacks. Fix: use anchor_at.
- ST-M15 (2117): Dexie has no place for template_vars and backup_red. Fix: meta.
- ST-M16 (2210): "template uses attribute" as a text match misses spaces and fallback forms. Fix: use the tokenizer.
- ST-M17 (1999, 2010, 2238): an item due today under "Solo de hoy en adelante", when applies_from is later today. Fix: a test, and the spec creates the rule via the API.

Nits:
- N1 (1241): the Pins row repeats the bullet.
- N2 (1251 vs 2067): "make lint test vuln build" vs "make lint test build".
- N3 (1262): P5 "62–78" overlaps P0.
- N4 (1257–1259): item 17 is missing from P1/P2 (same as comprehension #3).
- N5 (1259): the P2 curl check needs an Origin header.
- N6 (1379): "APP_VERSION in .env" wording.
- N7 (1385): Dependabot docker-compose ecosystem (the reviewer believes it was added in early 2025; not confirmed).
- N8 (1428): ADR titles are English in a Spanish file.
- N9 (1416–1422): NFR order.
- N10 (1545): last_push_date vs last_push.date (same as Stage 5 N3).
- N11 (1563 vs 1750): the preview rule is worded differently.
- N12 (1578, 1580): flags listed as changeable fields.
- N13 (1637): the excluded characters are listed redundantly.
- N14 (1646): ".env variables" means the process env.
- N15 (1829 vs 1743): settings creator closure wording.
- N16 (1876): reserved name → suffix or error?
- N17 (1915): "active" is undefined.
- N18 (1933–1940): broken list; thumb 1 MB limit missing in item 52.
- N19 (1967, 1974, 1975): unknown-variable label; plain vs unmatched `{{`; duplicate line.
- N20 (2007): "on day 21" is ambiguous.
- N21 (2048): "version from main".
- N22 (2051): UI word in a Go test.
- N23 (2098, 2263): bullets out of place.
- N24 (1583, 1981, 2212): one condition phrased three ways.
- Extra: resize "long edge" (2131); hourly loop when the lock is held (2298).
