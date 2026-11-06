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

## Done (T1–T8, P1 exit, P2 exit, P3 exit, P4 exit, P5 exit, P6 exit, P7 exit)

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
  NOT RUN: the release workflow and `upgrade.sh` end to end (both need a `v*` tag; D6); Verification
  Plan checks 1-4 and 7-9 on her iPhone through the tunnel; a real removable-disk unplug and a
  `nofail` boot (not used yet: D8); the Ley 8968 check. The owner repeats check 5 on the home PC
  before real data (deploy/README.md section 8).

## T9 Clean-up pass (checklist written; first run before the first release tag)

The checklist is `README.md` "Release checklist"; StrykerJS's work folders are in `.gitignore`
(design doc item 4).

- Done when: the checklist has been run once, before the first release tag.

## T10 Skills (done)

Installed at user level: `security-audit` from github.com/cloudflare/security-audit-skill (owner's
choice), at `~/.agents/skills/security-audit`, symlinked into `~/.claude/skills`. Its two Node
validator scripts were read first: file reads only, no network or shell calls. Run the full audit
only on an explicit request; it starts many agents.

## Found, not fixed (owner decides)

App bugs and rule breaks seen during setup. Setup tasks never fix app code; each line becomes its
own change.

- No app code existed before P0, so nothing was found in old code.
- `pnpm audit` reports 1 low and 1 moderate advisory in dev dependencies (below the `high` gate).
- This PC's default Node (fnm) is 24; the pin is 26.13.0. `make` fails fast (`node-check`) and the
  hooks switch with fnm. Run `fnm default 26.13.0` if you want it everywhere.
- TypeScript is held at 6.0.x: typescript-eslint (`<6.1`) and SvelteKit 3 (`^6`) do not accept 7.
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
  critical, and more); found by `make image-scan` (devsecops). Third-party: it only reports on a
  push, and blocks a release and the weekly rescan until upstream rebuilds it or the owner
  allow-lists the CVEs. Our backup image removes its copy of `gosu`.
- `cloudflare/cloudflared:2026.10.7` has CVE-2026-78661 (golang.org/x/net, fixed in 0.62.0); found
  by `make image-scan`. It only reports on a push, and blocks a release and the weekly rescan until
  a cloudflared tag with the fix is pinned.
