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

## Done (T1–T8, P1 exit, P2 exit, P3 exit)

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
- Public repo: the old review reports in `docs/designs/history/review-release-1/round-1/` and the
  root goldfish_brainstorm_findings.md name a home folder path (line 3 / lines 6–7). Strip it
  before the first push.
