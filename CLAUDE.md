# CLAUDE.md

open-customer: a self-hosted app (Go API + SvelteKit PWA) for one small service business to keep
clients, appointments and hand-sent WhatsApp reminders.

<!-- Budget: this file loads in EVERY session. Keep it under 200 lines (guard `agent-context`).
     Only what an agent cannot learn from the code: commands, traps, reasons, bans tied to
     incidents. No overview of folders or architecture. Add a line only after an agent repeats a
     mistake. -->

**Designing or choosing without the owner?** Read `technical-taste.md` first. **Setting up?** Work
through `TASKS.md` in order.

**Architecture source:** `docs/designs/release-1.md` for the app; build it phase by phase (its
"Build order"). `docs/designs/devsecops.md` for the security pipeline; `docs/threat-model.md` lists
every threat and its check. A change that makes a doc false fixes the doc in the same change.

## Never commit

- deploy/.env, or any password, setup link, session token or VAPID private key.
- Real client data: names, phones, photos, database dumps, exports, backups. Tests use made-up data.
- Build output: web/build, bin/, coverage files, `.claude/settings.local.json`.
- This is a public repo: no home paths, personal emails, or employer or internal names in code,
  tests, fixtures or commit messages.

## Commands

All Go commands go through `make`: on a clean checkout web/build is missing and the `go:embed`
fails to compile until `make web-stub` has run. Node must be the pinned one (`make node-check`;
with fnm, `fnm use` in web/).

- `make generate`: oapi-codegen, sqlc (Docker image), openapi-typescript.
- `make check-generated`: `generate`, then fails on any diff.
- `make check-pins`: Postgres (digest included), Node, Go, golangci-lint, gitleaks and cosign pins
  agree across files.
- `make lint`: `go mod tidy -diff`, golangci-lint, `go build` for darwin and windows, deadcode
  (exact `DEADCODE_BUDGET`), shellcheck, svelte-check, eslint, prettier check, knip, jscpd (fails
  on an empty scan).
- `make test`: Go unit tests + Vitest, guards included. `make test-db`: Go tests tagged `db`
  (testcontainers).
- `make cover`: the 90% gate (`COVER_FLOOR`) on the packages in `COVER_PKGS` (`internal/auth`,
  `internal/attributes`, `internal/appointments`, `internal/templates`, `internal/reminders`).
- `make vuln`: govulncheck. `make build`: web build, then `bin/open-customer`. `make image`: the
  app and backup Docker images (`open-customer:$(VERSION)`, `open-customer-backup:$(VERSION)`).
- `make image-scan [MODE=push|release]`: Trivy on the two `make image` images, `PG_IMAGE` and the
  compose cloudflared image (`scripts/security/image-scan.sh`). `push` (default) blocks on our two
  images only; `release` blocks on all four. Needs network (the Trivy database).
- `make secrets`: gitleaks over the whole git history (`.gitleaksignore` honoured). `make test-sec`:
  the scan wrappers', release-sign's and check-pins' tests (`scripts/security/tests/run.sh`, fake
  `docker` and `cosign`).
- `make release-sign IMAGE=<repository>:<tag>`: release workflow only (registry login, cosign,
  OIDC token; not in the gate): push, digest from the push, CycloneDX SBOM (Trivy), then `cosign
  sign` and `cosign attest` on `<repository>@<digest>` (`scripts/security/release-sign.sh`).
- `make config-scan`: Trivy config on the two Dockerfiles (high or critical blocks) and zizmor
  `--offline` on `.github/workflows` (medium or high blocks), both with no network
  (`scripts/security/config-scan.sh`). `deploy/compose.yaml` is checked by the guard
  `TestComposeHardening` instead.
- `make sast`: Semgrep with only the rules written in `security/semgrep` (one file per rule, a
  fixture each in `security/semgrep/testdata`; never a registry rule), no network
  (`scripts/security/sast.sh`): `semgrep --test` on the fixtures, then the scan; any finding blocks.
  `.semgrepignore` lists what is skipped (tests, fixtures).
- `make fuzz [FUZZTIME=60s]`: the four Go fuzz targets (`FuzzDecodeJSON`, `FuzzMultipartUpload`,
  `FuzzParseTemplate`, `FuzzBackupStatusRead`), one after another, each for `FUZZTIME`; any crash
  fails. Manual: not in the gate or CI (`make test` runs their seeds). A crash input lands in the
  package's `testdata/fuzz/<Name>/`: keep it as a seed and fix the code with its own test.
- `make e2e`: build, fresh e2e database, Playwright WebKit: the CUJ specs and `ui-probe` (axe +
  layout, empty, error and busy states on every route at 393 and 1440 px).
- `make test-deploy`: builds the backup image and runs `deploy/tests/backup_test.sh` in it against
  a throwaway Postgres (Docker; no host port; `oc-deploy-*` containers removed on exit), then
  `deploy/tests/scripts_test.sh` (the operator scripts, with a fake `docker`) on the host.
- `make dast`: the running-app scan (`scripts/security/dast.sh`): the app image from `make image`
  and a throwaway Postgres on a private network (`oc-dast-*`, no host port), a made-up owner signed
  in, then ZAP (API scan signed out and signed in, baseline) and Nuclei. Reports in `dast-report/`.
- `make dev`: Docker Postgres + `serve` on :8080 + Vite on :5173.

**Gate (run before saying "done", report the command and its output):**
`make check-pins lint test vuln build check-generated test-db cover test-deploy secrets test-sec config-scan sast`,
plus `make image image-scan dast` for a change to an image, a scan or what the app serves. CI
(`.github/workflows/ci.yml`) also runs the gitleaks action, `pnpm audit --audit-level high` and
`make e2e` (after `make build`) in job `gate`, and `make image image-scan MODE=push dast` (and the
zizmor and Semgrep SARIF uploads) in job `security`.

A check that could not run is reported as NOT RUN, never as passing.

## Load-bearing constraints (precedence order; read the rule file before touching a layer)

1. **Fail-closed boundaries** (`rules/boundaries.md`): every outside input is parsed once, at one
   place; log the error path and a code, never the value.
2. **Never crash past the boundary**: one bad record or request must not take down the rest.
3. **Typed contracts between parts** (`rules/api.md`): one definition of each route, message and
   error code; no literal copies.
4. **State ownership**: one owner per piece of truth; derived values are computed, never stored.
   "Done" and the due list are computed on read; never add a stored copy.
5. **Maintainability** (`rules/maintainability.md`): search before you write; nothing stale
   survives; no silent failure.
6. **Tests** (`rules/testing.md`): never skip, focus, loosen or re-record a test to get green.
7. **Contract first.** api/openapi.yaml is the source of every route and shape. Change the spec,
   run `make generate`, then write the handler. Never hand-edit generated code (Go `gen` and `sqlc`
   output, the web `schema.d.ts`). The spec holds only the current phase's endpoints.
8. **Module import rules.** The allowed imports per module are the list in the design doc's "Code
   layout and module rules". depguard checks the coarse ones; the rest is a review check. Each
   module writes only its own tables.
9. **Transactions are explicit.** Every module function takes `ctx` and a `db.DBTX`, except the
   two in-memory limit checks `auth.BeginLogin` and `auth.BeginPasswordChange`, which run before
   the handler opens its transaction. Every non-GET handler that reads or writes the database runs
   in `db.WithTx` and commits before writing the response. The snapshot readers
   (bootstrap, due list, daily timer, export) use a read-only REPEATABLE READ transaction.
10. **Time is injected.** Modules take the clock from the platform clock package and the business
    time zone from settings; no direct `time.Now()` in a module.
11. **Language.** Code, comments and docs are English. Every UI string is Spanish and lives in the
    web `es.json`; no inline Spanish in components.

**Debugging?** Read `rules/troubleshooting.md` first (Step-0 questions before theorizing).

## Enforcement

- Hooks (`.claude/hooks/`): PostToolUse runs `check-edited.sh` (lint + types on the edited file);
  Stop runs `run-tests-on-stop.sh` (red tests block the turn once). Registered in
  `.claude/settings.json`.
- Strict types: Go compiler + golangci-lint (`errcheck`, `exhaustive`, `wrapcheck`, `nilerr`,
  `nolintlint`);
  TypeScript `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`,
  `noImplicitOverride`, `noFallthroughCasesInSwitch`, `noImplicitReturns`; typescript-eslint
  `strictTypeChecked`. Never loosen a flag; fix the consumer.
- Guard tests in `internal/guards` (Go: agent-context, nolint budget, catch-blocks, test-hygiene,
  forbid-only, threat-model, security-allow-list for `security/allow.json`) and `web/tests/guards`
  (Vitest: layers, catch-blocks, test-hygiene, forbid-only, raw-colors, design-tokens,
  design-parity, ui-lint, eslint suppression budget, bulk file and inline comments). Allow-lists
  only shrink.
- Lint suppressions: a pinned total that only falls (guard `suppression-budget`, `svelte-ignore`
  included).
- UI pipeline (`rules/ui.md`): tokens and components follow `design/soft-glass.pen` (guards
  `design-tokens`, `design-parity`); only `web/src/lib/components/ui/` may use surface and radius
  classes, and no file may use an arbitrary `-[…]` value (eslint `better-tailwindcss`). Runtime:
  `web/tests/e2e/ui-probe.spec.ts` (axe, layout probe, data states, every route); its allow-list
  `ui-probe.allow.ts` only shrinks.
- Coverage: `make cover` keeps a 90% floor on the gated packages (owner decision). Never write a
  test only to raise the number; mutation testing in the clean-up pass checks the tests are real.
- `code-reviewer` subagent: a fresh reviewer re-runs the gate and fault-injects every guard.

## Versions (don't bump in passing)

Pinned at P0, each in the one place the design doc's "Pins" table names. A bump changes every
copy in one commit, and `make check-pins` must stay green. Every image is pinned
`<tag>@sha256:<index digest>` and every Action by its full commit SHA with `# vX.Y.Z` (guard
`TestSupplyChainPins`): a bump changes the tag and the digest (or SHA and comment) together.

- Go 1.27.3: `go.mod` (`go 1.27.0` + `toolchain go1.27.3`); `golang:` in the `Dockerfile`.
- Node 26.13.0 and pnpm 12.16.1: `web/package.json` (`engines.node`, `packageManager`); `node:` and
  `pnpm@` in the `Dockerfile`.
- Postgres 18.6: the `postgres` stage of `security/tools.Dockerfile` (with its digest), read into
  `PG_IMAGE` by the `Makefile`; `deploy/compose.yaml`,
  `deploy/backup/Dockerfile` and the CI service repeat it, digest included.
- golangci-lint v2.15.0: `GOLANGCI_LINT_VERSION` in the `Makefile`; `version:` in CI.
- sqlc 1.31.1: the `sqlc` stage of `security/tools.Dockerfile`, read into `SQLC_IMAGE`.
- The scanner images (Trivy, gitleaks, zizmor, ZAP, Nuclei, curl, Semgrep): one stage each in
  `security/tools.Dockerfile`, read into `<NAME>_IMAGE` by the `Makefile`; Dependabot bumps them.
- cosign v3.1.4: `COSIGN_IMAGE` in `deploy/scripts/lib.sh` (deployers lack the `Makefile`) and
  `cosign-release` in `release.yml`; Dependabot reads neither, so bump both by hand.
- Go tools (oapi-codegen, govulncheck, deadcode): `tool` lines in `go.mod`.
- TypeScript stays on 6.0.x: typescript-eslint and SvelteKit do not accept 7 yet.
