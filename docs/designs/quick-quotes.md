# Servicios place and quick quotes

*   **Author:** jnaranj09
*   **Status:** Proposed
*   **Date:** 2026-11-07
*   **Predecessor:** [quick-quotes-brief.md](history/quick-quotes-brief.md) (three brainstorm rounds;
    moved there in stage S0)
*   **Review:** round 1 reports in `history/review-quick-quotes/round-1/` (moved there in stage S0)
*   **Builds on:** [release-1.md](release-1.md) (the app as built, P0-P7)

--------------------------------------------------------------------------------

## Overview

open-customer serves one owner of a small service business on an iPhone PWA. Clients often ask
her for a price on WhatsApp before they book. This design adds a quote page, **Cotizar**: she picks
services from her own catalog and the page shows the reply text, with each line and its price, the
total and the total duration. One button copies that text so she can paste it into the WhatsApp
chat that is already open. A second button turns the same services into a new appointment.

The quote page lives in a new fifth top-level place, **Servicios**. The service catalog moves there
from Ajustes, so the place has two tabs: **Cotizar** and **Mis servicios** (the catalog, as it
works today).

Two words are used in a fixed sense in this document: a **place** is one entry of the tab bar
(compact) or the sidebar (wide); a **tab** is one of the two links inside the Servicios place.

The feature runs on the phone. It changes no server code, no database table and no API route.

## The Problem

A client writes "¿Cuánto sale acrílico con diseño y pedi?". Today she answers in one of two ways
(an agent's description of a typical small studio, not measured; the owner may correct it):

*   She types the prices from memory. A wrong price is then a promise she either keeps at a loss or
    takes back in front of the client.
*   She opens Ajustes → Servicios, reads each price, adds them up, and types the reply, while she
    may have a client in the chair.

The app already holds the exact data the reply needs: each service's name, price and duration.
Nothing in the app puts that data into a message. When the client then says "ok, el sábado", she
books the appointment from scratch in Nueva cita and picks the same services a second time.

The service catalog also sits in Ajustes, one level down, next to settings she touches once. After
setup she reads the catalog far more often than she edits it, and quoting makes that worse.

**Proposed solution:** a Cotizar page that builds the reply from the catalog and copies it in one
tap. A convert button carries the same services into Nueva cita. Both pages sit in a Servicios
place one tap from every screen.

## Goals

Each goal has a check, and the stage that proves it (S0 to S4, Technical Plan section 9).

1.  **Answer a WhatsApp price question in seconds.**
    Check (S4, the owner): on her iPhone, starting with the Cotizar tab open, she copies a 3-service
    quote and pastes it into an open WhatsApp chat in under 15 seconds, 3 times out of 3.
2.  **The copied text is correct and does not hide a missing price.** Every line shows the catalog
    price. A service with no price says "precio a consultar" and is never counted as 0. The total
    says when it leaves something out. A repeated service shows the price of one copy.
    Check (S3): unit tests of `quoteText` for each row of the missing-price table, with the
    expected amounts built through `formatMoney`, never typed by hand.
3.  **No client is needed.** Most people who ask are unknown numbers. She quotes without creating
    or choosing a client.
    Check (S3): the e2e journey runs after `resetData()` (no client exists) and copies a quote.
4.  **Works with the PC off.** The quote uses only the catalog and settings already on the phone.
    Check (S3): an e2e test with the API cut off (`serverOff`) builds the text and copies it.
5.  **Convert without retyping and without losing work.** One tap carries the services, in the
    order she picked them, into Nueva cita. If an unsaved booking is already there, she is asked
    before it is replaced.
    Check (S4): e2e tests for convert into an empty Nueva cita and over an unsaved one; unit tests
    of `convertQuote` for all three results, including a failed write.
6.  **Every tap ends in something she can see.** After a copy the Copiar button itself shows
    a check icon and "Copiado" for about 2 seconds (no toast). A message on the page and a manual way to copy if
    the phone refuses. A message on the page if convert fails.
    Check (S3, S4): e2e tests with a stubbed clipboard (success shows "Copiado" on the button;
    missing API, synchronous throw, rejection show `quote_copy_failed`); the convert failure
    through the `convertQuote` unit test.
7.  **Servicios is one tap from every screen and opens on Cotizar.**
    Check (S3): e2e test: tap Servicios, land on Cotizar; the Mis servicios tab reaches the catalog.
8.  **Five places fit on her phone.**
    Check (S2): at 393 px the new probe check `tab-fit` finds every tab inside the tab bar's content
    box, and each tab keeps a 44 px tap target; at 1440 px the sidebar shows five rows. The check
    was proven able to fail by forcing an overflow (section 6).
9.  **The catalog works exactly as before after the move.**
    Check (S2): the setup journey (release-1 CUJ 2) passes through the new place.
10. **A removed quote line stays removed.** Today an emptied draft comes back when the form opens
    again (the save effect in `web/src/lib/cache/formDraft.svelte.ts` never clears); with tabs she
    would hit this daily.
    Check (S1): unit tests of the pure `draftStep` rule and an e2e test on Nueva cita, seen red
    before the fix; (S3) an e2e test that empties the quote, switches tabs and comes back.

## Non-Goals

*   **Saving quotes.** No quote list, history, expiry or follow-up. The open quote is a draft on the
    phone only. When the client comes back days later, the chat history holds the old quote and a
    new one takes seconds.
*   **An editable quote message.** The text is a fixed format whose words live in `es.json`. She
    adds anything else (a discount, "el precio puede variar") in WhatsApp after pasting.
*   **Changing prices in a quote.** No per-line price, no item outside the catalog, no discount.
    An appointment copies catalog prices and cannot hold a different one, so a changed quote price
    would be lost at convert. Extras she charges often become services ("Diseño sencillo").
*   **Client data in a quote,** or requiring a client.
*   **Sending through `wa.me`.** Without a saved number it opens a contact picker, not the chat she
    already has open.
*   **Freezing prices.** The quote follows the current catalog; the appointment copies prices when
    it is saved, as today.
*   **Keeping the quote after convert.** Its services live on in the Nueva cita draft.
*   **A cap on quote lines.** The existing 20-service limit still applies when the appointment saves.
*   **A redirect from the old path** `/ajustes/servicios`.
*   **New UI primitives.** The tab links, the text block, the Cotización panel and the wide tables
    with their steppers are built from the existing `Card`, `IconButton` and `TextButton` and from
    layout, border and text classes.
*   **Any other change** to Ajustes (it only loses its Servicios row) or to how the catalog screen
    behaves (only its place, heading and links change).

## Open Questions

*   **Five tabs at 393 px.** Settled in S0 (section 10): with 4 px side padding every label fits,
    "Servicios" included, so no shorter label is needed.
*   **The iOS clipboard.** Copying inside the tap should work in the installed PWA, but it is not
    tested on her iPhone yet. The owner's S0 test is still pending (section 10); section 3 says
    what is built for each result.

## Critical User Journeys (CUJs)

All data below is made up.

### CUJ Q1: Quote a price question and paste it

*   **Given**: The catalog has "Acrílico" (₡15 000, 90 min), "Diseño sencillo" (₡3 000, 15 min) and
    "Cejas" (no price, 30 min). The business name is "Estudio Ejemplo". No client is involved.
*   **When**:
    1.  A new number writes on WhatsApp: "¿Cuánto sale acrílico con dos diseños y cejas?"
    2.  She opens the app and taps **Servicios**. It opens on **Cotizar**.
    3.  She taps Acrílico, Diseño sencillo twice and Cejas.
    4.  She taps **Copiar**, switches to WhatsApp and pastes.
*   **Then**:
    *   The page shows the same text she pastes, line by line.
    *   "Diseño sencillo" shows as one line with ×2 and the price of one copy.
    *   "Cejas" says "precio a consultar", and the total says it leaves something out.
    *   The duration line says "2 h 30 min".
    *   The Copiar button shows a check icon and "Copiado" for about 2 seconds.
    *   The quote stays on the page. If she leaves and comes back, it is still there.

### CUJ Q2: The client books from the quote

*   **Given**: The quote from CUJ Q1 is open. Nueva cita has no unsaved draft.
*   **When**:
    1.  The client answers "ok, el sábado a las 10".
    2.  She taps **Convertir en cita**.
    3.  Nueva cita opens with the same services, in the same order, and today's date.
    4.  She creates or picks the client, sets Saturday 10:00 and saves.
*   **Then**:
    *   The appointment is saved with the catalog prices of that moment, as any new appointment.
    *   The app lands on Hoy.
    *   Cotizar is empty the next time she opens it.
    *   If she had backed out of Nueva cita instead of saving, the services would still be in the
        Nueva cita draft.

### CUJ Q3: Convert while a booking is half written

*   **Given**: She started a Nueva cita for Ana (client and services chosen, not saved). Then a
    price question came in and she built a quote.
*   **When**: She taps **Convertir en cita**.
*   **Then**: Cotizar asks, on the page, whether to replace the unsaved booking.
    *   "Reemplazar": the Nueva cita draft gets the quote's services, and she continues as in CUJ Q2.
    *   "Cancelar": nothing changes. Both the quote and Ana's draft stay.

### CUJ Q4: The phone refuses the copy

*   **Given**: A quote is open, and iOS refuses the clipboard write.
*   **When**: She taps **Copiar**.
*   **Then**: A message above the text tells her to press and hold the text to copy it by hand. It
    stays until her next pick, copy or convert. The text can be selected the normal iOS way. Nothing
    is logged except a code.

## Technical Plan (Architecture)

### What changes, in one picture

```text
Phone (SvelteKit PWA). The server does not change.

Tab bar / sidebar:  Hoy · Enviar · Clientas · Servicios · Ajustes
                                                 │
                         routes/servicios/+layout.svelte
                         (h1 "Servicios" + tab links: Cotizar, Mis servicios)
                                 │                         │
                   /servicios  (Cotizar)          /servicios/catalogo
                   +page.svelte                   (catalog page, moved from Ajustes)
                   {#if cache.snapshot}
                        QuoteForm.svelte
                          ServicePicker ──▶ lines (quote draft, 'cotizacion:new')
                          quoteText(lines, services, settings) ──▶ text on screen
                          Copiar ──▶ navigator.clipboard.writeText(text)
                          Convertir ──▶ convertQuote(lines, today, replace)
                                          │ reads / writes 'cita:new' in Dexie
                                          ▼
                                 'ok' → clear quote draft → goto /citas/nueva
                                        (Nueva cita restores its draft, as today)
```

Nothing new goes over the network. The quote reads only what `/bootstrap` already put in the Dexie
cache. Convert hands off through the same Dexie draft that Nueva cita already reads when it opens.

### 1. The quote text (`web/src/lib/quote.ts`, new)

A pure module with no Svelte, no Dexie and no network. It exports one function:

*   `quoteText(lines, services, settings): string`, where `settings` is
    `{ name, currency, locale }` from `cache.snapshot.settings`. Internally it turns each line into
    a row (name, count, unit price in minor units or none, line total), sums the priced lines, and
    counts the unpriced ones. A line whose service is not in `services` is skipped. The row type
    stays private.
*   Every word and symbol comes from `es.json` through `t()`, the way `lib/services.ts` does. Money
    goes through `formatMoney(minor, currency, locale)` (`lib/money.ts`), so the quote prints
    amounts exactly as Nueva cita does.

The screen shows the result of `quoteText`, and Copiar copies the same string, so the screen and the
paste cannot disagree.

**Text format.** The lines in order:

1.  A heading: `quote_heading_named` "Cotización · {name}", or `quote_heading` "Cotización" when
    the business name is blank.
2.  One line per picked service, in the order she first picked each one:
    *   once: `quote_line` "• {name}: {price}"
    *   repeated: `quote_line_repeat` "• {name} ×{count} ({unit} c/u): {total}"
    *   no price, once: `quote_line` with `{price}` = `services_price_unset` ("precio a
        consultar", the key the catalog already uses); repeated: `quote_line_unpriced_repeat`
        "• {name} ×{count}: {price}", the same `{price}`
3.  A total line, from the table below. It is left out when no line has a price.
4.  A duration line: `quote_duration` "Duración aproximada: {duration}".

Illustrative only (the real amounts are whatever `formatMoney` prints; for CRC that likely
includes decimals and non-breaking spaces):

```text
Cotización · Estudio Ejemplo
• Acrílico: ₡15 000,00
• Diseño sencillo ×2 (₡3 000,00 c/u): ₡6 000,00
• Cejas: precio a consultar
Total: ₡21 000,00 + lo que está a consultar
Duración aproximada: 2 h 30 min
```

**Missing prices.** A price of 0 counts as a price and prints `formatMoney(0, …)`.

| Lines | Total line |
| --- | --- |
| all priced | `quote_total` "Total: {amount}" |
| some without a price | `quote_total_partial` "Total: {amount} + lo que está a consultar" |
| none priced | no total line |

**Duration.** The sum of each line's `duration_min` times its count. Nueva cita uses the same sum
for the end time; it moves out of `endFor` into a shared `durationOf(lines, services)` in
`lib/booking.ts`. Formats: under 60 minutes `service_minutes` "{n} min"; whole hours
`duration_hours` "{h} h"; otherwise `duration_hours_minutes` "{h} h {m} min". The catalog requires
1 to 1440 minutes per service (`internal/services/services.go`), so a quote with lines never has 0
minutes, and the page shows no text when there are no lines.

### 2. The Cotizar page (`/servicios`)

*   **Mounting.** `routes/servicios/+page.svelte` shows the busy line `services_loading`
    ("Cargando servicios…") while `cache.snapshot` is null, then mounts `QuoteForm.svelte`, the way
    Nueva cita mounts `AppointmentForm` only once the snapshot exists. The form's draft therefore
    always restores against a loaded catalog.
*   **Picking services.** `QuoteForm` gets only active services (`archived` false). On the phone it
    uses `ServicePicker`, unchanged; on wide it uses the catalog table (Layout, below). Both work on
    the same `Line[]` (`service_id`, `count`) with the `tapService` / `removeService` that Nueva
    cita uses.
*   **The draft.** The open quote is a Dexie draft under `draftKey('cotizacion', 'new')`, through
    the same `formDraft` lifecycle the two existing forms use. Its schema is
    `QuoteDraftSchema = z.object({ lines: z.array(LineSchema) })`, in `quote.ts`; `LineSchema` is
    exported from `booking.ts` and also used by Nueva cita's `FormSchema`. On restore, lines whose
    service is not in `activeServiceIds(services)` are dropped, so an archived service never shows
    as picked. Cotizar shows no restored-draft note.
*   **Starting over.** While there are lines, a "Nueva cotización" button (`quote_new`) calls the
    draft's `startOver()`, which empties the quote without asking, as "Empezar de nuevo" does on the
    other forms.
*   **The text block.** The quote text in a `Card` rendered as a paragraph (`as="p"`, `pad="md"`)
    with the layout classes `whitespace-pre-line select-text`. It can be selected with the normal
    iOS press-and-hold, so it is both the preview and the manual way to copy. On wide the
    Cotización panel is the `Card` and the text is a plain paragraph inside it.
*   **The message slot.** One `Callout` above the text block shows at most one message, from a
    tagged union `none | confirm_replace | copy_failed | convert_failed`. The two failures use
    `role="alert"`. The slot goes back to `none` on her next pick, copy or convert.
*   **States.**
    *   Loading: while `cache.snapshot` is null, only the page's `services_loading` status line,
        at both widths (Mounting, above). `QuoteForm` is not mounted yet. The `cargando · 1440`
        frame draws the line inside the table; the build shows the page line instead.
    *   No active service: `QuoteForm` renders the empty state itself, at both widths, with the
        existing `EmptyState` and the keys `ServicePicker` uses (`appointment_services_empty_title`,
        `appointment_services_empty_body`, `appointment_services_empty_action` "Configurar
        servicios"), linking to the Mis servicios tab (`/servicios/catalogo`). Neither
        `ServicePicker` nor the table is rendered. On wide the empty state sits where the table
        would be, and the panel shows `quote_empty`.
    *   Services but nothing picked: the text `quote_empty`; Copiar, Convertir and Nueva cotización
        are not shown.
    *   Offline: everything works. Copy needs no network; convert writes only to Dexie.
*   **Layout** (S0 frames `Servicios · Cotizar · <state> · 393` and `· 1440` in `soft-glass.pen`).
    *   Phone: `ServicePicker`, the message slot, the text block, then Copiar and Convertir en cita
        side by side, then Nueva cotización (a `TextButton`) under them.
    *   Wide: two columns, top-aligned in one grid. Left, the catalog as a table in a `Card` with
        the columns Servicio, Duración and Precio ("precio a consultar" when there is none) and a
        stepper per row: "−", the count and "+" (`IconButton` variant `accent`; a row not picked
        shows only "+"). "+" calls `tapService` and "−" calls `removeService`, as the chips do.
        Right, a fixed-width (400 px) Cotización panel (a `Card`): the message slot at its top, the
        text, Copiar and Convertir en cita at full width, and Nueva cotización at the bottom.
    *   The empty and nothing-picked states keep the same wide frame: the panel shows
        `quote_empty`, and the table's place shows the empty state or the rows with "+" only.

### 3. Copiar

The text is already built (a `$derived` value) when she taps. The click handler calls
`navigator.clipboard.writeText(text)` first, in the same tap, with no `await` before it. iOS allows
a clipboard write only while it is handling the tap.

One failure path covers three cases: `navigator.clipboard` missing (not a secure context), the call
throwing at once, and the promise rejecting. On failure the message slot shows `copy_failed`
(`quote_copy_failed`: press and hold the text to copy it), and the app logs
`console.warn('code=clipboard_failed')`, never the text. On success there is no toast (owner, S0):
the Copiar button's own label changes to a check icon and "Copiado" (`quote_copied`) for about 2
seconds, then goes back to "Copiar".

**If the S0 iPhone test fails:** Copiar still ships and still tries, and a permanent hint
(`quote_hold_hint`) sits above the text block telling her to press and hold to copy. The frame for
that case is drawn in S0 either way. The test result is recorded in section 10.

### 4. Convertir en cita

The hand-off is one function in `web/src/lib/cache/quoteHandoff.ts` (new):

`convertQuote(lines, today, replace): Promise<'needs_confirm' | 'ok' | 'failed'>`

1.  Read the current Nueva cita draft (`cita:new`) with `getDraft(key, FormSchema)`. A draft that
    fails to parse counts as none.
2.  If a draft exists and `replace` is false, return `'needs_confirm'`.
3.  Write `{ ...blankForm(today), lines }` with `putDraft`, after parsing it with `FormSchema`,
    awaited. `blankForm(day)` is the empty Nueva cita form, extracted from `AppointmentForm` into
    `booking.ts` so both use one definition. The existing 500 ms `autosave` is not used: Nueva cita
    reads its draft once, when it opens, and could open before a delayed write lands.
4.  Return `'ok'`, or `'failed'` if the write throws (logging `code=quote_convert_failed`).

The page wires it, inside `submitOnce` (`lib/submit.ts`) so a second tap while it runs does nothing:

*   `'needs_confirm'`: the message slot shows `confirm_replace` with "Reemplazar" (`quote_replace`,
    calls again with `replace` true) and "Cancelar" (the existing `cancel` key).
*   `'ok'`: clear the quote through the draft's own `clear()` (it also cancels a pending 500 ms
    save, so the quote cannot be written back after the clear), then
    `goto(resolve('/citas/nueva'))` with no `volver`.
*   `'failed'`: the message slot shows `convert_failed`; the quote stays.

`today` is `clock.today(settings.timezone)`, as `routes/citas/nueva/+page.svelte` computes it.

Why no `volver`: Nueva cita sends both Save and Back to `volver`. The quote is already cleared, so
going back to Cotizar would show an empty page. With no `volver`, both go to Hoy, the default.

Nueva cita needs no change for this. It opens, finds its draft, restores it with `restoreForm`, and
shows its usual "Recuperamos lo que habías escrito." note (see Risks).

### 5. The Servicios place and its two tabs

*   **Routes.**
    *   `/servicios` is Cotizar, always.
    *   `/servicios/catalogo` is Mis servicios: today's `/ajustes/servicios` page, moved once, with
        the same behaviour.
    *   `/ajustes/servicios` is removed with no redirect. Links to it exist only inside the app, and
        all of them change.
    *   There is no rule that picks the tab from the data. First setup goes Servicios → Cotizar's
        empty state → "Configurar servicios" → the catalog.
*   **Layout.** A new `routes/servicios/+layout.svelte` owns the header at both widths: the `h1`
    ("Servicios", `services_title`), an action slot, and the tab links. Each tab page owns only its
    content; a page that has a primary action registers it in the slot through a context the layout
    sets (`routes/servicios/header.svelte.ts`: the layout calls `provideHeaderAction()`, a page
    calls `useHeaderAction(snippet, show)`, which fills the slot while `show()` is true and
    clears it again when the page is destroyed). Mis servicios registers "Nuevo servicio" on wide
    only (`show` is "wide and the data loaded"); Cotizar registers none. On wide the slot sits at
    the right of the `h1` row, with the tab bar under it. On the phone (owner, 2026-11-08) the slot
    is empty, so no action row is drawn: "Nuevo servicio" is the floating "+" of Hoy there (item
    17). The moved catalog page drops its own `h1`. This is a top-level place, so there is no back
    bar: the "Volver a Ajustes" bar and the Ajustes two-column layout come from
    `routes/ajustes/+layout.svelte` and stop applying once the page leaves `/ajustes`. The wide
    layout of each tab comes from its `.pen` frame; it is never the phone layout stretched
    (`.claude/rules/ui.md`).
*   **The tab links.** Two links in a `<nav>` labelled `services_tabs_label`. On the phone (owner,
    S0) each is a pill `Card`: `as="a"`, `href={resolve(path)}`, `pad="note"`, `radius="pill"`,
    `selected={current}`, `aria-current={current ? 'page' : undefined}`, layout classes
    `flex min-h-tap flex-1 items-center justify-center text-sm font-bold`. `current` is an exact
    match of `page.url.pathname` against the link's path (not the prefix match the places use,
    because `/servicios` is a prefix of `/servicios/catalogo`). No new primitive and no `.pen`
    component; S0 draws them in the frames. On wide (owner, S0) the same two links are a plain text
    tab bar across the content width, with an edge line under it: the current link is bold accent
    text with a 2 px accent underline, the other is muted. It is built from layout, border and text
    classes only, which ui.md allows outside `ui/`. Order: Cotizar, then Mis servicios (an agent
    call: the default tab comes first). This is not an ARIA `tablist`: the tabs are two URLs.
*   **Mis servicios on wide** (S0 frame `Servicios · Mis servicios · 1440`): a table in a `Card`
    with the columns Servicio, Duración, Precio and Color (a dot); a row opens the edit sheet, as
    today. Archived services sit under the table in a collapsed "Archivados (n)" section that she
    opens with a disclosure button. The phone keeps today's list.
*   **Navigation items.** `NAV_ITEMS` (`lib/components/navItems.ts`) gets a fifth entry between
    Clientas and Ajustes, with a new key `nav_services` ("Servicios") as label and tab label and a
    neutral icon chosen in S0 (Lucide `tag`), added to `.pen` and to `PATHS` in
    `icons/paths.ts`. The place stays current on both tabs through the existing prefix match
    (`lib/nav.ts`).
*   **Ajustes.** It loses its Servicios row (`SettingsList.svelte`) and the `settings_services`
    key. Nothing else changes.
*   **Page titles.** `quote_title` ("Cotizar") for `/servicios`, `services_title` for
    `/servicios/catalogo` (`lib/titles.ts`).

### 6. Five tabs on a 393 px screen

With today's padding (12 px a side) each tab gets about 66 px at 393 px, and about 42 px for a bold
12 px label (`BottomNav.svelte`, the `tab` variant of `NavItem.svelte`; arithmetic). Measured in
WebKit at 393 px in S2, with 12 px padding the five tabs were 58.8, 58.9, 69.6, 74.0 and 65.7 px
wide (Hoy, Enviar, Clientas, Servicios, Ajustes): the tabs grow past their share to fit their
labels, and the bar's pill (outer box 16.0 to 377.0 px, content box 25.0 to 368.0 px) held them.

*   The fix goes in the primitive: S0 draws five tabs in `.pen`, then S2 reduces the `tab` variant's
    side padding in `NavItem` to the measured value (4 px, `px-1`; section 10), in one change.
*   A new probe check, `tab-fit`, in `web/tests/e2e/probe.ts`. It finds the tab bar by its
    `nav_label` (passed as an argument, since probe checks use no outside names), and reports a
    finding for each `a` inside the bar's pill `div` whose box is not inside the pill's content box
    (the pill's box less its border and padding; 0.5 px tolerance). With no tab bar (wide) it
    reports nothing. It runs in the compact probe project, `base` state, on every signed-in route,
    through the same `sift` as the other checks; there the spec also reports a finding when no bar
    is drawn, so a missing bar cannot pass.
*   How it was proven: with today's padding the five tabs did fit at 393 px, so the check could not
    be seen red on the fifth item alone. Its failure was proven by forcing an overflow (`px-6` on
    the tab: 17 compact routes failed, a tab ending at 475.5 px against a bar ending at 377.0 px).
    The first version compared against the pill's outer box and so missed up to 8 px of overrun;
    the content-box version is the one that ships (S2 fix round, results in the S2 report).
*   The value is settled in S0: 4 px on each side (`px-1`), the value drawn in the `.pen`
    (`NavItem/Tab` padding `[8, 4]`), which keeps parity with the design and leaves margin for a
    wider font (section 10). No shorter tab label is needed.

### 7. Draft fixes found on the way

Two bugs in today's code. Each gets its own commit and a test that fails before the fix:

*   **An emptied form brings back its old draft.** `formDraft` saves only when the form differs from
    its start, and never clears the draft when the form goes back to the start. Fix: a pure
    `draftStep(loaded, now, initial): 'wait' | 'save' | 'clear'` in `lib/cache/drafts.ts`. Nothing
    is saved or cleared until the stored draft has been loaded (`'wait'`); after that, a form equal
    to `initial` returns `'clear'` and any other form returns `'save'`. `formDraft` calls it from
    its save effect and clears through the autosave handle's `clear()`, which cancels the pending
    timer. The rule is a pure function because Vitest runs in `node` and cannot mount a component;
    the e2e test of Goal 10 covers the wiring.
*   **Archived services pass through a restored Nueva cita draft.** `AppointmentForm` passes every
    cached service id to `restoreForm`, archived ones included, against that function's own
    comment. Fix: a pure `activeServiceIds(services): Set<string>` in `booking.ts`, used by
    `AppointmentForm` and `QuoteForm`.

### 8. Rollout and rollback

*   No database, API or server change, so there is no migration and nothing to roll back on the
    server.
*   An old cached app shell keeps the old navigation until she taps the update prompt. That is
    consistent: the old shell only knows `/ajustes/servicios`.
*   Rolling back to an old build is safe. An old build never reads `cotizacion:new`; the draft stays
    unused in Dexie until the next logout clears it.

### 9. Stages

Each stage ships on its own and passes the gate in `CLAUDE.md`, including its image rule: every
stage after S0 changes what the app serves, so each also runs `make image image-scan dast`.

| Stage | What it does | Exit |
| --- | --- | --- |
| **S0. Design and device check** | iPhone clipboard test in the installed PWA, result and iOS version recorded here. `.pen` frames (no new components): five-tab `BottomNav` measured at 393, five-row sidebar, Ajustes without Servicios, both Servicios tabs at 393 and 1440, the Cotizar page with its text block, empty, busy, message-slot and hold-hint states. Move the brief and the review reports to `docs/designs/history/`. Fix the release-1 and PRD statements this design makes false. | Frames reviewed by the owner. Clipboard result recorded. |
| **S1. Draft fixes and refactors** | Rename the local `quote` in `AppointmentForm`. Export `LineSchema`. Extract `durationOf` and `blankForm`. Add `'cotizacion'` to `draftKey`. Then the two fixes in section 7, each with an e2e test that fails first. | The gate, `make e2e`. |
| **S2. Servicios place, catalog moved** | Fifth nav item (pointing at `/servicios/catalogo` until S3), icon, `tab-fit` check then the padding fix, `servicios/+layout.svelte` with the `h1` and the header action slot (`header.svelte.ts`), catalog moved to `/servicios/catalogo`, Ajustes row and key removed, every link, title, probe row, e2e step and lint-suppression key re-pointed once. | The gate, `make e2e`, ui-probe at both widths, screenshots of the tab bar on every place. |
| **S3. Cotizar and Copiar** | The nav item's `href` becomes `/servicios`; the tab links. `quote.ts`, `QuoteForm`, the text block, the message slot, Copiar with success and failure, Nueva cotización, the empty and busy states, the probe row, the CUJ Q1 e2e test with a clipboard stub. | The gate, `make e2e`, screenshots at 393 and 1440. |
| **S4. Convert** | `convertQuote`, the inline confirm, its failure, clearing the quote, opening Nueva cita with no `volver`. | The gate, `make e2e`, e2e tests for CUJ Q2 and Q3, the owner's 15-second check. |

In S2 the Servicios place holds only the catalog, with a heading and no tab links; S3 adds Cotizar
and the links. The catalog page moves once.

### 10. S0 results

*   **Frames** (owner-approved, 2026-11-07), in `design/soft-glass.pen`: `Servicios · Cotizar ·
    <state> · 393` and `· 1440` for con líneas, copiado, confirmar reemplazo, no se pudo copiar, no
    se pudo convertir, aviso mantener presionado, nada elegido, catálogo vacío and cargando;
    `Servicios · Mis servicios · 393` and `· 1440`; `Ajustes` (no Servicios row) and
    `Ajustes · 1440`. Wide frames show the five-row sidebar. No new component and no new variable.
    The `cargando` frames stand for the page-level `services_loading` line (section 2, States).
    Owner decisions: pill tab links on the phone and an underline tab bar on wide (section 5); no
    toast on Cotizar (section 3); every amount keeps its digits together, with the no-break spaces
    `formatMoney` prints (`₡6 000,00`, U+00A0 between the groups, checked in Node's ICU).
*   **Five tabs.** In the `.pen`, `NavItem/Tab` side padding goes from 12 px to **4 px**
    (Tailwind `px-1`). At 393 px each tab is 65.8 px wide: 393 − 32 (bar `px-4`) − 16 (pill `p-2`)
    − 4 × 4 (gaps), divided by 5. The labels are 12 px bold, measured in Pencil: Hoy 23 px, Enviar
    35 px, Clientas 46 px, Servicios 51 px, Ajustes 42 px. At 12 px padding there is 41.8 px for a
    label, so by this arithmetic Clientas and Servicios would not fit; at 4 px there is 57.8 px. At
    375 px (62.2 px tabs) 4 px still fits, but 6 px would not. Each tab stays 65.8 × 56 px, above
    the 44 px target. The browser's font differs by a pixel or two, and S2 measured WebKit at 393 px
    with 12 px padding: the tabs were 58.8, 58.9, 69.6, 74.0 and 65.7 px, so the arithmetic's
    "does not fit" was too strict there (tabs grow with their labels). 4 px is kept: it is the
    frame's value and gives the most room; the S2 `tab-fit` probe is the real check.
*   **iPhone clipboard test:** pending (owner). When it is done, record here: pass or fail, the iOS
    version, and that it ran in the installed PWA.
*   **Wording** taken from the frames: `quote_empty`, `quote_copy_failed`, `quote_hold_hint` (item
    25), `quote_replace_prompt` and `quote_convert_failed` (item 30).

## Risks

**Accepted:**

*   **One quote at a time** (agent). A second asker's quote replaces the first. Both chats keep
    their pasted text.
*   **Nueva cita's total counts a missing price as 0** (agent). After convert, Nueva cita's "Total"
    (`totalOf`) adds unpriced lines as 0, with no note. The quote itself never does. Changing Nueva
    cita's total is out of scope (Future Improvements).
*   **"Recuperamos lo que habías escrito." after convert** (agent). Nueva cita shows its usual
    restored-draft note about services she picked on Cotizar, not there.
*   **The copy may need press-and-hold** (agent). If iOS refuses the clipboard write, the text on
    the page is the fallback (section 3).
*   **Some Cotizar states are not in the layout probe** (agent). The probe table can open a page and
    its sheets, not a page with lines or a message showing. The probe covers the `base` state and the
    services feed; the e2e tests cover the text block, the buttons and the message slot.
*   **Two ways to pick on Cotizar** (agent): chips on the phone, a table with a "− n +" stepper on
    wide (owner's S0 frames). Both call the same `tapService` / `removeService`, so the lines and
    the text cannot differ; the e2e tests run at both widths.
*   **"Servicios" three times on Cotizar** (agent): the place, the `h1` and `ServicePicker`'s group
    label. Settled visually in S0; e2e locators use `exact: true` or a role scope.

**Handled:**

*   Five tabs too wide: section 6.
*   Convert opens Nueva cita before its draft is written: section 4 (awaited write).
*   A pending save writes the quote back after convert: section 4 (clear through the draft handle).
*   A removed line comes back: section 7.
*   An archived service in a restored draft: sections 2 and 7.
*   Rollback: section 8.

## Alternatives Considered

Each alternative was rejected during the brainstorm, while writing this draft, or in review round 1.
"(owner)" means the owner decided; "(agent)" means the agent decided and the owner may reopen it.

### A. The shape of the feature

#### Alternative A1: An editable quote template (owner)

A `quote` kind of message template, edited in Ajustes with the existing chip editor, rendered on the
server.

*   **Pros**: She controls the wording. Reuses the template editor she already knows.
*   **Cons**: Templates have no kind today, so quote variables would leak into reminder templates
    and the reverse; that needs a migration, a variable list per kind, a settings field and a render
    endpoint. Only the server renders templates (release-1 Alternative E8), so Copiar would fail
    with the PC off, and copying after waiting for the server risks iOS refusing the clipboard
    write.
*   **Reason for Rejection**: The owner chose a fixed format built on the phone. She adds any extra
    wording in WhatsApp after pasting.

#### Alternative A2: A "Copiar cotización" button on Nueva cita, no separate page (owner)

*   **Pros**: The smallest build. Convert is free: she just keeps filling the same form.
*   **Cons**: A quote would overwrite an unfinished booking draft. A quote and a booking are
    different jobs on one screen.
*   **Reason for Rejection**: The owner wants a separate page with its own draft.

#### Alternative A3: Store quotes on the server (owner)

*   **Pros**: A history of quotes; convert days later from a list.
*   **Cons**: A new table, API routes, export columns and cleanup, for a list nobody reads. The
    WhatsApp chat already holds the old quote, and a new one takes seconds.
*   **Reason for Rejection**: No real use for the history in a one-person studio.

#### Alternative A4: Per-line prices, extra items or discounts in the quote (owner)

*   **Pros**: Matches the real conversation ("te lo dejo en 18 mil").
*   **Cons**: An appointment copies catalog prices and has no per-line override, so a changed quote
    price is lost at convert and the booking total disagrees with what she promised. Fixing that is
    a separate appointments feature.
*   **Reason for Rejection**: Catalog prices only; extras become services, one-off discounts are
    typed in WhatsApp.

#### Alternative A5: Require a client for a quote (owner)

*   **Pros**: Could use the client's name in the text.
*   **Cons**: Most people who ask are unknown numbers who may never book; creating a client for each
    is wasted work and fills the client list.
*   **Reason for Rejection**: The client is chosen only in Nueva cita.

#### Alternative A6: Send through `wa.me` instead of copying (agent)

*   **Pros**: One tap opens WhatsApp with the text.
*   **Cons**: Without a saved number, `wa.me` opens a contact picker, not the chat she already has
    open with the asker.
*   **Reason for Rejection**: Copy and paste fits the open chat.

### B. Where the page lives

#### Alternative B1: A "Cotizar" button next to "Nueva cita" (owner)

*   **Pros**: No change to the tab bar, sidebar or Ajustes; about half the work.
*   **Cons**: The catalog stays buried in Ajustes.
*   **Reason for Rejection**: The owner chose a fifth place, Servicios, holding the catalog and the
    quote.

#### Alternative B2: A fifth place only for quotes, catalog left in Ajustes (owner)

*   **Pros**: No catalog move: no re-pointed links, probe rows or setup-journey changes.
*   **Cons**: The catalog stays in Ajustes, and quoting and the prices it reads live in two places.
*   **Reason for Rejection**: The owner wants the catalog and the quote together.

#### Alternative B3: A choice inside the "+" button (owner)

*   **Pros**: No tab bar change at all.
*   **Cons**: One more tap for every new appointment, and quoting stays hidden behind "+".
*   **Reason for Rejection**: The owner chose the Servicios place.

#### Alternative B4: Open Servicios on Cotizar only when a service exists (agent)

A redirect, or a nav link computed from the cache.

*   **Cons**: A redirect at `/servicios` makes the catalog tab bounce away. A nav link from the cache
    has no answer before the cache loads and breaks the active highlight.
*   **Reason for Rejection**: Cotizar is always `/servicios`. With no service, Cotizar's empty state
    links to the catalog in one tap.

#### Alternative B5: One route with `?tab=`, or the catalog at `/servicios` (agent)

*   **Cons**: A query parameter is a second outside input to parse. With the catalog at the root,
    the default tab (Cotizar) would not be the fixed nav link.
*   **Reason for Rejection**: Two routes, Cotizar at the root: each tab gets its own title, probe row
    and URL.

#### Alternative B6: Redirect `/ajustes/servicios` to the new path (agent)

*   **Reason for Rejection**: One owner, and links to it exist only inside the app and all change. A
    redirect is code and a probe row for no reader.

#### Alternative B7: A new tab primitive, a `NavItem` variant, or an ARIA `tablist` (agent, review round 1)

*   **Cons**: `NavItem` requires an icon, so a text-only variant bends it. A new segmented primitive
    builds for a second caller (Hoy's Día/Semana toggle) this design does not migrate, and adds a
    `.pen` component and a `design-parity` mapping. `tablist` means panels inside one page with
    arrow-key handling, not two URLs.
*   **Reason for Rejection**: Two `Card` links with `aria-current` (section 5).

#### Alternative B8: A shorter tab label instead of fixing the tab padding (agent)

*   **Reason for Rejection**: The rules fix layout problems in the primitive first. A label change
    needs the owner's word; it stays the fallback (Open Questions).

#### Alternative B9: Move the catalog twice, to `/servicios` then `/servicios/catalogo` (agent, review round 1)

*   **Cons**: Two re-keys of the budget-guarded lint suppression and two probe-row moves for one
    page.
*   **Reason for Rejection**: S2 moves it once, to its final path; the nav item points there until
    S3.

### C. How convert hands off to Nueva cita

#### Alternative C1: Pass the services in a query parameter (agent)

*   **Cons**: A second outside-input parser for the same data. Nueva cita loads its draft after
    reading the URL, so the draft would win over the parameter.
*   **Reason for Rejection**: Convert writes the draft Nueva cita already reads.

#### Alternative C2: Write `cita:new` through the existing 500 ms `autosave` (agent)

*   **Cons**: Nueva cita can open before the delayed write lands, and a failed write is only logged.
*   **Reason for Rejection**: `convertQuote` writes and waits; the page navigates only on `'ok'`.

#### Alternative C3: Convert steps in the route, with generic `readDraft` / `writeDraft` helpers (agent, review round 1)

*   **Cons**: A route is not reachable from Vitest, so the failure path would have no test, and the
    two helpers would each have one caller.
*   **Reason for Rejection**: One `convertQuote` in `lib/cache` with a tagged result, unit-tested.

#### Alternative C4: Ask only when the Nueva cita draft is "not blank", by a hand-written field list (agent, review round 1)

*   **Cons**: A second definition of an empty booking that drifts when `FormSchema` gains a field.
    With the draft fix in section 7, a `cita:new` draft exists only after a real change.
*   **Reason for Rejection**: Ask whenever a parsed `cita:new` draft exists.

#### Alternative C5: Merge the quote into an unsaved Nueva cita (agent)

*   **Cons**: Mixes two bookings' services without her seeing it.
*   **Reason for Rejection**: Ask, then replace.

#### Alternative C6: Clear the quote only after the appointment saves (agent)

Through a marker field in the Nueva cita draft, a `?de=cotizacion` flag, or on every new save.

*   **Cons**: A marker makes the booking form own part of the quote's life. A flag is lost when she
    reopens Nueva cita from "+". Clearing on every save wipes a quote she never converted.
*   **Reason for Rejection**: The quote is cleared as soon as the draft write succeeds. Nothing is
    lost: the Nueva cita draft holds the services.

#### Alternative C7: Send Save and Back to Cotizar (`volver`) after convert (agent)

*   **Cons**: Save also goes to `volver`, and the quote is already cleared, so she would land on an
    empty page after booking.
*   **Reason for Rejection**: No `volver`; both go to Hoy.

#### Alternative C8: A bottom sheet or a toast for the confirm and the errors (agent)

*   **Cons**: A sheet needs its own frame, focus handling and probe states. The toast has one tone
    and hides itself, so a press-and-hold instruction would vanish while she needs it.
*   **Reason for Rejection**: One inline message slot (section 2). Cotizar shows no toast at all:
    success shows on the Copiar button (section 3, owner).

### D. The quote text and its rules

#### Alternative D1: Count a missing price as 0 (agent)

*   **Cons**: The total looks lower than the real price, which she then has to take back.
*   **Reason for Rejection**: Unpriced lines say "precio a consultar"; the total says what it leaves
    out.

#### Alternative D2: Nail wording in the default text ("puede variar según el diseño") (agent)

*   **Reason for Rejection**: The app holds no business-specific text in code. She types it after
    pasting.

#### Alternative D3: A line list on screen plus a separate fallback text field (agent)

*   **Cons**: Two layouts to design and probe.
*   **Reason for Rejection**: One read-only text block is both the preview and the manual fallback.

#### Alternative D4: Freeze prices at quote time (agent)

*   **Cons**: Needs per-line prices in appointments (A4).
*   **Reason for Rejection**: The quote follows the catalog.

#### Alternative D5: A 20-service cap on the quote (agent)

*   **Cons**: Copies a Go constant into TypeScript and needs a drift test; the server already refuses
    more than 20 at save.
*   **Reason for Rejection**: No cap on the quote.

#### Alternative D6: An "unavailable" state for archived lines (agent)

*   **Cons**: A new UI state for a rare case.
*   **Reason for Rejection**: Archived lines are dropped on restore.

#### Alternative D7: Several quote templates (agent)

*   **Reason for Rejection**: No need shown; there is no template at all in v1.

#### Alternative D8: A threat-model row for the quote text (agent, review round 1)

*   **Cons**: No test can prove more than the function signature already does (it has no client
    parameter), so the row would need a test written only to support it.
*   **Reason for Rejection**: The fact is stated once, in Non-Functional Requirements.

--------------------------------------------------------------------------------

## Non-Functional Requirements

*   **Security & Privacy**: `quoteText` has no client parameter: the quote holds only service
    names, prices, durations and the business name, so the clipboard and the pasted text add no
    client-data exposure. A failed copy logs only `code=clipboard_failed` (guard `catch-blocks`
    checks that a catch logs a code). No new outside input is parsed: the quote reads the
    already-parsed Dexie cache, and its draft is parsed with zod on read and before every write.
    No row is added to `docs/threat-model.md` (Alternative D8).
*   **Performance**: Building the text is a pure function over at most a few dozen lines, run on each
    pick. No network call.
*   **Rollout & Rollback**: Phone-only; no migration, no API change. See Technical Plan section 8.

--------------------------------------------------------------------------------

## Additional Resources

*   [release-1.md](release-1.md): services, appointments and their price copy, templates and
    Alternative E8, drafts, the phone cache, screen layouts.
*   [quick-quotes-brief.md](history/quick-quotes-brief.md): the three brainstorm rounds.
*   `.claude/rules/ui.md`: the UI pipeline (`.pen` first, primitives, probe, screenshots).
*   `CLAUDE.md`: the gate and the load-bearing constraints.

## Future Improvements

*   **Per-line prices on appointments.** Would let a quote carry a discount or a special price
    through convert. A separate appointments feature.
*   **An editable quote format.** If she keeps rewriting the same words after pasting, a quote
    template (Alternative A1) becomes worth its cost.
*   **Nueva cita's total with missing prices.** Nueva cita still counts a missing price as 0.
*   **One segmented control.** Hoy's Día/Semana toggle and the Servicios tab links could share a
    primitive once a third caller appears.

--------------------------------------------------------------------------------

## Detailed Implementation Plan

Files are grouped by stage, in build order. Each stage passes the gate in `CLAUDE.md` before the
next starts. Paths are from the repository root. "release-1 item N" means an entry of release-1's
Detailed Implementation Plan.

### Target Files to Create/Modify

#### Stage S0: Design and device check

##### 1. `design/soft-glass.pen` (Modify)

*   **Type / Scope**: Design source (ui.md pipeline steps 1-3).
*   **Proposed Changes**: Frames only; no new components.
    *   `BottomNav` with five `NavItem/Tab` refs and the new icon; `NavItem/Tab` with reduced side
        padding. Measure at 393 px; record the label widths in this document (done: 4 px, Technical
        Plan section 10).
    *   The sidebar with five rows.
    *   Frames at 393 and 1440: Servicios → Cotizar (empty catalog, nothing picked, with lines,
        busy, each message-slot state, the hold-hint variant), Servicios → Mis servicios (the moved
        catalog, wide layout of its own), Ajustes without its Servicios row. On wide: the shared
        header row, the underline tab bar, the Cotizar catalog table with its steppers, the
        Cotización panel, and the Mis servicios table with its collapsed "Archivados (n)". The
        phone tab links and the text block are drawn from the existing `GlassCard` and `Callout`
        components; the wide pieces are plain frames from existing components and tokens.
    *   The final Spanish wording of every new `quote_*` key is taken word for word from these
        frames.
*   **Why**: Every frame is designed before code (guards `design-tokens`, `design-parity`).

##### 2. `docs/designs/quick-quotes.md` (Modify)

*   **Proposed Changes**: Record the S0 results (Technical Plan section 10): the iPhone clipboard
    test (pass or fail, iOS version, installed PWA) and the measured tab label widths.

##### 3. History files (Move)

*   `concepts_brief.md` → `docs/designs/history/quick-quotes-brief.md`.
*   `zoom_out_report.md`, `comprehension_report.md`, `critic_report.md`,
    `intermediate_zoom_out_report.md`, `implementation_readiness_report.md`,
    `implementation_readiness_report_r2.md`, `consolidated_review_report.md` →
    `docs/designs/history/review-quick-quotes/round-1/`.
*   **Why**: release-1 item 87 keeps no `.md` in the repo root except README and the harness files.

##### 4. `docs/designs/release-1.md` (Modify)

*   **Proposed Changes**: Fix only the statements this design makes false, each with a link to
    `quick-quotes.md`, cited by section:
    *   CUJ 2's empty-state text and release-1 item 75's empty state: "Ajustes → Servicios" becomes
        "Servicios → Mis servicios".
    *   "Screen layouts", the Shell bullet, and release-1 items 70 and 71 (the places, `BottomNav`):
        four places become five (Hoy, Enviar, Clientas, Servicios, Ajustes).
    *   Release-1 item 77 (the Ajustes sub-routes and root list): Servicios leaves Ajustes.
    *   Release-1 item 67 (drafts): the drafts now include the quote, and `draftStep`.
    *   The "Build order" table and the P0-P7 phase rows are not rewritten; the items above are
        edited in place because they describe code this design changes.

##### 5. `docs/prd.md` (Modify)

*   **Proposed Changes**: Under "Servicios", add FR-S4 (cotización rápida: elegir servicios, ver
    total y duración, copiar el texto, convertir en cita) and FR-S5 (Servicios como lugar propio con
    dos pestañas). Add both rows to the release table.

##### 6. `CLAUDE.md` (Modify)

*   **Proposed Changes**: "Architecture source" adds `docs/designs/quick-quotes.md` for the Servicios
    place and quotes.

#### Stage S1: Draft fixes and refactors

##### 7. `web/src/lib/components/AppointmentForm.svelte` (Modify)

*   **Proposed Changes**:
    *   Commit 1 (refactor): rename the local `quote` (the `withSavedLines` result) to `catalog`, so
        "quote" means one thing in the code. Use `blankForm(day)` for its initial form.
    *   Commit 2 (fix): the restore passes `activeServiceIds(data.services)` to `restoreForm`.

##### 8. `web/src/lib/booking.ts` and `web/src/lib/booking.test.ts` (Modify)

*   **Proposed Changes**:
    *   Export `LineSchema` (zod, `{ service_id, count }`); `FormSchema` builds its `lines` from it.
    *   Extract `durationOf(lines, services): number` (total minutes) from `endFor`; `endFor` calls
        it. Tests: no lines (0 minutes), one line, a repeated line.
    *   Extract `blankForm(day: string): Form` from `AppointmentForm`'s inline initial form.
    *   New `activeServiceIds(services: Service[]): Set<string>`. Test: an archived service is not in
        the set (fails before the function exists, and its use in item 7 is the fix).

##### 9. `web/src/lib/cache/drafts.ts` and `web/src/lib/cache/drafts.test.ts` (Modify)

*   **Proposed Changes**:
    *   `draftKey` accepts `'cita' | 'clienta' | 'cotizacion'`.
    *   New pure `draftStep<T>(loaded: boolean, now: T, initial: T): 'wait' | 'save' | 'clear'`
        (equality as `formDraft` compares today, by JSON).
    *   The top comment says which forms keep a draft (appointment, client, quote).
    *   Tests: not loaded → `'wait'` for any value; loaded and equal to `initial` → `'clear'`;
        loaded and different → `'save'`.

##### 10. `web/src/lib/cache/formDraft.svelte.ts` (Modify)

*   **Proposed Changes**: Track `loaded` as `$state` (so the save effect re-runs when it flips), set
    true once `load()` has resolved, after any `set()` of a restored draft. The save effect calls
    `draftStep`: `'save'` → the autosave handle's `save(now)`; `'clear'` → `formDraft`'s own
    `clear()` (which cancels the pending timer through the autosave handle and logs
    `code=draft_clear_failed` on failure); `'wait'` → nothing. In code, name the two handles
    distinctly (`store` for the autosave handle; `formDraft`'s returned `clear` stays `clear`). The
    top comment names the three forms.

##### 10a. `web/tests/e2e/booking.spec.ts` (Modify)

*   **Proposed Changes**: Two e2e tests that go red against today's code, one per S1 fix:
    *   "an emptied Nueva cita draft stays empty": pick a service, remove it, leave and reopen Nueva
        cita: no service is picked and no "Recuperamos…" note shows.
    *   "a draft line with an archived service is dropped": write a `cita:new` draft holding a
        service, archive that service, reopen Nueva cita: the line is gone.
*   **S1 commits, in order:** (1) `LineSchema`, `durationOf`, `blankForm`, `activeServiceIds`,
    `draftKey` and `draftStep` with their unit tests; (2) the `AppointmentForm` rename and
    `blankForm` use; (3) the archived-line fix with its e2e test; (4) the `formDraft` fix with its
    e2e test. Each fix's e2e test is seen red without the fix.

#### Stage S2: Servicios place, catalog moved

Commit order: (1) the move, the layout and the fifth nav item, so the route exists before
`resolve(item.href)` type-checks; (2) the `tab-fit` check; (3) the padding fix. With 12 px padding
the five tabs already fit at 393 px, so commit 2 is green; the check's failure is proven by a forced
overflow (section 6), not by a red commit. The gate applies to every commit.

##### 11. `web/src/lib/components/navItems.ts` (Modify)

*   **Proposed Changes**: The `href` union gains `'/servicios/catalogo'` (replaced by `'/servicios'`
    in S3). A fifth entry between Clientas and Ajustes: `href: '/servicios/catalogo'`, the S0 icon,
    `label` and `tabLabel` `nav_services`. The comment says "The five places".

##### 12. `web/src/lib/components/icons/paths.ts` (Modify)

*   **Proposed Changes**: Add the S0 icon to `PATHS` (which defines `IconName`). For Lucide `tag`,
    both of its `d` strings, including the dot `'M7.5 7.5h.01'`.

##### 13. `web/src/lib/components/ui/NavItem.svelte` (Modify)

*   **Proposed Changes**: The `tab` variant's side padding reduced to the S0-measured value, 4 px
    (`px-1`; Technical Plan section 10). Nothing else.

##### 14. `web/src/lib/components/ui/BottomNav.svelte`, `web/src/lib/components/Sidebar.svelte` (Modify only if needed)

*   **Proposed Changes**: Both render `NAV_ITEMS`, so a fifth item needs no change unless the S0
    frame changes the bar's gaps or padding. Any change matches the frame.

##### 15. `web/tests/e2e/probe.ts` (Modify)

*   **Proposed Changes**: The `tab-fit` check (Technical Plan section 6), taking the bar's
    `nav_label` as an argument, returning `Finding { check: 'tab-fit', target, detail }`, called in
    the compact project's `base` state on every route and passed through `sift`.

##### 16. `web/src/routes/servicios/+layout.svelte` (Create)

*   **Proposed Changes**: The header row at both widths: the `h1` "Servicios" (`services_title`)
    and the action slot (Technical Plan section 5), then the page. In S3 it also renders the tab
    links.
*   **`web/src/routes/servicios/header.svelte.ts` (Create)**: `provideHeaderAction()` (called by
    the layout: sets the context, returns the holder whose `$state` snippet the layout renders) and
    `useHeaderAction(snippet, show)` (called by a page: fills the slot while `show()` is true,
    clears it on destroy).

##### 17. `web/src/routes/ajustes/servicios/+page.svelte` → `web/src/routes/servicios/catalogo/+page.svelte` (Move)

*   **Proposed Changes**: Moved with `git mv`; what the page does is unchanged. Its own `h1` is
    removed (the layout owns the heading); "Nuevo servicio" moves into the layout's action slot
    through `useHeaderAction` (item 16) on wide and is the floating "+" (`IconButton` `add`, the
    wrapper and `bottom-fab` of Hoy; inert offline, the main area's `pb-32` keeps the last row
    clear) on the phone, opening the same sheet as today. The back bar and
    the Ajustes two-column layout came from `routes/ajustes/+layout.svelte` and no longer apply, so
    its 1440 px layout is rebuilt to match the S0 Mis servicios wide frame (ui.md: wide is never the
    phone layout stretched): a table in a `Card` with the columns Servicio, Duración, Precio and
    Color, then a collapsed "Archivados (n)" section with a disclosure button. The phone keeps
    today's list.
    Its top comment says the page is the Mis servicios tab of Servicios.

##### 18. Links and text that named the old place (Modify)

*   `web/src/lib/components/SettingsList.svelte`: the Servicios row is removed.
*   `web/src/lib/settingsPage.test.ts`: the example that used `/ajustes/servicios` switches to
    `/ajustes/atributos` (Plantillas is already tested).
*   `web/src/lib/components/ServicePicker.svelte`: the "Configurar servicios" link points to
    `/servicios/catalogo`.
*   `web/src/lib/titles.ts` and `titles.test.ts`: `/ajustes/servicios` becomes
    `/servicios/catalogo` with `services_title`.
*   `web/src/lib/services.ts`: its top comment says "Servicios → Mis servicios".

##### 19. `web/src/lib/i18n/es.json` (Modify)

*   **Proposed Changes**: Add `nav_services` ("Servicios"), the wide table headers
    `services_col_name` ("Servicio"), `services_col_duration` ("Duración"), `services_col_price`
    ("Precio") and `services_col_color` ("Color"), `services_price_unset` ("precio a consultar",
    used by the wide table and the quote text), and `services_archived_toggle` ("Archivados
    ({count})"). Delete `settings_services` by hand: no guard flags an unused key.

##### 20. Tests and lint for the move (Modify)

*   `web/tests/e2e/ui-probe.spec.ts`: the `/ajustes/servicios` row re-keyed to
    `/servicios/catalogo`; the spec fails when its keys and the routes on disk differ. Its base
    test calls `tabFit(page, es.nav_label)` in the `probe-compact` project, next to the other
    checks.
*   `web/tests/e2e/setup.spec.ts`: CUJ 2 reaches the catalog by tapping the nav link
    `es.nav_services` (`exact: true`, so it does not also match "Mis servicios" or "Configurar
    servicios"). In S3 the same step also taps the `services_tab_catalog` tab.
*   `web/tests/e2e/navigation.spec.ts`: a new test "the five places, in order" asserting the five
    link names at 393 and 1440 px.
*   `web/src/lib/nav.test.ts`: in S2, the place (`href` `/servicios/catalogo`) is active for
    `/servicios/catalogo`; in S3, with `href` `/servicios`, it is active for `/servicios` and
    `/servicios/catalogo`.
*   `web/eslint-suppressions.json`: the suppression keyed by the old catalog file path is re-keyed
    to the new path, same count. The commit says it is a move, not a new violation.

#### Stage S3: Cotizar and Copiar

##### 21. `web/src/lib/quote.ts` and `web/src/lib/quote.test.ts` (Create)

*   **Proposed Changes**:
    *   `QuoteDraftSchema = z.object({ lines: z.array(LineSchema) })`.
    *   `quoteText(lines: Line[], services: Service[], settings: { name: string; currency: string;
        locale: string }): string`, the format in Technical Plan section 1, joined with `\n`. Row
        type and duration formatting private; amounts named `*Minor`.
    *   Tests: each row of the missing-price table; a price of 0; a repeated priced line (exact
        "×2" and "c/u"); a repeated unpriced line; blank business name; durations of 45, 120 and
        150 minutes; a line whose service is not in `services` is skipped. Expected amounts are
        built with `formatMoney`.

##### 22. `web/src/lib/components/QuoteForm.svelte` (Create)

*   **Type / Scope**: Feature component (composes `ui/` primitives; no surface or radius classes).
*   **Proposed Changes**:
    *   Props: `services` (active only) and `settings`; S4 (item 29) adds `today`, which only convert
        reads.
    *   `formDraft(draftKey('cotizacion', 'new'), QuoteDraftSchema, { lines: [] }, …)`; the restore
        keeps lines in `activeServiceIds(services)`.
    *   On the phone, `ServicePicker`; on wide, the catalog table with its "− n +" stepper
        (`IconButton` variant `accent`, labels `appointment_service_remove` and `quote_service_add`)
        beside the Cotización panel (Technical Plan section 2, Layout). The text block
        (`Card as="p" pad="md"`, `whitespace-pre-line select-text`; inside the panel on wide);
        the message slot (`Callout`, the tagged union of Technical Plan section 2); when there are
        lines, Copiar (`quote_copy`) and Nueva cotización (`quote_new`, calls `startOver()`);
        Convertir en cita is added in S4 (item 29), not before; `quote_empty` when there are none;
        the hold hint (`quote_hold_hint`) only if the S0 clipboard test failed.
    *   No active service: `EmptyState` rendered by `QuoteForm` itself at both widths
        (`appointment_services_empty_title`, `appointment_services_empty_body`,
        `appointment_services_empty_action`, `href={resolve('/servicios/catalogo')}`), instead of
        the picker or the table; on wide the panel shows `quote_empty`.
    *   Copiar: `navigator.clipboard.writeText(text)` called first, no await before it; missing
        API, synchronous throw and rejection → slot `copy_failed` and
        `console.warn('code=clipboard_failed')`; success → the button's label becomes the `check`
        icon and `quote_copied` for about 2 seconds (no toast); a screen-reader-only `aria-live="polite"`
        region holds `quote_copied` for the same time, so the change is announced.

##### 23. `web/src/routes/servicios/+page.svelte` (Create)

*   **Proposed Changes**: `<p role="status">` with `services_loading` while `cache.snapshot` is
    null (as Nueva cita does; the probe finds it by role); then `QuoteForm` with
    the active services and the settings (`today = clock.today(settings.timezone)` arrives with convert in S4, item 29).

##### 24. `web/src/routes/servicios/+layout.svelte`, `navItems.ts`, `nav.test.ts`, `titles.ts` (Modify)

*   **Proposed Changes**: The layout renders the two tab links (Technical Plan section 5: pill
    `Card` links on the phone, the underline text tab bar on wide, exact-match `current`, `<nav>`
    labelled `services_tabs_label`), Cotizar first. The nav item's `href` and union become
    `'/servicios'`. `titles.ts` gains `/servicios` → `quote_title`. `web/tests/e2e/setup.spec.ts`:
    CUJ 2 now taps the nav link `es.nav_services` (exact), then the `services_tab_catalog` tab,
    then asserts `expect(page).toHaveURL(/\/servicios\/catalogo$/)` before its empty-state check
    (Cotizar's empty state shows the same `appointment_services_empty_title`).

##### 25. `web/src/lib/i18n/es.json` (Modify)

*   **Proposed Changes**: `services_tab_quote` ("Cotizar"), `services_tab_catalog` ("Mis
    servicios"), `services_tabs_label` ("Secciones de Servicios", an `aria-label`), `quote_title` ("Cotizar"), `quote_heading` ("Cotización"),
    `quote_heading_named` ("Cotización · {name}"), `quote_line` ("• {name}: {price}"),
    `quote_line_repeat` ("• {name} ×{count} ({unit} c/u): {total}"), `quote_line_unpriced_repeat`
    ("• {name} ×{count}: {price}", `{price}` is `services_price_unset`; a single unpriced line uses
    `quote_line`), `quote_total` ("Total: {amount}"), `quote_total_partial` ("Total: {amount} + lo que
    está a consultar"), `quote_duration` ("Duración aproximada: {duration}"), `duration_hours`
    ("{h} h"), `duration_hours_minutes` ("{h} h {m} min"), `quote_empty` ("Toca los servicios que
    te preguntaron para armar la cotización."), `quote_copy` ("Copiar"), `quote_copied`
    ("Copiado", the button's label after a copy), `quote_copy_failed` ("No se pudo copiar. Mantén
    presionado el texto de abajo para copiarlo."), `quote_new` ("Nueva cotización"),
    `quote_service_add` ("Agregar una copia de {name}", the wide stepper's "+"),
    `quote_col_count` ("Cantidad", the screen-reader-only header of the stepper column), and
    `quote_hold_hint` ("Para copiar, mantén presionado el texto de abajo y toca Copiar.") only if
    the S0 clipboard test failed (no guard flags an unused key).

##### 26. `web/tests/e2e/ui-probe.spec.ts`, `web/src/lib/components/ServicePicker.svelte` (Modify)

*   **Proposed Changes**: A probe row for `/servicios`: `ready` 'Acrílico' (a service of the setup
    catalog), the busy text `services_loading`, and a services feed whose empty text is
    `appointment_services_empty_title`; both widths. The other states are an accepted risk (Risks).

##### 27. `web/tests/e2e/quote.spec.ts` (Create) and `web/tests/e2e/steps.ts` (Modify)

*   **Proposed Changes**:
    *   Data: the services the setup spec creates (Acrílico, Relleno, Pedicure; all priced); the
        unpriced cases are covered by `quote.test.ts`. `resetData()` in `beforeEach`, so no client
        exists.
    *   `serverOff` moves from `offline.spec.ts` to `steps.ts` (two callers now). `draftStored`
        is already there and takes any draft form (`cita`, `clienta`, `cotizacion`), so the quote
        tests can read `cotizacion:new` and `cita:new`.
    *   Clipboard stub through `addInitScript`: records the written text on `window`, with three
        failing variants (no `navigator.clipboard`, `writeText` throwing, `writeText` rejecting).
    *   Tests: CUJ Q1 on Acrílico once and Relleno twice: the recorded clipboard text equals the
        text block's `textContent`, and contains "Acrílico", "Relleno ×2" and "c/u" (the exact
        format is proven by `quote.test.ts`; the spec cannot import `quoteText`, because
        `lib/i18n` imports `es.json` in a way Playwright's loader rejects, see `steps.ts`); the
        Copiar button shows "Copiado"; each failing variant shows `quote_copy_failed`; "tap
        Servicios lands on Cotizar"; "empty the quote, switch tabs, come back: still empty";
        offline (`serverOff`): the quote builds and copies; "Nueva cotización" empties the quote.
    *   Wide (1440 px viewport): pick with the table's stepper (Acrílico "+" twice, then "−" once),
        the panel's text equals the recorded clipboard text after Copiar and contains "Acrílico"
        but not "×2", and the Copiar button's accessible name is "Copiado".

#### Stage S4: Convert

##### 28. `web/src/lib/cache/quoteHandoff.ts` and `web/src/lib/cache/quoteHandoff.test.ts` (Create)

*   **Proposed Changes**:
    *   `convertQuote(lines: Line[], today: string, replace: boolean): Promise<'needs_confirm' | 'ok'
        | 'failed'>`, as in Technical Plan section 4, using `getDraft` and `putDraft` from `db.ts`
        and `FormSchema`, `blankForm` from `booking.ts`. The catch logs
        `code=quote_convert_failed`.
    *   Tests, on fake-indexeddb as `drafts.test.ts` runs: no `cita:new` → `'ok'` and the stored
        draft has the lines and `today`; an existing draft with `replace` false → `'needs_confirm'`
        and the draft is unchanged; with `replace` true → `'ok'` and replaced; a write failure
        (`vi.spyOn(IDBObjectStore.prototype, 'put')` throwing a `QuotaExceededError`
        `DOMException`; `restoreMocks` undoes it) → `'failed'` and `cita:new` unchanged. The test
        checks once that Dexie surfaces the throw as a rejection. No module is mocked (guard
        `test-hygiene`).

##### 29. `web/src/lib/components/QuoteForm.svelte` (Modify)

*   **Proposed Changes**: Convertir en cita runs `convertQuote` inside `submitOnce`: `'needs_confirm'`
    → slot `confirm_replace` with `quote_replace` ("Reemplazar", runs again with `replace` true) and
    the existing `cancel`; `'ok'` → the draft's `clear()`, then `goto(resolve('/citas/nueva'))`;
    `'failed'` → slot `convert_failed`.

##### 30. `web/src/lib/i18n/es.json` (Modify)

*   **Proposed Changes**: `quote_convert` ("Convertir en cita"), `quote_replace` ("Reemplazar"),
    `quote_replace_prompt` ("Ya tienes una cita nueva sin guardar. ¿La reemplazamos con estos
    servicios?"), `quote_convert_failed` ("No se pudo pasar a una cita nueva. Tu cotización sigue
    aquí; inténtalo de nuevo."), wording from the S0 frames.

##### 31. `web/tests/e2e/quote.spec.ts` (Modify)

*   **Proposed Changes**: CUJ Q2 (a client inserted with `insertClient` from `seed.ts`; convert into
    an empty Nueva cita: same services in order, today; pick that client, fill 10:00, save; lands
    on Hoy, Cotizar empty after); CUJ Q3 (an unsaved Nueva cita draft: "Cancelar" keeps
    both; "Reemplazar" replaces it).

### Verification Plan

*   **Every stage after S0**: the gate in `CLAUDE.md`, including `make image image-scan dast` (each
    stage changes what the app serves).
*   **Unit Tests** (`make test`): `quote.test.ts`, `quoteHandoff.test.ts`, `booking.test.ts`,
    `drafts.test.ts`, `nav.test.ts`, `titles.test.ts`, `settingsPage.test.ts`; the Go guards.
*   **Static Analysis & Linting** (`make lint`): svelte-check, eslint with the re-keyed suppression,
    knip, jscpd.
*   **Test Coverage Requirement**: `make cover` is Go only and unchanged (no Go change). The web
    tests are judged by the cases above, not a percentage.
*   **E2E** (`make e2e`, Playwright WebKit): `setup.spec.ts`, `navigation.spec.ts`,
    `quote.spec.ts`, `offline.spec.ts`, `ui-probe.spec.ts` at 393 and 1440 px.
*   **Screenshots**: ui.md pipeline step 6 at 393 × 852 and 1440 × 900: the tab bar on every place,
    the sidebar, both Servicios tabs, Ajustes.
*   **Manual (owner)**: the S0 iPhone clipboard test; the 15-second quote-and-paste check after S4.

--------------------------------------------------------------------------------

*Drafted with EGM*
