# Servicios place and quick quotes (Cotizar)

* **Author:** jnaranj09
* **Date:** 2026-11-07
* **Round:** 3. Round 1: six lenses, 6 Blocker / 13 Major / 8 Nit; the owner then chose the shape.
  Round 2: six lenses, all completed; after deduplication 2 Blocker / 16 Major / 9 Nit.
  Round 3 (the owner's navigation decision): six lenses (design, codebase-details, zoom-out,
  comprehension, implementation-readiness, structural), all completed; after deduplication
  3 Blocker / 7 Major / 11 Nit. The round-3 recommendations are applied below and have not been
  reviewed again.

## Overview

A client asks for a price on WhatsApp ("¿Cuánto sale acrílico con diseño y pedi?"). Today the owner
types the answer by hand. This feature adds a quote page, "Cotizar". She picks services from her
catalog, and the page shows the quote text it will copy: each line with its price, the total and
the total duration. The page has two buttons:

* **Copiar**: copies that text to the clipboard, so she pastes it into the WhatsApp chat that is
  already open.
* **Convertir en cita**: opens the existing "Nueva cita" form with the same services filled in;
  she then picks the client, date and time.

The quote page lives in a new fifth top-level place, "Servicios". The service catalog moves into
the same place, out of Ajustes. That move changes the app's shell (tab bar, sidebar, Ajustes) and
ships as its own stage (S1n) before the quote page.

The app serves one owner of a small service business, on an iPhone PWA. All UI strings are Spanish
and live in `web/src/lib/i18n/es.json`.

### Baseline (owner decisions)

* **Separate page, fixed format** (owner decision after round 1). Its own page and its own draft,
  so a quote never overwrites an unfinished booking. The copied text uses a fixed format made of
  es.json strings that the owner cannot edit. It is built on the phone from the cached catalog. No
  editable quote template, no template kind, no Ajustes setting, no server endpoint, no schema
  change.
* No client is needed to quote. The client is chosen only in Nueva cita.
* Quotes are not stored on the server. The open quote is a phone draft (Dexie, like `cita:new`).
* Catalog prices only. No per-line price edit, no non-catalog items, no discount. Extras are
  services ("Diseño sencillo"); she types a one-off discount in WhatsApp after pasting. Reason:
  appointments copy catalog prices and have no per-line override (`docs/designs/release-1.md`,
  "Appointments").

### Navigation (owner decision, round 3)

* **A fifth place, "Servicios"**, in the tab bar and the sidebar, between Clientas and Ajustes.
  The service catalog moves out of Ajustes (`/ajustes/servicios` today) into it.
* **Two tabs inside "Servicios":** "Mis servicios" (the current catalog screen, moved) and
  "Cotizar" (this feature). The owner named both labels.

The agent calls that implement this decision are A10-A14 below.

### Agent calls (made to keep the loop moving; the owner may overturn any)

Changed in rounds 2 and 3 where marked. Each entry is numbered as in Key Decisions below.

* **A1. Text shape** (round 2: unit price on repeats, the total suffix, and an illustrative
  example). The format is:
  * a heading: "Cotización · {name}", or just "Cotización" when `settings.name` is blank;
  * one line per service, in first-pick order;
  * a repeated service shows its line total plus the price of one copy;
  * a total line;
  * a duration line.

  Every piece of wording, including "•", "·", "×" and "c/u", comes from es.json. Nothing in it is
  specific to nails or to a country. Here is an example with made-up data. It only illustrates the
  format: the real amounts are whatever `formatMoney` prints, and with CRC's 2 decimals and Intl's
  non-breaking spaces that is likely "₡15 000,00" (not run).

  ```
  Cotización · Estudio Ejemplo
  • Acrílico: ₡15 000,00
  • Diseño sencillo ×2 (₡3 000,00 c/u): ₡6 000,00
  • Cejas: precio a consultar
  Total: ₡21 000,00 + lo que está a consultar
  Duración aproximada: 2 h 30 min
  ```

* **A2. Missing price** (round 2: full rule). A price of 0 counts as priced and shows
  `formatMoney(0)`. The same text is shown on screen and copied.

  | Lines | Total line |
  | --- | --- |
  | all priced | "Total: {amount}" |
  | some unpriced | "Total: {amount} + lo que está a consultar" |
  | none priced | no total line |

* **A3. Duration** (round 2: three strings, and the zero case). The duration is the sum of
  `duration_min`, computed by a `durationOf(lines, services)` helper taken out of `endFor`
  (`booking.ts:55-58`), so the end time and the quote share one sum. The name `minutesOf` is already
  taken (`booking.ts:39`). Formats:
  * "45 min" reuses `service_minutes` (`es.json:350`);
  * "2 h" and "2 h 30 min" are new keys;
  * a total of 0 leaves the duration line out.
* **A4. Convert** (round 2: rewritten for Blockers B1 and B2; round 3: no `volver`, R3-M4).
  Convertir en cita runs these steps:
  1. Read `cita:new` with `getDraft(key, FormSchema)`.
  2. If it is not blank (`isBlankForm`, which ignores `day`), ask inline whether to replace it.
  3. Build the full draft with `formFromLines(lines, today)`: today in the business zone, every
     other field empty.
  4. Write it, parsed and awaited, through a new `writeDraft` in `lib/cache/drafts.ts`.
  5. Only if that write succeeds, clear the quote draft and go to `/citas/nueva` with no `volver`
     parameter. Save and Cancel both go to `volver` (`citas/nueva/+page.svelte:13-15,36-41`), so
     with no parameter both land on Hoy, the default. A return to Cotizar would only show an
     empty page, because the quote is already cleared.

  If the write fails, she stays on the page, sees an error toast, and the app logs
  `code=quote_convert_failed`.

  Backing out of Nueva cita loses nothing, because `cita:new` keeps the lines. Nueva cita shows its
  usual "Recuperamos lo que habías escrito." banner; this is accepted for v1.
* **A5. Order and repeats.** Reuse `Line` (`service_id`, `count`) and `ServicePicker` with no
  change: first-pick order, with counts. Its label "Servicios" (`appointment_services`,
  `es.json:184`) is kept on the quote page. With no label prop, it repeats the place heading; this
  is accepted for v1 (round-3 Nit 3).
* **A6. Archived and missing services** (round 2: replaces "shows as unavailable"). When the quote
  draft loads, it keeps only lines whose service is in the cache and not `archived`. This is the
  `restoreForm` pattern (`booking.ts:357-371`). There is no "unavailable" state and no new chip look.
* **A7. No cap on the quote page** (round 2: replaces "same 20-service cap"). The cap exists only on
  the server, `maxServices = 20` (`internal/appointments/lines.go:13`), and it counts copies. A
  quote that goes over it after convert gets the existing "Elige hasta 20 servicios" error when the
  appointment saves.
* **A8. Copiar and its fallback** (round 2). The text is a `$derived` value from the builder. The
  click handler calls `navigator.clipboard?.writeText(text)` synchronously. One failure path covers
  three cases:
  * a missing API;
  * a synchronous throw;
  * a rejected promise.

  On failure, the app shows the error toast and a hint to press and hold the text block to copy, and
  logs `console.warn('code=clipboard_failed')`, never the text. It does not try to select the text
  from code. Success shows "Copiado". Both toasts go through `toaster.say`.
* **A9. Empty and loading states** (round 3: which empty state shows, Nit 5). With no active service,
  only `ServicePicker`'s own empty state shows ("Aún no tienes servicios" and "Configurar servicios",
  `ServicePicker.svelte:24-30`), and its link goes to the catalog tab. With services but no lines,
  Copiar and Convertir are not rendered and the page shows its empty text. While `cache.snapshot` is
  null, the page shows the same busy state as Nueva cita (`citas/nueva/+page.svelte:30-31`); the
  place layout shows no busy state of its own.
* **A10. Routes** (round 3, R3-B1). The routes are:
  * `/servicios` is Cotizar, always. The nav item's `href` stays a fixed literal, `'/servicios'`,
    added to the union at `navItems.ts:5`.
  * `/servicios/catalogo` is "Mis servicios", the catalog page moved with no behaviour change.
  * There is no rule that depends on the data, and no redirect. First setup goes Servicios, then
    `ServicePicker`'s empty state, then "Configurar servicios".
  * `/ajustes/servicios` is removed with no redirect. An old link shows the app's not-found page
    (there is no `+error.svelte`, so it is SvelteKit's default; the `<title>` falls back to
    `page_title_app`, `titles.ts:31-35`). One owner, links only from inside the app.
  * The nav item is active on both tabs through the prefix match (`nav.ts:6-8`). Add a `nav.test.ts`
    case.
  * The quote draft key stays `draftKey('cotizacion', 'new')`. It is a Dexie key, not a route.
* **A11. The place layout** (round 3, R3-M3). A new `routes/servicios/+layout.svelte` owns the
  place's `h1` ("Servicios", `services_title`) and the tab links. Each tab page owns only its
  content and actions: the catalog page drops its own `h1` (`+page.svelte:56`) and keeps
  "Nuevo servicio" in a header row. Compact: the `h1`, then the tabs, then the page. Wide: the same
  order, with the content at the catalog's current `max-w-190`; the quote page's wide arrangement
  comes from its S0 frame (ui.md: wide is never the phone layout stretched). No "Volver" bar: this
  is a top-level place. `<title>` keys: `services_title` for `/servicios/catalogo`, and a new
  `quote_title` ("Cotizar") for `/servicios` (`titles.ts:6` makes a missing key a type error).
* **A12. The tab control** (round 3, R3-M2). The two tabs are links in a `<nav>` with its own
  `aria-label` key and `aria-current="page"`. They are not an ARIA `tablist`, so there is no
  arrow-key handling. Each tab is active on an exact path match: a prefix match would light up
  Cotizar on both, because `/servicios` is a prefix of `/servicios/catalogo`. The control is a new
  variant `segment` of the `NavItem` primitive, drawn in `soft-glass.pen` first (ui.md stages 1 and
  3, guard `design-parity`). Tab order follows the owner's wording: "Mis servicios", then
  "Cotizar". Hoy's Día/Semana toggle (buttons with `aria-pressed`, `routes/+page.svelte:225-240`) is
  not moved onto it.
* **A13. Labels and icon** (round 3). The labels:
  * The place label is a new `nav_services` key ("Servicios"), used as both `label` and
    `tabLabel`.
  * The tab labels are new keys `services_tab_catalog` ("Mis servicios") and `services_tab_quote`
    ("Cotizar").
  * `settings_services` (`es.json:318`) is deleted with the Ajustes row. No guard catches unused
    keys, so the move removes it by hand.

  The icon is a neutral Lucide icon (proposal: `tag`; nothing nail-specific), added to `.pen`
  first and then to `icons/paths.ts`. In e2e tests, the new link locators use `exact: true`,
  because "Servicios" also matches "Mis servicios" and "Configurar servicios".
* **A14. Five tabs at 393 px** (round 3, R3-B3). At 393 px each of the five items gets about 66 px
  (393 minus `px-4`, `p-2` and four `gap-1`; `BottomNav.svelte:12-13`). With `px-3`
  (`NavItem.svelte:34`), that leaves about 42 px for a bold 12 px label. "Servicios" and "Clientas"
  are probably wider. This is arithmetic, not a measurement. The `.pen` `BottomNav` is 358 px wide,
  with four refs.
  * Fix it in the primitive (ui.md stage 8): lay out five tabs in `.pen` first, then reduce
    `NavItem` `tab`'s side padding (and give it `min-w-0` if the measurement needs it) in the same
    change.
  * A shorter `tabLabel` is the fallback only if the owner picks the word (Open Question 1).
  * Add a check that cannot miss the overflow: at 393 px, every tab's box lies inside the pill's
    box. See it red once with five tabs and the old padding. Today's probe likely misses it: the bar
    is `position: fixed`, a `flex-1` tab grows with its label, and the pill has no
    `overflow: hidden` (`probe.ts:85-173`; not run).
  * 393 px is the narrowest width checked.
  * This changes a primitive used on every compact screen, so the stage-6 screenshots cover the tab
    bar on each place.

Offline: "Mis servicios" behaves as it does today: it reads the cache, and "Nuevo servicio" is
disabled with its reason (`WriteButton.svelte:29-34`). Cotizar works fully offline.

## Round 1 findings and what the owner decision did to them

Resolved by "fixed format on the phone":

* **Gone, because there is no quote template:** B1 (templates have no kind), B6 (no Ajustes
  field), M5 (no template chosen), M7 (preview has no quote sample), M8 (render module) and M10
  (`business.name` variable).
* **B2 (server-only rendering): gone.** The text is a fixed es.json format, not a template, so the
  rejected browser renderer (Alternative E8, `release-1.md:1059`) is not reopened. Copiar works
  offline from the cache.
* **B4 (no loops in templates): gone.** The phone builds the lines.
* **M3 (screen and copy disagree): resolved.** The screen shows the copied text itself (A8, round
  2), built by one builder with `formatMoney` (`web/src/lib/money.ts`).

Where the round-1 "Still open" items went:

| Round-1 item | Now |
| --- | --- |
| B3 (iOS clipboard) | A8, and Major M10 below |
| M1 (missing price) | A2 |
| M2 (convert) | A4, and B1, B2, M1, M2 below |
| M4 (duration) | A3 |
| M6 (archived services, cap) | A6, A7 |
| M9 (order) | A5 |
| M11 (docs) | M15 below |
| M12 (goals) | M13 below |
| M13 (harness) | M14 below |
| N1, N3, N4, N6 | kept, in the staging |
| N2 | dropped (Nit 4) |
| N5 | dropped (A5) |
| N7 | "Navigation (owner decision, round 3)" |

## Analysis (round 2)

Six lenses, all completed: design, codebase-details, zoom-out, comprehension,
implementation-readiness and structural. After deduplication there are 2 Blocker, 16 Major and 9
Nit findings. Each one lists the lenses that raised it.

### Blockers

**B1. Nothing owns "clear the quote draft after the appointment saves".**
*Lenses: design, implementation-readiness (Blocker); codebase-details, comprehension, zoom-out,
structural (Major).*

The problem: `AppointmentForm.svelte:125-129` clears only its own draft, and `FormSchema`
(`booking.ts:137-145`) has no origin field. Every possible wiring has a cost:
* a query flag is lost when she reopens Nueva cita from "+";
* a marker field (an optional field is safe to parse, since `z.object` strips unknown keys) makes
  the booking form own part of the quote's lifecycle;
* clearing the quote on every save wipes a quote she never converted.

The lines are not at risk after convert, because cancelling Nueva cita does not delete `cita:new`.

**Recommendation (applied in A4):** clear the quote draft as soon as the `cita:new` write succeeds.
"Backing out keeps the quote" is dropped, and the rejected wirings are recorded in Alternatives.

**B2. A debounced write loses the race to Nueva cita, and fails silently.**
*Lenses: implementation-readiness (Blocker); design, codebase-details, zoom-out (Major).*

The problem:
* `autosave.save` writes 500 ms later and only logs on failure (`drafts.ts:21-27`).
* Nueva cita reads the draft once, in `onMount` (`formDraft.svelte.ts:19-26`).
* `putDraft` (`db.ts:317`) does not parse its input, and only the cache module may call it (guard
  `layers`).

**Recommendation (applied in A4):** add an awaited `writeDraft(key, schema, value)` to `drafts.ts`
that parses and then writes. Navigate only after it succeeds. On failure, show a toast and log
`code=quote_convert_failed`. Add a unit test with a failing Dexie stub.

### Major

**M1. A draft with "only the lines" blanks the date.**
*Lenses: design, codebase-details, comprehension, implementation-readiness, zoom-out.*

The problem: restoring the draft replaces the whole form (`AppointmentForm.svelte:71-77`). So
`day: ''` overrides the today/`dia` default (`AppointmentForm.svelte:60`), and the restore always
shows "Recuperamos lo que habías escrito." (`es.json:191`).

**Recommendation (applied in A4):** add a pure `formFromLines(lines, today): Form` in `booking.ts`
that writes today in the business zone. Accept the banner for v1. Back from Nueva cita goes to
`/cotizacion` through `volver`. *(Superseded in round 3, R3-M4: there is no `volver`; Save and Back
go to Hoy.)*

**M2. "Non-empty `cita:new`" is undefined.**
*Lenses: comprehension, design, implementation-readiness; zoom-out disagrees.*

The problem: `formDraft` saves when the form differs from an `initial` that depends on `dia`
(`formDraft.svelte.ts:28-35`). So a draft can differ only by its day. Comprehension also reads the
code as never rewriting or deleting the draft when the form returns to `initial`: if she removes
every line, the old ones come back (not verified here). Zoom-out's alternative, "a draft exists",
would prompt on a date-only draft.

**Recommendation (applied in A4):** add `isBlankForm(form)` in `booking.ts`: no client, no lines,
empty `time`, `end` null, blank notes, no attrs, with `day` ignored. Read the draft with
`getDraft(..., FormSchema)`, so an invalid draft counts as blank. Ask in an inline `Callout` with two
buttons, not in a new sheet. Check the stale-save behaviour in `formDraft`, and fix it in its own
commit if it is real. *(Round 3, R3-M5: confirmed in the code; the fix is now a fixed S1a item.)*

**M3. Archived services are in the cache, and the existing restore lets them through.**
*Lenses: all six.*

The problem: `ListServices` has no archived filter, and `AppointmentForm.svelte:74` passes every
service id to `restoreForm`. That contradicts the function's own comment (`booking.ts:357-358`: an
archived service "must not reach the request"); checked here. `ServicePicker` has no "unavailable"
look (`ServicePicker.svelte:15-17`).

**Recommendation (applied in A6):** the quote restore keeps only active, cached services, with no
new UI. Fix `AppointmentForm.svelte:74` to pass active ids only, in its own commit with a
`booking.test.ts` case. This bug exists today.

**M4. The 20-service cap is server-only and counts copies.**
*Lenses: codebase-details, implementation-readiness, comprehension, zoom-out.*

The problem: the only cap is `internal/appointments/lines.go:13,21`, applied to `serviceIds` (one id
per copy). The web has no constant and the spec has no `maxItems`. A TypeScript copy of the value
would need a drift test (maintainability.md, "one owner per shared value").

**Recommendation (applied in A7):** no cap on the quote page. The existing 422 on save covers the
rare case.

**M5. `totalOf` and the end-time sum cannot be reused as they are.**
*Lenses: design, codebase-details, zoom-out, comprehension, implementation-readiness.*

The problem: `totalOf` counts a missing price as 0 (`booking.ts:62-68`; checked), and the duration
sum is inline in `endFor` (`booking.ts:55-58`).

**Recommendation (applied in A2 and A3):** the builder makes its own priced sum and unpriced count,
and `totalOf` stays as it is. Take `durationOf` out of `endFor` in a refactor commit. Record a known
difference in the design doc: after convert, Nueva cita's "Total" counts unpriced lines as 0, so the
honest-totals goal ends at the convert.

**M6. "ServicePicker unchanged" contradicted N5 and the "unavailable" state.**
*Lenses: codebase-details, implementation-readiness, comprehension, zoom-out.*

**Recommendation (applied in A5 and A6):** keep `ServicePicker` unchanged, and drop both the label
prop and the unavailable state.

**M7. The text rules left the client-facing cases open.**
*Lenses: comprehension, design, implementation-readiness.*

The problem: in "Diseño sencillo ×2: ₡6 000" a client reads ₡6 000 as the price of one. The brief
did not cover a price of 0, "all unpriced", or the screen in those cases. "(sin lo que está a
consultar)" reads poorly.

**Recommendation (applied in A1 and A2):** show the unit price on repeats ("c/u"), adopt the A2 table
with one text for screen and copy, and use the suffix "+ lo que está a consultar". The owner may
reword it in es.json.

**M8. The example text does not match `formatMoney`.**
*Lenses: comprehension, codebase-details, design, implementation-readiness.*

The problem: CRC has exponent 2 (`money.test.ts:5-7`), so Intl very likely prints decimals, and it
groups digits with non-breaking spaces.

**Recommendation (applied in A1):** mark the example as illustrative. Tests build the expected text
with `formatMoney`, never from a typed literal. Any decimal handling specific to quotes is out of
scope, so that the quote matches Nueva cita.

**M9. One duration string cannot make "45 min" or "2 h".**
*Lenses: comprehension, design, codebase-details, implementation-readiness.*

**Recommendation (applied in A3):** use three forms (reusing `service_minutes`), and leave the line
out when the total is 0. Add unit tests for 0, 45, 120 and 150 minutes.

**M10. The clipboard fallback assumed a rejected promise, and selecting text from code.**
*Lenses: design, comprehension, implementation-readiness, codebase-details, zoom-out.*

The problem:
* `navigator.clipboard` is undefined outside a secure context, so the call throws synchronously.
* Selecting text from code after the promise settles is unreliable on iOS.
* `ui/` has no read-only multi-line primitive.
* A `console.warn` code never leaves the phone; only uncaught errors reach `POST /client-errors`.

**Recommendation (applied in A8):**
* Show the quote text as one read-only block. It is both the screen and the fallback, so one layout
  is designed, not two. The block is a new `ui/` primitive or variant, so `.pen` comes first.
* Use one handler for all three failure cases, with a unit test of each.
* Log only with `console.warn`, and do not claim the log helps triage.
* The S0 device check decides whether the fallback becomes the main path.

**M11. Missing states: empty quote, offline cold start, blank business name.**
*Lenses: comprehension, implementation-readiness, design.*

The problem: none of these states was defined. `ui.md` says a control she must not use is not
rendered. The name lives in `cache.snapshot.settings.name` (`api/schemas.ts:15`); `business.svelte.ts`
does not expose it. The server requires a non-empty name (`internal/settings/settings.go:193-196`,
per codebase-details), but the openapi spec has no `minLength`.

**Recommendation (applied in A1 and A9):** define the three states, and add the empty and busy states
to the ui-probe entries.

**M12. Module boundaries were not named.**
*Lens: design.*

**Recommendation (applied in Key Decisions 2 and 3):**
* A pure `web/src/lib/quote.ts` holds `buildQuote(lines, services, settings)` and
  `quoteText(quote)`, and calls `t()` the way `services.ts` does.
* `booking.ts` owns the `Form` helpers (`formFromLines`, `isBlankForm`, `durationOf`) and exports a
  `LineSchema`. `FormSchema` and the quote draft schema `z.object({ lines: z.array(LineSchema) })`
  are both built from it.
* `booking.ts` never imports `quote.ts`.
* The Dexie handoff lives in the route, through `lib/cache`.
* The quote page reuses `formDraft` with `draftKey('cotizacion', 'new')`.

**M13. Half the Goals had no check, and none was mapped to a stage.**
*Lenses: structural, implementation-readiness.*

The problem:
* "One tap to convert" contradicts the confirm.
* "Under 15 seconds" had no runner and no start point.
* `make cover` is Go only (`COVER_PKGS`), so the builder's coverage is not gated.

**Recommendation (applied in Goals):** each goal now has a check and a stage.

**M14. Staging missed items, and the navigation question blocks S0 without saying so.**
*Lenses: structural, implementation-readiness, design, zoom-out, comprehension.*

The problem:
* These items had no stage: the clipboard stub (`addInitScript`), the wide layout, N3, N4, N6 and
  `docs/prd.md`.
* The `.pen` frames and the ui-probe entry need the navigation answer.
* S1 was one big step, against the "small steps" rule.
* The exit criteria lacked `make e2e` (CI only), the ui.md stage 6 screenshots and the 15-second
  check.
* The S0 clipboard check had no pass or fail branch.

**Recommendation (applied in Staging):** make the owner's answer the S0 entry gate, and fix the route
path at `/cotizacion` now. Split S1 into S1a (refactors), S1b (page and Copiar) and S1c (convert),
each with concrete exits. *(Round 3: the owner answered; the route is `/servicios` (A10), and S1n is
added for the move.)*

**M15. The doc list had wrong section names, missed sections, and would rewrite history.**
*Lenses: structural, zoom-out, codebase-details, implementation-readiness.*

The problem:
* The real titles are "Phone cache and offline behavior" and "Critical User Journeys (CUJs)".
* Navigation lives in the "Screen layouts" Shell bullet (around `release-1.md:618`) and in item 71.
* Item 67 is at `release-1.md:2276`.
* Missing: items 68 (es.json), 75 (`citas/nueva` now reads a draft another page writes) and 79 (the
  e2e spec list).
* "Build order" and "Verification Plan" describe the finished P0-P7 release.
* Stale code comments: `drafts.ts:1`, `formDraft.svelte.ts:1`, `FormCard.svelte:10` (if it is
  reused), and `navItems.ts:13` (if a tab is added; round 3: it is, see R3-M6).
* Nothing says whether the clipboard adds a threat to `docs/threat-model.md`.

**Recommendation (applied in Staging S0 and S1):** `docs/designs/quick-quotes.md` owns its own
staging, CUJ and checks. release-1 gets only the statements the feature makes false, plus a link.
State in the threat model whether the clipboard is a new threat.

**M16. The brief's structure blurred owner and agent decisions.**
*Lens: structural.*

The problem:
* "Still open" repeated the agent calls.
* Key Decisions repeated the Baseline without saying who decided (technical-taste.md asks for
  agent choices to be marked).
* Alternatives lacked the options rejected by each agent call, and mixed verdict labels.

**Recommendation (applied):** the agent calls are numbered A1-A9. Key Decisions are tagged [owner] or
[agent]. Alternatives uses two verdicts and adds the agent-call alternatives.

### Nits

1. **E8 gloss inverted** (comprehension). `release-1.md:1059` is the rejected "Alternative E8:
   Rendering templates in the browser", not "one template renderer". Fixed in the round-1 section.
2. **Round-1 counts** (structural). The header claims 6 Blockers and 8 Nits, but B5 and N8 are never
   named in this brief. Check them against the round-1 report when the design doc is written.
3. **Undefined terms** (comprehension). These need a glossary or a link in the design doc: CUJ,
   ui-probe, dast, `.pen`, "the gate", and the round-1 ids.
4. **Drop the round-1 N2 confirm on "Empezar de nuevo"** (zoom-out). `formDraft.startOver` resets
   without asking on both existing forms (`formDraft.svelte.ts:53-57`), and losing a quote costs
   less than losing those.
5. **×N comes free** (zoom-out, codebase-details). `tapService` already counts, and "×{count}" exists
   as `appointment_service_count` (`es.json:189`). Reuse it.
6. **One toast** (codebase-details). "Copiado" and the failure text go through `toaster.say`
   (`toaster.svelte.ts`); release-1 bug L9 forbids a second toast.
7. **ui-probe wording** (implementation-readiness). "ui-probe entries" means `PROBES` rows in
   `ui-probe.spec.ts`, not `ui-probe.allow.ts` entries; that allow list stays empty.
8. **Rollout note** (implementation-readiness). An old cached service-worker shell does not know
   `/cotizacion` until it updates. Rollback is safe, because an old build never reads
   `cotizacion:new`. *(Round 3: the path is now `/servicios`; see round-3 Nit 10.)*
9. **"Fixed text" wording** (structural). The brief now says "fixed format (es.json strings, not
   owner-editable)".

## Analysis (round 3)

Six lenses, all completed: design, codebase-details, zoom-out, comprehension,
implementation-readiness and structural. All six gave the same verdict: "not ready to draft" for
the brief as it stood. After deduplication there are 3 Blocker, 7 Major and 11 Nit findings. Each
one lists its lenses and where its recommendation is applied. Ids are prefixed "R3-" so that they
do not collide with the round-2 ids.

### Blockers

**R3-B1. The "open on Cotizar" rule cannot work with the routes the brief chose.**
*Lenses: all six (Blocker in design, codebase-details, zoom-out, comprehension,
implementation-readiness and structural).*

The problem: the brief put the catalog at `/servicios` and the quote at `/servicios/cotizar`, and
said the place opens on Cotizar when an active service exists. `NavItem.href` is a fixed literal
union (`navItems.ts:5`), and both shells render it as is (`BottomNav.svelte:17`,
`Sidebar.svelte:57`). Every way to make the rule work breaks something:
* If `/servicios` redirects to Cotizar, the "Mis servicios" tab (also `/servicios`) bounces, so the
  catalog can never be reached once a service exists.
* If the href comes from `cache.snapshot`, the static nav table depends on the cache. It has no
  answer while the snapshot is null (cold start) and changes under her finger. It also loses the
  active highlight on the catalog, because `isActivePath('/servicios/cotizar', '/servicios')` is
  false (`nav.ts:6-8`).
* If the redirect happens only on a nav tap, a hidden "came from the nav" state is needed.

The rule's "no active service" also disagreed with the catalog's own empty test, which is
`services.length === 0`, archived ones included (`ajustes/servicios/+page.svelte:72`). The
empty-catalog case is already covered: `ServicePicker` shows "Configurar servicios" when it has no
services (`ServicePicker.svelte:24-30`).

**Recommendation (applied in A10):** Cotizar is `/servicios`, always, and the catalog is
`/servicios/catalogo`. The nav href stays a fixed literal, there is no rule that depends on the data,
and there is no redirect. The owner's intent ("she quotes far more often than she edits") is kept.
The "Mis servicios when empty" branch becomes one tap through the empty state.

**R3-B2. The rest of the brief still described the round-2 navigation.**
*Lenses: all six.*

The problem: `/cotizacion` was still the route at lines 103 (A4, `volver=/cotizacion`), 215 (M1),
361 (M14, "fix the route path at `/cotizacion` now"), 412 (Nit 8) and 510 (S1b). The S0 entry gate
was "the owner answers Open Question 1", but Open Questions said none were open, and the N7 row
pointed at the same missing question. The round header still said 2.

**Recommendation (applied):** A4 and Staging use the new routes. The round-2 M1, M14, M15 and Nit 8
carry a "superseded in round 3" note instead of being rewritten. The S0 gate is now "navigation
decided (round 3)", and the N7 row points at the navigation section. `draftKey('cotizacion', 'new')`
stays, and A10 says it is a storage key, not a route.

**R3-B3. Five tabs at 393 px are not designed, and the probe the brief relied on probably cannot see
the failure.**
*Lenses: implementation-readiness (Blocker); design, codebase-details, zoom-out, comprehension
(Major).*

The problem:
* The arithmetic gives about 66 px per tab with `px-3`, so about 42 px for a bold 12 px label
  (`BottomNav.svelte:12-13`, `NavItem.svelte:34`). "Servicios" and "Clientas" are likely wider.
  Design estimates five tabs at about 354 px against 329 px available. None of this was rendered.
* The `.pen` `BottomNav` is 358 px wide with exactly four refs, and `NavItem/Tab` has padding
  [8, 12] (`soft-glass.pen:983-1119`).
* The brief said "the ui-probe layout check proves it". Implementation-readiness reads `probe.ts`
  as unlikely to catch it (not run): the nav is `position: fixed`, so it adds nothing to the
  document's `scrollWidth`; a `flex-1` tab with `min-width: auto` grows instead of spilling; the
  pill has no `overflow: hidden`, so `clip` never fires (`probe.ts:85-173`). Target size is checked
  (`probe.ts:93-107`).
* `icons/paths.ts` has no icon for "Servicios".

**Recommendation (applied in A14, A13 and S0):** design five tabs in `.pen` first, then fix the
`NavItem` `tab` padding in the primitive. Add a containment check that is seen red once. Fall back
to a shorter `tabLabel` only with the owner's word (Open Question 1). Pick a neutral icon, `.pen`
first. Screenshot the tab bar on every place, because this changes a shared primitive.

### Major

**R3-M1. The catalog move had no stage, an incomplete file list and no exits.**
*Lenses: all six.*

The problem: moving a working screen is a behaviour change, and S1b's "the navigation entry" came
before the decision. Besides the five files the brief named (`SettingsList.svelte:22-26`,
`ServicePicker.svelte:29`, `titles.ts:17`, `settingsPage.test.ts:26` and release-1:166), the move
touches:
* the `ui-probe.spec.ts:183-190` row `/ajustes/servicios`; the spec fails when its keys and the
  routes on disk differ (`ui-probe.spec.ts:459`);
* `setup.spec.ts:107`, where CUJ 2 reaches the catalog through `openSettings(page,
  es.settings_services)`;
* `eslint-suppressions.json:67-71`, a suppression keyed by the old file path (the `rounded-pill`
  swatch at `ajustes/servicios/+page.svelte:43`, per three lenses; lint not run). The budget pins
  only the total (`suppressionBudget.guard.test.ts`, per codebase-details);
* `navItems.ts:5` (the union) and `:13` ("The four places");
* the stale comments at `services.ts:1` and `ajustes/servicios/+page.svelte:14` ("Ajustes →
  Servicios");
* `es.json:318` `settings_services`, unused afterwards;
* the `.pen`: `BottomNav` (four refs), the Ajustes screen's Servicios row (around
  `soft-glass.pen:4513`) and the Services screen frame (around `:3785`).

Checked and needing no change: guard `hasShell` (`guard.ts:50-52`), the Go SPA fallback, the
service-worker shell and `ui-probe.allow.ts` (no `/ajustes/servicios` entry).
`settingsPage.test.ts:26` still passes, but its example goes stale.

**Recommendation (applied in Staging S1n):** add a stage S1n "Servicios place, catalog moved", after
S1a and before S1b, with the full list and its own exits. The suppression is re-keyed to the new path
in the move commit, keeping the total. It is a move, not a new violation, and the commit message
says so. Fixing the swatch in a primitive is out of scope.

**R3-M2. The tab control had no primitive, no `.pen` component and no semantics.**
*Lenses: all six.*

The problem: `components/ui/` has no tabs or segmented primitive, and neither does the `.pen`. The
only similar control is Hoy's hand-made `aria-pressed` toggle in a route
(`routes/+page.svelte:225-240`). A hand-made tab row in a route breaks ui.md stage 4. An ARIA
`tablist` is the wrong pattern for two URLs. The prefix match would light up both tabs.

**Recommendation (applied in A12):** route links with `aria-current` and an exact match, as a new
`NavItem` variant `segment`, drawn in `.pen` first. Hoy's toggle is not moved.

**R3-M3. The place layout, the headings and the titles were undefined.**
*Lenses: design, codebase-details, zoom-out, comprehension, implementation-readiness.*

The problem: on wide, the catalog sits today in Ajustes' right column, next to the 280 px list
(`ajustes/+layout.svelte:27-34`). It owns its own `h1` "Servicios" (`+page.svelte:55-56`), and on
compact it gets a "Volver a Ajustes" bar (`:41-46`). The new place had no layout, no `h1` owner, no
wide arrangement and no `TITLES` keys (`titles.ts:6`). "Servicios" would appear as the place, the
`h1` and the picker label.

**Recommendation (applied in A11 and A13):** `routes/servicios/+layout.svelte` owns the `h1` and the
tabs at both widths, each page owns only its content, and the `<title>` keys are named.

**R3-M4. `volver` also controls where Save goes, so after convert and save she lands on an empty
Cotizar.**
*Lenses: design, codebase-details, comprehension.*

The problem: Nueva cita sends both `onsaved` and `oncancel` to `back`
(`citas/nueva/+page.svelte:36-41`; checked here). A4 clears the quote before it navigates, so
`volver=` the quote page returns her to an empty page after she books.

**Recommendation (applied in A4):** convert opens `/citas/nueva` with no `volver`, so Save and Back
both go to Hoy. A return to Cotizar has nothing left to show. Recorded in Alternatives.

**R3-M5. Tab switching makes the `formDraft` stale-draft bug an everyday one.**
*Lens: design (round-2 M2 had left it as "check").*

The problem: `formDraft` saves only when the form differs from `initial`, and it never clears the
draft when the form returns to `initial` (`web/src/lib/cache/formDraft.svelte.ts:28-35`; checked
here). For the quote, `initial` is `{ lines: [] }`. She removes every pick, switches to Mis
servicios and comes back: the page remounts, loads the old draft (`:19-26`), and the removed lines
return. This bug affects both existing forms too. A smaller race: the 500 ms debounced save
(`drafts.ts:21-27`) is not flushed on unmount, so a late save can land after the new instance loads
(code reading, not run).

**Recommendation (applied in S1a):** a fixed behaviour-fix commit: when `get()` equals `initial` after
a change, clear the draft. Add a unit test, and an S1b e2e test that empties the quote, switches tabs
and comes back. The 500 ms window is recorded as accepted.

**R3-M6. The stale-doc list did not cover the navigation change.**
*Lenses: codebase-details, implementation-readiness, zoom-out, structural, comprehension.*

The problem: these become false:
* `release-1.md:166`: the CUJ 2 empty state "Ajustes → Servicios". Checked here.
* `release-1.md:618`: the Shell bullet. Checked by reviewers only.
* `release-1.md:2296`: "one of the four places". Checked here.
* `release-1.md:2300`: "the same four places … (Hoy, Enviar, Clientas, Ajustes)". Checked here.
* `release-1.md:2334`: BottomNav, "Hoy, Enviar, Clientas, Ajustes". Checked here.
* `release-1.md:2372`: the empty state again. Checked here.
* `release-1.md:2386`: item 77's sub-routes. Checked by reviewers only.
* `release-1.md:2390`: the Ajustes root list. Checked here.
* `docs/prd.md`: not checked.

**Recommendation (applied in S1n):** fix exactly these in the move stage. Replace round-2 M15's hedge
"(if a tab is added)".

**R3-M7. The navigation decision did not fit the brief's own structure.**
*Lenses: structural, comprehension.*

The problem:
* The four navigation agent calls were unnumbered bullets inside an owner section.
* The owner's tab labels were tagged as an agent call.
* There were no Key Decisions, Goals, Non-Goals or Alternatives rows for the navigation, including
  the three options the owner rejected.
* The title and Overview still described only a quote page.

**Recommendation (applied):** the owner facts stay in the navigation section, and the calls are
A10-A14. The changes:
* The labels are tagged [owner].
* KD6 [owner] and KD7 [agent] are added.
* Each navigation goal has a check and a stage, and the Non-Goals bound the move.
* Alternatives has three "Rejected (owner)" rows and the agent rows.
* The title and Overview name the move.

### Nits

1. **Old path** (all six). There is no redirect from `/ajustes/servicios`, and the not-found page is
   accepted (A10). There is no `+error.svelte` (implementation-readiness).
2. **Offline "Mis servicios"** (all six). The behaviour is unchanged: it reads the cache, and
   "Nuevo servicio" is disabled with its reason (`WriteButton.svelte:29-34`). Cotizar is fully
   offline. `offline.spec.ts` does not visit the catalog; an offline e2e for it is optional. Stated
   after A14.
3. **"Servicios" three times** (zoom-out, comprehension, design). The place label, the `h1` and the
   picker label (`es.json:184`) all say "Servicios". Accepted for v1 without a label prop (A5).
4. **Locators** (comprehension). Use `exact: true` and give the tab `<nav>` its own `aria-label`
   (A12, A13).
5. **Two empty states on Cotizar** (structural, implementation-readiness). Only `ServicePicker`'s
   empty state shows when there is no service (A9). It gives the Cotizar `PROBES` row its empty
   text.
6. **Narrower phones** (design, codebase-details). 393 px is the narrowest width checked (A14).
7. **The 15-second goal's start** (comprehension). It now starts from the Cotizar tab open (Goals).
8. **Ajustes after the move** (comprehension). It loses its Servicios row and is otherwise
   unchanged; wide still opens on Negocio (Non-Goals).
9. **`nav.test.ts`** (codebase-details). Add a case for `/servicios` and its two subpaths (A10).
10. **Rollout** (implementation-readiness, design). An old cached shell runs old JS that knows only
    `/ajustes/servicios`, and it is consistent until she taps the update prompt
    (`service-worker.ts:20`, per implementation-readiness). Rollback is safe: there is no data
    change, and an old build never reads `cotizacion:new`.
11. **Tab order** (zoom-out). The order follows the owner's wording, "Mis servicios" then "Cotizar",
    and the default (Cotizar) is the second tab. Stated in A12; the owner may swap them.

## Goals vs. Non-Goals

**Goals (v1)** (round 2: every goal has a check and a stage, M13; round 3: navigation goals,
R3-M7)

* **Answer a WhatsApp price question in seconds.** Check: on her iPhone, starting from the Cotizar
  tab already open, a 3-service quote is copied and pasted into an open WhatsApp chat in under 15
  seconds, 3 times out of 3. The owner runs it as an S1c exit item.
* **Works offline.** Check: an e2e test with the server stopped, following the `offline.spec.ts`
  pattern: Cotizar builds the text and Copiar succeeds against the stub (S1b).
* **No client needed to quote.** Check: the CUJ e2e test copies a quote without any client data
  (S1b).
* **Convert without losing an unsaved booking:** one tap, plus a confirm when a non-blank booking
  draft exists. Check: e2e tests for convert into a blank draft and over a non-blank one, a builder
  test for `isBlankForm`, and a unit test for a failed write (S1c).
* **Honest totals in the quote.** A missing price is visible on screen and in the copied text, and
  is never summed as 0. Check: builder unit tests for the A2 table (S1b). Nueva cita's total is a
  known difference (M5).
* **Every action ends in visible feedback.** Check: e2e tests for Copiar success and failure with
  the `addInitScript` clipboard stub (missing API and rejection), and for a convert-write failure
  (S1b, S1c).
* **Five places fit.** Check: at 393 px, the containment check (every tab inside the pill, seen red
  once) and the ui-probe target size; at 1440 px, the sidebar in the ui-probe wide run (S1n).
* **The catalog works the same after the move.** Check: CUJ 2 in `setup.spec.ts` passes through the
  new nav item and `/servicios/catalogo` (S1n).
* **Servicios opens on Cotizar.** Check: an e2e test taps Servicios and lands on `/servicios` with
  Cotizar current, and the Mis servicios tab reaches the catalog (S1b; in S1n, before Cotizar
  exists, see S1n).
* **A removed quote stays removed.** Check: the `formDraft` unit test, and an e2e test that empties
  the quote, switches tabs and comes back to an empty quote (S1a, S1b).

**Non-Goals (v1)**

* Storing quotes on the server, quote history or follow-ups.
* An editable quote template; per-line price edits, non-catalog items, discounts.
* Client data in a quote; requiring a client.
* Sending through `wa.me`.
* Freezing prices at quote time. The draft follows the catalog, and the appointment copies prices
  when it is saved.
* Keeping the quote after convert. Its lines live on in `cita:new` (round 2, B1).
* A cap or an "unavailable" state on the quote page (round 2, A6 and A7).
* A quote-specific banner in Nueva cita after convert (round 2, A4).
* Any other change to Ajustes beyond removing its Servicios row (round 3).
* Any change to the catalog screen's behaviour; only its place, its `h1` and its links change
  (round 3).
* A redirect from `/ajustes/servicios` (round 3, A10).
* Moving Hoy's Día/Semana toggle onto the new tab control (round 3, A12).

## Alternatives Weighed

| Option | Verdict | Reason |
| --- | --- | --- |
| Editable quote template (template kind, Ajustes setting, server render endpoint) | Rejected (owner) | Needs a migration, a variable set per kind, a settings field, a new module and an endpoint. Copiar fails when the PC is off, and the copy happens after an await, which risks breaking the iOS clipboard rule. |
| "Copiar cotización" button on Nueva cita, no separate page | Rejected (owner) | A quote would overwrite an unfinished booking draft; the owner wants a separate page. |
| Store quotes server-side | Rejected (owner) | No reader and no follow-up flow; it adds a table, export work and cleanup. |
| Per-line price override | Rejected (owner) | Appointments copy catalog prices with no per-line override; she types a one-off in WhatsApp. |
| Require a client | Rejected (owner) | Most askers are unknown numbers. |
| Quote entry next to "Nueva cita" (sidebar button, no new place) | Rejected (owner, round 3) | The owner chose a fifth place, Servicios. |
| A fifth tab only for quotes, catalog left in Ajustes | Rejected (owner, round 3) | The owner wants the catalog and the quote together under Servicios. |
| A choice inside the "+" button (cita or cotización) | Rejected (owner, round 3) | The owner chose a fifth place, Servicios. |
| Several quote templates | Rejected (agent) | YAGNI. |
| `wa.me` send instead of copy | Rejected (agent) | Without a number it opens a contact picker, not the open chat. |
| Pass services to Nueva cita in a query param | Rejected (agent) | A second outside-input parser. It also loses to the draft load order. |
| Built-in default text with nail wording ("puede variar según el diseño") | Rejected (agent) | Breaks "no nail value in code"; she types it after pasting. |
| Clear the quote after the appointment saves, through a `FormSchema` origin field | Rejected (agent, round 2) | Makes the booking form own part of the quote's lifecycle, for little gain: `cita:new` already keeps the lines. |
| Clear the quote after save, through a `?de=cotizacion` flag | Rejected (agent, round 2) | The flag is lost when she reopens Nueva cita from "+". |
| Clear the quote on every new-appointment save | Rejected (agent, round 2) | Wipes a quote she never converted. |
| Write `cita:new` through the debounced `autosave` | Rejected (agent, round 2) | Navigation can beat the 500 ms write, and a failure is silent (B2). |
| Merge into an existing `cita:new` instead of asking to replace | Rejected (agent, round 2) | Mixes two bookings' services without her seeing it. |
| Count an unpriced line as 0 (`totalOf` as is) | Rejected (agent) | Breaks "honest totals". |
| Catalog order instead of pick order | Rejected (agent) | `tapService` already keeps pick order; a second order is new code. |
| "Unavailable" chip for archived lines | Rejected (agent, round 2) | A new UI state (`.pen` frame, probe state) for a rare case; the restore drops them instead (A6). |
| Client-side 20-service cap | Rejected (agent, round 2) | A TypeScript copy of a Go constant that needs a drift test; the server already refuses at save (A7). |
| A list layout plus a separate fallback text field | Rejected (agent, round 2) | Two layouts to design and probe; one read-only text block serves both purposes (A8). |
| A confirm sheet for replacing the draft | Rejected (agent, round 2) | A new sheet needs a frame, focus and inert handling; an inline `Callout` is lighter (M2). |
| Freeze prices at quote time | Rejected (agent) | The draft follows the catalog; the appointment copies prices on save. |
| Open on Cotizar only when an active service exists (redirect, or a nav href from the cache) | Rejected (agent, round 3) | Breaks the catalog tab or ties the static nav to the cache, with no answer on a cold start (R3-B1). `ServicePicker`'s empty state already links to the catalog. |
| Catalog at `/servicios`, Cotizar at `/servicios/cotizar` | Rejected (agent, round 3) | The default tab must be the place root for a fixed nav href (A10). |
| One route with `?tab=` | Rejected (agent, round 3) | A query parameter is a second outside input to parse; two routes give each tab its own title, probe row and URL. |
| Redirect `/ajustes/servicios` to the new path | Rejected (agent, round 3) | One owner and links only from inside the app; a redirect route is code and a probe concern for no reader (A10). |
| ARIA `tablist` for the two tabs | Rejected (agent, round 3) | The tabs are two URLs; `tablist` implies in-page panels and arrow-key handling (A12). |
| Hand-made tab row in the route | Rejected (agent, round 3) | Breaks ui.md stage 4 (surface and radius classes only in `ui/`) (A12). |
| `volver` set to the Cotizar tab after convert | Rejected (agent, round 3) | Save also goes to `volver`, and the quote is already cleared, so she would land on an empty page (R3-M4). |
| A shorter tab label instead of fixing the tab padding | Rejected (agent, round 3) as the first choice | ui.md stage 8 fixes the primitive; a label change needs the owner's word (Open Question 1). |

## Key Decisions & Staging for Next Steps

1. **[owner]** No server change: no migration, no new endpoint, no openapi change.
2. **[agent]** One pure builder in `web/src/lib/quote.ts` (`buildQuote`, `quoteText`), used by both
   the screen and Copiar. Strings live in es.json (A1-A3, M12).
3. **[agent]** `booking.ts` owns `LineSchema`, `formFromLines`, `isBlankForm` and `durationOf`. The
   quote draft is `draftKey('cotizacion', 'new')` through `formDraft`. Convert writes a full
   `cita:new` through an awaited `writeDraft`, then clears the quote, then opens `/citas/nueva` with
   no `volver` (A4, B1, B2, R3-M4).
4. **[agent]** The clipboard write runs synchronously inside the tap. The read-only text block
   doubles as the fallback (A8). Checked on her iPhone first.
5. **[agent]** Archived lines are dropped on restore; there is no cap on the quote page (A6, A7).
6. **[owner]** A fifth place "Servicios" between Clientas and Ajustes, with the tabs "Mis servicios"
   and "Cotizar"; the catalog moves out of Ajustes (round 3).
7. **[agent]** Cotizar is `/servicios` and the catalog is `/servicios/catalogo`, with no data-based
   default and no redirect. A `servicios/+layout.svelte` owns the `h1` and the tab links, which are
   a new `NavItem` `segment` variant with an exact match. The five-tab fit is fixed in the `NavItem`
   primitive and checked by a containment check. The move ships as its own stage (A10-A14, R3-B1,
   R3-B3, R3-M1-M3).

Staging (round 2: entry gate, split S1, concrete exits, M14; round 3: S1n and the round-3 items):

* **S0. Decide and check.**
  * Entry: navigation decided (round 3). Open Question 1 is answered only if the S0 frame shows that
    "Servicios" does not fit after the padding fix.
  * Device check: on her installed PWA, Copiar pastes into WhatsApp. Record the result and the iOS
    version in `quick-quotes.md`. On a fail, the press-and-hold text block becomes the main path.
  * Then write `docs/designs/quick-quotes.md`, with its own staging, CUJ, checks, the A2 table and
    the known differences (M5, M1 banner).
  * In release-1, fix only the statements this feature makes false (M15, R3-M6). Fix `docs/prd.md`.
    State in the threat model whether the clipboard is a new threat.
  * Design the `.pen` frames:
    * `BottomNav` with five refs and the new icon, and `NavItem/Tab` with the reduced padding,
      measured at 393 (A14);
    * the sidebar with five rows;
    * the `NavItem` `segment` variant (A12);
    * Ajustes without its Servicios row;
    * the Servicios place at 393 and 1440, with each tab;
    * the quote page at 393 and 1440 (wide is not the phone layout stretched);
    * the text block primitive;
    * the empty, busy and confirm states.
* **S1a. Refactors only, no behaviour change.** Each item is its own commit:
  * rename `quote` at `AppointmentForm.svelte:56`;
  * export `LineSchema`;
  * extract `durationOf` from `endFor`;
  * add `writeDraft`;
  * add `'cotizacion'` to `draftKey` (`drafts.ts:8`);
  * update the comments at `drafts.ts:1` and `formDraft.svelte.ts:1`.

  Then two separate behaviour fixes, each with a test:
  * `AppointmentForm.svelte:74` passes active ids only;
  * `formDraft` clears the draft when the form returns to `initial` (R3-M5).

  Exit: the gate.
* **S1n. Servicios place, catalog moved** (round 3, R3-M1). This stage does not add Cotizar. In this
  stage `/servicios` is the moved catalog page: the nav href is `/servicios` from day one, and S1b
  moves the catalog to `/servicios/catalogo` and adds the tab links when Cotizar arrives. So no
  one-tab strip ships. The work:
  * `NAV_ITEMS`, its `href` union and the comment at `navItems.ts:5,13`; `nav_services`; the icon;
  * the `NavItem` tab padding fix and the containment check;
  * the five-item `BottomNav` and the sidebar;
  * `routes/servicios/+layout.svelte` with the `h1`;
  * the catalog page moved, without its own `h1`; its comment at `:14` and `services.ts:1` updated;
  * `TITLES` (`titles.ts:17`);
  * the `SettingsList` row and `settings_services` removed;
  * the `ServicePicker.svelte:29` link;
  * the `settingsPage.test.ts:26` example switched to another section;
  * the `ui-probe.spec.ts:183-190` row re-keyed;
  * CUJ 2 in `setup.spec.ts:107` through the new nav item;
  * the `eslint-suppressions.json:67` key re-keyed;
  * a `nav.test.ts` case;
  * the release-1 lines in R3-M6.

  Exits:
  * the gate, `make e2e`, and `make image image-scan dast` (what the app serves changes);
  * ui-probe green at both widths;
  * ui.md stage 6 screenshots at 393 × 852 and 1440 × 900 of the tab bar on every place, the
    sidebar, Servicios and Ajustes.
* **S1b. Cotizar, builder, Copiar.** The work:
  * `/servicios` becomes Cotizar and the catalog moves to `/servicios/catalogo` (titles, probe row,
    the `ServicePicker` link and CUJ 2 follow);
  * the `NavItem` `segment` tab links with an exact match, and `quote_title`;
  * `buildQuote`, `quoteText` and the restore filter;
  * the empty and busy states;
  * Copiar success and failure;
  * the `PROBES` rows for both tabs at both widths;
  * the CUJ e2e test with the clipboard stub;
  * the "Servicios opens on Cotizar" and "removed quote stays removed" e2e tests.

  Exit:
  * the gate, `make e2e`, and `make image image-scan dast` (the app serves a new page);
  * ui.md stage 6 screenshots at 393 × 852 and 1440 × 900.
* **S1c. Convert.** Covers `formFromLines`, `isBlankForm`, the inline confirm, the write and its
  failure, the quote clear and the navigation to `/citas/nueva` with no `volver`. Exit:
  * the gate and `make e2e`;
  * e2e tests for convert into a blank and a non-blank draft, and for Save landing on Hoy;
  * the owner's 15-second check.

  Afterwards, move this brief to `docs/designs/history/`.

## Open Questions for the Owner

1. **Only if the S0 frame shows that "Servicios" does not fit at 393 px after the tab padding fix:**
   which shorter tab-bar label do you want for the place? The sidebar keeps "Servicios" either way,
   as "Enviar" (`tab_send`) does for "Para enviar". The label is a Spanish UI word the owner picks,
   and it changes what is built.
