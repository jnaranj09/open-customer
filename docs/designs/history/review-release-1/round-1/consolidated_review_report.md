# Consolidated Review Report — release-1 design

*   **Target:** `docs/designs/release-1.md` (1,879 lines; FeatureRange 1–1052, DetailedRange 1053–1879)
*   **Date:** 2026-10-30
*   **Mode:** consolidated (all five stages, no fixes applied between stages; every finding refers to the same unedited document)

## Stage status

| Stage | Reviewers | Status | Blocker | Major | Nit | Report |
| --- | --- | --- | --- | --- | --- | --- |
| 5 Initial Zoom Out (Z5) | zoom-out ×2 | completed | 0 | 8 | 10 | [zoom_out_report.md](./zoom_out_report.md) |
| 6 Comprehension (C6) | comprehension | completed | 0 | 7 | 8 | [comprehension_report.md](./comprehension_report.md) |
| 7 Critic (K7) | design, structural, style | completed | 0 | 15 | 33 | [critic_report.md](./critic_report.md) |
| 8 Intermediate Zoom Out (Z8) | zoom-out | completed | 0 | 9 | 7 | [intermediate_zoom_out_report.md](./intermediate_zoom_out_report.md) |
| 9 Implementation Readiness (R9) | implementation-readiness, codebase-details, style | completed | 5 | 33 | 30 | [implementation_readiness_report.md](./implementation_readiness_report.md) |

No reviewer failed or needed a retry. Raw total: 5 Blocker, 72 Major, 88 Nit. After merging overlaps there are **23 action items**: 5 Blocker, 16 Major, 2 Nit groups.

## Overall verdict

*   **The feature design (lines 1–1052) holds up.** No reviewer found a Blocker in the problem, the goals or the architecture.
*   **The weak spots are a small set of design rules** that several stages found on their own: `come_back_hour`, offline sending, backup-before-upgrade, "before" reminders for late bookings, and search.
*   **The implementation plan (lines 1053–1879) cannot be built as written.** It has no build order, no request/response shapes, a broken clean-checkout build, an offline shell that cannot open, and an upgrade flow that almost always aborts.
*   **Several cuts proposed by the zoom-out lenses reverse decisions you already made** (offline "sent" queue, SAD rewrite, `owner_task`, sub-options, i18n). They are listed under "Needs your decision". They are not applied automatically.

## Action items

### 🔴 Blocker

#### A1. Add a build order with checkpoints
*   **Sources:** R9-B1.
*   **Scope:** lines 1062–1854.
*   **Problem:** The 88 items have no order. Nothing defines when the repo must build and pass tests.
*   **Action:** Group the items into phases. Proposed:
    *   P0: repo + tooling + CI skeleton
    *   P1: DB + migrations + platform
    *   P2: auth + setup
    *   P3: catalog modules
    *   P4: reminders + push
    *   P5: web app
    *   P6: deploy + backup

    Each phase ends with named `make` targets that must pass.

#### A2. Write the API contract details into the design
*   **Sources:** R9-B2, R9-M20, R9-M23, R9-M24, R9-N12, R9-N22, R9-N23.
*   **Scope:** item 15 (lines 1222–1231), Technical Plan "API".
*   **Problem:** About 50 request and response bodies are undefined: setup, bootstrap, due item (including its stable ID), PATCH semantics, problem types, pagination, push payload.
*   **Problem:** The oapi-codegen strict server gives handlers no ResponseWriter, so cookies and headers must be declared in the spec.
*   **Problem:** Hand-registering media and export next to the generated mux would panic at startup.
*   **Action:** Add schema tables for the key bodies. Declare `Set-Cookie`, `X-App-Version` and `If-Match` in the spec. Route multipart and streaming either through strict-server non-strict handlers or a separate sub-mux with non-overlapping paths. Define the health, manifest and icon routes.

#### A3. Fix the backup-before-upgrade and rollback flow
*   **Sources:** R9-B3, K7-M7, K7-M8, K7-M9, Z5#11, Z5#13, Z8#3, C6-8, R9-M8, R9-M11, R9-M12.
*   **Scope:** lines 654–682, 1304, 1835–1847, 1871.
*   **Problem:** `backup-now.sh` waits 10 minutes, but the loop checks only hourly, so upgrades almost always abort.
*   **Problem:** `upgrade.sh` does not stop on a backup failure.
*   **Problem:** A rollback can re-run the migration it is undoing.
*   **Problem:** The guard condition is worded two ways ("DB not empty" vs "any user exists").
*   **Problem:** The 48 h red line contradicts the "red within 2 h" verification step.
*   **Problem:** The restore has no write access to the media volume.
*   **Problem:** The behavior when the USB disk is unplugged is not stated.
*   **Action (Z8 proposal):** `upgrade.sh` runs the backup directly with `docker compose run --rm backup once` and stops on failure. Remove the request file, the polling and the migration guard in `serve`. `restore.sh` takes the target version and sets `APP_VERSION` before starting `app`. State that the backup disk is mounted at boot, and make the red-line rule consistent.
*   **Note:** removing the guard drops brief item "dump before every migration is enforced by the app". This needs your decision (D3).

#### A4. Make a clean checkout build
*   **Sources:** R9-B4, R9-N5.
*   **Scope:** lines 1108, 1133–1140, 1368, 1378.
*   **Problem:** `//go:embed all:build` points at the gitignored `web/build`, so lint, test, test-db and govulncheck fail before `make build` runs.
*   **Problem:** `./...` walks into `web/node_modules`.
*   **Problem:** Items 35 and 36 disagree about where the embed lives.
*   **Action:** Commit a placeholder `web/build/.gitkeep` + `index.html`, or build the web app first in every CI job. Exclude `web/node_modules` (for example with a `go.mod` boundary file or by moving the embed package). Make item 35 reference `web.Build` only.

#### A5. Make the offline shell open, and define the update flow
*   **Sources:** R9-B5.
*   **Scope:** item 65 (lines 1640–1647), line 584.
*   **Problem:** The precache leaves out `index.html`, the manifest and the icons, so a cold open with the server off fails CUJ 6.
*   **Problem:** The SPA's build version is never injected, and the update handshake is undefined.
*   **Action:** Precache the shell explicitly. Inject `version` at build time (a Vite define from the same value as `-ldflags`). Define the update flow: compare the build version with `X-App-Version`, then on tap call `skipWaiting` and reload.

### 🟡 Major

#### A6. Offline sending: hide stale cached items, or remove offline sending
*   **Sources:** C6-2, C6-9, K7-M10, K7-M11, Z8#1.
*   **Scope:** CUJ 6 (lines 180–189), lines 305, 511–523.
*   **Problem:** The cached list is from the last refresh, so offline she can send a "before" reminder after the appointment started.
*   **Problem:** A Dexie schema change can wipe unsent confirmations.
*   **Problem:** A rejected replay can retry forever and show a false "server off" banner.
*   **Problem:** Goal 6 says nothing is shown as saved offline, but the outbox does exactly that.
*   **Action:** Choose (D1):
    *   (a) keep the outbox, hide cached "before" items after `starts_at`, keep the outbox across schema changes, drop items on 4xx, and add the Goal 6 exception;
    *   (b) Z8: drop offline sending, so the list is read-only offline.

#### A7. Remove `come_back_hour`
*   **Sources:** Z5#1, C6-1, K7-M3, K7-N28, Z8#4, R9-M5. All five stages raised it.
*   **Scope:** lines 159–161, 260, 319–320, 449, 1583.
*   **Problem:** With the defaults (push 8:00, come-back 9:00), the push never counts come-back items. On a come-back-only day no push is sent.
*   **Action:** Remove `come_back_hour`, so come-back items are due from the start of their due day. Add `CHECK (push_hour < 20)`. State that a 0 count still records the day as done.

#### A8. No "before" reminder for late bookings, and no flood from backfilled visits
*   **Sources:** K7-M4, K7-M5, Z8#2.
*   **Scope:** lines 448–449, 463–464.
*   **Problem:** A same-day booking gets a "24 h before" item.
*   **Problem:** Notebook visits entered after the fact flood the come-back list.
*   **Action (Z8):** one rule: an item exists only if its due day is on or after the day the appointment was created (`created_at`). Optionally allow only one "before" rule in release 1.

#### A9. Fix the setup and admin flow
*   **Sources:** Z5#3, R9-M2, R9-M3, R9-M4.
*   **Scope:** lines 244–255, 525–539, items 19, 40, 44.
*   **Problem:** `create-owner` leaves no settings row, and the wizard refuses to run once a user exists.
*   **Problem:** The setup-token lifecycle (create, re-issue, expiry) is undefined.
*   **Problem:** The icon cannot be uploaded during setup: there is no session yet, the `media` CHECK fails, and the server accepts JPEG only.
*   **Action:** Drop `create-owner` (the setup link covers it), or make it also create settings. Define the token lifecycle. Upload the icon after the session exists, as the wizard's last step, through `POST /settings/icon` (PNG allowed there, not stored in `media`).

#### A10. Search: one path that matches short names
*   **Sources:** Z5#5, K7-M6, R9-M17, Z8#7.
*   **Scope:** line 558, item 47.
*   **Problem:** A query with no digits matches every client with a phone (`LIKE '%%'`).
*   **Problem:** The trigram threshold misses "ana".
*   **Problem:** Server and local search can return different results.
*   **Action:** Choose (D2):
    *   (a) local search only, because all active clients are cached;
    *   (b) fix the server query: skip the digit clause when there are no digits, and use `ILIKE` prefix + `word_similarity`.

#### A11. `/bootstrap`, the list endpoints and the module rules
*   **Sources:** Z5#6, C6-7, R9-M13, R9-N11.
*   **Scope:** lines 265–301, 513, items 33, 40, 43.
*   **Problem:** A cross-module `pgx.Batch` cannot be built from the modules' read functions.
*   **Problem:** `/bootstrap` duplicates the list endpoints.
*   **Problem:** `starter` and `testclock` are missing from the layout.
*   **Problem:** `auth → starter` writing into other modules' tables breaks the stated rule.
*   **Problem:** The Origin check is placed in `auth` in one section and in `httpapi` in another.
*   **Action:** Bootstrap calls each module's read function in sequence, or one `bootstrap` query file owned by `httpapi`. Drop the unused list endpoints, or state who uses them. Add `starter` and `testclock` to the layout, with an explicit allowed dependency (setup orchestration may call module create functions). Put the Origin check in `httpapi` middleware.

#### A12. Remove the stored "completed" tidy job
*   **Sources:** Z5#7, Z8#5, K7-N17, K7-N27.
*   **Scope:** lines 261, 567–569.
*   **Problem:** The tidy job duplicates `IsDone`.
*   **Problem:** Once it has run, she cannot cancel the appointment.
*   **Problem:** Its effect on `version` is unstated.
*   **Action:** Remove the job. `completed` is never set in release 1. Export and screens use `IsDone`.

#### A13. Add a short Risks section, and make Ley 8968 a go-live check
*   **Sources:** K7-M1, K7-M12, Z8#10, Z8#11.
*   **Scope:** whole document, lines 118, 1016.
*   **Problem:** Several risks are not written anywhere: the author and the PC are a single point of failure, the login is password-only on the internet, and the tunnel reaches the home network.
*   **Problem:** Ley 8968 is not tied to go-live.
*   **Action:** Add an "Accepted risks" list, with no new mitigations. Add "check Ley 8968 obligations" to the pre-handover checklist.

#### A14. Make the document readable on its own
*   **Sources:** C6-3, C6-4, C6-5, C6-12, C6-13, C6-14, K7-M2, K7-N12, K7-N13, R9-M22, R9-M31, R9-N2.
*   **Scope:** lines 1033, 1040, 1159–1205, scattered terms.
*   **Problem:** "Additional Resources" and "Future Improvements" are H3s nested under the NFRs.
*   **Problem:** The PRD changes are given as bare IDs.
*   **Problem:** The ADRs have no titles, and the "Superseded" heading also lists kept ones.
*   **Problem:** Phase 2 and Phase 3 are undefined.
*   **Problem:** Local terms and synonyms are undefined (relleno/come-back, "after rules", SINPE Móvil, bandeja, the review, the SAD bug).
*   **Action:** Promote the two headings to H2. Gloss every PRD and ADR ID. Fix the ADR status groups. Define the phases in one paragraph. Add a short glossary.

#### A15. Make every goal measurable, with a check in the Verification Plan
*   **Sources:** K7-M13, Z8#9, R9-M28, R9-M29, R9-M30.
*   **Scope:** lines 73, 79, 1856–1879.
*   **Problem:** Two Goal 3 checks have no data source.
*   **Problem:** The 90% coverage target is not enforced.
*   **Problem:** "Hoy < 1 s" has no method or pass rule.
*   **Problem:** Goals 2 and 7 have no check.
*   **Action:** Measure Goal 3 with tests that already exist (`due_test.go` cases), not a daily snapshot. Add a `go test -coverprofile` gate per package. Define "Hoy < 1 s": time from tap until the first appointment row is painted, 3 of 3 runs. Goal 2: time CUJ 3 with a stopwatch. Goal 7: grep the migrations and code for regional literals.

#### A16. Complete the data model and DB wiring
*   **Sources:** R9-M9, R9-M10, R9-M15, R9-M16, R9-M25, R9-M26, R9-M33.
*   **Scope:** lines 303–398, items 20, 26, 28, 81.
*   **Problem:** Some types, foreign keys and indexes are missing.
*   **Problem:** `f_unaccent` must call `public.unaccent('public.unaccent', $1)` with schema-qualified names, or index builds and restores fail on PG 17+.
*   **Problem:** The sqlc overrides are incomplete for pgx/v5 (`timestamptz`, nullable UUIDs).
*   **Problem:** The goose wiring is not specified.
*   **Problem:** `DB_HOST`/`DB_USER`/`DB_NAME` are undefined.
*   **Problem:** The container paths and the media volume ownership under distroless nonroot are not set.
*   **Problem:** "Requires one owner ID" is ambiguous.
*   **Action:** Write full DDL in the plan, or mark the shorthand and list every FK and index. Fix `f_unaccent`. Complete the overrides. Specify the goose setup. Add the DB variables. Add an init step for media volume ownership.

#### A17. Define the missing domain rules and screens
*   **Sources:** R9-M6, R9-M7, R9-M14, R9-M18, R9-M19, R9-M21, K7-N31, Z8#15.
*   **Problem:** "Enviados" has no source or screen.
*   **Problem:** No screen shows days other than today.
*   **Problem:** Session ID exposure, login throttling and password-change behavior (other sessions revoked?) are undefined.
*   **Problem:** The appointment edit rules are undefined (can she edit a past or done appointment, or change its services?).
*   **Problem:** The effect of archiving a template, rule or service is undefined.
*   **Problem:** The export layout is unclear (duplicate labels, decimal commas, archived attributes).
*   **Action:** Add each rule in a sentence or a table row.
    *   "Enviados" = today's reminder-log rows, shown in `/enviar`.
    *   Add a date switcher on Hoy.
    *   A password change revokes other sessions.
    *   Archiving a template or service leaves rules inactive; they are shown with a warning.
    *   The export uses label + key suffix for duplicates, and the CSV library quotes the values.

#### A18. The e2e test clock
*   **Sources:** R9-M1, Z8#12.
*   **Scope:** items 79, 80.
*   **Problem:** The endpoint needs the `e2e` tag, but the e2e tests run against the release binary.
*   **Problem:** Two different gates are named.
*   **Problem:** The `go list` check proves nothing.
*   **Action:** Choose (D4):
    *   (a) build a separate e2e binary with the tag, and check the release binary with `go version -m` and a symbol check;
    *   (b) Z8: remove `testclock`, and let the e2e tests seed appointments with past dates instead.

#### A19. Close the open tooling choices
*   **Sources:** R9-M27, R9-M32, R9-N13, R9-N17, R9-N18, R9-N30.
*   **Problem:** How pnpm gets into the Node image and CI (corepack) is not stated.
*   **Problem:** Some versions are left unpinned: golangci-lint (and its major version), govulncheck, the Playwright browsers, cloudflared, the Postgres minor version, distroless (by digest).
*   **Problem:** sqlc via `go tool` needs cgo.
*   **Problem:** The full module paths are missing.
*   **Action:** Add a pin table. Run sqlc through its Docker image or a separate `tools/go.mod`.

#### A20. Fix the unsupported or contradictory claims
*   **Sources:** K7-M14, K7-M15, Z8#13, Z8#14, K7-N5.
*   **Scope:** lines 988–989, 1009–1029.
*   **Problem:** The React "~40 KB" figure was never measured.
*   **Problem:** The NFR "one or two queries" contradicts the due-list design.
*   **Problem:** The NFRs repeat body text.
*   **Action:** Drop the size figure, and say "a larger runtime". Shrink the NFRs to short bullets that link to their sections.

#### A21. Scope cuts that reverse earlier decisions (reviewer proposals)
*   **Sources:** Z5#2, Z5#4, Z5#8, Z5#12, C6-6, Z8#6, Z8#8.
*   **Problem:** The zoom-out lenses propose cutting:
    *   sub-options, until the interview decides;
    *   the SAD rewrite (keep one architecture doc);
    *   the i18n plumbing behind Goal 7;
    *   `owner_task`, which has no journey.
*   **Problem:** Comprehension asks for an `owner_task` example if it stays.
*   **Action:** Your decision, D5. If you keep them, add an `owner_task` CUJ, and record the reason for the others in Alternatives.

### 🟢 Nit groups

#### A22. Wording and consistency
*   **Sources:**
    *   C6-10, C6-11, C6-15
    *   K7-N1–N4, N6–N11, N14–N16, N18–N26, N32
    *   R9-N1–N4, N6–N10
    *   Z5#18
*   **Action:**
    *   Use exact heading names.
    *   Remove subjective adjectives.
    *   Make the hedging consistent.
    *   Make the image name consistent (`postgres:18-alpine`).
    *   Make the button labels consistent.
    *   Change item 11 to Modify.
    *   Say what happens to the stage report files.
    *   State the sample client is not stored.
    *   Make the CUJ "When" steps numbered lists.

#### A23. Small design and accuracy notes
*   **Sources:**
    *   Z5#9, #10, #14, #15, #16, #17
    *   K7-N29, N30, N33
    *   R9-N14–N16, N19–N21, N24–N29
*   **Action:** Each is a one-line fix in the report it came from. Examples:
    *   Push subscription health on the status screen.
    *   Check ZIP download in the iOS PWA.
    *   Debounce the refresh on focus.
    *   Currency exponent for zero-decimal currencies.
    *   `IsValidNumber`.
    *   `@theme inline`.
    *   `apple-mobile-web-app-capable` is deprecated.
    *   An `<img>` fallback for `createImageBitmap`.
    *   restic repository detection.

    The zoom-out simplification nits (If-Match, phone IDs, VPS profile, curly quotes, extras) go with D5.

## Needs your decision

| ID | Question | Options |
| --- | --- | --- |
| D1 | Offline sending (A6) | (a) keep the outbox + hide items after start + the Goal 6 exception; (b) read-only offline, no sending |
| D2 | Search (A10) | (a) local only; (b) fix the server query |
| D3 | Migration guard (A3) | (a) remove it, and `upgrade.sh` runs the backup itself; (b) keep the guard and fix `backup-now` |
| D4 | e2e clock (A18) | (a) a separate tagged e2e binary; (b) remove it and seed past dates |
| D5 | Reviewer cuts (A21) | Per item: sub-options, SAD rewrite, i18n plumbing, `owner_task`, plus the Z5 nits (If-Match, phone-generated IDs, VPS profile, curly quotes) |

## Resolution (2026-10-30)

All findings were resolved with the owner, one decision at a time, then by applying the agent's recommendations. The edits are in `docs/designs/release-1.md`. The findings above refer to the document **before** these edits; their line numbers no longer match.

| ID | Decision / outcome |
| --- | --- |
| D1 | (b) Read-only offline. No outbox. Cached "before" items hidden after `starts_at`. Alternative G2 added. |
| D2 | (b) Server search fixed (digit clause only with 3+ digits, substring name match, word-start ranking). Local search uses the same rules. |
| D3 | (b) Keep the guard. `upgrade.sh` runs `backup once` directly (no polling), `.env` changes only after a good backup. Guard condition: pending migrations + a user exists + `last_copy_at` older than 60 min. |
| D4 | (b) No test clock. e2e specs seed past rows (including `created_at`) through `web/tests/e2e/seed.ts` (item 80). |
| D5 | Option trees of any depth, value stores only the picked option ID; a typed "group" type moved to Future Improvements. SAD reduced to an ADR index with titles. i18n kept (NFR-10, generic priority), lint rule dropped. `owner_task` cut (Future Improvements). `If-Match` dropped (last write wins). Server-generated IDs only. Caddy/VPS profile removed (item 84 removed, items renumbered to 87). Curly-quote parsing dropped. |
| A1 | Build order P0–P7 with exit checks added. |
| A2 | API conventions, problem types, schemas, request bodies, headers in spec, multipart/streaming in the strict server (with a P1 spike and fallback), push payload. `sessions.id` added. |
| A3 | Done with D3, plus: snapshots tagged with the app version, `restore.sh` sets `APP_VERSION` from the tag, a `restore` Compose profile with media read-write, `flock` between loop and `once`, USB at boot via fstab `nofail`, `rslave` bind propagation, red-line rule made consistent. |
| A4 | `make web-stub`, `GO_PKGS` without `node_modules`, `make vuln`, item 35 uses `web.Build`. |
| A5 | Explicit shell precache, navigations from cached `index.html`, versioned caches, `kit.version.name` = Go version, update flow defined. |
| A6 | Done with D1. |
| A7 | `come_back_hour` removed; `push_hour` CHECK 0–19; a 0 count still records the day. |
| A8 | "No items before the appointment existed" rule (`created_at`), with tests. |
| A9 | `create-owner` removed; setup-token lifecycle defined; icon uploaded after `POST /setup`, stored as files, `icon_updated_at` replaces `icon_media_id`. |
| A10 | Done with D2. |
| A11 | Bootstrap reads in one `REPEATABLE READ` transaction; unused list endpoints removed; `starter` in the layout with its allowed dependencies; Origin check in `httpapi`. |
| A12 | Tidy job and stored `completed` status removed. |
| A13 | "Accepted risks" section; Ley 8968 is a pre-handover check. |
| A14 | Headings promoted to H2; "Terms used in this document" added; PRD and ADR IDs glossed. |
| A15 | Goal 3 measured by `due_test.go` + the 4-week missed-reminder log; `make cover` gate; "Hoy < 1 s" method; Goal 2 and Goal 7 checks. |
| A16 | DDL shorthand rules, FKs, CHECKs, indexes, `f_unaccent` SQL, sqlc overrides, goose Provider wiring, DB variables, container paths, volume ownership, fixed Compose subnet, `ARG VERSION`. |
| A17 | "Enviados" source and section, date switcher on Hoy, session and password rules, appointment edit rules, archived template/service behavior, export layout. |
| A18 | Done with D4. |
| A19 | Pin table; sqlc through its Docker image; full module paths; corepack. |
| A20 | Size claim removed; NFRs shrunk to links. |
| A21 | Done with D5. |
| A22–A23 | Most nits applied. Not applied: Z5#14 (Technical Plan vs plan duplication), Z5#16 / Z8 extras (pagination, `/readyz`, Dispositivos, free space kept), K7-N2, N3, N9, N10, N20, N21, N23–N25 (wording of rejection reasons and heading names), R9-N4, N10, N17, N20. |

## Traceability matrix

| Raw finding | Action |
| --- | --- |
| Z5#1 | A7 |
| Z5#2, #4, #8, #12 | A21 |
| Z5#3 | A9 |
| Z5#5 | A10 |
| Z5#6 | A11 |
| Z5#7 | A12 |
| Z5#9, #10, #15, #17 | A21 / A23 (D5) |
| Z5#11, #13 | A3 |
| Z5#14, #16 | A23 |
| Z5#18 | A22 |
| C6-1 | A7 |
| C6-2, C6-9 | A6 |
| C6-3, C6-4, C6-5, C6-12, C6-13, C6-14 | A14 |
| C6-6 | A21 |
| C6-7 | A11 |
| C6-8 | A3 |
| C6-10, C6-11, C6-15 | A22 |
| K7-M1, K7-M12 | A13 |
| K7-M2 | A14 |
| K7-M3 | A7 |
| K7-M4, K7-M5 | A8 |
| K7-M6 | A10 |
| K7-M7, K7-M8, K7-M9 | A3 |
| K7-M10, K7-M11 | A6 |
| K7-M13 | A15 |
| K7-M14, K7-M15, K7-N5 | A20 |
| K7-N12, K7-N13 | A14 |
| K7-N17, K7-N27 | A12 |
| K7-N28 | A7 |
| K7-N29, N30, N33 | A23 |
| K7-N31 | A17 |
| K7-N1–N4, N6–N11, N14–N16, N18–N26, N32 | A22 |
| Z8#1 | A6 |
| Z8#2 | A8 |
| Z8#3 | A3 |
| Z8#4 | A7 |
| Z8#5 | A12 |
| Z8#6, #8 | A21 |
| Z8#7 | A10 |
| Z8#9 | A15 |
| Z8#10, #11 | A13 |
| Z8#12 | A18 |
| Z8#13, #14 | A20 |
| Z8#15 | A17 |
| Z8#16 | A23 |
| R9-B1 | A1 |
| R9-B2, M20, M23, M24, N12, N22, N23 | A2 |
| R9-B3, M8, M11, M12 | A3 |
| R9-B4, N5 | A4 |
| R9-B5 | A5 |
| R9-M1 | A18 |
| R9-M2, M3, M4 | A9 |
| R9-M5 | A7 |
| R9-M6, M7, M14, M18, M19, M21 | A17 |
| R9-M9, M10, M15, M16, M25, M26, M33 | A16 |
| R9-M13, N11 | A11 |
| R9-M17 | A10 |
| R9-M22, M31, N2 | A14 |
| R9-M27, M32, N13, N17, N18, N30 | A19 |
| R9-M28, M29, M30 | A15 |
| R9-N1, N3, N4, N6–N10 | A22 |
| R9-N14–N16, N19–N21, N24–N29 | A23 |
