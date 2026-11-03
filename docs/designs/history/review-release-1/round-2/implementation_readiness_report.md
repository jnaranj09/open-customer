# Implementation Readiness Report: Stage 9 (round 2)

Target: `docs/designs/release-1.md`, DetailedRange lines 1114–2088.
Reviewers (fresh contexts, direct dispatch, all completed):
- `egm-implementation-readiness` (IR), with implementation-readiness-standard
- `egm-codebase-details` (CD), with implementation-readiness-standard
- `egm-style` (ST), with no standard

Verdict: **NOT READY.** 3 Blockers and 24 Majors after merging duplicates.
- The file list and the test cases are thorough.
- The failures are in four places:
  - the phase order;
  - the Go API between modules, including how a transaction is passed;
  - the dev/e2e runtime setup;
  - API contract gaps.

---

## 🔴 Blockers

**R-B1: phase exit checks cannot pass with the items in that phase.** (IR-B1, CD-M9, CD-M10)
- Scope: lines 1139–1150, 1178, 1183, 1187, 1243–1251, 1422–1430, 1628.
- P0 `make lint test vuln` needs `web/package.json`, which is item 62 in P5.
- P1 `check-generated` runs openapi-typescript, which also needs P5.
- The P1 spec content is undefined: line 646 keeps health out of `openapi.yaml`, yet P1 lists "15–16 (health only)".
- The multipart/binary spike is placed "at P1 start", but media is in P3 and export in P4.
- Item 8 CI runs `cover`, `e2e` and `check-generated` from P0.
- P2 does not compile:
  - `auth.CompleteSetup` → `starter.Seed` calls the `create` functions of templates, reminders and attributes, which are in P3/P4.
  - `serve.go` (P1) calls push `EnsureKeys` (P4), `auth.NewSetupToken` (P2) and `scheduler.Run` (P4). `status.Status()` (P1) needs P4 data.
- Fix:
  - Make the Makefile and CI phase-aware: the web and generate steps skip until `web/` exists, or a minimal `web/package.json` goes into P0.
  - Say which CI steps each phase adds.
  - Move starter seeding to P4, or move the create functions earlier.
  - Say which `serve.go` steps exist per phase.
  - Define the P1 spec and the phase of the spike.

**R-B2: no module Go interface, and no convention for passing a transaction.** (IR-B2, IR-M5, IR-M12; Stage 5 Nit 6)
- Scope: lines 320–321, 1210, 1472, 1514, 1532, 1538, 1603, 1628, 1650, 1743.
- Line 320 promises one Go interface per module, but no method signature is given anywhere.
- Bootstrap (REPEATABLE READ), `starter.Seed(tx, …)` and the exporter all need a caller's transaction to reach the module functions. `WithTx` does not say how that happens: `*sqlc.Queries`, `pgx.Tx`, `DBTX`, or a value in `ctx`.
- It is not said which tables each module's `.sql` file may read. `reminders` reads the tables of other modules.
- depguard bans `cmd/open-customer` from importing `reminders`, but `serve.go` wires the modules, so lint fails.
- The attribute-archive warning (`TemplatesUsing`) has no owner module and no endpoint.
- Fix:
  - Write the interface table with every method signature.
  - Fix one transaction convention, e.g. every method takes `(ctx, db.DBTX)`.
  - List which tables each module may touch.
  - Allow `cmd/` in depguard.
  - Pick the server or the client side for the archive warning.

**R-B3: dev and e2e cannot run under the stated config and security rules, and the e2e harness is unspecified.** (IR-B3, CD-B1, CD-M11, IR-M16; Stage 6 #4)
- Scope: lines 562–563, 742–756, 1189, 1251, 1445, 1817, 1970–1987.
- These rules together break `make dev` (Vite on `http://localhost`) and CI e2e over plain HTTP:
  - `PUBLIC_URL` must be `https`;
  - `Origin` must equal `PUBLIC_URL`;
  - the cookie is `Secure`.
- WebKit may also drop `Secure` cookies on `http://localhost` (unverified).
- Required variables include `TUNNEL_TOKEN`, `RESTIC_PASSWORD` and `BACKUP_TARGET`, which the app never reads.
- The e2e harness is unspecified:
  - the `webServer` command, port and env;
  - how `setup.spec` gets the setup token (stdout only);
  - which tables `seed.ts` truncates;
  - how the other specs log in (an Argon2id hash made outside Go?);
  - `pg` and `@types/pg` are missing from item 62;
  - the e2e Postgres image is not pinned.
- Fix:
  - A defined dev/test mode, e.g. `DEV_INSECURE=true` for localhost only: allows http, drops `Secure`, sets the allowed origins. Or HTTPS for dev and e2e.
  - The exact list of app variables, and the Compose `environment:` block.
  - Full `playwright.config.ts` and seed rules.

---

## 🟡 Majors

### API contract

| ID | Source | Lines | Finding | Fix |
|---|---|---|---|---|
| R-M1 | IR-M1, CD-M6, ST-M3 | 579, 1401, 1708, 1879 | `POST /media` parts: `file`, or `full` + `thumb`. No `Media` schema. Size limit is per part or per request? | `full`+`thumb`; add `Media`; state the limit. |
| R-M2 | IR-M2 | 630–642, 1381–1403 | No PATCH bodies. Missing GET response and query shapes: `/push/key`, `/sessions`, per-client lists, `/appointments?from&to`, `/reminders/log?date`, `/bootstrap` params, the option-create response, DELETE responses. Which fields may change: rule `kind`/`service_id`? `timezone`/`currency`? | Add a PATCH-body table and a GET table. |
| R-M3 | IR-M3 | 1349, 1359, 1410 | Absent vs `null` in PATCH, and `attrs` as `json.RawMessage`, need `x-go-type` / `nullable-type` in oapi-codegen. Item 16 sets neither. | Name the extensions and the config. |
| R-M4 | IR-M4, ST-M5 | 1361, 1465 | The typed errors leave out 401/403/410/413/415/429. `PreconditionFailed` has no status (If-Match was cut). `Conflict` cannot carry `appointments[]`/`clients[]`. The strict-server error hook is not named. The Spanish titles table is missing. | Full table; Problem extension fields; name `ResponseErrorHandlerFunc`. |
| R-M5 | IR-M15 | 1387, 1595 | The login body has `device`, but sessions read it from `User-Agent`. | Remove one. |
| R-M6 | IR-M10 | 562, 1593 | Sliding 90-day expiry, but nothing re-issues `Set-Cookie`, so the phone's cookie still dies at 90 days. | The middleware re-issues the cookie, together with the `last_seen_at` write. |

### Ops, backup and config

| ID | Source | Lines | Finding | Fix |
|---|---|---|---|---|
| R-M7 | IR-M6, CD-N5, CD-N6 | 1477, 1482 | Guard reads `users` before it exists. Status path is hard-coded. `ALLOW_MIGRATE_WITHOUT_BACKUP` is not in the config table. | Fresh DB = no table, or goose version 0. Add `BACKUP_STATUS_FILE` and the override to item 20 and the table. |
| R-M8 | IR-M7 | 678, 706, 1800, 2041 | `status.json` format is undefined: time format, clearing `last_error`, what a skipped run writes. A stale "marker missing" stays red. | Give a JSON example plus rules for each outcome. |
| R-M9 | IR-M8 | 2040, 2053 | `restore.sh` cannot read the `app=` tag or resolve `latest`. | `backup.sh snapshot-info <id\|latest>` that prints JSON. |
| R-M10 | IR-M9, CD-M4 | 126, 687, 2002 | PG18 data path left open. Both reviewers say the volume goes at `/var/lib/postgresql` (`PGDATA=/var/lib/postgresql/18/docker`). | Check, then state the path and close the Open Question. |
| R-M11 | CD-M5 | 1477 | goose Provider gets an `embed.FS` rooted above `migrations/`, so it finds no migrations. | `fs.Sub(embedFS, "migrations")`; require the `-- +goose Up` markers. |
| R-M12 | CD-M3 | 1119, 1128 | `corepack enable` may not exist on the newest Node LTS. Reviewer: Corepack was dropped from Node 25+ (unverified release). | Pin Node 24, or install pnpm with `pnpm/action-setup` / `npm i -g`. |
| R-M13 | IR-M13 | 1639, 2080 | No source for the ISO 4217 list or the exponents. A hard-coded list (`CRC`) and the `+506` sample fail the Goal 7 grep. | Name the source (e.g. `golang.org/x/text/currency`). Fix the grep exclusions. |
| R-M14 | IR-M14 | 1236, 1256, 1826 | `VERSION` derivation, `v1.2.3` vs `1.2.3`, `--build-arg VERSION`, GHCR login and `packages: write` are all undefined. | Specify each. |

### Internal contradictions

| ID | Source | Lines | Finding |
|---|---|---|---|
| R-M15 | CD-M7, ST-M2 | 1742 | "three kinds" of rules; there are two. Item 55 should also state the kind/service_id CHECK that Go must mirror. |
| R-M16 | IR-M11, CD-M8, ST-M6 | 1951 | "come-back time" setting has no column, field or behavior. |
| R-M17 | ST-M1 | 1501 | `expires_at` is listed as both NULL and NOT NULL. |
| R-M18 | ST-M4, CD-N1 | 1120, 1133, 1135, 1234, 2030 | The Pins rule (`18.<minor>`, distroless digest) disagrees with the item text (`18-alpine`, tag only). |
| R-M19 | ST-M7 | 1350 | The spike has no pass/fail rule ("if it does not fit"). |
| R-M20 | CD-M1 | 1334–1337, 2057 | Items 14 and 87 do not match the disk. The round-1 reports are already in `docs/designs/history/review-release-1/round-1/`. Two root files that item 87 deletes no longer exist. The round-2 reports now sit in the root. |
| R-M21 | CD-M2 | 6, 47, 126, 793, 1094–1096, 1337 | Item 14 rewrites only some links. The Predecessors, Open Questions and Alternatives links break. The "original SAD" link would point at the new index. |

---

## 🟢 Nits

| Source | Lines | Issue |
|---|---|---|
| IR-N3 | 1502 | Shorthand vs explicit DDL: `body` NULL, non-uuid PKs, `position` `int` vs `smallint` (CD-N4). Say explicit DDL wins. Give the FK targets. |
| IR-N6 | 1865, 1928 | "Enviados" offline cached copy: no Dexie table. |
| IR-N7 | 1772 | Web Push `TTL` and urgency not set. Apple may drop the push. |
| IR-N8 | 1640 | Manifest has no `display`, `start_url`, `scope` or `background_color`. |
| IR-N9 | 1691 | Overlap boundary: use `start < other.end AND end > other.start`. |
| IR-N10 | 1779–1787 | `scheduler_state` keys and formats not given; use `>= push_hour`; the "record date even when count 0" test is missing. |
| IR-N11 | 1592, 1602 | Cookie and setup token: encoding and length not given. |
| IR-N12 | 1430 | Graceful-shutdown timeout and stop order not given. |
| IR-N13 | 1999 | `depends_on` graph and healthcheck intervals not given. |
| IR-N14 | 1348, 1546 | `X-App-Version` is set both in middleware and in the spec. Pick one. |
| IR-N15 | 1792 | CSV date/phone formats and the ZIP filename not given. |
| IR-N16 | 1803 | "shows unknown" does not map to a `Status` field. |
| IR-N17 | 1801 | `/client-errors` rate limit numbers not given. |
| IR-N19 | 1350 | How the strict server mounts at `/api/v1` (`BaseURL` vs `servers:`). |
| CD-N2 | 1192 | Go 1.25 `ignore` directive in `go.mod` could replace the `GO_PKGS` grep. |
| CD-N3 | 1800 | `golang.org/x/sys` missing from the item 1 deps. |
| CD-N4 | 1215–1226 | No sqlc overrides for nullable `date` and `jsonb`. `smallint` → `int16`. |
| CD-N7 | 484, 1743 | `DueList(now)` vs `DueList(ctx, now)`. |
| CD-N8 | 1745 | Name the default link (`https://wa.me/`) until the iPhone test. |
| CD-N9 | 1146–1147, 1188, 2070 | `internal/auth` coverage gate not in P2's exit check. `make cover` method not given. |
| CD-N10 / ST-N20 | 678, 2078 | "about an hour" vs "within 2 hours". |
| CD-N11 | 1843 | Precached manifest and icons go stale after the wizard. |
| CD-N12 | 1880 | iOS push permission needs a user tap. |
| CD-N13 | 1210, 1236 | depguard rules and the `go build` path are given only as intent. |
| CD-N14 | 2038 | `flock` in Alpine (busybox or `util-linux`). `docker run` sqlc writes root-owned files; pass `-u`. |
| ST-N8 | 1275 | "state the real gap": the gap itself is not named. |
| ST-N9 | 1751 | "(the SAD bug)" is insider shorthand. |
| ST-N10 | 1349 | "awkward union type" is subjective. |
| ST-N11 | 1557 | "Serves files." repeats the line before it. |
| ST-N12 | 1648 | "reserved names" are not listed. |
| ST-N13 | 1893 | "right input per type": give the type → input mapping. |
| ST-N14 | 1752 | Pronoun with an unclear subject. |
| ST-N15 | 1785 | "A late start": say whose start. |
| ST-N16 | 1858 | "update flow from 'API'": unclear reference. |
| ST-N17 | 1873 | "review check": say "enforced in code review only". |
| ST-N18 | 1977 | "is not checked": say "has not been verified". |
| ST-N19 | 2068 | "proves the stub works" claims more than the check shows. |
| ST-N21 | 1268, 1301, 1303, 1332 | SAD index rules stated twice. |
| ST-N22 | 1298 | Heading says "moved from", but nothing is moved. |
| ST-N23 | 1736 | Reminder test case listed in the templates tests. |
| ST-N24 | 1163 | "which is handled": passive voice, no owner named. |

## Checked by CD and found accurate (no finding)

- **Go and DB tooling:**
  - Go `tool` lines;
  - `setup-go`/`setup-node` reading their files;
  - pgx `OpenDBFromPool`;
  - oapi-codegen v2 keys;
  - the strict server's multipart and `io.Reader` support;
  - 3.0.3 being needed;
  - the PG17+ `search_path` reason for `f_unaccent`;
  - Argon2id parameters;
  - `phonenumbers.GetSupportedRegions`.
- **Web:**
  - SvelteKit `version.name` and `$service-worker`;
  - `adapter-static` `fallback`;
  - Tailwind v4 plugin order and `@theme inline`;
  - openapi-fetch throwing on a non-JSON 2xx.
- **Ops:**
  - Compose `rslave`, `profiles` and `ipam`;
  - restic `cat config`, `--tag` and `forget --prune`;
  - empty named volumes copying ownership from the image;
  - `America/Mexico_City` having no DST since 2022.

Unverified by CD:
- Playwright WebKit with an active service worker;
- Safari `imageOrientation: 'from-image'`;
- WebKit `Secure` cookies on localhost.
