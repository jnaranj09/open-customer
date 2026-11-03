# Goldfish Review Report

**Target:** `docs/designs/release-1.md` (lines 1053-1879, "Detailed Implementation Plan" to the end)

**Stage:** 9, Implementation Readiness Test. One review round, no resolution.

**Question asked:** can an engineer build release 1 from this plan without making impactful decisions?

**Verdict:** No, not yet. Many low-level choices are already fixed: Argon2 parameters, middleware order, cache headers, the photo pipeline, phone normalization, the due-list and scheduler test matrices, and the tooling choices. But four things stop the plan from being followable as written:

- There is no build order.
- The API contract is left for the engineer to write.
- The build breaks on a clean checkout.
- Several flows cannot work as specified: backup-before-upgrade, opening the app offline, the e2e test clock, and icon upload during setup.

## Reviewer Status

| Reviewer | Status |
| --- | --- |
| `egm-implementation-readiness` | completed |
| `egm-codebase-details` | completed (no codebase yet; library claims checked from the reviewer's own knowledge, with no web access; unverifiable claims are marked "unverified") |
| `egm-style` | completed |

## Severity Breakdown

| Severity | Count |
| --- | --- |
| Blocker | 5 |
| Major | 33 |
| Nit | 30 |

When reviewers overlapped, their findings were merged into one. The severity used is the highest any reviewer gave.

The synthesizer checked these load-bearing claims against the file:

- The CI step order (lines 1133-1140) and the `.gitignore` of `web/build` (line 1108).
- The hourly backup loop against the 10-minute wait in `backup-now.sh` (lines 654-665, 1835, 1845).
- The 48 h red-line rule against verification step 6 (lines 631, 1871).

## Prioritized Findings (Blockers -> Major -> Nits)

### B1. No phases, build order or checkpoints - Severity: Blocker
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1062-1854 (the whole file list)
* **Discrepancy Description**: The plan is a flat list of 88 files. Many items depend on each other, but the dependencies are only implied:
  - the OpenAPI spec must exist before the handlers;
  - the migrations before sqlc;
  - the web build before the Go embed;
  - the backup image before the migration guard can be tested.

  No stage is defined that leaves the repo buildable and testable.
* **Rationale & Recommendation**: The standard requires incremental phases, each with an exit check. Group the items into phases, for example:
  - P0: skeleton, CI, Makefile, empty embed.
  - P1: schema, platform, auth, setup.
  - P2: attributes, clients, services, appointments.
  - P3: templates, reminders, push, scheduler.
  - P4: web screens.
  - P5: deploy, backup, scripts.
  - P6: export, status, e2e, docs.

  Name the `make` targets that must pass at the end of each phase.

### B2. The API contract is left to the engineer - Severity: Blocker
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1222-1231 (item 15), 1345-1350, 1649-1658
* **Discrepancy Description**: Item 15 says to write "every endpoint ... with request and response schemas", but the Technical Plan only lists the paths (lines 588-602). That leaves about 50 request and response bodies to be invented. The biggest gaps:
  - The `POST /setup` body, and how the token in the URL fragment is sent.
  - The `GET /setup/status` and `/bootstrap` response shapes.
  - The due-list item shape, including its stable ID. That ID is used by the outbox, the pending confirmation and the Dexie key.
  - The `POST /reminders/log` and `POST /appointments/{id}/status` bodies.
  - What PATCH means: merge or replace, and whether `attrs` are merged. This decides the `touchedKeys` input to `Validate`.
  - Where `allow_overlap` and `allow_duplicate_phone` go, and the status and problem type of the duplicate-phone warning.
  - What a retried create returns when `ON CONFLICT DO NOTHING` matches.
  - The problem `type` URIs and their Spanish titles.
  - Cursor encoding, which endpoints are paginated, and the page size.
  - The `PATCH /settings` fields and the `POST /settings/icon` format.
* **Rationale & Recommendation**: Put the OpenAPI document, or at least a schema table per endpoint, in the plan or an appendix. Include the list of problem types and their HTTP statuses.

### B3. "Back up now" almost never finishes inside its 10-minute wait, so upgrades are blocked - Severity: Blocker
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1835, 1845-1846 (items 86-87); comes from lines 654-665 and 669-674
* **Discrepancy Description**: `backup.sh` checks for work once at start and then every hour. `backup-now.sh` writes `request-now` and polls `status.json` for at most 10 minutes. The request is normally picked up only at the next hourly check, so `upgrade.sh` usually aborts. Without a fresh backup, the migration guard then stops `serve`.
* **Rationale & Recommendation**: Choose one fix and write it in:
  - make the loop check for `request-now` every few seconds, and keep the hourly cadence only for the 24 h check; or
  - have `backup-now.sh` run the backup directly (for example `docker compose exec backup /backup.sh --now`).

  Either way, define a lock that stops two runs at once.

### B4. The Go build, lint and tests fail on a clean checkout because of `go:embed` - Severity: Blocker
* **Reviewer Lens:** `egm-codebase-details`, `egm-implementation-readiness`, `egm-style`
* **Affected Scope:** Lines 1108, 1133-1140, 1368, 1378, 1858-1861
* **Discrepancy Description**:
  - `web/embed.go` uses `//go:embed all:build`, and `web/build` is gitignored (line 1108).
  - In CI, `make lint`, `make test`, `make test-db` and `govulncheck ./...` run before `make build` (lines 1135-1139).
  - When an embed pattern matches no files, Go does not compile the package, so all of those steps fail on a fresh checkout. The same happens to a developer who runs `go test ./...` before `make web`.
  - Related problem: Go's `./...` also walks into `web/node_modules`, which can contain `.go` files.
  - Related problem: item 35 (line 1368) says `static.go` has the embed directive, but item 36 puts it in `web/embed.go`. Item 35's version cannot work, because embed paths cannot reach parent directories.
* **Rationale & Recommendation**:
  - Either make `lint`, `test` and `test-db` depend on `web` (and reorder CI), or have the Makefile create `web/build/.keep` before any Go command. adapter-static wipes its output folder, so recreate the file each time rather than committing it.
  - Keep `node_modules` out of Go patterns, for example with an `ignore ./web/node_modules` directive in `go.mod`. The Go version that added `ignore` is unverified; the reviewer believes it is 1.25.
  - Change item 35 to say it imports `web.Build` and serves it through `fs.Sub` and `http.FileServerFS`.

### B5. The service worker cannot open the app offline, and the update flow is undefined - Severity: Blocker
* **Reviewer Lens:** `egm-codebase-details`, `egm-implementation-readiness`
* **Affected Scope:** Lines 1627, 1640-1647 (item 65), 1649-1658, 1697-1707, 1869
* **Discrepancy Description**:
  - Precaching `build` and `files` from `$service-worker` leaves out three things:
    - the adapter-static fallback `index.html`;
    - the manifest, which Go builds at runtime;
    - the icons, which Go also builds at runtime.

    So a cold open with the server off fails. That breaks CUJ 6 and the push target `/enviar`.
  - Cache-first for the shell, with no `skipWaiting` / `clients.claim` policy, gives "toca para actualizar" nothing to trigger.
  - The SPA's own version, which is compared against `X-App-Version`, is never injected into the Node build stage.
* **Rationale & Recommendation**:
  - Precache `/`, `/manifest.webmanifest` and the icon URLs explicitly.
  - Answer navigation requests network-first, falling back to the cached shell on a network error or 5xx/530.
  - Define the update handshake, for example: the new service worker waits, the prompt posts `SKIP_WAITING`, then the page reloads.
  - Pass `VERSION` as a build argument to the Node stage and expose it to the app, for example as `__APP_VERSION__`.

### M1. The e2e test clock cannot run against the binary CI builds - Severity: Major
* **Reviewer Lens:** all three
* **Affected Scope:** Lines 1140, 1769-1782 (items 79-80), 1864
* **Discrepancy Description**:
  - e2e runs against `make build`, which has no `e2e` tag, so `POST /api/v1/test/clock` does not exist during e2e.
  - Two gates are named (an env flag and a build tag) without saying how they relate.
  - No file is named that registers the endpoint under the tag.
  - `go list -tags '' ./...` does not prove the release binary lacks testclock.
  - The plan does not say how the offset clock reaches every module's `Clock`.
  - Nothing forbids SQL `now()` or `CURRENT_DATE` in time-based queries. Those would ignore the fake clock.
* **Rationale & Recommendation**:
  - Add a `build-e2e` target (`-tags e2e`) and point `make e2e` and CI at it.
  - Use one gate.
  - Name the tagged file, for example `cmd/open-customer/testclock_e2e.go`.
  - Prove absence with `go list -deps ./cmd/open-customer | grep testclock` returning nothing, or with a test that gets a 404 from the release binary.
  - Add the rule that all time comparisons take parameters from `Clock`.

### M2. Icon upload during setup cannot work - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1407-1412, 1444-1450, 1511-1520, 1714-1718; conflicts with lines 318, 367-369, 527-528, 544
* **Discrepancy Description**:
  - The wizard asks for the icon before any session exists, but `POST /settings/icon` requires auth.
  - `icon_media_id` points at `media`, whose CHECK constraint requires a client or appointment owner.
  - There are two icon sizes but only one ID.
  - The media module accepts only JPEG.
* **Rationale & Recommendation**: Decide how icons are stored, for example settings-owned files or bytea for the 512 and 180 sizes. Decide whether `POST /setup` takes the icon as multipart, or whether the icon is uploaded after login. Update items 28, 40 and 44 to match.

### M3. The setup token lifecycle is undefined - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1245-1264, 1407-1412
* **Discrepancy Description**:
  - Tokens are stored hashed, so they cannot be shown again.
  - The plan does not say whether each `serve` start or `setup-link` run creates a new token, or whether older tokens are invalidated.
  - `https://<PUBLIC_URL>/setup#<token>` repeats the scheme, because `PUBLIC_URL` already includes `https://`.
* **Rationale & Recommendation**: State the rule, for example: one new 24 h token per start (while no user exists) or per `setup-link` run, with older unused tokens expired. Use `${PUBLIC_URL}/setup#<token>`. Say how the wizard checks the token before the last step.

### M4. `create-owner` leaves the app with no settings row - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1258-1263; conflicts with lines 251 and 310
* **Discrepancy Description**: `create-owner` only creates a user. The settings row and the starter content come from `CompleteSetup`. After `create-owner`, setup is refused because a user exists, so screens that need settings break.
* **Rationale & Recommendation**: Either have `create-owner` also collect the settings and call `starter.Seed`, or let the wizard run in a "settings only" mode while the settings row is missing. Choose one.

### M5. The `push_hour` count misses come-back items - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1578-1590 (item 58); conflicts with lines 320, 449, 843
* **Discrepancy Description**: The defaults are `push_hour` 8 and `come_back_hour` 9. Come-back items only become due from 9:00, so at 8:00 a day with only come-backs counts 0 and no push is sent. That is the case Alternative F1 says push exists for.
* **Rationale & Recommendation**: Count items due at any time today, or require `come_back_hour <= push_hour`. Add a scheduler test for this case.

### M6. The "Enviados" list has no data source and no screen - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1722-1729 (item 74); conflicts with CUJ 4 at line 167
* **Discrepancy Description**: CUJ 4 moves a sent item to "Enviados" with its exact text. But:
  - `DueList` drops logged items;
  - the only log read is per client (`GET /reminders/log?client_id=`);
  - item 74 shows no Enviados section.
* **Rationale & Recommendation**: Add today's log, either as an endpoint or as part of `/bootstrap`, plus an Enviados section in item 74. Or drop "Enviados" from the CUJ.

### M7. No screen shows agenda days other than today - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1683-1757 (items 70-77)
* **Discrepancy Description**: The navigation is Hoy, Enviar, Clientas and Ajustes, and Hoy shows only today. `WeekView` is built but no route uses it. CUJ 3 books a Saturday, but she cannot see the Saturday agenda.
* **Rationale & Recommendation**: Add an agenda route, or a date switcher on Hoy, and say where `WeekView` is used.

### M8. The restore procedure lacks write access and run details - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1786-1796 (item 81), 1847 (item 87)
* **Discrepancy Description**: The plan does not cover:
  - Media access: the backup container mounts `media` read-only (line 642), so `restore.sh` cannot replace media through it.
  - Running restic: the container's entrypoint is the backup loop, so it is unclear how restic restore is launched.
  - The loop during restore: nothing says whether the backup loop is stopped while a restore runs.
* **Rationale & Recommendation**: Give the exact command sequence:
  - a `docker compose run --rm --entrypoint ...` with a writable media mount;
  - stopping and restarting `backup` around the restore.

### M9. Database connection settings are missing - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1269-1272 (item 20), 1798-1801 (item 82)
* **Discrepancy Description**: The plan says "DATABASE_URL is built from DB_* parts", but only `DB_PASSWORD` is defined (lines 686-698). Undefined:
  - host, port, user, database name and sslmode;
  - the `POSTGRES_*` variables for `db`;
  - the `PG*` variables that `pg_dump` and `pg_restore` need.
* **Rationale & Recommendation**: List every variable with its default, and say which container gets which.

### M10. Container paths, volume ownership and the trusted-proxy default - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 1120-1128, 1786-1796, 1352-1362
* **Discrepancy Description**:
  - The mount paths in `app` for `media` and `backup-status` are not specified, yet the guard hardcodes `/backup-status/status.json`.
  - Volume ownership: distroless `nonroot` runs as UID 65532 and has no shell. A named volume mounted on a path that does not exist in the image is created owned by root, so the app cannot write photos.
  - `TRUSTED_PROXY_CIDRS` defaults to "the Compose network", but Compose picks that subnet at runtime.
  - The Dockerfile uses `$VERSION` without declaring `ARG VERSION`.
* **Rationale & Recommendation**:
  - In the final stage, `COPY --chown=65532:65532` empty `/data/media` and `/backup-status` directories.
  - Fix the mount paths.
  - Declare a fixed Compose subnet and use it as the default CIDR.
  - Add `ARG VERSION`, and say whether `-trimpath` is used.

### M11. The migration guard on first install, the "empty" definition and the `status.json` format - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-style`
* **Affected Scope:** Lines 1301-1309 (items 26-27), 1836; conflicts with line 674
* **Discrepancy Description**:
  - On a new database the `users` table does not exist yet, so the check "any user exists" fails with an error.
  - Three different conditions are used for "empty": "the database is not empty" (line 674), "any user exists" (item 26) and "an empty DB" (item 27).
  - The `status.json` fields have no defined formats.
  - With the USB marker missing, no dump runs at all, so every upgrade is blocked while the disk is unplugged. The plan does not say whether that is intended.
* **Rationale & Recommendation**:
  - Treat goose version 0 or a missing `users` table as empty, and use one definition of empty everywhere.
  - Give a `status.json` example with RFC 3339 UTC times.
  - Confirm the USB-unplugged behavior, or allow a local-only dump.

### M12. The red backup line contradicts verification step 6 - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-style`
* **Affected Scope:** Lines 1597-1606 (item 60), 1871; conflicts with line 631
* **Discrepancy Description**: Line 631 shows red only when the last copy is more than 48 h old. Step 6 expects red within 2 hours of unplugging the USB disk, which cannot happen under that rule. Item 60 defines no red or amber rules.
* **Rationale & Recommendation**: Define the rule in item 60, for example red when `last_error` is set or the last copy is more than 48 h old, and align step 6 and line 631 with it.

### M13. `/bootstrap` "one `pgx.Batch` through the modules' read functions" cannot be built as stated - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`, `egm-implementation-readiness`, `egm-style`
* **Affected Scope:** Lines 1348-1349, 1319-1326
* **Discrepancy Description**:
  - sqlc batch code (`:batchone`, `:batchmany`, `:batchexec`) runs one statement over many parameter sets. It cannot combine different queries into one shared batch.
  - Module functions run their own queries, so they cannot add to an outside batch.
  - `DueList` needs two round trips anyway.
  - The handler would also break the rule on line 1348 that each handler calls one module function.
* **Rationale & Recommendation**: Run the module reads one after another in a single read-only `REPEATABLE READ` transaction, owned by a named `bootstrap` service function. Drop "one batch".

### M14. Session IDs, login throttling and password-change behavior - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-style`
* **Affected Scope:** Lines 1258-1264, 1361-1362, 1393-1430 (items 38-42)
* **Discrepancy Description**:
  - `DELETE /sessions/{id}` needs a public ID, but `sessions` is keyed only by `token_hash`.
  - The per-username delay is undefined: start value, sleep or 429, and memory bounds.
  - It is not stated whether a reset or a password change revokes other sessions.
  - It is not stated whether `/auth/password` needs the current password.
  - `device` has no source.
  - The `must_change_password` gate's exceptions mix area names with paths, and they block `/auth/logout`.
* **Rationale & Recommendation**:
  - Add `id uuid` to `sessions`.
  - Specify the delay, for example 1 s doubling to 30 s, then 429 with `Retry-After`, with entries expiring after 15 min.
  - Specify revocation.
  - Take `device` from a truncated User-Agent.
  - List the gate's exceptions as exact routes, including `POST /auth/logout`.

### M15. The migration DDL leaves types, foreign keys and indexes to the engineer - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1311-1317 (item 28)
* **Discrepancy Description**: The plan leaves out:
  - most column types, nullability and defaults;
  - every foreign key and its `ON DELETE` rule;
  - indexes the queries need: `appointments(ends_at)`, `reminder_log(appointment_id)`, `media(client_id)`, `media(appointment_id)` and `sessions(user_id)`;
  - a CHECK constraint on `come_back_hour`.
* **Rationale & Recommendation**: Write the full DDL, or list the types, foreign keys and indexes explicitly.

### M16. `f_unaccent` must use schema-qualified names on PostgreSQL 17 and later - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`
* **Affected Scope:** Line 1315 (with line 308)
* **Discrepancy Description**: Since PG 17, maintenance operations run with a restricted `search_path`. That includes index builds, REINDEX, restore and autovacuum. An unqualified `unaccent` wrapper used in the trigram index will fail during those operations on the PG 18 target.
* **Rationale & Recommendation**: Spell out the function, for example:

  ```sql
  CREATE FUNCTION public.f_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
  AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;
  ```

### M17. Short client-name searches will not match - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1466-1484 (items 47-48)
* **Discrepancy Description**: The `%` operator compares trigram similarity across the whole string, with a 0.3 threshold. "ana" against "Ana López" usually falls below it, and CUJ 3 depends on this kind of search.
* **Rationale & Recommendation**:
  - Use `word_similarity` (`<%`) or `ILIKE`, and define the ordering.
  - Say whether the picker searches locally or on the server.
  - Add a test for a 3-letter query.

### M18. Appointment edit rules are undecided - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1491-1509 (item 50)
* **Discrepancy Description**: These rules are not decided:
  - How a request marks `ends_at` as set by hand, and whether a hand-set end survives later service edits.
  - How price snapshots behave when services are edited.
  - Whether a status change bumps `version` or needs `If-Match`.
  - Whether `completed` counts as active for the overlap check.
* **Rationale & Recommendation**: State each rule and add a test for each.

### M19. What happens when a template, rule or service is archived - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1522-1530, 1543-1550
* **Discrepancy Description**: The plan does not say:
  - what happens to an active rule whose template or service is archived;
  - how `first_name` is derived;
  - whether `task_text` is validated when saved;
  - how a literal `{{` is escaped.
* **Rationale & Recommendation**: Define each case in items 54 and 56.

### M20. The Web Push payload contract is missing - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 1568-1576 (item 57), 1640-1647 (item 65)
* **Discrepancy Description**: The plan does not define:
  - the JSON fields that the server and the service worker share;
  - where the Spanish text comes from;
  - TTL and urgency;
  - whether a failed send still marks today's push as done.
* **Rationale & Recommendation**: Give the payload, the TTL (for example 12 h) and the retry rule.

### M21. The export layout is under-specified - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 1326, 1592-1595 (item 59); conflicts with line 474
* **Discrepancy Description**:
  - The plan does not define:
    - the CSV columns, and how services and amounts are formatted;
    - how two attributes with the same label avoid CSV header collisions;
    - whether archived records are exported;
    - the structure of `datos.json`.
  - Line 474 includes pending items in the export; item 59 leaves them out.
  - "Cursor reads in batches of 500" does not fit sqlc, which does not model `DECLARE CURSOR`.
* **Rationale & Recommendation**:
  - Specify the columns, the de-duplication rule for headers, the archived-record policy and an outline of `datos.json`.
  - Make line 474 and item 59 agree on pending items.
  - Use keyset pagination (`WHERE id > $1 ORDER BY id LIMIT 500`) inside one `REPEATABLE READ` transaction.

### M22. The PRD's release-1 scope and change list are left to the doc writer - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-style`
* **Affected Scope:** Lines 1159-1179 (item 12)
* **Discrepancy Description**:
  - The plan requires "every FR and NFR ID marked In or Out", but only about a dozen are decided here.
  - The change bullets are too short to act on:
    - "NFR-1 replaced" does not say with what.
    - The Goal wording differs from Goal 1.
    - "Add the interview and the iPhone test" does not say where.
    - The FR changes have no pointers to the sections they come from.
* **Rationale & Recommendation**: Put the full In/Out table in the plan, use Goal 1's exact wording, and add a source-section pointer to each change.

### M23. Strict-server handlers cannot set cookies or headers unless the spec declares them - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`
* **Affected Scope:** Lines 1229-1230, 1236, 1343, 1405
* **Discrepancy Description**:
  - oapi-codegen strict handlers get no `http.ResponseWriter`.
  - Login, setup and logout need `Set-Cookie`, which must be declared as a response header in the spec, or set in middleware.
  - Declaring `X-App-Version` on every operation would force every handler to fill it in.
* **Rationale & Recommendation**: State which headers are declared on which operations. Make `X-App-Version` middleware-only.

### M24. Hand-registered media and export handlers would clash with the generated mux - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`
* **Affected Scope:** Lines 1226, 1350
* **Discrepancy Description**: If `POST /media` and `GET /export` are in the spec, which item 15 requires, the generated std-http code registers them too. Registering the same pattern twice in a Go 1.22+ `ServeMux` panics at startup. Strict server does support multipart request bodies and streamed responses.
* **Rationale & Recommendation**: Either use the strict multipart and streaming types, or keep the operations in the spec but exclude them from Go generation (`exclude-operation-ids` or tags) and register them by hand.

### M25. The sqlc type overrides are incomplete for pgx/v5 - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`
* **Affected Scope:** Line 1118
* **Discrepancy Description**:
  - Nullable uuid columns need a second override with `nullable: true`.
  - `json.RawMessage` needs its full import path.
  - The plan does not say how `timestamptz`, `date` and nullable `text` columns map, or whether `emit_pointers_for_null_types` is set.
* **Rationale & Recommendation**: Write out the exact overrides block and emit options.

### M26. The goose wiring is not specified - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`
* **Affected Scope:** Lines 1304, 1602
* **Discrepancy Description**: goose needs a `*sql.DB`. Getting one from the pgx pool takes `pgx/v5/stdlib` (`OpenDBFromPool`), which is not listed. The pending-migrations check is not named.
* **Rationale & Recommendation**: Name the goose v3 `Provider` API (`NewProvider`, `HasPending`) and the stdlib import. Unverified: the goose minor version that added `HasPending`. Pin a version at or after it.

### M27. Getting pnpm into the Node stage and CI - Severity: Major (partly unverified)
* **Reviewer Lens:** `egm-codebase-details`
* **Affected Scope:** Lines 1058, 1124
* **Discrepancy Description**: `pnpm install --frozen-lockfile` needs pnpm installed. Unverified: the reviewer recalls that Node 25 and later no longer bundle corepack.
* **Rationale & Recommendation**: Install a pinned pnpm explicitly (`npm i -g pnpm@x`, or `pnpm/action-setup` in CI).

### M28. The 90% coverage target has no measurement or enforcement - Severity: Major
* **Reviewer Lens:** `egm-style`, `egm-implementation-readiness`
* **Affected Scope:** Line 1863
* **Discrepancy Description**:
  - No command, Makefile target or CI step measures coverage.
  - "`internal/appointments` (status and done rules)" does not say whether the target applies to the package or to specific files.
  - It is not said whether `db`-tagged tests count.
* **Rationale & Recommendation**: Name the command and a threshold check in `make test` or CI, and name the exact files or packages in scope.

### M29. The "Hoy < 1 s" check has no method or pass rule - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Line 1868
* **Discrepancy Description**: The check does not define:
  - the start and end events;
  - the measuring tool;
  - whether all three runs or only the median must pass;
  - whether the start is cold or warm.
* **Rationale & Recommendation**: Define each, for example a cold start (app killed), measured from tap to data shown with a screen recording, with all three runs under 1 s.

### M30. Some Goals have no verification step - Severity: Major
* **Reviewer Lens:** `egm-style`, `egm-implementation-readiness`
* **Affected Scope:** Lines 1856-1875 (Goals at lines 63-92), 1181-1213
* **Discrepancy Description**:
  - Goal 2 (booking under 30 s) and Goal 7 (nothing business-specific hardcoded) have no check.
  - Goals 5, 6 and 8 are covered only indirectly.
  - The Goal 3 query is "documented in `docs/sad.md`", but item 13 does not include it.
  - Line 65 says release 1 is done when all checks pass.
* **Rationale & Recommendation**:
  - Map every Goal to a step: a timed booking for Goal 2, and a review or grep for hardcoded business values for Goal 7.
  - Label the existing steps with the Goals they cover.
  - Add the Goal 3 query to item 13.

### M31. The ADR list mixes statuses under "Superseded" - Severity: Major
* **Reviewer Lens:** `egm-style`, `egm-implementation-readiness`
* **Affected Scope:** Lines 1189-1205
* **Discrepancy Description**:
  - "009/015 kept" sits under "Superseded".
  - 010, 016 and 017 do not say whether they are superseded, amended or deferred.
  - 008 says "now with Tailwind" next to a new ADR 023 for Tailwind.
* **Rationale & Recommendation**: Use one list per status, with each entry written as "NNN: status, reason".

### M32. Choices left open in the build instructions - Severity: Major
* **Reviewer Lens:** `egm-style`, `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 1611, 1672, 1796
* **Discrepancy Description**:
  - Line 1611 offers either `America/Mexico_City` (before 2022) or `Europe/Madrid`.
  - Line 1672 says "a lint rule or review check".
  - Line 1796 leaves the PG 18 volume path as an open question. The reviewer believes, unverified, that PG 18 images declare the volume at `/var/lib/postgresql`, with `PGDATA` at `/var/lib/postgresql/18/docker`.
* **Rationale & Recommendation**: Make each choice in the text, or link it to an Open Question with an owner and the phase it must be resolved by.

### M33. "Requires one owner ID" is ambiguous - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Line 1520 (against line 369)
* **Discrepancy Description**: The schema requires at least one of `client_id` or `appointment_id`. "One owner ID" reads as exactly one.
* **Rationale & Recommendation**: Write "rejects an upload with neither `client_id` nor `appointment_id`", and say whether both are allowed together.

### Nits

| # | Lines | Lens | Issue and recommendation |
| --- | --- | --- | --- |
| N1 | 1057 | style | The sentence says "change them in one place" and then lists four files. Write "these four files". |
| N2 | 1157, 1556, 1879 | style | "EGM Step 6", "the SAD bug" and "Drafted with EGM" are unexplained jargon. Replace each with plain wording. |
| N3 | 1154-1157, 1218, 1851-1854 | style, IR | Item 11 should be a Modify (moved from `design_draft.md`). The Predecessors links (line 6) and Additional Resources links (lines 1035-1037) point at files item 88 deletes; list the link updates. Four root report files are neither moved nor deleted. Decide what happens to them. |
| N4 | 1159, 1181, 1215, 1854 | style | The deletions are stated in items 12-14 and again in item 88. Keep them in one place and drop the justification sentence. |
| N5 | 1368 | style | Item 35's embed wording is misleading. Refer to `web.Build` from item 36 (see B4). |
| N6 | 1496-1498, 1582, 1604 | style | Bare signatures with no behavior (`Transition`, `IsDone`, `TidyCompleted`, "Heartbeat.", the client-error endpoint has no path). Describe what each one does. |
| N7 | 1655 | style | "5xx (including 530/502/524)" repeats itself. Also say whether a real 500 is shown as offline or as an error. |
| N8 | 1744 | style | "appointment history (online)" is ambiguous. Write "outside the cached window, loaded only when online". |
| N9 | 1835 | style | "The check" refers to a six-step routine. Write "Runs the six steps in 'Backups'". |
| N10 | 1736 | style | The status button labels mix forms (adjective, verbs, a past-tense phrase). Use one form, or point to `es.json` for the final copy. |
| N11 | 1113, 276 | IR | The CSRF Origin check sits in `auth` in the Technical Plan but in `httpapi` in the plan. Pick one. depguard covers only part of the module rules. Say how `starter` writes data. |
| N12 | 1239-1243, 1599-1603 | IR | `/healthz`, `/readyz`, the manifest and the icon routes have no named handler, and it is unclear whether they are in the spec. |
| N13 | 1058, 1078 | IR, CD | golangci-lint, govulncheck, the Playwright browsers, the `cloudflared` tag and the Postgres minor version are not pinned. Running sqlc via `go tool` needs cgo. |
| N14 | 1495, 1736 | IR | Money is stored in cents. Say how currencies with zero decimal places are entered and shown (the ISO 4217 exponent). |
| N15 | 1263 | IR | Give the character set for the 12-character temporary password. |
| N16 | 1754 | IR | The sentence builder covers only come-back rules. Describe the UI for `before_appointment` and `owner_task` rules. |
| N17 | 1058, 1078 | CD | Drop the goose CLI tool line, since the library is enough. Consider a separate `tools/go.mod` for sqlc and oapi-codegen to keep tool dependencies out of the main module graph. |
| N18 | 1072-1077 | CD | Name the exact module paths: oapi-codegen v2 and its runtime, `sqlc-dev/sqlc`, the testcontainers postgres module, and `pgx/v5/stdlib`. Pin `google/uuid` to v1.6.0 or later, because `NewV7` returns an error that must be handled. |
| N19 | 1396 | CD | The Argon2id PHC string is hand-written. Spell out the format, the `RawStdEncoding` base64 and the constant-time compare. |
| N20 | 1076, 1572-1576 | CD | The webpush-go test subscription needs real P-256 and auth keys. Unverified: the `mailto:` handling, Apple's JWT `exp` limits and maintenance status. Pin a version. |
| N21 | 1448, 1473, 1479 | CD | phonenumbers: use `IsValidNumber`, not `IsPossibleNumber`, and name `GetSupportedRegions()`. The CR example is correct. |
| N22 | 1231, 1236 | CD | A 3.0.3 `oneOf` generates a clumsy union type. Plan a decoding helper in `attributes/validate.go`. |
| N23 | 1655-1657, 1763 | CD | openapi-fetch throws on a non-JSON 200, so map that to Offline explicitly. Unverified: whether `application/problem+json` is typed as `error`. |
| N24 | Tailwind | CD | Use `@theme inline` for the runtime accent color. List `tailwindcss()` before `sveltekit()` in the Vite plugins. |
| N25 | 1627-1640 | CD | SvelteKit details: `register: true` is already the default; `tsconfig.json` must extend `.svelte-kit`; the service worker needs type references; `setAppBadge` is on `self.navigator`. |
| N26 | 1637 | CD | `apple-mobile-web-app-capable` is deprecated. The manifest's `display: standalone` is what iOS reads. |
| N27 | 1679 | CD | Unverified: whether iOS Safari supports the `createImageBitmap` options. Add an `<img>` fallback. |
| N28 | 1059, 1776 | CD | Unverified: Playwright WebKit routing when a service worker is active. State that the offline shell stays a manual check. |
| N29 | 1829, 1837, 1839 | CD | Define the "repository missing" test (for example `restic cat config`). Use a bash shebang with `set -euo pipefail`. Unverified: restic's exit code 10 for a missing repository. |
| N30 | 1110-1113, 1126, 1603 | CD | State the golangci-lint major version (v2 uses a different config format) and spell out the depguard rules. Pin the distroless image by digest; unverified whether `static-debian13` tags exist. Prefer `x/sys/unix.Statfs`. |

### Claims checked and found correct (codebase-details)
- sqlc with `pgx/v5`, and sqlc reading goose-format migrations.
- goose embedding a `migrations/` subfolder.
- `go:embed` cannot use parent paths, and the `all:` prefix is needed for `_app`.
- The adapter-static fallback with `ssr = false`.
- The `$service-worker` exports.
- `image/jpeg.DecodeConfig`.
- `time/tzdata`, and Mexico City's DST history before 2022.
- `x/time/rate`.
- `fake-indexeddb` with Vitest.
- testcontainers on GitHub runners.
- The distroless `nonroot` image with a `healthcheck` subcommand.
- `postgres:18-alpine` providing a matching `pg_dump`.
- oapi-codegen v2 config keys.
- `pg_trgm` and `unaccent` are trusted extensions.
