# Goldfish Review Report

**Target:** `docs/designs/quick-quotes.md` (`605-916`, "Detailed Implementation Plan")
**Stage:** 9, Implementation Readiness Test
**Baseline:** these findings assume that the "Smallest change set" in `intermediate_zoom_out_report.md` (M-A to M-J, N-1 to N-9) has been applied. Findings that report already covers are left out. Decisions tagged "(owner)" stand.

**Verdict: not ready.** Even with the zoom-out changes, the plan would still fail the gate in one place: the `formDraft` unit test cannot run in the current Vitest setup. It also leaves the implementer to guess about test data, probe states, how to make a write fail in a test, and archived services in the quote.

## Reviewer Status

| Reviewer | Status |
| --- | --- |
| `egm-implementation-readiness` | completed |
| `egm-codebase-details` | completed |
| `egm-style` | completed |

Severity counts after deduplication: **1 Blocker, 15 Major, 20 Nit**.

## Prioritized Findings (Blockers -> Major -> Nits)

### B1. The `formDraft` unit test cannot run in the current Vitest setup - Severity: Blocker
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 704-709 (item 10) and 898-899 (Verification Plan). Goal 10 and the two tests that zoom-out M-B adds depend on it.
* **Discrepancy Description**: `formDraft` only works inside a mounted component, because it relies on `onMount` and `$effect` (`web/src/lib/cache/formDraft.svelte.ts:19-35`). Vitest is set up with `environment: 'node'`, it picks up only `src/**/*.test.ts`, and it sets no `browser` resolve condition (`web/vite.config.ts:52-57`). In that setup Svelte loads its server runtime, where neither hook runs. The project has no jsdom, happy-dom or `@testing-library/svelte` (`web/package.json`), and no `*.svelte.test.ts` file exists. The planned test ("change, then revert: the stored draft is gone and a new mount loads nothing") therefore needs new test tooling, and choosing that tooling is a dependency decision the plan leaves to the implementer.
* **Rationale & Recommendation**: Write one of these into item 10 and into S1a:
  * **(a), preferred:** move the load-gate rule and the "equal to initial clears" rule into a pure function in `drafts.ts`, for example `draftStep(loaded, now, initial): 'wait' | 'save' | 'clear'`. `formDraft` calls it, and `drafts.test.ts` tests it on fake-indexeddb. The existing e2e test ("empty the quote, switch tabs, come back") covers the wiring.
  * **(b):** name the new dev dependency with its pinned version, the Vitest project for `*.svelte.test.ts` (with `resolve.conditions: ['browser']` and a DOM environment), and the `$effect.root` + `flushSync` pattern. Because this is new tooling, the owner must approve it.

### M1. The test for the archived-service restore fix cannot fail - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`, `egm-style`
* **Affected Scope:** Lines 680 and 689 (items 7-8), 366-368.
* **Discrepancy Description**: The fix is in the component: `AppointmentForm.svelte:74` passes `new Set(data.services.map((s) => s.id))`. The test goes in `booking.test.ts`, but `restoreForm` already drops any id it is not given (`booking.ts:366`), and `booking.test.ts:488` already covers that case. The new test would pass before the fix, which breaks `rules/testing.md` ("seen red once").
* **Rationale & Recommendation**: Move the rule into a pure function: either `activeServiceIds(services)` in `booking.ts`, which `AppointmentForm` calls, or `restoreForm` taking `Service[]` and dropping archived services itself. Then test that function with an archived service. Otherwise name an e2e case in `booking.spec.ts` that seeds a `cita:new` draft holding an archived service.

### M2. Archived services in the quote: the screen and the text disagree, and the draft can load too early - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 836-839 (item 25), 265-266. This interacts with zoom-out M-I.
* **Discrepancy Description**: Under M-I, archived services are skipped only in the builder. But `ServicePicker` still draws an archived service while it is in `lines` (`ServicePicker.svelte:15-17`). The chip shows as picked, the quote text leaves it out, and convert carries the line into `cita:new`. That breaks the plan's own "cannot disagree" claim (line 212). The other option, filtering on restore, has its own problem: the page calls `formDraft` during component init, before `cache.snapshot` exists, so the filter would drop every line. Nueva cita avoids this only because it mounts `AppointmentForm` inside `{#if cache.snapshot !== null}` (`routes/citas/nueva/+page.svelte:30-43`).
* **Rationale & Recommendation**: Add `web/src/lib/components/QuoteForm.svelte` (Create). It owns `lines` and `formDraft`, and the page mounts it only once the snapshot is loaded. Pass only active services to `ServicePicker`. Convert writes only the lines the builder kept. Say that this overrides the "only in the builder" part of M-I. Reuse the M1 helper.

### M3. The quote e2e tests have no defined data - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 857-860 (item 27), 884-888 (item 30), Goals 2-3.
* **Discrepancy Description**:
  * The e2e catalog is built once by `setup.spec.ts:12-20`: "Estudio Prueba", USD, and three services that all have a price. The CUJs instead use CRC, "Estudio Ejemplo", "Diseño sencillo" and a service with no price ("Cejas").
  * `seed.ts` has no `insertService`, and `resetData` does not truncate `services` (`seed.ts:17-23`). A service one spec adds stays for every later spec, including the probe projects, and whether they see it depends on run order (`playwright.config.ts:31-57`).
  * The plan does not say whether `resetData()` runs (Goal 3 needs "no client"), how the expected text is built, or how the test checks what was copied.
* **Rationale & Recommendation**: Pick one of two options:
  * (a) Run CUJ Q1 on the existing services (Acrílico, then Relleno twice) and leave the no-price case to `quote.test.ts`.
  * (b) Add an `insertService`/archive pair to `seed.ts` with `afterAll` cleanup, using names no other spec uses.

  In both cases:
  * Call `resetData()` in `beforeEach`.
  * Build the expected text by importing `quoteText`/`formatMoney`, the same way `offline.spec.ts` imports `draftKey`.
  * Have the clipboard stub record its argument on `window` and compare it with the text block's `textContent`.
  * Say whether the `draftStored` poll (`offline.spec.ts:52-73`, which now has two callers) moves to `steps.ts`.

### M4. The tab containment probe check is not specified - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`, `egm-style`
* **Affected Scope:** Lines 792-793 (item 21), 353-355, 728-729 (item 13).
* **Discrepancy Description**:
  * Checks in `probe.ts` run in the browser with "no outside names" (`probe.ts:67`). Each returns a `Finding { check, target, detail }`, and the allow-list matches findings by `check` id.
  * The plan gives no check id and no selector. It does not say whether the box to compare against is the `<nav>` or the inner pill `div` (`BottomNav.svelte:12-13`), what happens at 1440 px where the bar is absent, or where the check is called.
  * Items are said to be "in build order", but item 13 (smaller padding) comes before item 21 (the check that must be "seen failing once"). Followed in order, the check never goes red.
* **Rationale & Recommendation**:
  * Specify a check named `tab-fit`. Pass it the bar's `nav_label` as an argument, and compare each `a` in `nav > div` against that div's rectangle with a 0.5 px tolerance. It returns no finding when the bar is absent. Run it at `probe-compact` in the `base` state on every route, and send its findings through `sift`.
  * State the commit order: commit 1 adds the fifth `NAV_ITEMS` entry and the check (red), and commit 2 reduces the padding (green).

### M5. The ui-probe table cannot express Cotizar's states - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 855-856 (item 27), 271.
* **Discrepancy Description**:
  * A `Probe` row supports only `open`, `ready`, `sheets` (which must open a `role="dialog"`) and `feeds` (`ui-probe.spec.ts:71-80, 407-418, 484`). "With lines" and the replace-confirm `Callout` cannot be expressed, so axe and the 44 px check never see the text block, Copiar or Convertir.
  * The plan does not give the `ready` text.
  * "The same busy state as Nueva cita" means `agenda_loading` ("Cargando citas…"), which is the wrong text for this page. The busy line must match a `*_loading` key.
* **Rationale & Recommendation**:
  * Give the row: `ready: 'Acrílico'` and `feeds: [bootFeed('services', { bootstrap: ['services'], text: es.appointment_services_empty_title })]`, with `services_loading` as the busy text.
  * Then either add an optional `states?: { name; open(page) }[]` to `Probe` and list it as a probe change, or say plainly that only `base` and the services feed are probed and record that as a risk.

### M6. No allowed way to make a draft write fail in a unit test - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`, `egm-codebase-details`
* **Affected Scope:** Lines 701-702 (item 9) and Goal 5. This also applies to `convertQuote … 'failed'` after M-A.
* **Discrepancy Description**: The plan says "rejects when the Dexie stub fails", but no Dexie stub exists: `drafts.test.ts:1-13` uses the real Dexie on fake-indexeddb. The `test-hygiene` guard allows `vi.mock` only for `node:*`, `fs` and `pg`, and `INTERNAL_MOCKS` is empty and may only shrink (`testHygiene.guard.test.ts:17-23`). Mocking `./db` would turn the guard red.
* **Rationale & Recommendation**: Name the technique, for example `vi.spyOn(IDBObjectStore.prototype, 'put')` throwing a `DOMException('quota', 'QuotaExceededError')`; `restoreMocks: true` undoes it. Check once that Dexie surfaces this as a rejection. Name the test and its file, for example: "convertQuote returns 'failed' and leaves cita:new as it was".

### M7. The S0 `.pen` list still adds components that turn `design-parity` red - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 618-622 (item 1), 729 (item 13), 803-814 (items 22-23).
* **Discrepancy Description**: M-D and M-E replace the segment variant and the text-block primitive with `Card`, but item 1 still adds `NavItem/Segment` and a quote-text-block primitive to `.pen`. `designParity.guard.test.ts:116-133` fails when a `.pen` variant or component has no code to match it, and `DESIGN_ONLY` may only shrink. Every stage must pass the gate (line 381), so S0 would fail.
* **Rationale & Recommendation**: Apply M-D and M-E to item 1 as well: S0 adds only frames, built from `Card` and `Callout`. Remove the segment sentence in item 13 and remove items 22-23.

### M8. The tab links as `Card`: props, padding and tap size not given - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness`
* **Affected Scope:** Lines 846-849 (item 26), after M-D.
* **Discrepancy Description**: `Card` gets its padding only from `pad`, and its `class` may not set padding (`ui/Card.svelte:9-11, 56-66`). Most `pad` values give a box under 44 px for one line of `text-sm`. The plan leaves `variant`, `pad`, `radius`, the layout classes, the selected look and how "current" is computed to the implementer, and the probe's 44 px check would reject a wrong guess.
* **Rationale & Recommendation**: Write the markup, for example `<Card as="a" href={resolve(...)} pad="note" radius="pill" selected={current} aria-current={current ? 'page' : undefined} class="flex min-h-tap flex-1 items-center justify-center text-sm font-bold">`. Give the rule for `current` and name the S0 frame the links must match.

### M9. `toaster.say(t(...))` does not type-check - Severity: Major
* **Reviewer Lens:** `egm-implementation-readiness` (Major), `egm-codebase-details` (Nit)
* **Affected Scope:** Lines 843-844 (item 25), 881 (item 29).
* **Discrepancy Description**: `toaster.say` takes a `MessageKey` and calls `t()` itself (`toaster.svelte.ts:24-33`). Passing `t('quote_copied')` passes a `string`, which svelte-check rejects under `--fail-on-warnings`. The `quote_copied` toast is still there after M-F. The fix is one line, but as written the code fails the gate, which is why this is Major and not Nit.
* **Rationale & Recommendation**: Write `toaster.say('quote_copied')`, as `routes/ajustes/servicios/+page.svelte:26` does.

### M10. The `quote_line_repeat` placeholder is ambiguous and does not match section 1 - Severity: Major
* **Reviewer Lens:** all three
* **Affected Scope:** Lines 779-780 (item 20), 219.
* **Discrepancy Description**: Item 20 gives `"• {name} {count} ({unit} c/u): {total}"`, while section 1 gives `×{n}` and `{line total}` and says it reuses `appointment_service_count` (`"×{count}"`, `es.json:189`). An implementer who passes the raw number prints "Diseño sencillo 2 (...)". A unit test written from the same guess would still pass.
* **Rationale & Recommendation**: Either rename the placeholder to `{times}`, filled with `t('appointment_service_count', { count })`, or put the "×" in the key and drop the reuse claim. Use the same placeholder names in section 1, and add a unit test that checks the exact "×2".

### M11. The image and DAST gate is applied to fewer stages than CLAUDE.md requires - Severity: Major
* **Reviewer Lens:** `egm-codebase-details`, `egm-style`
* **Affected Scope:** Lines 903-905, and the stage exits at lines 386-389.
* **Discrepancy Description**: CLAUDE.md requires `make image image-scan dast` for any change to what the app serves. S1a (the AppointmentForm fix and the `formDraft` change) and S1c (convert) both change the served web build, but the plan requires these targets only for S1n and S1b.
* **Rationale & Recommendation**: Require them for every stage that changes `web/` (every stage after S0, renamed per M-H). Better still, cite "the gate in CLAUDE.md with its image rule" instead of copying the command list, so the copy cannot go stale.

### M12. Item 3 both edits and forbids editing the P0-P7 items - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 645-649.
* **Discrepancy Description**: Item 3 edits release-1 Target Files items 67, 75 and 77, which were built in P5. Its last bullet then says that "the finished P0-P7 items" are not rewritten. "Item" is also used without saying which numbering it refers to.
* **Rationale & Recommendation**: "The Build order table and the P0-P7 phase rows are not rewritten. Target Files items 67, 75 and 77 are edited in place because they describe code this design changes."

### M13. One quote test cannot be implemented - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 827-828.
* **Discrepancy Description**: The test "the text never contains anything but the inputs" is false as written, because the text also holds `es.json` words and `formatMoney` output. No test can check it, and the builder's signature already guarantees the property.
* **Rationale & Recommendation**: Delete the clause. Keep the property once, in the NFR ("the builder's signature has no client parameter").

### M14. The Copiar fallback is not defined - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 633-634.
* **Discrepancy Description**: "Becomes the main path" and "stays as a try" are not instructions. If the S0 clipboard test fails, the plan does not say what S1b builds: whether Copiar stays, what its failure message says, or whether the press-and-hold hint is always visible.
* **Rationale & Recommendation**: State what gets built in that case and name the S0 frame that shows it. For example: "Copiar ships with its failure message, and a permanent press-and-hold hint sits above the text block."

### M15. Item 16 describes a back bar the catalog page does not have - Severity: Major
* **Reviewer Lens:** `egm-codebase-details` (Major), `egm-implementation-readiness` (Nit)
* **Affected Scope:** Lines 746-748 (item 16).
* **Discrepancy Description**: The plan says to remove the page's "Volver a Ajustes" bar, but the page has no such bar (`ajustes/servicios/+page.svelte:54-67`). It comes from `routes/ajustes/+layout.svelte:40-46` through `showSettingsBack`, and it goes away by itself once the page leaves `/ajustes`. This is a false statement about current code, so the implementer would search for markup that does not exist.
* **Rationale & Recommendation**: Rewrite: remove the `h1` at line 56; the header row keeps "Nuevo servicio"; the back bar and the Ajustes two-column layout go away with the move, so the wide layout comes from the S0 frame.

### Nits

1. **`QuoteDraftSchema` is never defined** (line 837): write `z.object({ lines: z.array(LineSchema) })` and name its file (`quote.ts` or `booking.ts`). The `{ lines: LineSchema[] }` at line 263 mixes TypeScript and zod. (all three)
2. **No source for `today`** (line 879): write `clock.today(data.settings.timezone)`, as `citas/nueva/+page.svelte:19` does. (readiness)
3. **`goto('/citas/nueva')`** (line 880): write `goto(resolve('/citas/nueva'))`, the pattern the codebase uses. (readiness, codebase)
4. **The `settingsPage.test.ts` change duplicates an existing test** (lines 754-755): lines 29-31 already test Plantillas. Switch to `/ajustes/negocio` or `/ajustes/atributos`. (readiness, codebase)
5. **The icon is wrong in three ways** (lines 717, 723): `IconName` is `keyof typeof PATHS`, so the key goes into `PATHS`; item 11 hard-codes `'tag'` while item 12 says "chosen in S0"; and Lucide `tag` has a circle, so give both `d` strings, including `'M7.5 7.5h.01'`. (all three)
6. **`min-w-0` on the label** (line 729): in the `tab` variant the label is a bare text node (`NavItem.svelte:38-39`), so the flex item is the `<a>`. (codebase)
7. **Item 2 says "(Create)"** (line 629), but the file already exists, so it should be "(Modify)". (readiness)
8. **The `navigation.spec.ts` test is new** (line 796): name it and what it asserts (the five link names in order, at 393 and 1440). (readiness)
9. **The route `setup.spec.ts` takes after the move is not given** (lines 794-795, 853): `openSettings(page, es.settings_services)` stops compiling once the key is deleted. Cotizar's empty state shows the same `appointment_services_empty_title`, so the test can pass before it reaches the catalog. Specify the taps (nav `nav_services` exact, then the "Mis servicios" tab or `appointment_services_empty_action`), then assert `/servicios/catalogo`. (readiness, codebase)
10. **The offline e2e test does not stop the server** (line 859): it cuts the API with `serverOff` (`offline.spec.ts:13, 45-49`). Say whether `serverOff` moves to `steps.ts` or is copied. (codebase)
11. **Keys with no wording** (lines 777-786, 878): `quote_empty` (section 2 says "an empty text"), `quote_copy_failed`, `quote_replace_prompt`, `quote_convert_failed` and `services_tabs_label` have none. Say "wording taken word for word from the S0 frame". Write "Cancelar (existing `cancel`)" so no duplicate key is added. (readiness, style)
12. **Stale text the move leaves behind**: `web/src/lib/services.ts:1` still says "Ajustes → Servicios". (readiness)
13. **Item 31 is in the wrong stage, and the review reports have no home** (lines 890-893): move item 31 to S0. `docs/designs/history/concepts_brief.md` already exists (the release-1 brief), so the bare name at lines 6, 396 and 591 becomes ambiguous. Root reports (`critic_report.md`, `intermediate_zoom_out_report.md`, this one) also break release-1 item 87; give them a home under `docs/designs/history/review-*/round-<N>/`. (style, readiness)
14. **Wrong reason given for `readDraft`** (lines 698-699): the `layers` guard does not stop routes from importing `db.ts` (`sourceScan.ts:137-151`). M-A removes `readDraft`, but drop the reason too. (readiness)
15. **"stage" has two meanings** (lines 616, 910): ui.md "stages" and build Stages S0-S1c. Use "ui.md pipeline steps". (style)
16. **The reason for deleting `settings_services` reads backwards** (lines 774-775): write "delete by hand; no guard flags an unused key." (style)
17. **Three "Servicios" labels on one screen** (lines 219, 255): the h1, the nav item and the ServicePicker group label. Accept this in S0, and note that e2e locators need `exact` or a role scope. (codebase)
18. **Small wording fixes**:
    * Line 688: write "no lines (0 minutes)".
    * Pick one name for the button: "Convertir", "Convertir en cita" or "Convert".
    * Line 906: "web builder" should be `quote.test.ts`.
    * Line 748: say what the updated comment says.
    * Line 795: add why `exact: true` is needed (so the locator does not also match "Mis servicios").

    (style)
19. **Line-number references into release-1.md** (lines 641-647): the codebase reviewer confirmed lines 166, 618, 2296, 2300, 2334 and 2372 are correct today. The edits in this item move the lines after them, so cite section or item anchors instead. (style)
20. **Checked and correct, no action needed**: `AppointmentForm.svelte:56,74`, `booking.ts:51-60,137-145,357-358`, `drafts.ts:8,30-33`, `formDraft.svelte.ts:28-35`, `ServicePicker.svelte:29`, `titles.ts:17`, `SettingsList.svelte:22-26`, `es.json:318`, `ui-probe.spec.ts:183-190,458-460`, `eslint-suppressions.json:67`, `setup.spec.ts:107`, `navItems.ts`, `nav.ts isActivePath`, `citas/nueva/+page.svelte:36-41`, and the release-1 lines above.
