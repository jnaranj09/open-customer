# CLAUDE.md

open-customer: a self-hosted app (Go API + SvelteKit PWA) for one small service business to keep
clients, appointments and hand-sent WhatsApp reminders.

<!-- Budget: this file loads in EVERY session. Keep it under 200 lines (guard `agent-context`).
     Only what an agent cannot learn from the code: commands, traps, reasons, bans tied to incidents.
     No overview of folders or architecture. Add a line only after an agent repeats a mistake. -->

**Designing or choosing without the owner?** Read `technical-taste.md` first. **Setting up?** Work
through `TASKS.md` in order.

**Architecture source:** `docs/designs/release-1.md` is the only one. Build it phase by phase (its
"Build order"). A change that makes the doc false fixes the doc in the same change.

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
- `make check-pins`: Postgres, Node, Go and golangci-lint pins agree across files.
- `make lint`: `go mod tidy -diff`, golangci-lint, deadcode (exact `DEADCODE_BUDGET`), shellcheck,
  svelte-check, eslint, prettier check, knip, jscpd.
- `make test`: Go unit tests + Vitest, guards included. `make test-db`: Go tests tagged `db`
  (testcontainers).
- `make cover`: the 90% gate (`COVER_FLOOR`) on the packages in `COVER_PKGS` (`internal/auth`).
- `make vuln`: govulncheck. `make build`: web build, then `bin/open-customer`. `make image`: the
  app Docker image.
- `make e2e`: build, fresh e2e database, Playwright WebKit (from P6).
- `make dev`: Docker Postgres + `serve` on :8080 + Vite on :5173.

**Gate (run before saying "done", report the command and its output):**
`make check-pins lint test vuln build check-generated test-db cover`. CI
(`.github/workflows/ci.yml`) also runs gitleaks and `pnpm audit --audit-level high`.

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
9. **Transactions are explicit.** Every module function takes `ctx` and a `db.DBTX`. Every non-GET
   handler runs in `db.WithTx` and commits before writing the response. The snapshot readers
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
  forbid-only) and `web/tests/guards` (Vitest: layers, catch-blocks, test-hygiene, forbid-only,
  eslint suppression budget, bulk file and inline comments). Allow-lists only shrink.
- Lint suppressions: a pinned total that only falls (guard `suppression-budget`).
- Coverage: `make cover` keeps a 90% floor on the gated packages (owner decision). Never write a
  test only to raise the number; mutation testing in the clean-up pass checks the tests are real.
- `code-reviewer` subagent: a fresh reviewer re-runs the gate and fault-injects every guard.

## Versions (don't bump in passing)

Pinned at P0, each in the one place the design doc's "Pins" table names. A bump changes every
copy in one commit, and `make check-pins` must stay green.

- Go 1.27.3: `go.mod` (`go 1.27.0` + `toolchain go1.27.3`); `golang:` in the `Dockerfile`.
- Node 26.13.0 and pnpm 12.16.1: `web/package.json` (`engines.node`, `packageManager`); `node:` and
  `pnpm@` in the `Dockerfile`.
- Postgres 18.6: `PG_IMAGE` in the `Makefile`.
- golangci-lint v2.15.0: `GOLANGCI_LINT_VERSION` in the `Makefile`; `version:` in CI.
- sqlc 1.31.1: `SQLC_IMAGE` in the `Makefile`.
- Go tools (oapi-codegen, govulncheck, deadcode): `tool` lines in `go.mod`.
- TypeScript stays on 6.0.x: typescript-eslint and SvelteKit do not accept 7 yet.
