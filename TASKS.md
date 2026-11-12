# Setup tasks

Work through these in order. Each task ends with its "Done when" line proven by a command you ran and
its real output. Remove tasks that do not fit this repo's shape or have nothing left to do (T10
with every skill already installed); never leave one empty.

Rules for every task:

- Keep the tools the repo already uses. Add missing rules to them; never swap a tool.
- Do not change app behaviour inside a setup task. Old violations go into a suppression budget, not a
  bulk fix.
- Every guard is seen red once (break → red → restore) before it counts as done.
- **The design doc stays true.** A task that adds a tool, a target, a config line or a folder
  updates the matching item in `docs/designs/release-1.md` in the same change (Makefile → item 2,
  `.gitignore` → item 4, golangci config → item 5, CI → item 8, web package and config → items
  62–63, guards and harness files → item 88).

## Done (T1–T8, P1 exit, P2 exit, P3 exit, P4 exit, P5 exit, P6 exit, P7 exit, Security pipeline exit)

Each "Done when" was proven by a command run on this tree; the guards and checks were each seen red
once by breaking what they protect, then restored.

- **T1 Gate (= P0).** `make check-pins lint test vuln build` exits 0 from a clean `web/build`.
  Versions are in `CLAUDE.md` "Versions". `make image` builds and `docker run … version` prints
  `dev`. Dependabot supports `docker-compose` (written into design doc item 10).
- **T2 Hooks.** Registered in `.claude/settings.json`. Proven: a TS and a Go type error fail the
  edit hook; a missing shellcheck fails it naming T1; a failing Go test blocks Stop once, then
  stands down.
- **T3 CI.** `.github/workflows/ci.yml` runs the gate plus gitleaks and `pnpm audit --audit-level
  high`. First run green on the first push (run 41729156000): every step passed, which is the P0
  exit check ("CI green").
- **T4 Strictness.** Strict TS flags, typescript-eslint `strictTypeChecked`, golangci-lint
  `exhaustive`, `wrapcheck`, `nilerr`, `nolintlint`. `$effect` copies:
  `svelte/prefer-writable-derived` (named in `.claude/rules/ui.md`). Suppression pins: 0 `//nolint`,
  0 bulk, 0 inline (the real totals).
- **T5 Test rules.** Vitest `allowOnly: false` + `restoreMocks: true`; the Vitest and Playwright
  lint rules; `thelper`, `tparallel`. Proven: `it.only` fails the run, `it.skip` fails lint, a bare
  `t.Skip` fails guard `forbid-only`, `allowOnly: true` fails the web `forbid-only` guard.
- **T6 Guards.** Go in `internal/guards`, web in `web/tests/guards`, all run by `make test`.
- **T7 Decay sensors.** `deadcode -test` (exact `DEADCODE_BUDGET`, 0 since P1), `unused`, `dupl`
  (80), knip, jscpd, all in `make lint`. knip reads `.svelte` files (checked). jscpd is set to read
  them, but no `.svelte` file reaches its `minTokens` yet, so that is not shown. jscpd has
  `failOnEmpty`, so a fully empty scan fails instead of passing (found in P2 task 18b).
- **T8 Boundary.** Every row of the `.claude/rules/boundaries.md` table names its parse site and its
  bad-input test; `DisallowUnknownFields` is in design doc item 32. The tests themselves land with
  each parse site, in the phase that builds it.
- **P1 exit.** `make check-pins lint test vuln build check-generated test-db` exits 0, each target
  run once from a fresh `git clone` of branch p1-p2 at commit 1b1c7c0 (the final review fix wave;
  the commit after it changes only this file). The `serve` and `curl` checks were run at the P1
  exit itself, not re-run in the fix wave: the built binary in an isolated container network
  namespace, not `go run` on the host (host :8080 belongs to an unrelated container), gave
  `/readyz` 200 and `{"needs_setup":true}` on `/api/v1/setup/status`, also after a restart. CI
  green on PR #1 at 32eda09 (run 41764577606). NOT RUN: the plain-host :8080 variant.
- **P2 exit.** The same clone run at 1b1c7c0 also ran `make cover` (internal/auth 96.4%, floor 90%),
  exit 0. The setup, login, logout, settings, manifest, icon and admin-subcommand `curl` checks were
  run at the P2 exit, before the fix wave, the same way (built binary, isolated network namespace),
  and not re-run since. CI green on PR #1 at 32eda09, including `make cover` (run 41764577606).
  NOT RUN: the plain-host :8080 variant.
- **P3 exit.** `make check-pins lint test vuln build check-generated test-db cover` exits 0, run as
  one command from a fresh `git clone` of branch p3 at d31ef86. `make cover`: internal/auth 96.3%,
  internal/attributes 93.5%, internal/appointments 98.4%, internal/templates 98.8% (floor 90.0%).
  The Goal 7 grep (`Costa|CRC|America/Costa_Rica|+506|Uña|Nails`) over `internal/*.go` and `*.sql`
  has no hit outside `internal/locale/es.go` and `_test.go` files. The catalog `curl` checks were
  run on the image built from that clone (`open-customer:d31ef86`), in an isolated container network
  namespace with `curlimages/curl:8.22.0` (host :8080 belongs to an unrelated container): setup
  (Europe/Madrid, ES), attributes and select option, service (color lowercased), client (E.164
  phone), duplicate phone 409, appointments (end time and total, 409 overlap, `allow_overlap`,
  back to back), status 422 / 200 / 409 `invalid-transition`, day list (3 items in order), bad UUID
  422, media upload (client taken from the appointment) and GET headers, templates (201 and 422 on
  an unknown variable), 401 without a cookie. All matched. After that, an owner change (a
  `service_ids` edit keeps the saved price of services already on a visit) landed at 946683e; the
  gate passed on it locally, and the clone run and `curl` checks were not re-run. CI green on PR #2
  at 946683e, including `make cover` (run 41913102949). NOT RUN: the plain-host :8080 variant.
- **P4 exit.** `make check-pins lint test vuln build check-generated test-db cover` exits 0, run as
  one command from a fresh `git clone` of branch p4 at ebd2a21. `make cover`: internal/auth 96.3%,
  internal/attributes 93.5%, internal/appointments 98.4%, internal/templates 98.1%,
  internal/reminders 98.5% (floor 90.0%). `TestDueListCUJ4AndCUJ5` and
  `TestGetRemindersDueReturnsCUJ4And5Items` PASS under the `db` tag. The Goal 7 grep
  (`Costa|CRC|America/Costa_Rica|+506|Uña|Nails`) over `internal/*.go` and `*.sql` has no hit
  outside `internal/locale/es.go` and `_test.go` files. The `curl` checks were run on the image
  built from that clone (`open-customer:ebd2a21`), in an isolated container network namespace with
  `curlimages/curl:8.22.0` (host :8080 belongs to an unrelated container): setup (Europe/Madrid,
  ES), services, templates, preview (`unknown_vars`), rules (`applies_from` set for before, null for
  come back; 422 on `service_id` for a before rule), three clients, appointments, `GET
  /reminders/due` (3 items: Ana "before" with a `https://wa.me/34` link, Marta come-back message,
  Lucía `booking_prompt` for Pedicure), PATCH of Lucía's future booking (`ends_at` +75 min; her item
  gone), `POST /reminders/log` 201 then 200, `GET /reminders/log` (1 item; 422 without `date`),
  `GET /push/key` without a cookie (87 characters), `POST /push/subscriptions` 201 then 200, `GET
  /status` (`migration_version` 2, `backup.red` true, 1 subscription, heartbeat age under 120 s),
  `GET /bootstrap` (every key, `due` = Marta's come-back; 422 on a range over the limit), `GET
  /export` (dated file name; the four files in the zip), `POST /client-errors` 204 x10 then 429
  with `Retry-After`, 401 without a cookie, and `docker logs` has no line matching "Ana Rojas", "Marta", "Lucía" or `wa.me`.
  All matched. CI green on main at e6c19ff (the p4 merge), including gitleaks, `pnpm audit` and
  `make cover` (run 41953355128). NOT RUN: the plain-host :8080 variant.
- **P5 exit.** `make check-pins lint test vuln build check-generated test-db cover` exits 0, run as
  one command from a fresh `git clone` of branch p5 at e24db48. `make cover`: internal/auth 96.3%,
  internal/attributes 93.5%, internal/appointments 98.4%, internal/templates 98.1%,
  internal/reminders 98.5% (floor 90.0%). The image built from that clone (`open-customer:e24db48`)
  ran with `postgres:18.6-alpine` on a private network, published on `127.0.0.1:18080` with
  `PUBLIC_URL=http://localhost:18080` (host :8080 belongs to an unrelated container). Browser:
  Playwright Chromium (pinned `@playwright/test`) with the "iPhone 15" device profile at 390 x 844
  and a desktop profile at 1440 x 900, from a scratch script that is not committed. WebKit was
  installed but cannot start on this host (the browser process exits 127, probably missing system libraries),
  so WebKit is NOT RUN. Made-up data: Ana Rojas, Marta Solano, Lucía Rojas, Ána López; services
  Acrílico, Relleno, Pedicure. Results, compact (390 x 844) and wide (1440 x 900):
  CUJ 1 setup wizard (9 steps, install steps shown for an iPhone user agent, icon skipped) and
  login: PASS compact; the wide size ran login and the setup card only. CUJ 2 services, a client
  and an appointment attribute (with options), two templates written with chips (live preview),
  a before rule and a come-back rule: PASS compact (wide shows the same screens, checked as
  screenshots). CUJ 3 booking Acrílico and Pedicure with Forma, end time 16:30, overlap sheet with
  "Cancelar" and "Agendar igual", a photo on the appointment and in the client gallery: PASS
  compact; wide form checked as a screenshot. CUJ 4 "Enviar", the wa.me link with the rendered
  text, "¿Lo enviaste a …?" and "Sí, lo envié" (Ana, then Marta after the BottomSheet portal
  change; the dialog is a child of `body`): PASS compact; PASS wide (new-tab link, then the
  question, then "Sí"). CUJ 5 a later "Pedicure" booking gives the booking prompt, "Agregar a la
  cita" adds "Relleno" (price 45,00 € = 25 + 20) and the item goes; a cancelled later booking
  brings the message back: PASS compact; the wide pane shows the prompt with its three actions.
  CUJ 6 server stopped, reload: banner "Sin conexión con el sistema. Datos de las HH:MM.", cached
  Hoy, "+" faded with its reason, the draft restored, "Enviar", "Omitir" and "Agregar a la cita"
  disabled with the reason, after start and a focus event the banner is gone and the buttons work:
  PASS at both sizes. Screenshots of Hoy, Para enviar, Nueva cita, Cita, Clientas, Clienta,
  Ajustes, Plantillas, Recordatorios, Estado, Login and Setup were compared with the mockup frames
  (compact) and D2 (wide): no gap. At 1440 the back chevron is absent on `clientas/nueva`;
  `h1` computes to "Fraunces Variable" and the body to "Nunito Variable", both loaded. `docker logs`
  has no client name, phone or `wa.me` line and no ERROR line. NOT RUN: real Safari on an iPhone
  (install, "Activar notificaciones" and the push prompt, the real WhatsApp hand-off), WebKit (see
  above), the plain-host :8080 variant. After the owner's walk through the running app, 35c8980
  fixed field borders, hover tints, the update prompt on the auth screens, the accent on the icon,
  shadows, glow and the login screen, one font (Nunito: `h1` now computes to "Nunito Variable";
  Fraunces is removed) and the full-day grid; the gate exits 0 again with the same `make cover`
  numbers. CI green on main at 6df35a9 (the p5 merge), including gitleaks, `pnpm audit` and
  `make cover` (run 42078630269).
- **P6 exit.** CI run 42104952848 on branch p6 at 4dadd29: every step green, including `make e2e`:
  17 passed (Playwright WebKit, "iPhone 15" profile, against the built binary and a
  `postgres:18.6-alpine` service container). The specs and the CUJs they cover: `setup.spec.ts`
  (CUJ 1 and 2, plus "the setup link does not work after setup"), `booking.spec.ts` (CUJ 3: two
  services and the summed end time, the overlap sheet with "Agendar igual", a photo with made-up
  Exif GPS stored without Exif), `reminders.spec.ts` (CUJ 4: the wa.me link with the rendered
  message, "Sí" → "Enviados" and still there after a reload; CUJ 5: due 21 days after the visit,
  hidden by a later Relleno booking until it is cancelled, the booking prompt with "Agregar a la
  cita", "No vino"), `offline.spec.ts` (CUJ 6: cached Hoy with the banner, the read-only due list,
  a draft kept), `export.spec.ts` (CUJ 8: the ZIP with `clientas.csv`, `citas.csv`,
  `servicios.csv`, `datos.json` and the attribute column named by label). App defects the e2e runs
  found and fixed on this branch: 6100e2d (the new-attribute sheet stayed open after creating a
  text attribute) and 2cb607d + 4dadd29 (a sent item came back with "Enviar" after a reload before
  the refresh was stored; the hidden ids are now stored in the phone cache). Earlier red CI runs:
  42097441087 (`offline.spec.ts` waited for a live answer, not the stored cache; fixed in the
  test, 736e686) and 42099685797 (the sent-item defect above). Local runs use a git-ignored script
  that runs the pinned `mcr.microsoft.com/playwright:v1.64.1-noble` image on its own Docker
  network, because WebKit lacks system libraries on the author's host and host :8080 is taken.
  NOT RUN: WebKit through `make e2e` on the author's host; real Safari on an iPhone; the install
  step and push notifications; the cold open with the server off through the service worker
  (manual check in the Verification Plan); CUJ 7 (P7).
- **P7 exit** (2026-11-06). CI run 42275161287 on branch p7 at 1bce327: gate success. Two earlier
  runs on p7 failed and were fixed: 42274188479 (gitleaks `generic-api-key` false positive on the
  variable name `argonIDKey`; fixed by a `.gitleaksignore` entry in f0b9f76) and 42274446831
  (shellcheck SC2015 on six `A && B || C` lines: the CI runner's shellcheck is older than 0.11;
  fixed in 1bce327 with `if` statements). Ran: `make check-pins lint test vuln build
  check-generated test-db cover test-deploy` exits 0 on a fresh clone of p7 (and again on the tree
  after the new-volumes guard fix); `make test-deploy`; the restore drill on the dev PC with locally
  built 0.0.0-drill images and no cloudflared (D6): check 5 (setup, client, appointment and photo
  through the API, backup, `down -v`, `restore.sh latest`, old login works, photo bytes identical,
  new photo upload works), check 6 simulated (marker removed: `backup-now.sh` exits non-zero,
  "Disco de respaldo no visible", status line red, nothing written) and the Goal 7 grep (check 10).
  NOT RUN: the release workflow and `upgrade.sh` end to end (both need a `v*` tag; D6); the
  release workflow's keyless cosign signature and SBOM attestation (`make release-sign`, added by
  the security pipeline): there is no OIDC token outside Actions, so the first `v*` tag is its
  test, and `upgrade.sh` to that release is the first verify of a real signature; Verification
  Plan checks 1-4 and 7-9 on her iPhone through the tunnel; a real removable-disk unplug and a
  `nofail` boot (not used yet: D8); the Ley 8968 check. The owner repeats check 5 on the home PC
  before real data (deploy/README.md section 8).
- **Security pipeline exit** (2026-11-07), branch devsecops (`docs/designs/devsecops.md`, built).
  CI: PR #4 run 42389556805 and push run 42389512897, `gate` (with `make e2e`) and `security` green. Ran locally: `make check-pins
  lint test vuln build check-generated test-db cover test-deploy secrets test-sec config-scan sast`
  and `make image image-scan dast`: all 16 targets (the 13 of the gate list plus those 3) exit 0.
  Every new guard and wrapper was seen red once by breaking what it protects (per task).
  `docs/threat-model.md` has no `gap:` row and its guard refuses the word (seen red on a probe).
  NOT RUN: `upgrade.sh`, `restore.sh` and `pull.sh` end to end against a real release on a
  deployed PC; `make scan-live` against a real deployment (the deployer's own); the weekly workflow's
  first scheduled run, its fuzz job included; `make e2e` (CI runs it); Dependabot digest-only
  updates for base images (seen only after a push); a real Cloudflare tunnel under the hardened
  compose. After the merge (PR #4, 53fc19e), branch protection on `main` and the `v*` tag ruleset
  were applied and read back on 2026-11-07. First release v0.1.0 (2026-11-07, run 42402380559):
  `release-on-main.sh` passed in a real Actions checkout, both images were pushed, keyless-signed
  and given their CycloneDX attestation. The ghcr packages were public with no change (they follow
  the public repo). Logged out of ghcr, both images pulled, `verify_image` from deploy/scripts/lib.sh
  returned the signed digests, and `cosign verify-attestation --type cyclonedx` passed. The
  first try of v0.1.0 stopped at the release scan (two new zlib CVEs in cloudflared, allow-listed
  in PR #9) before anything was pushed; its tag was deleted and recreated on 89e0c04.

## T9 Clean-up pass (checklist written; first run before the first release tag)

The checklist is `README.md` "Release checklist"; StrykerJS's work folders are in `.gitignore`
(design doc item 4).

- Done when: the checklist has been run once, before the first release tag.

## T10 Skills (done)

Installed at user level: `security-audit` from github.com/cloudflare/security-audit-skill (owner's
choice), at `~/.agents/skills/security-audit`, symlinked into `~/.claude/skills`. Its two Node
validator scripts were read first: file reads only, no network or shell calls. Run the full audit
only on an explicit request; it starts many agents.

## Quick quotes S4 (convert; owner's check NOT RUN)

Stage S4 of `docs/designs/quick-quotes.md`: "Convertir en cita" turns the open quote into the Nueva
cita draft. Ran: the unit tests for `handOff`, `make e2e-docker` with CUJ Q2, CUJ Q3, the
failed write and `convert works offline`, the CLAUDE.md gate, and `make image image-scan dast`.
NOT RUN: the owner's 15-second check on the phone (quote a price, Copiar, paste, then Convertir en
cita and finish the booking); CI on the branch.

## Next appointment (built; owner's check NOT RUN)

Stages S1 and S2 of `docs/designs/next-appointment.md`: the button "Agendar próxima cita" on an
appointment opens a sheet of date choices from the come-back rules and hands the picked one to
Nueva cita. Ran: the unit tests of `nextVisit` and `handOff`, `make e2e-docker` with CUJ N1 to N5
and the sheet in `ui-probe`, the CLAUDE.md gate, fault injection on `nextVisit`, the UI review at
393 × 852 and 1440 × 900, and the `code-reviewer` pass. NOT RUN: the owner's check on the phone
(open an appointment, tap "Agendar próxima cita", pick a date, save, read the toast); CI on the
branch; `make image image-scan dast` (only the web bundle changed).

Left for the owner from the code review (small, not fixed):

- An appointment opened directly in a new tab: after the round trip through Nueva cita its back
  arrow is `history.back()` with nothing behind it (it leaves the app). Desktop or new tab only.
- The wording "En 21 días · 3 dic" counts from the visit. On a visit of 21 days ago it reads "En 21
  días · <today>". A wording call ("A los 21 días · 3 dic").

## Found, not fixed (owner decides)

App bugs and rule breaks seen during setup. Setup tasks never fix app code; each line becomes its
own change.

- No app code existed before P0, so nothing was found in old code.
- `pnpm audit` reported 1 low and 1 moderate advisory in dev dependencies (below the `high` gate);
  on 2026-11-07 they were still there (deepmerge GHSA-gcqx-w48m-w2h4, low, no fix; minimatch
  GHSA-g3hv-75wg-m249, moderate, fixed in 11.0.1). `make deps-scan` replaced `pnpm audit` in CI and
  reports neither (Trivy sees the minimatch one as MEDIUM, below its gate).
- This PC's default Node (fnm) is 24; the pin is 26.13.0. `make` fails fast (`node-check`) and the
  hooks switch with fnm. Run `fnm default 26.13.0` if you want it everywhere.
- TypeScript is held at 6.0.x: typescript-eslint (`<6.1`) and SvelteKit 3 (`^6`) do not accept 7.
- Muted text on a bottom sheet can fall under the 4.5:1 contrast of WCAG AA. The sheet is glass
  (`glass-panel`, 78% white) over the dimmed page, so the colour behind the text depends on what
  the page has there. Seen on 2026-11-12: a `ListRow` subtitle (`text-ink-muted`) in the next
  appointment sheet at 1440 px measured 4.02:1 over a dark button of the page (axe, `ui-probe`).
  That sheet now uses buttons with their own fill, but the `TextField` hint and a disabled
  button's reason use the same ink in every sheet, and axe reports them as "could not be
  determined" there, not as passing. The fix is in the `BottomSheet` surface or the `ink-muted`
  token, through `design/soft-glass.pen`.
- Owner exception recorded: the design doc keeps the 90% `make cover` floor, which the bootstrap
  skill would not use. Kept by owner decision.
- `make vuln` lists 1 known vulnerability in golang.org/x/crypto (`openpgp`, GO-2026-5932) that our
  code does not call (govulncheck passes). kin-openapi went to v0.149.0 when the image scan found
  CVE-2026-100353 and CVE-2026-100354 in the app binary.
- `scripts/check-pins.sh` accepts a non-numeric postgres tag (`postgres:latest` passed); found
  before P7, not fixed in P7.
- shellcheck is not pinned: CI uses the runner's version and the local gate whatever is installed,
  so the two can disagree (seen on p7: SC2015 only in CI).
- `postgres:18.6-alpine` ships `gosu` built with Go 1.24.6, with Go stdlib CVEs (CVE-2025-68121
  critical, and more); found by `make image-scan` (devsecops). Allow-listed until 2027-05-06 (owner
  decision 2026-11-07): `deploy/compose.yaml` runs Postgres as `user: postgres`, so the image
  entrypoint never runs `gosu`. Each of the 26 entries in `security/allow.json` is scoped to
  `usr/local/bin/gosu`, so the same CVE in our own binaries still blocks. Our backup image removes
  its copy of `gosu`. On or before 2027-05-06: bump the pinned Postgres image if it ships a rebuilt
  `gosu` and delete the entries; otherwise ask the owner for a new date. An expired entry turns the
  release and the weekly rescan red again.
- `cloudflare/cloudflared:2026.10.7` has three findings, found by `make image-scan`, allow-listed
  until 2027-02-05 (owner decisions 2026-11-07): CVE-2026-78661 (golang.org/x/net, fixed in
  0.62.0), scoped to `usr/local/bin/cloudflared`; and CVE-2026-76844 and CVE-2026-85091 (HIGH, in
  the Debian `zlib1g` package), scoped to the image (`@image:cloudflare/cloudflared`), so the same
  CVEs in our own images still block. cloudflared is a Go binary and Go ships its own
  `compress/zlib`, so it most likely does not use the system zlib (not checked). 2026.10.7 is the
  newest tag and no rebuilt one exists. Delete the three entries when Dependabot brings a fixed or
  rebuilt tag; if none exists by 2027-02-05, ask the owner for a new date. An expired entry turns
  the release and the weekly rescan red again.
- `make dast` loads 6519 Nuclei templates (v10.5.1) and 262 of them do not run: Nuclei reports
  them as templates with a runtime error, and dast.sh prints "262 templates did not run" but stays
  green (seen 2026-11-07). Not investigated: which templates, and whether any of them would apply
  to this app (`nuclei -validate` lists them). A silent loss of coverage until someone looks.
- `api/openapi.yaml` has no `maxLength` on any text field (security audit A8). The modules already
  refuse long text (client name 120 runes, other names and labels 80, notes and template body
  2000, a text attribute value 500), but the contract does not say so and no web input has a
  `maxlength`, so she learns the limit only from the 422. The owner picks the limits for the spec
  and the Spanish message the form shows.
