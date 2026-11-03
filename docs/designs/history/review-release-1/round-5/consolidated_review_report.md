# Consolidated Review Report: release-1.md (round 5)

Target: `docs/designs/release-1.md` (2349 lines), after the round-4 fixes (W1–W15).
Mode: consolidated report, all five stages, no extra context. Every reviewer ran in a fresh context. The owner's decisions were given as constraints.

## Stage reports

| Stage | Report | Blocker / Major / Nit |
| --- | --- | --- |
| 5 Initial Zoom Out | `zoom_out_report.md` | 0 / 1 / 6 |
| 6 Comprehension | `comprehension_report.md` | 0 / 4 / 13 |
| 7 Critic (design, structural, style) | `critic_report.md` | 0 / 6 / 26 |
| 8 Intermediate Zoom Out | `intermediate_zoom_out_report.md` | 0 / 3 / 16 |
| 9 Implementation Readiness (IR, CD, style) | `implementation_readiness_report.md` | 0 / 33 / 59 |

The counts are before duplicates are merged.

## Verdict

**No Blockers, for the second round in a row.**

- Stage 5 says the design passes: the scope is tight and it goes at the real problem.
- Stage 8 says no fix needs a new component, endpoint or table.
- Stage 9 has many Majors because three reviewers read the plan line by line, and the plan is now detailed enough to show small gaps. Most of them are a missing parameter, a missing method, or one line that disagrees with another.

The findings that matter most:

- **Writes have no transaction (Y1).** Handlers pass the pool, so an appointment and its service lines commit separately. This is the one data-safety gap, and four reviewers found it independently.
- **The healthcheck would never pass (Y11).** A Compose healthcheck written as a string needs a shell, and distroless has none.
- **"Reactivar" can never override an overlap (Y5).** The request has no `allow_overlap`.
- **Unknown `/api/` paths would get the HTML page with a 200 (Y2),** so the phone would try to parse HTML as JSON.
- **P5 screens still leave product choices to the engineer (Y8).** This is the only item that needs the owner.

Round 4's fixes introduced two errors, both small:
- `Intl.DisplayNames` is misused for time zones (Y13).
- Item 33 names the wrong handler files for multipart (Y18).

## Needs the owner

- **Y8: screen-level choices for P5.** These are her daily UI, so they are her call, or the interview's. Proposed defaults:
  - **Week view:** the week starts on Monday.
  - **"Hoy":** cancelled appointments are hidden; "No vino" appointments show greyed out.
  - **Same service twice:** tapping a picked service again adds a second line, shown as "×2".
  - **End time:** shown on the new-appointment form, computed from the services, and editable.
  - **After saving** an appointment or a client: go back to the screen she came from.
  - **Client edit:** the same form as a new client.
  - **Archiving a photo:** long-press the photo, then "Archivar foto".
  - **Option-tree editor:** one list per level, "+ Agregar opción" at each level, drag to reorder, "Archivar" in the row menu.
  - **"Omitir":** no confirmation sheet. A 5-second "Deshacer" toast shows before the `POST` is sent.
  - **Drafts:** only the new-appointment and new-client forms. The key is `<form>:<record id or "new">`. Saved 500 ms after the last input, and cleared after a successful save.

  The other option: leave Y8 open until the interview and fill it in before P5. P0–P4 do not depend on it.

## Action items (the agent's recommendation, to apply)

### 🟡 Majors

**Data safety and server**
- **Y1. Transactions.**
  - Every non-GET handler runs its module call in `db.WithTx` (read-write) and commits before it writes the response.
  - Bootstrap, `GET /reminders/due`, the timer and export use a read-only `REPEATABLE READ` transaction. Other GET handlers pass the pool.
  - `CompleteSetup` takes `q` like every other method.
  - Item 51 gets a rollback test.
  - Sources: Stage 5 M1, Stage 6 #1, Stage 8 MA1, CD-M2.
- **Y2. Unknown `/api/` paths and static serving.**
  - A catch-all `/api/` handler answers 404 `not-found` problem JSON, also for a wrong method.
  - `/` goes to the index handler.
  - No directory listings: unknown files and directories go to the fallback.
  - `_app/version.json` is `no-cache`.
  - Tests in item 37.
  - Sources: IR-M1, ST-M10, CD-N5.
- **Y3. Setup callback types.** `auth` defines `SetupSettings`, `SettingsCreator` and `Seeder`, typed with `q db.DBTX`. `cmd` converts them to `settings.SettingsInput`. Sources: IR-M2, CD-M3, ST-N15.
- **Y4. Module reads.**
  - `appointments.sql` may read `clients(id, name, archived_at)`.
  - `media.sql` may read `appointments(id, client_id)` and `clients(id, archived_at)`.
  - The handler passes the business `*time.Location` to `ListRange`.
  - Source: IR-M3.
- **Y5. Reactivar.** `POST /appointments/{id}/status` takes `allow_overlap`. The method is `SetStatus(id, status, allowOverlap)`. The UI shows the overlap sheet on "Reactivar". Sources: IR-M4, CD-M1.
- **Y10. `ChangePassword(session, current, new)`.** It keeps that session and clears `must_change_password`. Source: CD-M4.
- **Y17. Who writes which error.**
  - `auth.Login` and `status.LogClientError` return `RateLimited`, and their handlers write the 429.
  - The per-IP limiter covers only `POST /auth/login`.
  - `decodeJSON` and the multipart readers check `*http.MaxBytesError` first, which gives 413.
  - Sources: ST-M6, ST-M12, IR-N3, CD-N4.

**Deploy and tooling**
- **Y11. The healthcheck uses exec form.** `test: ["CMD", "/usr/local/bin/open-customer", "healthcheck"]`. Source: CD-M6.
- **Y6. Pins.**
  - `check-pins` scans only `deploy/compose.yaml`, `deploy/backup/Dockerfile`, `.github/workflows/ci.yml` and the root `Dockerfile`, never `*.md`.
  - The golangci-lint version is also compared.
  - `go.mod` uses `go 1.N.0` with `toolchain go1.N.P`, so `go mod tidy` keeps the toolchain line.
  - Sources: IR-M5, ST-M2, CD-M8.
- **Y12. The web package and CI setup.**
  - Add `vite`, `@sveltejs/vite-plugin-svelte` and `typescript`.
  - Add the CI setup steps: setup-go, setup-node, pnpm, `pnpm install --frozen-lockfile`, and `playwright install` (P6).
  - `make dev` depends on `web-stub`.
  - Sources: CD-M7, CD-N21, IR-N13, CD-N2.
- **Y16. The phase table.**
  - Item 17 goes in P1 (`serve`) and P2 (admin subcommands).
  - `schema.d.ts` (item 66) is generated from P1.
  - The P4 exit check seeds data with a `db`-tagged Go test.
  - P5 lists items 64–78.
  - The P2 curl check sends `-H Origin` and adds `make test`.
  - Sources: Stage 6 #3, ST-M3, ST-M4, ST-N3, ST-N4, CD-N1, CD-N19.

**Phone and screens**
- **Y9. The sources for the Bootstrap fields.**
  - `templates.Vars()` and `status.BackupRed()` are added to the method table and to the bootstrap reads, and `server_time` comes from the `Clock`.
  - Dexie `meta` stores `templateVars` and `backupRed`.
  - Sources: ST-M8, CD-M5, IR-N7, ST-M15.
- **Y13. `Intl` labels.**
  - Currency names use `Intl.DisplayNames(['es'], {type: 'currency'})`.
  - Time zones show their IANA ID with a `DateTimeFormat` `longGeneric` label.
  - Source: CD-M9.
- **Y15. One "Activar notificaciones" button.** It is used for the first permission in CUJ 1 step 5 and for re-subscribing. It shows on "Hoy" and on `estado` while `push.ts` reports "needs activation". Sources: Stage 6 #4, Stage 8 MA3, design N10.
- **Y19. Setup fields.**
  - The wizard sends `locale: "es"`. `push_hour` is optional (default 8).
  - Settings validation: locale, the `accent_color` pattern, and the name.
  - The setup check order: "user exists" (409) comes before the token check (410).
  - Sources: ST-M9, CD-N18, IR-N1, IR-N2.
- **Y20. Small behavior gaps.**
  - `wa.me` is the default link; `whatsapp://` only if the iPhone test fails.
  - The offline filter uses `anchor_at`.
  - The "template uses this attribute" check uses the tokenizer.
  - A rule created today with "Solo de hoy en adelante" includes the item due today; the CUJ 5 spec creates its rule through the API.
  - Sources: ST-M13, ST-M14, ST-M16, ST-M17.
- **Y18. Handler files.**
  - `handlers_settings.go` reads the icon multipart and registers the manifest and icon routes.
  - `handlers_media.go` and `handlers_export.go` write binary bodies.
  - `settings.Countries()` serves `GET /setup/options`.
  - Sources: ST-M7, ST-M11, IR-N14, CD-N11.
- **Y7. Template tests where the code is.**
  - Nested paths go to `attributes` tests.
  - Spanish dates go to `locale` tests.
  - `first_name` and archived-attribute rendering go to `reminders` tests.
  - The unknown-variable error names the token.
  - Sources: IR-M6, Stage 6 #2, ST-N19, Stage 8 N7.

**Doc**
- **Y14. Goal 7 gets one sentence:** the editable starter content (`internal/starter`) is the one exception. Not a starter table per country. Sources: design M1, Stage 8 MA2.
- **Y21. Wording.**
  - Line 20: "tells her when to ask a client to come back for a refill".
  - "The client's" instead of "she/her" where the client is meant.
  - "A chip in the text box".
  - The repo-path list: `go mod edit` plus a replace across `.go` files.
  - "The product Phase 3 gate".
  - Sources: style M1, M3, M4, N1; ST-M1, ST-M5.

### 🟢 Nits (apply the cheap ones)

- **Stage 8 deletions and one-clause fixes:**
  - `media.client_id NOT NULL`, with no CHECK;
  - `GET /settings` deleted;
  - `exporter` removed from the `reminders` importers;
  - `Status.last_push_date` dropped;
  - timer retry on a failed count;
  - the photo file write order;
  - the 60-day limit in "Shown until";
  - non-digits stripped from the phone query;
  - Goal 5 says 25 h;
  - the softened logout sentence;
  - the full N16 one-word list.
- **Backup container:** a `trap` and `stop_signal: SIGTERM`, so `docker compose stop` does not kill restic mid-run (CD-N8).
- **Small missing parameters:**
  - `Subscribe` takes `userID`;
  - `LogClientError` takes `sessionID`;
  - `Authenticate` returns a "refreshed" flag;
  - `status.Health()` and `Ready()`;
  - the 1 MB `thumb` limit;
  - the shorthand `body` type note.
- **IR one-liners:**
  - attribute edge cases (`[]`, `null`, `errors[].field = attrs.<key>`);
  - `service_ids: []` gives 422;
  - export column order and arrays;
  - the 60-day boundary and the name sort key;
  - the default values: `main.version`, `bin/open-customer`, the e2e DB reset, the `db` tag on handler tests, the restore unpack folder, an unknown `rule_id` gives 422.
- **CD:**
  - the handler-shape wording `(w, r, id, params)`;
  - the sqlc object form for pointers;
  - golangci-lint gets `$(GO_PKGS)`;
  - a webpush JWT `sub` test.
- **Style:** ST-N1, N2, N6, N9, N11–N14, N16, N17, N18, N20–N24; structural N5 (the colon in Terms); comprehension #6, #9, #11, #12, #14, #15.

### Do not do (Stage 8)

- Drop export paging (it reopens an owner decision kept in two rounds).
- Move the shell flags out of the Technical Plan.
- A starter table per country.
- Log a send after its "before" item expired.
- A restore free-space check.
- Excel phone and formula handling.
- A dry-run count in the overdue prompt.
- A concrete con for React.
- Anchor links and horizontal rules.
- Style churn (Style M2, N7, N9, N10).

## Contested

- **CD-N20 / ST-N7, Dependabot `docker-compose`:** two reviewers believe the ecosystem exists (GA in 2025). Neither confirmed the exact name. Item 10 already checks it at P0. Keep the check.

## Traceability matrix

| Raw finding | → Item |
| --- | --- |
| Stage 5 M1, Stage 6 #1, Stage 8 MA1, CD-M2 | Y1 |
| IR-M1, ST-M10, CD-N5 | Y2 |
| IR-M2, CD-M3, ST-N15 | Y3 |
| IR-M3 | Y4 |
| IR-M4, CD-M1 | Y5 |
| IR-M5, ST-M2, CD-M8 | Y6 |
| IR-M6, Stage 6 #2, ST-N19, Stage 8 N7 | Y7 |
| IR-M7 | Y8 (owner) |
| ST-M8, CD-M5, IR-N7, ST-M15 | Y9 |
| CD-M4 | Y10 |
| CD-M6 | Y11 |
| CD-M7, CD-N21, IR-N13, CD-N2 | Y12 |
| CD-M9 | Y13 |
| design M1, Stage 8 MA2 | Y14 |
| Stage 6 #4, Stage 8 MA3, design N10 (149) | Y15 |
| Stage 6 #3, ST-M3, ST-M4, ST-N3, ST-N4, CD-N1, CD-N19, ST-N5, IR-N4 | Y16 |
| ST-M6, ST-M12, IR-N3, CD-N4 | Y17 |
| ST-M7, ST-M11, IR-N14, CD-N11 | Y18 |
| ST-M9, CD-N18, IR-N1, IR-N2 | Y19 |
| ST-M13, ST-M14, ST-M16, ST-M17 | Y20 |
| style M1, M3, M4, N1, ST-M1, ST-M5 | Y21 |
| Stage 5 N1, N3, N4, N5; design N1, N4, N5, N7; Stage 6 #5, #7, #10, #16, #17; style N2–N6, N8, N11 | Nits (Stage 8 list) |
| Stage 5 N2, N6; design N3, N6, N8, N9, N10 (503); structural M1, N1–N4; style M2, N7, N9, N10; Stage 6 #8, #13 | Do not do |
| CD-N20, ST-N7 | Contested |
| All other IR-N, CD-N, ST-N | Nits |

---

## Resolution (2026-11-03)

The owner accepted the proposed defaults for Y8. The agent applied every other item with its recommendations, and most Nits. The edits are in `docs/designs/release-1.md`. The findings above refer to the document **before** these edits; their line numbers no longer match.

| ID | Outcome |
| --- | --- |
| Y1 | Every non-GET handler runs its module call in `db.WithTx` and commits before it writes the response. Bootstrap, `GET /reminders/due`, the timer and export are read-only `REPEATABLE READ`. Other GETs pass the pool. `CompleteSetup` and `exporter.Write` no longer open their own transaction. Rollback test in item 51. |
| Y2 | A catch-all `/api/` handler returns 404 problem JSON for unknown paths and wrong methods. `/` goes to the index handler. No directory listings. `_app/version.json` is `no-cache`. Tests in item 37. |
| Y3 | `auth` defines `SetupSettings`, `SettingsCreator` and `Seeder` (with `q db.DBTX`); `cmd` converts in a closure. |
| Y4 | Read-only exceptions for `appointments.sql` and `media.sql`; `ListRange(from, to, loc)`. |
| Y5 | `allow_overlap` on `POST /appointments/{id}/status`; `SetStatus(id, status, allowOverlap)`; the overlap sheet on "Reactivar". |
| Y6 | `check-pins` scans four named files, never `*.md`. The golangci-lint version is compared. `go 1.N.0` + `toolchain go1.N.P`. |
| Y7 | Formatting tests moved to `attributes`, `locale` and `reminders`; the unknown-variable error names the token; any stray `{{` is rejected. |
| Y8 | Owner accepted the defaults: the week starts Monday; cancelled appointments are hidden and "No vino" ones greyed; "×2" for a repeated service; an editable computed end time; back to the previous screen after saving; the same form for client edit; long-press "Archivar foto"; the option editor (one list per level, "+ Agregar opción", drag, "Archivar"); "Omitir" with a 5-second "Deshacer"; drafts only for new appointment and new client, key `<form>:<id or "new">`, saved after 500 ms, cleared on save. |
| Y9 | `templates.Vars()`, `status.BackupRed()`, `server_time` from the `Clock`; Dexie `meta` stores `templateVars` and `backupRed`. |
| Y10 | `ChangePassword(session, current, new)` keeps that session and clears `must_change_password`. |
| Y11 | Exec-form healthcheck in "Deployment" and item 81. |
| Y12 | `vite`, `@sveltejs/vite-plugin-svelte`, `typescript`; CI setup row; `playwright install` before e2e; `dev` depends on `web-stub`. |
| Y13 | Time zones by IANA ID with a `longGeneric` label; currency names with `DisplayNames` `{type: 'currency'}`. |
| Y14 | Goal 7 names the editable starter content as the one exception. |
| Y15 | One "Activar notificaciones" button for the first permission (CUJ 1 step 5) and re-subscribe, on "Hoy" and `estado`. |
| Y16 | Item 17 in P0/P1/P2; `schema.d.ts` in P1; the P4 exit check seeds with a `db` test; P5 lists 62–63 (finished) and 64–78; the P2 curl check sends `Origin` and adds `make test`. |
| Y17 | Handlers write 429 from `RateLimited`; the IP limiter covers only login; `MaxBytesError` gives 413 in `decodeJSON` and the multipart readers. |
| Y18 | Multipart and binary handler files corrected; `handlers_settings.go` registers the manifest and icon routes; `settings.Countries()`; `handlers_status.go` registers `/healthz` and `/readyz`. |
| Y19 | The wizard sends `locale: "es"` and no `push_hour` (default 8); settings validation for name, locale, color and push hour; setup checks "user exists" before the token. |
| Y20 | `wa.me` is the default; offline filter on `anchor_at`; the attribute-use check uses the tokenizer; same-day "Solo de hoy en adelante" test; the CUJ 5 spec creates its rule through the API. |
| Y21 | Line 20, "the client's", inline vs row chips, the repo-path change list, "product Phase 3 gate". |
| Nits applied | Stage 8 list: `media.client_id NOT NULL` with no CHECK, `GET /settings` removed, `exporter` out of the `reminders` importers, `Status.last_push_date` removed (key renamed `push_checked_date`), timer retry, file-then-row write order, the 60-day limit and boundary, phone digits stripped, Goal 5 at 25 h, the logout sentence, the N16 one-word list. Also: backup `trap`, `stop_signal: SIGTERM`, `flock -n` in the hourly loop; `Subscribe(userID, …)`, `LogClientError(sessionID, …)`, `Authenticate` `refreshed` flag, `status.Health()`/`Ready()`; the `thumb` limit; the shorthand `body` note and FK targets; attribute empty values and `errors[].field`; `service_ids: []` gives 422; export column order, arrays and a mid-stream error; `main.version` default; `bin/open-customer`; the e2e DB reset; the `db` tag on handler tests; the restore unpack folder; an unknown `rule_id` gives 422; the handler shape; the sqlc pointer object form; golangci-lint on `$(GO_PKGS)`; the webpush JWT `sub` test; the reserved-name suffix; "active" defined; the long-edge resize; the ADR 018–025 column. |
| Nits not applied | Structural N1–N4, Style M2/N7/N9/N10 and the other "Do not do" items (Stage 8 advised against them); CD-N9 (anonymous volumes, not confirmed); ST-N6, N8, N9, N12, N13; comprehension #6, #8, #11–#13 beyond what Y7 covered. |
| Do not do | Not applied, as Stage 8 advised. |
| Contested | Dependabot `docker-compose`: kept as a P0 check. |
