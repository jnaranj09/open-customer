---
paths:
  - '**/*_test.go'
  - 'internal/guards/**'
  - 'web/src/**/*.test.ts'
  - 'web/tests/**'
  - 'web/playwright.config.ts'
  - 'web/vite.config.ts'
---

# Rule: tests

Tests are first-class code. This file holds the judgment part; the rest is enforced. Each rule names
its sensor, or says "prose only".

## What a test proves

- **Behaviour through the public seam, not the internals.** Go: test the module's exported methods
  (with a real database for `db`-tagged tests); web: import the shipped module and assert what a
  caller sees. Prose only.
- **One behaviour per test.** The title (Go: the subtest name) states the behaviour and the
  condition ("a cancelled appointment makes no come-back item"). Sensor: `@vitest/eslint-plugin`
  `valid-title` (web); Go prose only.
- **No branching in a test, and every test asserts.** Table-driven Go tests are fine; the loop body
  has no `if` on the case. Sensor: `@vitest/eslint-plugin` `no-conditional-in-test`,
  `no-conditional-expect`, `expect-expect` (web); Go prose only (`thelper`, `tparallel` help).
- **No snapshot tests of agent-written output.** An agent re-records a snapshot instead of reading
  the diff. Sensor: `@vitest/eslint-plugin` `no-restricted-matchers` for snapshot matchers (web); Go
  has no snapshot library, prose only.
- **The 90% coverage floor is an owner exception, not a goal.** `make cover` keeps it on the gated
  packages. Never write a test only to raise the number; a test must fail when the behaviour it
  names breaks. Sensor: `make cover` for the floor; mutation testing (gremlins) in the clean-up pass
  for test quality.

## Fuzz tests

- **Every parser of outside input has a `Fuzz` test** (JSON body, multipart upload, template, backup
  status file), beside the package it drives, and its property is "never panic, never a 5xx", plus
  what that parser promises. Seeds: the bad inputs of the boundary tests as `f.Add`, and crashing
  inputs kept in `testdata/fuzz/<Name>/`. `make test` runs the seeds; `make fuzz` (not in the gate;
  CI runs it weekly, `security-weekly.yml` job `fuzz`) runs the fuzzer. A crash is a real bug: keep
  the input, fix the code with its own test. Prose only (`TestThreatModel` checks the `test:Fuzz...` references exist).

## Mocks, time, files

- **Mock only at process edges**: network, filesystem, clock, OS, other processes. Go: a fake only
  behind an interface at an edge (the push sender for the scheduler tests); the database is real
  (testcontainers). Web: `fake-indexeddb` and a fetch stub are edges. Sensor: guard `test-hygiene`.
- **Inject time; no real sleeps.** Go: the fake clock from the platform clock package, never
  `time.Sleep` in a test. Web: `vi.useFakeTimers()` is undone by `vi.useRealTimers()` in the same
  file. Sensor: guard `test-hygiene`.
- **A test that makes a temp folder removes it.** Go: `t.TempDir()`, never `os.MkdirTemp`. Sensor:
  guard `test-hygiene`.
- **A test file stays under 1000 lines**; split along its groups. Sensor: guard `test-hygiene`.

## Red first, never cheat

- **Every new test or guard is seen red once**, by breaking the code it protects, then restored.
  Prose only (the `code-reviewer` agent repeats it).
- **Never skip, focus, delete, loosen or re-record a test to get green.** Fix the code, or report the
  removal as a decision in the commit. The one allowed skip is a platform or privilege the test cannot
  run under, written as a conditional skip. Database tests are kept apart by the `db` build tag, not
  by a skip. Sensor: guard `forbid-only` (Go `t.Skip` outside a platform condition; Vitest
  `allowOnly: false`; Playwright `forbidOnly: true`), `@vitest/eslint-plugin` `no-focused-tests` and
  `no-disabled-tests`, `eslint-plugin-playwright` `no-skipped-test`; deleting or loosening is prose
  only.
- **A shared test helper never serves one caller.** Repetition inside a test body is fine. Prose only.

## Which layer

- **Unit tests for logic; integration tests where two real parts meet; e2e only for what crosses
  processes** (browser ↔ server, app lifecycle). The ratio is taste; there is no evidence for any
  pyramid. Prose only.
- **e2e waits on state, never on time** (web-first assertions, `expect.poll`). Sensor:
  `eslint-plugin-playwright` `no-wait-for-timeout`.
- **Mutation testing runs in the clean-up pass, never in the gate.** Sensor: gremlins (Go) and
  StrykerJS (web), run by hand.

## The lint budget

- **The suppression list only shrinks.** It holds the violations that existed when the strict rules
  landed. Fixed one? Remove it from the list and lower the pin in the same commit. Never re-suppress a
  new violation, and never run a "suppress everything" command on the whole repo. Sensor: guard
  `suppression-budget` (Go `//nolint` count; web `eslint-suppressions.json` total and inline
  `eslint-disable` / `@ts-*` / `svelte-ignore` count).
