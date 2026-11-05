---
paths:
  - 'web/src/**'
---

# Rule: UI

- **One component per job.** The shared pieces (bottom sheet, list row, empty state, banners,
  pickers) live in the web components folder, and feature routes use them instead of a hand-made
  copy. Sensor: a guard that fails on a copy outside the components folder (add per component);
  prose only until then.
- **No raw values in components.** Colors, spacing and radii come from the Tailwind theme in
  `app.css` (the accent reads the runtime `--accent` variable). No hex colors or arbitrary pixel
  values in a component. Sensor: the web guard `raw-colors` (no hex, `rgb(`, `hsl(` or `oklch(` literal in `web/src`
  outside `app.css`, except the `DEFAULT_ACCENT` line in `lib/color.ts` and the `theme-color` meta
  tag in `app.html`; no `-[#` or `-[<n>px]` in a `.svelte` file).
- **Every action ends in visible feedback.** A tap either changes what she sees or shows a success or
  error message. An async button shows a busy state and cannot double-submit. Prose only;
  per-feature tests.
- **Error text maps from a problem code, never from a raw error message.** Sensor: the web client
  tests (item 78 cases).
- **All UI text is Spanish and comes from `es.json` through `t()`.** No inline Spanish in a
  component. Prose only (reviewer check; no lint rule in release 1).
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
- **Visual changes are checked in the running app**, in Safari's iPhone view and at 1440 × 900, with
  a screenshot, not only by tests: tests cannot see CSS. Prose only.
- **Accessibility and touch basics**: every control has an accessible name, tap targets are at least
  44 px, focus moves into an opened sheet and back on close. Prose only.
