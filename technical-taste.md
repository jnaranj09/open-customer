# Technical taste — open-customer

This file is for AI agents that design and build this project. It tells you how decisions are made
here, so you can decide the same way without asking.

It is read on demand (before designing or choosing), not loaded every session. `CLAUDE.md` and
`.claude/rules/*.md` are binding and are not repeated here; this file gives the *why* and the
defaults they leave open. `docs/designs/release-1.md` holds the architecture decisions themselves.

**This file is never finished.** When a rule here conflicts with what the code and its tests show
today, or with a better practice learned since, say so and update the rule (a reversal goes in
"Reversals").

## How to use this file

**Precedence (highest first):**

1. The prompt of your run.
2. `CLAUDE.md` and `.claude/rules/*.md`. Guard tests enforce many of them.
3. This file.
4. The house pattern: the closest existing code that already does the same job.

**When experts split, pick the option a sensor can enforce** (a type, a lint rule, a guard test, a
hook). A rule that turns something red holds up under agents; a paragraph does not.

**Write your choice down.** In a design note or commit, mark a choice the owner did not make as an
agent decision with a one-line reason, so the owner can challenge it.

## Engineering canon

The research behind these rules: experts disagree about code *style* and agree about the *harness*.
Agents fail by volume (more code, more near-copies, more swallowed errors, less refactoring), so most
rules are checks and budgets. Grades: **Evidence** (an empirical study supports it), **Consensus**
(most experts and vendors agree), **Taste** (experts split; a side was picked). **Sensor** = what goes
red when the rule is broken; "prose only" = nothing yet. Prefer turning prose-only rules into sensors.

1. **Gate before done.** Every change passes compile, lint, test and build, run by the agent; report
   the command and its output. Evidence. Sensor: the edit and Stop hooks, the `make` gate, CI.
2. **Fault-inject every guard.** Break the protected code, see the test go red, restore; mutation
   testing in the clean-up pass. Evidence. Sensor: reviewer step 2; gremlins and StrykerJS.
3. **Never cheat a red test.** No skip, focus, loosen or re-record to get green. Consensus. Sensor:
   guard `forbid-only`, the Vitest/Playwright settings and lint rules; deleting is prose only.
4. **Parse untrusted input once, at one boundary**, failing closed; log path + code, never the
   value. Consensus. Sensor: Go typed decode + `DisallowUnknownFields` + `Validate()`; web zod.
5. **Strict types, never loosened**; major upgrades one at a time. Evidence / Taste for flags.
   Sensor: Go compiler + golangci-lint; TS strict flags via svelte-check + typescript-eslint.
6. **Small steps; tidy apart from behaviour.** Evidence / Consensus. Sensor: prose only.
7. **Parallel agents only in isolation** (separate worktrees). Taste. Sensor: prose only.
8. **Every recurring rule becomes a sensor**, named next to the rule. Consensus. Sensor: guard
   `agent-context`.
9. **Enforce the layers.** Consensus. Sensor: depguard (Go module rules); guard `layers` (web).
10. **Short always-loaded context.** `CLAUDE.md` under 200 lines, no overview; rules load by path.
    Evidence. Sensor: guard `agent-context`.
11. **State as tagged unions, checked exhaustively.** Consensus. Sensor: golangci-lint `exhaustive`;
    typescript-eslint `switch-exhaustiveness-check`.
12. **No silent catch.** A contained error logs a stable `code=`. Consensus. Sensor: guard
    `catch-blocks`; `errcheck`, `nilerr`.
13. **Search before you write; no early abstraction.** Consensus. Sensor: `dupl` (Go); jscpd (web).
14. **Scheduled clean-up pass** before each release. Consensus. Sensor: `deadcode`, `unused`, knip,
    `dupl`, jscpd; the pass itself is prose only (`TASKS.md` T9).
15. **Split by entanglement, not line count.** Evidence / Consensus. Sensor: prose only.
16. **"Why" comments stay.** Consensus. Sensor: prose only.
17. **Tests test behaviour**: public seam, mocks only at process edges, injected time, no snapshots
    of agent output. Consensus. Sensor: `@vitest/eslint-plugin`; guard `test-hygiene`.
18. **Fresh-context review** that re-runs the gate and fault-injects. Evidence. Sensor:
    `.claude/agents/code-reviewer.md`.
19. **Lint suppressions only shrink.** Taste. Sensor: guard `suppression-budget` (`//nolint` count,
    ESLint bulk suppressions); `nolintlint` requires a reason.
20. **Nothing stale survives**, including the design doc. Consensus. Sensor: guard `agent-context`
    (doc-paths); `deadcode`, knip; reviewer.
21. **A failure is never silent**: it reaches her screen, an exit code or an HTTP status. Consensus.
    Sensor: prose only + per-surface tests.
22. **Security basics for the shape**: OWASP for the web app, secrets never logged or committed,
    dependency audit. Consensus. Sensor: govulncheck, gosec, `pnpm audit`, gitleaks in CI.
23. **Plain over clever.** Concrete module types, an interface only where a test needs a fake.
    Taste. Sensor: prose only.

**Owner exception:** the design doc keeps a 90% coverage floor (`make cover`) on the gated packages,
against the usual "no coverage target" rule. Coverage proves lines ran, not that tests check them, so
mutation testing still runs in the clean-up pass.

## Budgets that keep agent decay in check

- **A scheduled clean-up pass** before each release: duplicates, dead code, slow tests, mutation
  testing on core modules. Behaviour-neutral, own commits, full gate green (`TASKS.md` T9).
- **The always-loaded context stays small**: `CLAUDE.md` ≤ 200 lines, rule files load by path.
- **Suppression and allow-lists only shrink.**

## Sensors

Which tool is the sensor for each row in this repo (filled at setup; update when a tool changes).

| Row | Go | Web (SvelteKit / TS) |
| --- | --- | --- |
| `types` | `go build`, `go vet` | svelte-check with the strict TS flags |
| `lint` | golangci-lint v2: `govet`, `staticcheck`, `errcheck`, `gosec`, `revive`, `depguard`, `exhaustive`, `wrapcheck`, `nilerr`, `nolintlint`, `unused`, `dupl` | eslint + typescript-eslint `strictTypeChecked`, eslint-plugin-svelte |
| `format` | gofmt | prettier + prettier-plugin-svelte |
| `tests` | `go test` (unit), `-tags db` with testcontainers | Vitest; Playwright WebKit (e2e, from P6) |
| `test-lint` | `thelper`, `tparallel`; branching in tests is prose only | `@vitest/eslint-plugin`, `eslint-plugin-playwright` |
| `forbid-only-skip` | guard `forbid-only` (`t.Skip`) | Vitest `allowOnly: false`, guard `forbid-only` (also Playwright `forbidOnly: true` once its config lands in P6) |
| `dead-code` | `deadcode -test` (exact budget in the Makefile), `unused` | knip (reads `.svelte`) |
| `duplication` | `dupl` (threshold 80) | jscpd (reads `.svelte`; tests excluded) |
| `boundary-schema` | typed structs + `DisallowUnknownFields` + `Validate()` | zod |
| `mutation` | gremlins (`README.md` release checklist, never in the gate) | StrykerJS (same) |
| `security` | govulncheck, gosec, gitleaks in CI | `pnpm audit --audit-level high` and gitleaks in CI |
| `suppression-budget` | `//nolint` count, guard `suppression-budget` | `eslint-suppressions.json` total and inline `eslint-disable` / `@ts-*` count, guard `suppression-budget` |
| shell (`scripts/`, `deploy/`, hooks) | shellcheck in `make lint` | — |

## Decision order (owner fills)

What to weigh first when this file is silent (e.g. safety before function; smallest fix that tells
the truth; copy the house pattern).

## Stop and ask (owner fills)

Choices an agent must not make alone (e.g. anything sent off the machine; weakening a security or
strict-types setting; anything that can lose user data; changing what a status or API field means).

## Product taste (owner fills)

## Visual taste (owner fills)

## Reversals: old → current (do not revive the old side)

## Rejected — don't propose these again
