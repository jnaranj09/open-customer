# Stage 5: Initial Zoom Out Review

**Target:** `docs/designs/quick-quotes.md` (lines 1-916). Reviewer: `egm-zoom-out` (completed).
Standard: `zoom-out-standard.md`. Verdict: passes with changes. 0 Blocker, 6 Major, 9 Nit.

## Major

* **M1. The catalog moves twice** (section 9; items 16, 18, 19, 21, 25, 27). S1n puts it at
  `/servicios`, S1b moves it to `/servicios/catalogo`: two re-keys of the budget-guarded
  `eslint-suppressions.json` and of the probe row. Fix: in S1n move it once to
  `/servicios/catalogo` and point the nav item there (`isActivePath` matches); in S1b change the one
  `href` to `/servicios`. Or merge S1n and S1b.
* **M2. Convert lives in a route, so its failure paths are untested** (Goals 5-6, section 4, items
  9, 29, 30). Vitest does not reach routes; no e2e can make an IndexedDB write fail as planned.
  Fix: one lib function (e.g. `convertQuote(lines, today, force)` in `lib/cache/`), tagged result
  `needs_confirm | ok | failed`, unit-tested with the Dexie stub; the route renders the confirm and
  calls `goto`. `readDraft` disappears. Change Goal 6's check to a unit test.
* **M3. `segment` does not fit `NavItem`** (section 5, item 22, B7). `NavItem` requires an icon;
  Hoy's Día/Semana toggle (`routes/+page.svelte:225-240`) is already a hand-made pill segmented
  control. Fix: in S0 check `.pen` for Hoy's toggle; build one `Segmented` primitive (link and button
  items) instead of a `NavItem` variant; record whether Hoy moves onto it later.
* **M4. The `formDraft` clear-on-revert fix can delete a draft on mount** (section 7, item 10). The
  save `$effect` runs at mount while the form equals `initial` and before `load()` resolves. Fix:
  define "change" as a user change (not the mount run, not the `set()` of a restore); test "stored
  draft + fresh mount → restored and still stored"; clear through the autosave handle's `clear()`
  (cancels the 500 ms timer), also in convert step 5.
* **M5. `isBlankForm` is a second, hand-listed "empty booking"** (section 4 step 2, item 28). The
  next `FormSchema` field silently counts as blank. With M4 fixed, a `cita:new` draft exists only
  after a real change. Fix: drop it; ask whenever a parsed `cita:new` draft exists (or compare with
  one shared `blankForm(day)`).
* **M6. There is no "error toast"** (sections 3-4, CUJ Q4, items 25, 29). `toaster.say` has no
  error tone, and a press-and-hold instruction must stay on screen. Fix: an inline `Callout`
  (role `alert`) above the text block until the next copy or change; keep the toast for "Copiado";
  say what convert failure uses.

## Nit

* **N1.** `quoteText(quote)` cannot call `formatMoney` without currency and locale; `Quote` has one
  consumer. Export one `quoteText(lines, services, settings)`, rows private.
* **N2.** Archived lines are filtered twice (restore and builder). Pick one.
* **N3.** `readDraft` repeats `autosave(...).load()`; gone with M2.
* **N4.** `formFromLines` copies Nueva cita's inline blank form (`AppointmentForm.svelte:60`);
  extract `blankForm(day)`.
* **N5.** T100's test checks a type fact. Point it at the test that proves the copy failure logs
  only a code, or record the clipboard as an accepted note.
* **N6.** Line numbers in the doc go stale (S1a moves them); cite sections, items and symbols.
* **N7.** The default tab (Cotizar) is the second tab; put it first or say why.
* **N8.** "Servicios" shows three times on Cotizar (`h1`, nav, `ServicePicker` label); settle in
  the S0 frame.
* **N9.** Say whether Cotizar shows a restore note when she comes back (likely none).

## Owner decisions: cost noted, no reversal

* Fifth place (B1-B3): most of the build; workable; M1 removes part of the cost.
* Separate page with its own draft (A2): why the hand-off exists; M2 and M5 shrink it.
* Fixed format on the phone (A1): no concern.
