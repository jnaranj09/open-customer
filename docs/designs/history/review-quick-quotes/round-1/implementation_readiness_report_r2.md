# Goldfish Review Report (round 2, the one allowed re-audit)

**Target:** `docs/designs/quick-quotes.md` (`710-1011`, "Detailed Implementation Plan")
**Stage:** 9, Implementation Readiness Test
**Baseline:** `implementation_readiness_report.md` (round 1: B1, M1-M15) and `consolidated_review_report.md`. Decisions tagged "(owner)" stand.

**Verdict: nearly ready.** The rewrite resolves the Blocker and 13 of the 15 Majors. Two more are partly resolved. Three new Majors remain. Each one would make a stage fail its gate, or ship a screen the rules forbid, if built as written.

## Reviewer Status

| Reviewer | Status |
| --- | --- |
| `egm-implementation-readiness` | completed |
| `egm-codebase-details` | completed |
| `egm-style` | completed (did not open code; its code-dependent findings were checked by the synthesizer or by the other two reviewers) |

Severity counts after deduplication: **0 Blocker, 3 Major, 10 Nit.**

## 1. Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| B1 | resolved | The pure `draftStep(loaded, now, initial)` is tested in `drafts.test.ts` (lines 795-799). This fits `vite.config.ts:52-57` (`environment: 'node'`). |
| M1 | partly | `activeServiceIds` and its call in `AppointmentForm` are in (lines 779, 788-789). The only test is "fails before the function exists" (a compile error). Nothing goes red if the wiring is removed. See Nit 8. |
| M2 | resolved | `QuoteForm` mounts only once `cache.snapshot` exists and gets only active services (lines 900-902, 914-915). |
| M3 | partly | The data, `resetData`, the clipboard stub and the moved helpers are defined (lines 946-951). The `quoteText` oracle cannot load under Playwright. See new Major N1. |
| M4 | resolved | The `tab-fit` id, argument, `Finding`, `sift`, project, state and commit order are given (lines 380-386, 810-811, 836-838). The call-site file is not named. See Nit 1. |
| M5 | resolved | The probe row is defined (lines 939-941). The other states are an accepted risk (line 444). |
| M6 | resolved | `vi.spyOn(IDBObjectStore.prototype, 'put')` is named, and `restoreMocks: true` is set at `vite.config.ts:56` (lines 970-973). |
| M7 | resolved | "Frames only; no new components" (line 723). |
| M8 | resolved | The `Card` props are given and are valid against `ui/Card.svelte` (lines 355-362). |
| M9 | resolved | `toaster.say('quote_copied')` (line 910). |
| M10 | resolved | One "×{count}" template, matching section 1 (lines 226, 929). The exact "×2" is tested (line 892). |
| M11 | resolved | `make image image-scan dast` runs on every stage after S0 (lines 417-418, 995-996). |
| M12 | resolved | Lines 713 and 758-759. |
| M13 | resolved | The untestable test is gone (lines 891-893). |
| M14 | resolved | The fallback is defined: Copiar plus a permanent `quote_hold_hint` (lines 302-304, 907). |
| M15 | resolved | The back bar is traced to `routes/ajustes/+layout.svelte:40-46` (lines 847-849). This fix also dropped the wide-layout clause. See new Major N3. |

## 2. Prioritized Findings (Blockers -> Major -> Nits)

### N1. `quote.spec.ts` cannot import `quoteText` - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 952-953 (item 27); lines 213 and 270 (`quote.ts` uses `t()`).
* **Discrepancy Description**:
  * Playwright cannot load `quote.ts`. `quote.ts` imports `t` from `lib/i18n`, and `web/src/lib/i18n/index.ts:1` is `import es from './es.json'`, with no import attribute. `web/tests/e2e/steps.ts:5-6` records that Playwright's loader rejects this import. That is why `steps.ts` keeps its own `fill()`.
  * There is a second path to the same import: `LineSchema` comes from `booking.ts`, which imports `time.ts`, which imports `./i18n`.
  * The spec also has no source for `quoteText`'s inputs: the service ids, the prices, and the settings (name, currency, locale). `seed.ts` has no reader for them.
  * The expected text would come from Node's `Intl`, while the page renders in WebKit, so the two could format money differently.
* **Rationale & Recommendation**: Drop "with `quoteText` imported and run on the same data". Keep the check that the recorded clipboard text equals the text block's `textContent`, and leave the exact format to `quote.test.ts`. Optionally, assert one or two fixed rows built with `fill(es.quote_line, …)`, using the price as the page shows it.

### N2. S3 uses the key `quote_convert`, which S4 adds - Severity: Major
* **Reviewer Lens:** `egm-style` (key typing checked by the synthesizer)
* **Affected Scope:** Lines 904-906 (item 22, S3) and line 984 (item 30, S4).
* **Discrepancy Description**:
  * Item 22 renders "Convertir en cita (`quote_convert`, wired in S4)" in S3, but the key is only added to `es.json` in S4.
  * `t()` takes a `MessageKey = keyof typeof es` (`web/src/lib/i18n/index.ts:3,9`), so S3 fails svelte-check.
  * Even with the key, S3 would ship a button that does nothing. That breaks "each stage ships on its own" (line 417) and the ui.md rule that a control she must not use is not rendered.
* **Rationale & Recommendation**: Render Convertir en cita only from S4 (item 29), and remove it from item 22. `quote_convert` stays in item 30.

### N3. No item builds the moved catalog's wide layout - Severity: Major
* **Reviewer Lens:** `egm-style` (checked against lines 348-354 and 728 by the synthesizer)
* **Affected Scope:** Item 17 (lines 845-850); section 5 (lines 352-354); item 1 (line 728).
* **Discrepancy Description**:
  * S0 draws Mis servicios with a "wide layout of its own" (line 728), and section 5 says each tab's wide layout comes from its `.pen` frame.
  * Item 17 says "behaviour unchanged" and only says the Ajustes two-column layout "no longer applies".
  * An implementer following item 17 ships the phone layout stretched at 1440 px, which ui.md forbids (owner rule).
* **Rationale & Recommendation**: Add to item 17: "its 1440 layout is rebuilt to match the S0 Mis servicios frame". Say also whether that work is in the move commit or in a separate commit after it (a move commit should change no behaviour).

### Nits (each one would cause a wrong implementation)

1. **`tab-fit` call site (item 15, lines 836-838).** The checks are called from `ui-probe.spec.ts` (`screenFindings` at `:293-295`, the base test at `:470-475`). Item 20 lists that file only for the re-key. Say that the base test calls `tabFit(page, es.nav_label)` in `probe-compact`. *(all three reviewers)*
2. **`formDraft` wiring (item 10, lines 803-805).**
   * `loaded` must be `$state`. Otherwise the save effect does not re-run when the load resolves.
   * `'clear'` must go through `formDraft`'s own `clear()` (`formDraft.svelte.ts:38-44`, which logs `code=draft_clear_failed`), not the raw autosave handle. The raw handle (`drafts.ts:30-33`) rejects with nothing logged.
   * Use distinct names for the two handles. Line 979's "the draft's `clear()`" is ambiguous.
3. **Cotizar busy line (item 23, line 914).** It must be `<p role="status">`, as at `citas/nueva/+page.svelte:31`. The probe finds it with `getByRole('status')` (`ui-probe.spec.ts:513`).
4. **S2 commit order (lines 810-811, 815).** The nav item points at `/servicios/catalogo` before that route exists. `BottomNav.svelte:17` calls the typed `resolve(item.href)`, so svelte-check is red as well as `tab-fit`. Either move the page first, or state which checks may be red in each commit and that the gate applies per stage.
5. **`nav.test.ts` S2 case is false (lines 877-878).** With `href` `/servicios/catalogo`, `isActivePath('/servicios/catalogo', '/servicios')` is false (`nav.ts:6-8`). Write it as: S2, active for `/servicios/catalogo`; S3, `href` `/servicios`, active for both paths.
6. **CUJ Q2 cannot save (item 31, line 989).** `resetData()` leaves no client. Saving needs a client and a time (`booking.ts:155-163`), and the converted draft has `clientId: null` and `time: ''`. Name the client (an `insertClient` call or the quick-add) and the time to fill.
7. **`draftStored` needs a key argument when it moves (line 949).** It is hard-coded to `draftKey('clienta', 'new')` (`offline.spec.ts:71`). The quote tests need `cotizacion:new` and `cita:new`.
8. **The S1 fixes have no test that goes red against the fix (lines 391, 423, 779, 788-789, 806).**
   * "Fails before the function exists" is a compile error. Nothing goes red if `AppointmentForm` keeps passing every service id.
   * The `formDraft` wiring is proven only in S3.
   * Items 8-10 are not assigned to commits, although item 7 commit 1 already calls `blankForm`.
   * Either name the commits and a test that breaks the protected code (for example, a `booking.spec.ts` case with a `cita:new` draft holding an archived service), or say in writing that these fixes are proven later.
9. **The S3 change to `setup.spec.ts` (lines 872-874) is listed in no S3 item.** It also needs `expect(page).toHaveURL(/\/servicios\/catalogo$/)` after the tab tap. Cotizar's empty state shows the same `appointment_services_empty_title`, so the assertion at `setup.spec.ts:108` passes before the tab is reached.
10. **`es.json` keys (item 25, lines 925-935).**
    * `services_tabs_label` is an `aria-label`, so no S0 frame shows its wording. Give the Spanish text.
    * `quote_hold_hint` should be added only if the S0 clipboard test failed. No guard flags an unused key.

Left out as too small or not affecting the build:
* item 26's stray `ServicePicker.svelte` heading;
* the padding value's source;
* the `titles.test.ts` mention;
* the `bind:lines` function-binding pattern;
* the offline cache proof;
* the `round-1/` filing of this round's reports;
* whether S0 runs the gate.

The style reviewer raised a risk that `clear()` followed by `goto` lets the save effect write the quote back. No reviewer confirmed it: `clear()` changes no tracked state, so the effect has no reason to re-run. It is not counted.
