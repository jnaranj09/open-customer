# Implementation Readiness Report: Stage 9 (round 3)

Target: `docs/designs/release-1.md`, DetailedRange lines 1173–2274.

Reviewers (fresh contexts, direct dispatch), all completed:
- `egm-implementation-readiness` (implementation-readiness-standard)
- `egm-codebase-details` (no code on disk; claims checked against tool knowledge and the disk layout)
- `egm-style` (no standard)

Verdict: not ready. 2 Blockers, 19 Majors, 34 Nits. Overlaps: IR-B1 = CD-M6 (module edges); IR-M1 = CD-M11 (P1 phase); IR-M6 = CD-M5 (export paging); IR-N3 = CD-M8 (e2e DB); CD-N7 = Stage 5 #8 (item 14).

---

### Implementation readiness (Stage 9): 1 Blocker, 6 Major, 9 Nit. Verdict: not ready.

**IR-B1 (Blocker): the module rules block reads several features need** (318–331, item 5 depguard, 29, 40, 50, 53, 60).
- `appointments` needs `services` (it copies each service's name, price and duration).
- `templates` needs `attributes` (to check variables at save; `vars.go`).
- `reminders` needs the attribute definitions and options (label paths, type formatting); they are not on the `reminders.sql` read list.
- `auth.CompleteSetup` creates the `business_settings` row and needs the `settings` checks.
- `status` reads `scheduler_state`, `push_subscriptions` and the goose version, but there is no `status.sql` and no allowed import.
- Fix: add each edge to the module rules and to depguard. Inject a settings creator into `auth` (like the seeder), or allow the import. Say which function `status` calls for each value.

Majors:
- **IR-M1:** P1 depends on P2 code.
  - The `GET /setup/status` handler and `auth.SetupNeeded` are P2.
  - Middleware steps 5–7 need `auth.Authenticate`.
  - The item 37 overlap test is P3.
  - `handlers_bootstrap.go` has no phase.
  - Fix: split item 34 by phase, move `SetupNeeded` to P1, move the overlap test to P3, put bootstrap in P4.
- **IR-M2:** the template `Data` map and the shared value formatter are not defined. The export needs the same formatting. Fix: define `templates.Data` with values already formatted. Name one `attributes.Format(def, options, raw, locale, target)` that `reminders` and `exporter` both call.
- **IR-M3:** "Enviados" can't show the client's name, because `LogEntry` has no `client_id` or `client_name`. `?date=` names no time field or zone. Fix: add the fields; `date` filters on `acted_at` as a local day.
- **IR-M4:** offline search is told to copy the server's `word_similarity` order, which it can't. "Starts a word" has no exact definition. Fix: define word-start exactly, and set the offline order to word-start, then name.
- **IR-M5:** Bootstrap does not say whether archived rows are included. The phone needs archived attributes, options, templates and services. Fix: include them with `archived: true`; clients are active only.
- **IR-M6:** the export is described two ways (line 675 pages of 500, item 29 a streamed query). Keep one.

Nits:
- IR-N1: `web/eslint.config.js` and the Prettier config are missing.
- IR-N2: appointment input checks: require a service or `ends_at`; 422 for archived clients and services.
- IR-N3: CI uses a service container while `make e2e` uses the dev container. Let `make e2e` respect an existing `E2E_DATABASE_URL`.
- IR-N4: list every manifest field.
- IR-N5: fallback syntax: escaping a `"`, and spaces inside `{{ }}`.
- IR-N6: no endpoint gives the chip list. Say it comes from cached attributes plus a fixed list in `web/`.
- IR-N7: say the push count N = `len(DueList)`, late items included.
- IR-N8: define overlap with half-open intervals, so touching appointments do not overlap.
- IR-N9: small values left open:
  - the `client-errors` rate limit;
  - icon upload checks;
  - the color format;
  - CSV money, phone and date formats;
  - the time zone of the export file name date.

### Technical accuracy (Stage 9): 1 Blocker, 11 Major, 13 Nit

**CD-B1 (Blocker): the binary is not on PATH** (item 7, items 81 and 86, L694, L744).
- `ENTRYPOINT ["/open-customer"]`, but the healthcheck and `docker compose exec app open-customer version` call it by bare name.
- In distroless, `/` is not on PATH. So `app` never becomes healthy, services that depend on it never start, and `upgrade.sh` step 1 fails.
- Fix: install it to `/usr/local/bin/open-customer`, or use the absolute path everywhere.

Majors:
- **CD-M1:** declaring `X-App-Version` in the spec makes the generated code overwrite the middleware header, sending `""` when a handler leaves it empty. Then the update prompt shows on every call. Fix: set it in the middleware only, and remove it from the spec.
- **CD-M2:** strict handlers can't write undeclared headers. Declare `Content-Disposition` (export) and `Cache-Control` (media) in the spec. `problem.Write` sets `Retry-After`.
- **CD-M3:** path and query binding errors go to `StdHTTPServerOptions.ErrorHandlerFunc` (a `text/plain` 400 by default), not to the strict handler. The `BaseURL: "/api/v1"` mount is also unstated. Fix: `gen.HandlerWithOptions(…, StdHTTPServerOptions{BaseURL, BaseRouter, ErrorHandlerFunc})`, plus a test.
- **CD-M4:** the sqlc `DBTX` has no `SendBatch`, so `DueList` can't send a `pgx.Batch` of different sqlc queries. Fix: run them one after another on `q` (one snapshot inside the tx), or give `reminders` its own interface with hand-written batch SQL.
- **CD-M5:** the export contradicts itself (L675 pages vs item 29 streaming), and sqlc `:many` loads every row into a slice. Fix: keep the 500-row paging and remove the item 29 sentence.
- **CD-M6:** the same module-edge gaps as IR-B1, plus `status.sql`.
- **CD-M7:** the service worker precaches the manifest and serves it cache-first, but the manifest changes in the wizard. iOS may get the default name (not confirmed). Fix: network-first, or don't precache it.
- **CD-M8:** `make e2e` (dev container) and CI (service container) disagree. Fix: if `E2E_DATABASE_URL` is set, use it and skip the container.
- **CD-M9:** `pnpm/action-setup` reads the root `package.json`. Set `package_json_file: web/package.json`.
- **CD-M10:** the item 12 PRD edit list misses FR-P3 (stored `completed`), FR-P5 (recalculation), NFR-3 (<100 ms per keystroke) and NFR-11 (default `America/Costa_Rica`).
- **CD-M11:** the P1 exit check needs code that arrives in P2 (same as IR-M1).

Nits:
- CD-N1: `go.mod` is missing `x/text`, `x/sys` and `oapi-codegen/nullable`.
- CD-N2: `DBTX` lives in the sqlc package. State `type DBTX = sqlc.DBTX` in `db.go`.
- CD-N3: sqlc pointer override: use the object form `{type: "time.Time", pointer: true}` (not confirmed).
- CD-N4: column-type gaps (`starts_at`, `ends_at`, `acted_at`, `duration_min`); `position` is `int` vs `smallint`.
- CD-N5: the middleware list has no recovery step; say where `MaxBytesReader` is applied.
- CD-N6: "pinned in one place" is not true (Postgres is pinned in 5 places). Use a `PG_IMAGE` variable plus a CI grep.
- CD-N7: items 14 and 87 vs the disk (round-1 and round-2 exist; no root reports).
- CD-N8: busybox may lack `flock` and RFC 3339 `date -d`. Add `util-linux-misc` (not confirmed) and use jq `fromdateiso8601`.
- CD-N9: Dependabot needs the `docker-compose` ecosystem and the directories listed (not confirmed).
- CD-N10: `offline.spec.ts` should use `serviceWorkers: 'block'`.
- CD-N11: golangci-lint `exclusions.paths` does not stop loading; run it on `$(GO_PKGS)`. Note the depguard rule shape.
- CD-N12: consider `static-debian13` (not confirmed).
- CD-N13: the release workflow builds the backup Dockerfile before P7 creates it.

Checked and found accurate (summary): Corepack and Node 25; `setup-node` `engines`; Go `tool` lines; goose Provider, `fs.Sub` and `GetDBVersion`; `stdlib.OpenDBFromPool`; the oapi-codegen keys `nullable-type`, `x-go-type`, `exclude-tags`, multipart and binary bodies, and the strict error handler signatures; `go:embed all:build`; `x/text/currency` `ParseISO` and `Rounding`; phonenumbers `GetExampleNumber`; `NULLS NOT DISTINCT` (PG 15+); the `f_unaccent` recipe; the PG 18 volume path; `dropdb --force`; the restic flags; `rslave`; volume ownership copy; distroless UID 65532; GHCR login; the SvelteKit service-worker facts; Tailwind v4 plugin order; openapi-fetch middleware; iOS 16.4 push and badge; webpush-go TTL and Urgency; Playwright `storageState`; Vite proxy keeps `Origin`. Every item cross-reference resolves; the item 14 link table covers every `../../` link; the ADR and FR IDs match the root SAD and PRD.

### Style (Stage 9): 0 Blocker, 2 Major, 12 Nit

**ST-M1 (Major): the client search's "Top 20" cap contradicts the "no paging" Lists rule** (1451, 1538, 1835). Fix: in the Lists row, name `GET /clients?q=` as the one exception (top 20); say whether `search.ts` also caps at 20.

**ST-M2 (Major): the two reminder kinds are named several ways** ("before" rule, "before-rule", "before-item", "Before:", come-back / Come-back) (1352, 1799, 1918–1941, 2114, 2139, 2148). Fix: define the two names once; use "before" and "come-back" in prose and the enum values in code spans.

Nits:
- ST-N1 (1832): "fine at this size" is unmeasured.
- ST-N2 (2256): the gate is "statement coverage", not "line coverage" (`go tool cover -func`).
- ST-N3 (2254): "proves the stub works" overclaims.
- ST-N4 (1454, 1529, 2043, 2077, 2268): "she" has no antecedent in plan items. Fix: "the owner".
- ST-N5 (1920): the `log.go` bullet ends in a dangling "plus per-client history".
- ST-N6 (1912): an archived-rule bullet in item 54 duplicates item 56.
- ST-N7 (1208 vs 1441): the spike result has two deadlines (P1 exit vs before P3).
- ST-N8 (1338, 1371, 1404, 2246): the `docs/sad.md` rule is stated three times.
- ST-N9 (1373–1375): the ADR column sentences undo each other.
- ST-N10 (1727, 1187, 1927, 2264): "Serves files." says nothing; the Node docs citation is vague; "(the SAD bug)" is unexplained; "about an hour" should be "within 60 minutes".
- ST-N11 (1719, 2238–2239): list the unauthenticated paths; name the subsection for "the five steps".
- ST-N12 (1449, 1491, 1187): explain `_cents` for 0-decimal currencies; split the nested `Status` row; move the Corepack reason out of the table cell.
