# Consolidated Review Report: quick-quotes.md

**Target:** `docs/designs/quick-quotes.md` (FeatureRange 1-604, DetailedRange 605-916). Mode:
consolidated (stages 5-9 run in order with no edits between them). Date: 2026-11-07.

| Stage | Report | Reviewers | Result |
| --- | --- | --- | --- |
| 5 Initial Zoom Out | `zoom_out_report.md` | zoom-out | 0 B / 6 M / 9 N |
| 6 Comprehension | `comprehension_report.md` | comprehension | 0 B / 5 M / 6 N |
| 7 Critic | `critic_report.md` | design, structural, style | 1 B / 9 M / 23 N |
| 8 Intermediate Zoom Out | `intermediate_zoom_out_report.md` | zoom-out | 0 B / 10 M / 9 N (merges 5-7) |
| 9 Implementation Readiness | `implementation_readiness_report.md` | implementation-readiness, codebase-details, style | 1 B / 15 M / 20 N |

All reviewers completed. No reviewer recommends reversing an "(owner)" decision.

Checked in code by the main session: Vitest runs with `environment: 'node'` and only
`src/**/*.test.ts` (`web/vite.config.ts:53-54`); `toaster.say` takes a `MessageKey`, not a string
(`web/src/lib/toaster.svelte.ts:24-27`); the "Volver a Ajustes" bar belongs to
`routes/ajustes/+layout.svelte`, not the catalog page.

## Action items

### 🔴 Blocker

* **A1. Move the draft lifecycle rule into a pure function so it can be unit-tested.** `formDraft`
  needs `onMount` and `$effect`; Vitest runs in `node` with no component-test library. Rule
  (Stage 8 M-B): nothing is saved or cleared until `load()` finishes; after that, a form equal to
  `initial` clears the draft (through the autosave handle's `clear()`, which cancels the 500 ms
  timer) and any other form saves. Put the decision in a pure helper in `lib/cache/drafts.ts`
  (e.g. `draftAction(loaded, current, initial) → 'none' | 'save' | 'clear'`) with tests for
  "fresh mount keeps the stored draft" and "change then revert clears it". `formDraft` calls it.
* **A2. Add a short Risks section.** Accepted (each tagged owner/agent): one quote at a time; Nueva
  cita's Total counts an unpriced line as 0; "Recuperamos…" shows after convert; press-and-hold is
  the copy fallback. Handled: one line each pointing at its section. Replaces the "honest" wording.

### 🟡 Major

* **A3. Convert as one lib function.** `convertQuote(lines, today, replace)` in `lib/cache`,
  returning `'needs_confirm' | 'ok' | 'failed'`, reading and writing only `cita:new`. Ask whenever
  a parsed `cita:new` draft exists. On `ok` the route clears the quote through its `formDraft`
  handle, then `goto('/citas/nueva')`. Remove `readDraft`, `writeDraft`, `formFromLines`,
  `isBlankForm`; add `blankForm(day)` extracted from `AppointmentForm`. Busy state via
  `submitOnce`. Goal 6's convert check becomes a unit test.
* **A4. Name the allowed way to make a draft write fail in a unit test.** No Dexie stub exists and
  guard `test-hygiene` forbids mocking `./db`. Use a process-edge failure (for example
  `IDBObjectStore.prototype.put` throwing a quota error under fake-indexeddb, as `db.test.ts`
  sets it up), and say so in the plan.
* **A5. One inline `Callout` slot on Cotizar,** a tagged union `none | confirm_replace |
  copy_failed | convert_failed`, `role="alert"` for failures, reset on the next pick, copy or
  convert. The toast stays only for "Copiado", called as `toaster.say('quote_copied')` (a key).
* **A6. No new primitives.** The tab links and the text block are `Card` (`as="a"` with
  `radius="pill"`, `selected`, `aria-current`, `min-h-tap`; `as="p"` with
  `whitespace-pre-line select-text`). Remove `NavItem/Segment` and the text-block primitive from
  S0, items 1, 13, 22, 23; give the tab links' exact props, padding and the exact-match `current`
  rule.
* **A7. Move the catalog once and rename the stages.** The catalog goes straight to
  `/servicios/catalogo`; the nav item points there until Cotizar lands, then `/servicios`. Delete
  the "no build ships one tab" claim. Stages S0-S4 in run order; tag each goal's check with its
  stage. `make image image-scan dast` on every stage that changes `web/` (CLAUDE.md).
* **A8. One exported `quoteText(lines, services, settings): string`.** Rows private; `minor` not
  `Cents`; drop the zero-duration branch (the catalog requires 1-1440 minutes).
* **A9. Mount the quote form only after the snapshot loads.** A `QuoteForm.svelte` child gets only
  active services, so restore cannot drop every line on a cold start and an archived service never
  shows as picked while the text skips it. (Overrides part of Stage 8 M-I: filter at the form, the
  builder still skips missing services.)
* **A10. The archived-restore fix needs a test that fails before the fix.** `restoreForm` already
  drops unknown ids; extract a pure `activeServiceIds(services)` and test it.
* **A11. Define the e2e data.** A priced and an unpriced service, a repeated pick and the business
  name, created through the setup steps or a new `seed.ts` helper, and how `resetData` clears them;
  the exact copy assertion (text read from the clipboard stub, built with `formatMoney`).
* **A12. Specify the tab-containment probe check:** id, selector, rule (every tab box inside the
  bar box at 393 px), call site, and an order that sees it red before the padding change.
* **A13. Make the probe rows expressible.** The "with lines" and replace-confirm states need a
  seeded draft or a setup step in the `Probe` table; the busy text is `services_loading`, not
  `agenda_loading`.
* **A14. Fix the repeat-line key.** One template, e.g. `quote_line_repeat`
  "• {name} ×{count} ({unit} c/u): {total}", matching section 1; drop the reuse of
  `appointment_service_count`.
* **A15. Drop threat row T100.** State once in NFR: the quote reads no client data; a failed copy
  logs only a code (guard `catch-blocks`); e2e stubs cover missing API, rejection and synchronous
  throw. Remove the untestable "text contains nothing but the inputs" test.
* **A16. Say what Copiar does if the S0 clipboard test fails** (the button stays as a try, the
  Callout shows press-and-hold, or the button is removed).
* **A17. Doc accuracy.** release-1 edits: say plainly which living items (67, 75, 77) change and
  that build-order history does not. Item 16: the "Volver a Ajustes" bar comes from
  `ajustes/+layout.svelte`, which needs no change. Cite symbols, not line numbers, where S1a moves
  lines.
* **A18. Doc structure.** One sentence defining "place" vs "tab"; a visible "Nueva cotización"
  button when there are lines (`startOver()`), no restore note on Cotizar; Pros and Cons for owner
  alternatives B2 and B3; Additional Resources and Future Improvements as H2; one relative link to
  the brief's post-move path; drop "Goldfish" and "about a minute".

### 🟢 Nit

* **A19.** The remaining nits from all stages, applied in passing: Cotizar tab first or the order
  tagged with a reason; "Servicios" three times on Cotizar settled in the frame; "seen failing
  once" explained as the fault-injection step; the total line's omission stated in the format
  list; `todayIn` named; the diagram borders; Spanish text for every new key; test names and routes
  given; small type and code inaccuracies (Stage 9 report); review reports moved to
  `docs/designs/history/review-quick-quotes/round-1/`.

## Traceability Matrix

| Action | Stage 5 | Stage 6 | Stage 7 | Stage 8 | Stage 9 |
| --- | --- | --- | --- | --- | --- |
| A1 | M4 | #3 | nit (clear vs convert) | M-B | B1 |
| A2 | | | Blocker, Major "honest" | M-G | |
| A3 | M2, M5, N3, N4 | #4 | nits (blank, busy) | M-A, M-C, N-6 | |
| A4 | M2 | #4 | | | M6 |
| A5 | M6 | | Major "error toast" | M-F | M9 |
| A6 | M3 | | | M-D, M-E | M7, M8 |
| A7 | M1 | #1, #7 | Majors (line 392, stages) | M-H | M11 |
| A8 | N1, N2 | #2 | nit (zero duration) | M-I | |
| A9 | | | | | M2 |
| A10 | | | | | M1 |
| A11 | | | | | M3 |
| A12 | | | | | M4 |
| A13 | | | | | M5 |
| A14 | | #6 | | | M10 |
| A15 | N5 | #5 | nit (T100) | M-J | M13 |
| A16 | | | | | M14 |
| A17 | N6 | | | N-7 | M12, M15 |
| A18 | N9 | #8 | Majors (terms, alternatives, brief, headings, fresh quote) | N-1-N-5, N-8 | |
| A19 | N7, N8 | #9, #10, #11 | other nits | N-9 | nits |
