# Implementation Readiness Report: Stage 9 (round 4)

Target: `docs/designs/release-1.md`, DetailedRange lines 1197–2308.

Reviewers (fresh contexts, direct dispatch), all completed:
- `egm-implementation-readiness` (implementation-readiness-standard)
- `egm-codebase-details` (no code on disk)
- `egm-style` (no standard)

Verdict: no Blockers. Ready to start P0–P2. 21 Majors and 48 Nits must be fixed before the phase that builds each part.

Overlaps:
- IR-M1 = CD-M1 (restic lock on a read-only mount);
- IR-M3 = CD-M5 (`/index.html` 301);
- IR-N1 = CD-M7 (`make generate` in P0);
- IR-N2 = CD-N2 (`check-generated`);
- IR-N3 = CD-N10 (dev proxy);
- ST-M1 = CD-N7 (Postgres pin count);
- IR-N15 = ST-N8 ("<owner>").

---

### Implementation readiness (Stage 9): 0 Blocker, 11 Major, 17 Nit. Verdict: ready to start (P0–P2 can begin), with conditions.

Majors (the phase each one blocks is in brackets):
- **IR-M1 [P7]:** the `restore` service mounts `/target` read-only, but `restic snapshots` and `restore` write lock files. Fix: `--no-lock` (check against the pinned version), or mount it read-write.
- **IR-M2 [P7]:** restored `/media` may end up owned by root, so `app` (65532) can't write new photos. Fix: `chown -R 65532:65532 /media` after the restore; add "upload a photo after the restore" to the drill.
- **IR-M3 [P5]:** with `http.FileServer`, `/index.html` answers a 301, so the SW caches a redirect and the offline cold open breaks. Fix: serve `/index.html` with a 200 (`ServeContent`/`ServeFileFS`), and add a test.
- **IR-M4 [P3/P5]:** `Appointment` has no `client_name`, and archived clients aren't cached. Fix: add `client_name` from a join.
- **IR-M5 [P3]:** photo ownership rules: the server fills `client_id` from the appointment, a mismatch → 422; appointment detail filters the client's media by `appointment_id`.
- **IR-M6 [P3]:** `Validate` doesn't say what happens to unknown keys, wrong-scope keys, writes to archived attributes, archived option IDs, or `""` for required. Fix: 422s plus tests.
- **IR-M7 [P5]:** the "¿Lo enviaste?" pending state has no storage. Fix: Dexie `meta` `{item_id, client_name, started_at}`; show the sheet on open if it is under 30 min old (same as design-4-2).
- **IR-M8 [P5]:** the wizard's time zone, currency and country lists have no source. Fix: `GET /setup/options` from the Go sources, labels via `Intl.DisplayNames('es')`; or state the `Intl` approach plus a fallback.
- **IR-M9 [P2]:** username rules: lowercase and trim, 3–32 chars of `[a-z0-9._-]`, `autocapitalize="off"` (same as design-n1).
- **IR-M10 [P6]:** Playwright parallel workers would break the shared DB. Fix: `workers: 1`, `fullyParallel: false`.
- **IR-M11 [P5]:** there is no screen to change the password at will or to log out, yet the lost-phone procedure depends on a password change. Fix: Ajustes root lists "Cambiar contraseña" and "Cerrar sesión".

Nits:
- IR-N1: `make generate` can't work in P0 (no spec, no queries).
- IR-N2: `check-generated` should use `git status --porcelain`.
- IR-N3: the dev proxy must also forward the manifest and icon paths.
- IR-N4: an unknown variable has no label to name. Show the key.
- IR-N5: a wrong current password must be 422 `validation`, not 401 (which would log her out).
- IR-N6: the `{{appointment.services}}` joiner; the `wa.me` encoding must be `%20`, not `+`.
- IR-N7: `IsDone(status, endsAt, now)`; `SampleData(locale, countryCode)`; `locale` importable by any module.
- IR-N8: preview goes into the spec in P4.
- IR-N9: the setup and cookie token encodings; `server_keys` names; branding file names.
- IR-N10: CSV value vocabulary (`estado`, `archivada`, money).
- IR-N11: `position` on create and on reorder.
- IR-N12: `GenerateKey` character rules must match the tokenizer.
- IR-N13: list order within groups; badge update and clear (same as design-4-3).
- IR-N14: `from`/`to` are inclusive, matched on the local date of `starts_at`.
- IR-N15: `backup_contact_name` = "<owner>" is a hard-coded install value.
- IR-N16: rules pointing at archived templates or services; the Reactivar overlap check; `service_ids` with an archived service.
- IR-N17: layout redirects when offline; `/setup` without a token.

### Technical accuracy (Stage 9): 0 Blocker, 7 Major, 16 Nit

Majors:
- **CD-M1:** the restic lock fails on the read-only `/target` (same as IR-M1). Fix: global `--no-lock` on `snapshots` and `restore`.
- **CD-M2:** restic groups snapshots by hostname, and every `compose run` and every recreated container gets a new one. Old groups are never pruned, so "the last 14 dailies" is broken, and each new host forces a full rescan. Fix: `hostname: open-customer-backup` on `backup`/`restore` (or `--host`), plus `forget --group-by paths`.
- **CD-M3:** jq `fromdateiso8601` accepts only `YYYY-MM-DDTHH:MM:SSZ`. Fix: every `status.json` time is UTC in exactly that format (`date -u +%Y-%m-%dT%H:%M:%SZ`), plus a Go parse test.
- **CD-M4:** `set -euo pipefail` exits before step 6, so `status.json` is never written on failure. In loop mode the container exits, and `restart: unless-stopped` hammers the broken disk. Fix: catch each step's failure (or an ERR trap) and write the status; never exit in loop mode.
- **CD-M5:** `http.FileServer` 301s `/index.html`, so the SW caches a redirect (same as IR-M3).
- **CD-M6:** Dependabot bumps pins that other pins must match. Postgres in compose and the Dockerfile drifts from `PG_IMAGE`, the tests and CI. `node:`/`golang:` drift from `engines.node`/`toolchain`. Fix: `ignore:` these in Dependabot and bump them by hand together, or read them from an `ARG`.
- **CD-M7:** `make generate` can't work in P0. Say `generate`/`check-generated` start in P1.

Nits:
- CD-N1: sqlc pointer override in object form (not confirmed).
- CD-N2: `check-generated` misses untracked files.
- CD-N3: Go 1.25 `ignore ./web/node_modules` directive in `go.mod`.
- CD-N4: add `svelte-kit sync` before `svelte-check` on a clean clone.
- CD-N5: signature mismatches: the `settings.Create` return type (wrap it in a closure in `cmd`), `starter.Seed` without `ctx`, `Record(LogEntry)` vs positional args.
- CD-N6: `clients` has no way to get `country_code`. The handler must pass it.
- CD-N7: the Postgres pin list, the CI grep step, and how the tests read `PG_IMAGE`.
- CD-N8: the Dependabot entries for `/deploy/*` must be added in P7.
- CD-N9: the `docker-compose` hedge can likely go (not confirmed).
- CD-N10: the dev proxy must also forward the manifest and icons.
- CD-N11: the CUJ 5 e2e seed (22 days, 21-day rule) lands on a late item. Seed 21 days, or use `include_overdue: true`.
- CD-N12: `playwright.config.ts` must split `E2E_DATABASE_URL` into the `DB_*` parts.
- CD-N13: the Goal 7 grep `[Nn]ail` matches "thumbnail". Use `\bnails?\b`.
- CD-N14: the goose CLI `tool` line pulls in every DB driver. Drop it, or keep it only for `goose create`.
- CD-N15: restic and jq versions follow Alpine. Add a row to Pins.
- CD-N16: check distroless debian13, webpush-go maintenance and the double `mailto:` risk at P0 (not confirmed).

Checked and found accurate (summary): Go `tool` lines; sqlc needs cgo; `NewV7`; the goose Provider API; `DBTX`; oapi-codegen `HandlerWithOptions` and the strict options; headers in the spec get `Set` by the generated code; the PG 18 paths; the SvelteKit service-worker facts; Tailwind v4; openapi-fetch `onResponse`; the Playwright features; iOS push and badge; `/usr/local/bin` on the distroless PATH; volume ownership copy; Compose `rslave` and `profiles`; busybox has `flock`; restic in Alpine community; Corepack; the GitHub Actions inputs; items 1–87 with no gaps; every cross-reference; phase lists vs item notes; items 14 and 87 match the disk; the link table covers every `../../` link.

### Style (Stage 9): 0 Blocker, 3 Major, 15 Nit

**ST-M1 (Major): the Postgres pin locations don't match** (1205 says five places; the Pins row at 1216 lists three). Fix: list all five, name `PG_IMAGE` as the source, and say how CI greps.
**ST-M2 (Major): the migration guard rule is a fragment, and all-vs-any is unclear** (1659). Fix: a numbered list: build the Provider; if `HasPending` AND version > 0 AND `last_copy_at` is stale, return an error unless overridden; call `Up`.
**ST-M3 (Major): the "Clean checkout" claim contradicts the CI step order** (2288 vs 1335–1344). `make test-db` comes after `make build` in the CI table.

Nits:
- ST-N1 (1211): the pnpm cell holds an explanation, and the Node.js docs citation is vague.
- ST-N2 (1205): "Most live in one place" is vague.
- ST-N3 (1400–1404): the ADR table sentences repeat; "its section here" is unclear.
- ST-N4 (1278): the `dev` cell is overloaded. Move the env and the Origin note into a bullet.
- ST-N5 (1753): "Serves files." says nothing.
- ST-N6 (1857–1859): a stray blank line splits the item 47 list.
- ST-N7 (1485, 1561, 2075, …): "she/her" in technical text. Use "the owner".
- ST-N8 (2129): "avisa a <owner>" is hardcoded in the spec text. Use `{backup_contact_name}`, with the default in `es.json`.
- ST-N9 (2172): "1 días" is wrong; the singular is "1 día".
- ST-N10 (2020): two possible DST test zones. Name one.
- ST-N11 (2298): "within about an hour" → "by the next hourly check (at most 60 min)".
- ST-N12 (1356): the Dependabot conditional has no place to record its outcome.
- ST-N13 (1866): "with CR" → "with region `CR`".
- ST-N14 (2284–2292): bold label colons are placed inconsistently.
- ST-N15 (1232): the P1 Items cell is hard to read.
