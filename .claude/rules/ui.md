---
paths:
  - 'web/src/**'
---

# Rule: UI

## UI pipeline (every UI change goes through these stages, in order)

- **1. Design first.** Every token and every reusable component exists in `design/soft-glass.pen`
  (Pencil) before it exists in code. A new one starts there, through Pencil or its MCP server; no
  access to the design file → stop and ask the owner. Prose only; a miss turns guard
  `design-tokens` or `design-parity` red.
- **2. Tokens follow the design.** Each `.pen` variable `x` is one CSS token `--x` in `app.css`,
  same value; a different name goes in `design/design-map.json` (`tokens`). A token derived at run
  time from the accent is on the guard's allow-list with a reason. Sensor: guard `design-tokens`.
  The allow-lists of both design guards only shrink: a stale entry fails the guard, a new entry is
  prose only (the reviewer rejects it unless the owner agreed).
- **3. Components follow the design.** Each reusable `.pen` component maps to one primitive in
  `web/src/lib/components/ui/` and each primitive maps back: `Button/Strong` is primitive
  `Button`, variant `strong`; a name that breaks that default gets a `components` entry in the
  design map in the same change. A primitive with variants exports `type <Name>Variant` (a union
  of string literals, in its module script or a `ui/` `.ts` file), and that union equals the
  primitive's `.pen` variants. Sensor: guard `design-parity`.
- **4. Only primitives style.** Surface classes (`glass-card`, `glass-panel`, `solid-panel`,
  `bg-glass`, `bg-glass-strong`, `bg-glass-tint`, any `*shadow-*`, `backdrop-*`) and radius classes
  (`rounded-*`), with or without the `!` modifier, appear only in `components/ui/`, in markup and in
  any script string (a `const look = '…'` included); routes and feature components compose
  primitives. A glass surface is a `Card` with a `variant`, `pad` and `radius`; its `class` is a
  plain string of layout classes, never padding and never an expression (a state such as `selected`
  is a prop). No `<style>` block outside `ui/`. Arbitrary values (`-[…]`, `[prop:value]`) appear
  nowhere: add a token or an `@utility` in `app.css`. Layout (flex, grid, gap, width, margin,
  position) and padding on other elements stay allowed outside `ui/`: they are not enforceable
  without layout primitives. Sensor: `eslint-plugin-better-tailwindcss` `no-restricted-classes` (two
  blocks in `eslint.config.js`), `no-restricted-syntax` (padding or an expression in a `<Card
  class>`, a spread on a Card, a `<style>` block) and the plugin's correctness rules
  (`no-unknown-classes`, `no-conflicting-classes`, `no-concatenated-classes`); guard `ui-lint` runs
  the real config on a probe for each bypass (`!`, variants, script strings, Card padding and
  expressions or spreads, `$derived` strings, `<style>`, drop and inset shadows) and on legitimate
  layout. The classes that existed when the rule landed are in `eslint-suppressions.json`, pinned by
  guard `suppression-budget`. An inline `style=` that sets a radius, shadow or surface is prose
  only: positioning needs `style=` (the time grid), so it cannot be banned. Also prose only: a class
  string returned from a function, used as a default parameter, or written as an object key (lint
  reads script variables, object values and `$derived` / `$derived.by` arguments, not those).
- **5. Compose, don't configure.** A primitive takes a `variant` (one prop with named values), not
  a pile of boolean props; content goes in as children / snippets, not as config props; a
  primitive is never copied into a feature folder (wrap or extend it). Copies: jscpd (10 lines or
  more), prose only below that; the API shape is prose only (reviewer).
- **6. Review before done.** Run the `web-design-guidelines` skill on the UI diff (these rule files
  win over its taste lines), then check the change in the running app with `agent-browser` (or
  Playwright WebKit for Safari) at 393 × 852 and 1440 × 900, with a screenshot of each. Tests
  cannot see CSS. Prose only (the reviewer asks for the findings and screenshots).
- **7. Runtime sensors.** Every route runs, at 393 × 852 and 1440 × 900 (Playwright projects
  `probe-compact` and `probe-wide`), through axe (`@axe-core/playwright`, tags `wcag2a`, `wcag2aa`,
  `wcag21a`, `wcag21aa`, `wcag22aa`; the spec fails when `target-size` did not run) and the layout
  probe: no sideways scroll, no control under 44 × 44 px, no field font under 16 px, no text cut
  by `overflow: hidden` (an ellipsis on a `nowrap` line is allowed) or spilling out of its box, with a
  60-letter one-word client name seeded. Each opened sheet gets the same checks, and the page
  behind it must be inert (only a `data-live-layer` layer, the toast, stays live). Each data feed
  of a screen is answered with no rows, a 500 and no answer: the screen shows its empty text, an
  alert and a busy status. A new route, sheet or feed gets an entry in the spec's table in the
  same change. Sensor: `web/tests/e2e/ui-probe.spec.ts`
  (fails on a route on disk with no entry); known findings are in `ui-probe.allow.ts`, each with
  its `UIP-` id and a `why`, and that list only shrinks (an entry that matches nothing, or names
  a route or state the spec does not probe, fails).
- **8. Fix the class, not the instance.** A reported visual problem is fixed in the token or the
  primitive that causes it, never with a one-off class or override in the screen where it was
  seen. The reply names the token or primitive changed and how many screens use it (count the
  routes that import it, directly or through a feature component). Sensor: stage 4's lint rejects
  a route-level surface, radius or Card-padding fix; reviewer for the rest.

## Rules

- **One component per job.** The shared pieces (bottom sheet, list row, empty state, banners,
  pickers) live in the web components folder (`ui/` for the primitives), and feature routes use
  them instead of a hand-made copy. Sensor: guard `design-parity` (no orphan primitive); a guard
  that fails on a copy outside the components folder (add per component); prose only until then.
- **No raw values in components.** Colors, spacing and radii come from the Tailwind theme in
  `app.css` (the accent reads the runtime `--accent` variable). No hex colors or arbitrary values
  in a component. Sensor: the web guard `raw-colors` (no hex, `rgb(`, `hsl(` or `oklch(` literal in `web/src`
  outside `app.css`, except the `DEFAULT_ACCENT` line in `lib/color.ts` and the `theme-color` meta
  tag in `app.html`; no `-[#` or `-[<n>px]` in a `.svelte` file); `no-restricted-classes`
  (stage 4) for every arbitrary value.
- **Every action ends in visible feedback.** A tap either changes what she sees or shows a success or
  error message. An async button shows a busy state and cannot double-submit. Prose only;
  per-feature tests.
- **Error text maps from a problem code, never from a raw error message.** Sensor: the web client
  tests (item 78 cases).
- **All UI text is Spanish and comes from `es.json` through `t()`** (wording:
  `spanish-texts.md`). No inline Spanish in a component. Prose only (reviewer check; no lint rule
  in release 1).
- **A control she must not use is not rendered** (or is visibly disabled with a reason, like
  "Enviar" while offline). Prose only.
- **The server holds truth; the phone cache is a read-only copy; derived values use `$derived`,
  never a stored copy.** The due list and "done" are never computed or stored on the phone beyond
  the cached server result. Prose only.
- **Effects only sync with external systems.** No `$effect` that copies props or other state into
  state; use `$derived`. Sensor: eslint-plugin-svelte `svelte/prefer-writable-derived` (in its
  recommended config) catches the `$state` + `$effect` copy; other copies are prose only.
- **Tests query like a user**: Playwright `getByRole` and `getByLabel`, then text; a test id last.
  Sensor: `eslint-plugin-playwright` `no-raw-locators` and `prefer-native-locators`.
- **Two layouts.** Compact below 1024 px, wide from 1024 px; wide is never the phone layout
  stretched (owner rule). Prose only; the exit pass checks both.
- **Accessibility and touch basics**: every control has an accessible name, tap targets are at least
  44 px, focus moves into an opened sheet and back on close, and the page behind an open sheet is
  inert; a `data-live-layer` layer (the toast) stays live and in the sheet's Tab cycle. Sensor:
  stage 7 (axe for names and WCAG rules, the layout probe for 44 px and the inert page); focus
  moving back on close is prose only.
