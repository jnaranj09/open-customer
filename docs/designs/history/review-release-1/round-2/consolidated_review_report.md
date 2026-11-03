# Consolidated Review Report: release-1, round 2

- **Target:** `docs/designs/release-1.md` (2088 lines).
- **Split:** FeatureRange is lines 1–1113. DetailedRange is lines 1114–2088.
- **Mode:** Consolidated, with all five stages.
- **Context:** no extra files given.
- **Reviewers:** each ran in a fresh context, with direct dispatch. All reviewers completed.
- **Round 1:** reports are in `docs/designs/history/review-release-1/round-1/`.

## Stage reports

| Stage | Report | 🔴 | 🟡 | 🟢 |
|---|---|---|---|---|
| 5 Zoom Out | `zoom_out_report.md` | 0 | 4 | 7 |
| 6 Comprehension | `comprehension_report.md` | 0 | 7 | 7 |
| 7 Critic (design, structural, style) | `critic_report.md` | 0 | 16 | 37 |
| 8 Intermediate Zoom Out | `intermediate_zoom_out_report.md` | 0 | 7 | 9 |
| 9 Implementation Readiness (IR, CD, style) | `implementation_readiness_report.md` | 3 | 21 | 44 |

## Verdict

**Not ready to implement.**
- Round 1's large gaps are closed: offline, update flow, build order, API contract skeleton, DDL.
- Round 2 found a different layer of problems:
  1. **The phases don't build.** Each phase needs files that only come in later phases.
  2. **Module boundaries are not specified.** The Go interfaces between modules are missing, and so is the way a transaction is passed from one to the next.
  3. **Dev/e2e can't run.** The rules for https, Origin and `Secure` block plain localhost.
  4. **Some correctness and data-safety bugs:**
     - in the reminder identity;
     - in the restore and retention steps;
     - in the push health signal;
     - in the PWA (service worker).
  5. **About 15 internal contradictions.** Most of them are text left over from round-1 edits.

## Already decided by the owner (not reopened)

These reviewer proposals go against decisions from round 1. They are settled: there is no action item for them, and they are not reopened.

| Proposal | Source | Owner decision |
|---|---|---|
| Flat attribute options instead of trees of any depth | Stage 5 M1, Stage 8 N9 | D5.1: trees of any depth |
| Search clients from the cache only, drop the server query | Stage 5 M2, Stage 8 N6, N9 | D2: fix the server query |

**Side effect:** Stage 8's "simplest combined change" assumed both of these cuts. So these items stay open:
- the paging convention;
- design-n9 (the `unaccent` fold set).

The I1 contradiction (style-2-2) is still a real text bug. The fix is to rewrite I1's reasoning so it matches D5.1.

---

## Action items

### 🔴 Blockers

**C1: Make every phase build and pass its exit check.**
- What is wrong:
  - The Makefile and CI web/generate steps need `web/`, which only comes in P5.
  - The P1 spec content is undefined, and so is the spike's phase.
  - P2 `CompleteSetup` → `starter.Seed` calls create functions from P3 and P4.
  - The P1 `serve.go` steps depend on P2 and P4.
  - CI runs every target from P0.
- Sources: IR-B1, CD-M9, CD-M10, Stage 6 #3, ST-M7.

**C2: Specify the module Go interfaces and one transaction convention.**
- What is missing:
  - A method signature table.
  - One rule for passing a transaction, e.g. `(ctx, db.DBTX)`.
  - Which tables each module's SQL may touch.
  - `cmd/` added to the depguard allow list.
  - An owner for the archive-warning lookup.
- Sources: IR-B2, IR-M5, IR-M12, Stage 5 N6.

**C3: Define dev/test mode and the e2e harness.**
- What is missing:
  - A localhost-only mode: http, no `Secure`, allowed origins.
  - The exact list of variables the app reads, plus the Compose `environment:` for `app`.
  - The Playwright `webServer` and env.
  - How the setup token is obtained in e2e.
  - The truncate list and the login path in `seed.ts`.
  - `pg` and `@types/pg` in the dependencies.
  - The e2e Postgres pin.
- Sources: IR-B3, CD-B1, CD-M11, IR-M16, Stage 6 #4, Stage 8 M7.

### 🟡 Majors

**Reminder correctness (Goal 3)**

- **C4:** key `come_back` on `(rule_id, appointment_id)`.
  - "before" keeps `starts_at` as its anchor, and the Rescheduling bullet applies to "before" only.
  - Sources: design-2-1, Stage 8 M2.
- **C5:** days-only offsets.
  - One `offset_days` column: `>= 0` for "before", `> 0` for come-back.
  - Drop `offset_hours` and the 2 h example.
  - Add the CHECK on offsets per kind.
  - Sources: design-2-2, Stage 8 M1, design-n10.
- **C6:** archived attributes and archived options render by their last label.
  - Save validation treats archived attributes as existing.
  - Sources: design-2-3, Stage 8 M5.
- **C7:** archived clients produce no items, with a test row.
  - Source: design-2-4.

**PWA and client**

- **C8:** "server off" means one of: a network error, a 5xx, or no `X-App-Version` header.
  - Do not decide by content type.
  - Sources: design-2-5, Stage 8 N3.
- **C9:** the service worker sends `/api/*` navigations to the network.
  - State how the export is triggered.
  - Source: design-2-6.
- **C10:** icons are left out of the precache, sent with `no-cache`, and added to the Cloudflare bypass.
  - Sources: design-2-7, Stage 8 N2, CD-N11.
- **C11:** clear Dexie on logout and on a 401.
  - List offline cached data in Risks.
  - Sources: design-2-8, Stage 8 N9.
- **C12:** re-POST `getSubscription()` on each standalone open.
  - Store the last push result in `scheduler_state`. The line is red when there are no subscriptions, or when the last attempt was delivered to none.
  - Drop `last_ok_at` and the 3-day rule.
  - Set the Web Push TTL and urgency.
  - Sources: design-2-11, Stage 8 M4, IR-N7.

**Backup and restore (data safety)**

- **C13:** fix restore and retention.
  - `restore.sh` takes the backup `flock`.
  - It drops and recreates the DB before `pg_restore`.
  - It ends with `docker compose up -d`.
  - It reads the tag through `backup.sh snapshot-info`.
  - `upgrade.sh` adds `--tag pre-upgrade`, and `forget` gets `--keep-tag pre-upgrade`.
  - `upgrade.sh` refuses to run on a `.env`/running version mismatch.
  - Sources: design-2-9, design-2-10, Stage 8 M6, IR-M8.
- **C14:** define the `status.json` format and its rules for each run outcome.
  - Align the "red within" time.
  - Sources: IR-M7, Stage 6 #10, CD-N10, ST-N20.
- **C15:** fix the migration guard.
  - A fresh DB means goose version 0.
  - Add `BACKUP_STATUS_FILE` and `ALLOW_MIGRATE_WITHOUT_BACKUP` to the config table.
  - Sources: IR-M6, CD-N5, CD-N6, design-n6, Stage 8 N5.

**API contract**

- **C16:** fill the contract gaps.
  - `POST /media` takes `full` + `thumb`, and a `Media` schema is added.
  - Add the PATCH-body table and the GET response/query table.
  - Name the `x-go-type` and nullable extensions.
  - Give the full typed-error table: status codes, Spanish titles, extension fields, and the strict-server error hook.
  - Drop the `device` login field.
  - Re-issue the session cookie when the session slides.
  - Sources: IR-M1–M4, IR-M10, IR-M15, CD-M6, ST-M3, ST-M5, Stage 6 #2.

**Tooling and accuracy**

- **C17:** `fs.Sub` for the goose embed, plus the goose markers.
  - Source: CD-M5.
- **C18:** PG18 volume at `/var/lib/postgresql`.
  - Two reviewers agree on this. Check it before applying.
  - Close the Open Question.
  - Sources: IR-M9, CD-M4, design-n11.
- **C19:** Node/Corepack.
  - Pin Node 24, or install pnpm without Corepack.
  - Reviewer claim, not yet checked.
  - Source: CD-M3.
- **C20:** name the ISO 4217 source and where the exponents come from.
  - Fix the Goal 7 grep exclusions.
  - Source: IR-M13.
- **C21:** specify the `VERSION` derivation, the build arg, and the GHCR login.
  - Source: IR-M14.

**Internal contradictions and doc hygiene**

- **C22:** remove the leftovers.
  - "Three" rule kinds at 899 and 1742.
  - "Come-back time" at 1951.
  - The "Enviados" offline copy at 1928.
  - `expires_at` NULL at 1501.
  - The pin mismatches.
  - The I1 reasoning, which must match D5.1.
  - Define "Hoy" in Terms.
  - Sources: style-2-1, style-2-2, style-2-3, Stage 6 #1, #5, #6, #8, CD-M7, CD-M8, IR-M11, ST-M1, ST-M2, ST-M4, ST-M6, CD-N1, Stage 8 M7.
- **C23:** file moves and links.
  - Rewrite items 14 and 87 to match the disk. `round-1/` already exists, and the round-2 reports are in the root.
  - List every link to rewrite.
  - Decide where the "original SAD" link points.
  - Sources: structural-2-1, Stage 6 #7, CD-M1, CD-M2.
- **C24:** one top-level "Risks" section.
  - Keep the accepted risks in it.
  - Add one pointer line per handled risk.
  - Add the login-delay DoS.
  - Add the rejected alternatives: Tailscale/WireGuard and passkeys.
  - Sources: structural-2-2, design-2-12, Stage 8 M3, Stage 8 N1.
- **C25:** move the mechanics out of the Technical Plan and into the numbered items.
  - Source: Stage 5 M3.

**Scope cuts (proposed by reviewers; these change scope)**

- **C26:** drop the device-sessions screen.
  - It removes `sessions.id`, `GET/DELETE /sessions`, the `Session` schema, and `ajustes/dispositivos`.
  - Sources: Stage 5 M4, Stage 8 N9.
- **C27:** smaller cuts.
  - `COMPOSE_SUBNET` and `ipam`: use the RFC 1918 defaults instead.
  - The VAPID env override and `DELETE /push/subscriptions/{id}`.
  - `users.role`.
  - The 10k delay-map cap.
  - Export paging.
  - Sources: Stage 8 N7, N8, Stage 5 N1–N3, Stage 8 N1.

### 🟢 Nits

The full lists are in each stage report. The groups:
- **Style wording** (Stage 7 style-n4–n23, Stage 9 ST-N8–N24): vague judgments, metaphors, passive voice, filler, insider shorthand.
- **Structure** (structural-n3–n9): solution text inside The Problem, list spacing, uneven rules, NFR placement, anchors, CUJ 8 bullets, the Goal 3 count.
- **Design small gaps** (design-n3–n5, n7–n9): "four services", wizard icon order, media write order, Reactivar overlap (a note only), CSV formula prefix (skip), `unaccent` vs NFD fold.
- **Readiness small gaps** (IR-N3, N8–N19; CD-N2–N4, N7–N9, N12–N14):
  - Schema and types: the shorthand vs DDL rule, sqlc nullable overrides.
  - Settings left unstated: manifest fields, overlap boundary, `scheduler_state` keys, encodings, shutdown, healthchecks, CSV formats, the `/api/v1` mount.
  - Tooling: the `ignore` directive, `x/sys`, the coverage gate in P2.
  - Small correctness points: the `wa.me` default, the iOS push user gesture, `flock`, the sqlc file owner.
- **Comprehension** (#9, #11–#14): EGM term, archived-rule test placement, ADR link column, Goal 8 count, the timer before setup and the `DueList` signature.

---

## Traceability matrix

| Raw finding | → Item |
|---|---|
| zoom-out M1, M2 | Settled (D5.1, D2) |
| zoom-out M3 | C25 |
| zoom-out M4 | C26 |
| zoom-out N1, N2, N3 | C27 |
| zoom-out N4 | C22 |
| zoom-out N5 | Nit (CD-N9) |
| zoom-out N6 | C2 |
| zoom-out N7 | Nit (Stage 5) |
| comprehension #1, #5, #6, #8 | C22 |
| comprehension #2 | C16 |
| comprehension #3 | C1 |
| comprehension #4 | C3 |
| comprehension #7 | C23 |
| comprehension #10 | C14 |
| comprehension #9, #11–#14 | Nit |
| design-2-1 | C4 |
| design-2-2, design-n10 | C5 |
| design-2-3 | C6 |
| design-2-4 | C7 |
| design-2-5 | C8 |
| design-2-6 | C9 |
| design-2-7 | C10 |
| design-2-8 | C11 |
| design-2-9, 2-10 | C13 |
| design-2-11 | C12 |
| design-2-12 | C24 |
| design-n6 | C15 |
| design-n11 | C18 |
| design-n3–n5, n7–n9 | Nit |
| structural-2-1 | C23 |
| structural-2-2 | C24 |
| structural-n3–n9 | Nit |
| style-2-1, 2-2, 2-3 | C22 |
| style-n4–n23 | Nit |
| Stage 8 M1 | C5 |
| Stage 8 M2 | C4 |
| Stage 8 M3 | C24 |
| Stage 8 M4 | C12 |
| Stage 8 M5 | C6 |
| Stage 8 M6 | C13 |
| Stage 8 M7 | C3, C22 |
| Stage 8 N1 | C24, C27 |
| Stage 8 N2 | C10 |
| Stage 8 N3 | C8 |
| Stage 8 N4 | Nit (skip CSV prefix; note on Reactivar) |
| Stage 8 N5 | C15 |
| Stage 8 N6 | Open (depends on D2) |
| Stage 8 N7, N8 | C27 |
| Stage 8 N9 | C11, C26; partly settled |
| IR-B1 | C1 |
| IR-B2 | C2 |
| IR-B3 | C3 |
| IR-M1–M4, M10, M15 | C16 |
| IR-M5, M12 | C2 |
| IR-M6 | C15 |
| IR-M7 | C14 |
| IR-M8 | C13 |
| IR-M9 | C18 |
| IR-M11 | C22 |
| IR-M13 | C20 |
| IR-M14 | C21 |
| IR-M16 | C3 |
| IR-N7 | C12 |
| IR-N1, N2, N4, N5, N6 | C22 / C14 |
| IR-N3, N8–N19 | Nit |
| CD-B1, CD-M11 | C3 |
| CD-M1, M2 | C23 |
| CD-M3 | C19 |
| CD-M4 | C18 |
| CD-M5 | C17 |
| CD-M6 | C16 |
| CD-M7, M8 | C22 |
| CD-M9, M10 | C1 |
| CD-N1 | C22 |
| CD-N5, N6 | C15 |
| CD-N10 | C14 |
| CD-N11 | C10 |
| CD-N2–N4, N7–N9, N12–N14 | Nit |
| ST-M1, M2, M4, M6 | C22 |
| ST-M3, M5 | C16 |
| ST-M7 | C1 |
| ST-N20 | C14 |
| ST-N8–N24 (others) | Nit |

---

## Resolution (2026-11-02)

C1–C5 were resolved with the owner one at a time. The owner then asked the agent to apply its recommendations for C6–C27, including the scope cuts C26 and C27. The edits are in `docs/designs/release-1.md`. The findings above refer to the document **before** these edits; their line numbers no longer match. Nits were not worked through one by one; a few were fixed along the way (noted below).

| ID | Decision / outcome |
| --- | --- |
| C1 | Phases rebuilt so each exit check passes with its own items: web toolchain and placeholder page in P0, P1 spec is `GET /setup/status` plus a throwaway spike with pass/fail rules, setup seeder (no-op in P2, `starter.Seed` in P4), `COVER_PKGS` and CI steps grow per phase. |
| C2 | Module rules: every function takes `(ctx, q db.DBTX, …)`, `db.WithTx`, modules own their structs, write only their own tables, explicit allowed-import list; Module interfaces table (item 32); `reminders.Preview`; archive warning built on the phone; `DueList` = load + pure `compute`. |
| C3 | Config table gets a "Read by" column; the app requires only `PUBLIC_URL`, `DB_PASSWORD`, `VAPID_SUBJECT`. `http://localhost`/`127.0.0.1` accepted (cookie drops `Secure` there); Origin check unchanged, `make dev` sets `PUBLIC_URL` to the Vite address. e2e harness: `webServer` runs the built binary, a `setup` project gets the token from `setup-link` and saves `storageState`, `seed.ts` truncates only the domain tables. `pg`/`@types/pg`, pinned Postgres. |
| C4 | `come_back` identified by `(rule_id, appointment_id)`; `anchor_at` NULL for it, `UNIQUE NULLS NOT DISTINCT`; "before" keeps `starts_at`. Tests added. |
| C5 | Days-only offsets: one `offset_days` (`>= 0` before, `> 0` come-back). `offset_hours` and the 2 h example removed; UI, starter rule, CUJ 4 and tests changed to days. |
| C6 | One archive rule: archived attributes and options render stored values by their last label; save counts archived attributes as existing; the warning is information only. |
| C7 | Archived clients produce no items; candidate loads skip them; test row added. |
| C8 | "Server off" = network error or no `X-App-Version` header; content type not used. Middleware sets the header before any handler. |
| C9 | Service worker lets `/api/*` navigations go to the network; export is an `<a download>` link to `/api/v1/export`. |
| C10 | Icons not precached, `no-cache`, in the Cloudflare bypass; reinstall needed for a new Home Screen icon. |
| C11 | Dexie deleted on logout and on `unauthorized`; listed in Risks. |
| C12 | Re-subscribe check on each open (button when a gesture may be needed); `last_push` result in `scheduler_state`; red line = no subscriptions or last push delivered to none; `last_ok_at` and 3-day rule removed; TTL 8 h, `Urgency: normal`. Timer before setup writes only the heartbeat (comprehension #14). |
| C13 | Restore: `snapshot-info`, stop `app` and `backup`, backup lock, `dropdb`/`createdb` before `pg_restore`, ends with `compose up -d`. Upgrade: version check first, `--tag pre-upgrade`, `forget --keep-tag pre-upgrade`. |
| C14 | `status.json` format and a run-outcome table; red rules incl. stale `checked_at`; "within about an hour" aligned. |
| C15 | Guard condition 2 = goose version above 0; `BACKUP_STATUS_FILE` and `ALLOW_MIGRATE_WITHOUT_BACKUP` in config. |
| C16 | `POST /media` `full` + `thumb` with limits; `Media` and `Option` schemas; PATCH-body and GET tables; `x-go-type` for `attrs` and `nullable-type` for PATCH; full error table with Spanish titles and extension fields; `problem.go` errors and the strict-server error handlers; `device` dropped; cookie re-issued with the hourly `last_seen_at` write. |
| C17 | `fs.Sub(embedFS, "migrations")`, goose markers, a test that the FS has migrations. |
| C18 | Checked in the image Dockerfile: PG 18 has `VOLUME /var/lib/postgresql`, `PGDATA=/var/lib/postgresql/18/docker`. `pgdata` mounted there; Open Question closed. |
| C19 | Checked in the Node.js docs: Corepack is not shipped from Node 25. pnpm via `pnpm/action-setup` in CI and `npm install -g pnpm@<version>` in the Dockerfile. |
| C20 | Currencies from `golang.org/x/text/currency`; sample phone from `phonenumbers.GetExampleNumber`; Goal 7 grep notes. |
| C21 | `VERSION` from `git describe` (Makefile) or the tag without `v` (release); `--build-arg VERSION`; GHCR login with `packages: write`; Dockerfile base images pinned. |
| C22 | "Two kinds" everywhere; "come-back time" removed; "Enviados" online only; `expires_at` contradiction removed; pin mismatches fixed; I1 rationale matches D5.1; "Hoy", "Clientas", "Ajustes" in Terms. |
| C23 | Original SAD moved to `history/sad-original.md`; item 14 has a table of every link to rewrite and a grep check; review rounds go to `round-<N>/`; item 87 matches the disk. ADR index link column described (comprehension #12). |
| C24 | Top-level "Risks" H2: accepted risks (plus the login-delay DoS and cached data after a closed session) and a handled-risks pointer table. Alternatives D3 (Tailscale/WireGuard) and K3 (passkeys) added. |
| C25 | Search clauses, the update handshake, the precache quirk, fstab options and Argon2 parameters moved to items 47, 66, 65, 83 and 38. |
| C26 | Device-sessions screen dropped: no `sessions.id`, `device`, `GET/DELETE /sessions`, `Session` schema or `ajustes/dispositivos`. Lost phone = password change or `reset-password`. |
| C27 | Removed `COMPOSE_SUBNET`/`ipam` (trusted proxies default to private ranges), the VAPID env override, `DELETE /push/subscriptions/{id}`, `users.role`, the 10k delay-map cap, and list/export paging. |
