---
paths:
  - 'cmd/**'
  - 'internal/**'
  - 'api/**'
  - 'web/src/**'
  - 'deploy/**'
---

# Rule: maintainability

Every change keeps these rules. Each one names its sensor, or says "prose only".

- **Search before you write.** Look for the existing helper, component or type before adding one. No
  new abstraction without a second real caller. Inline a shared helper once it grows a flag or branch
  for one caller. Sensor: golangci-lint `dupl` (Go); jscpd (web, configured for `.svelte`, not yet
  shown: no `.svelte` file reaches `minTokens`; both in `make lint`). jscpd fails when it analyzes no files
  (`failOnEmpty`); a partly empty scan still passes.
- **One owner per shared value.** A value two places must agree on (a port, a size, a color, a list
  one side mirrors) is defined once, and the other side imports it. Where one side cannot import the
  other (Go and TypeScript, CSS and code), a test reads both sides and fails when they drift. Sensor:
  `make check-pins` for image and tool versions, `make check-generated` for the API contract; prose
  only for other pairs until a drift test exists for the pair.
- **Every cross-file contract has a drift test that reads both sides.** A contract is anything one
  file must match in another that the type checker cannot see: a route, an error code (problem
  type), a cache table name, a wire string. Sensor: `make check-generated` for spec ↔ Go ↔ web
  types; prose only for the rest until each test lands.
- **Three or more call sites with the same obligation share one helper**, and a guard keeps callers
  on it. An obligation is a step every caller must take to be correct (decode with `decodeJSON`, open
  the write transaction, write errors with the problem helper). Never for look-alikes. Prose only
  until the guard lands.
- **Nothing stale survives the change that made it stale.** Delete dead code. Search every name the
  change renames, moves or removes, and the old wording (the old value, default or message), and fix
  every comment and living doc (`CLAUDE.md`, `.claude/rules/`, `technical-taste.md`, README, the
  design doc) the change made false, in the same commit. Sensor: `deadcode` + `unused` (Go), knip
  (web, `.svelte` included), guard `agent-context`
  (doc-paths), reviewer.
- **A failure is never silent: not in the app, not in the gate.** A failure of what the user asked
  for reaches a surface the user sees (a message, an exit code, an HTTP status; a log is not a
  surface), and its caller sees a failure, never a success. A failure that passes for a real result
  (an empty list, a default) logs a `code=` where it was caught. A check that skips because a tool is
  missing is a failure. Sensor: guard `catch-blocks`, `errcheck` + `nilerr` (Go); per-surface tests.
- **A contained error logs a stable `code=<token>`, never the value.** Never log a request body, a
  parsed value, a client's name or phone, a raw error message from outside input, a token or a file
  path, except the setup link `serve` and `setup-link` print by design (design doc 'Auth, sessions
  and handover'): one plain stdout line, never a slog record. Prose only.
- **A guard that stays green when the code it protects is broken is not a guard.** Break the code,
  see it red, restore. Prose only (reviewer step 2).
- **"Why" comments stay.** A comment that states a constraint or an incident is kept; a commit that
  removes one says so. Prose only.
- **Split by entanglement, not line count.** Split where the pieces would otherwise be tangled (you
  must read both to understand either). Refactor the biggest, most-changed file first. Prose only.
- **Plain over clever.** Plain functions and data over interface hierarchies; concrete module types,
  an interface only where a test needs a fake; explicit over magic; long, specific names. Prose only.
- **Small steps.** One task per change; a refactor commit changes no behaviour, a behaviour commit
  does not reshuffle code. Prose only.
- **Parallel agents only in separate worktrees**, one writer per build/test gate. Prose only.
