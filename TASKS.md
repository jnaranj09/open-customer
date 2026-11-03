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

## Done (T1–T8)

Each "Done when" was proven by a command run on this tree; the guards and checks were each seen red
once by breaking what they protect, then restored.

- **T1 Gate (= P0).** `make check-pins lint test vuln build` exits 0 from a clean `web/build`.
  Versions are in `CLAUDE.md` "Versions". `make image` builds and `docker run … version` prints
  `dev`. Dependabot supports `docker-compose` (written into design doc item 10).
- **T2 Hooks.** Registered in `.claude/settings.json`. Proven: a TS and a Go type error fail the edit
  hook; a missing shellcheck fails it naming T1; a failing Go test blocks Stop once, then stands down.
- **T3 CI.** `.github/workflows/ci.yml` runs the gate plus gitleaks and `pnpm audit --audit-level
  high`. **Not run yet:** it first runs after `git init` and a push to GitHub.
- **T4 Strictness.** Strict TS flags, typescript-eslint `strictTypeChecked`, golangci-lint
  `exhaustive`, `wrapcheck`, `nilerr`, `nolintlint`. `$effect` copies: `svelte/prefer-writable-derived`
  (named in `.claude/rules/ui.md`). Suppression pins: 0 `//nolint`, 0 bulk, 0 inline (the real totals).
- **T5 Test rules.** Vitest `allowOnly: false` + `restoreMocks: true`; the Vitest and Playwright
  lint rules; `thelper`, `tparallel`. Proven: `it.only` fails the run, `it.skip` fails lint, a bare
  `t.Skip` fails guard `forbid-only`, `allowOnly: true` fails the web `forbid-only` guard.
- **T6 Guards.** Go in `internal/guards`, web in `web/tests/guards`, all run by `make test`.
- **T7 Decay sensors.** `deadcode -test` (exact `DEADCODE_BUDGET` = 1), `unused`, `dupl` (80), knip,
  jscpd, all in `make lint`. knip and jscpd both read `.svelte` files (checked).
- **T8 Boundary.** Every row of the `.claude/rules/boundaries.md` table names its parse site and its
  bad-input test; `DisallowUnknownFields` is in design doc item 32. The tests themselves land with
  each parse site, in the phase that builds it.

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
- `DEADCODE_BUDGET` is 1 (`clock.Real.Now`, first used by `serve` in P1): lower it to 0 then.
- The image's `CMD ["serve"]` has no `serve` until P1; the image runs `version` only.
- Owner exception recorded: the design doc keeps the 90% `make cover` floor, which the bootstrap
  skill would not use. Kept by owner decision.
- Public repo: the old review reports in `docs/designs/history/review-release-1/round-1/` and the root
  goldfish_brainstorm_findings.md name a home folder path (line 3 / lines 6–7). Strip it before the
  first push.
