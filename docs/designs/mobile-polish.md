# Mobile polish: the floating "+", the empty attributes hint, and copy

*   **Author:** jnaranj09
*   **Status:** Planned on 2026-11-16. Stages S0 (design) and S1 (the floating "+") built on 2026-11-16; S2-S3 not built.
*   **Date:** 2026-11-16
*   **Builds on:** [release-1.md](release-1.md) (the app as built),
    [quick-quotes.md](quick-quotes.md) (the floating "+" of Mis servicios, the Copiar button) and
    [reminders-and-replies.md](reminders-and-replies.md) (Plantillas, Reglas, Respuestas rápidas)

Decisions are marked **(owner)** when the owner made them and **(agent)** when the agent chose and
the owner can challenge them.

--------------------------------------------------------------------------------

## Overview

Three small fixes the owner found while using the app on the phone:

1.  **Create actions.** On the phone, every "create something" action is the floating "+" in the
    bottom right corner. Today only Hoy (Nueva cita) and Mis servicios (Nuevo servicio) do this.
    Five screens still put a button at the top.
2.  **No attributes yet.** With 0 attributes, the client and appointment screens show nothing about
    them, so she cannot tell that attributes exist or where to create them.
3.  **Copy.** The copy feedback is easy to miss, and some texts she wants to paste elsewhere have no
    copy button.

No server code, database table or API route changes. All work is in `web/`, the `.pen` design file
and the docs.

## Goals

1.  **(owner)** On the phone, every create action is the floating "+" (`IconButton` `add` in a
    `fixed right-4 bottom-fab z-20` wrapper, as on Hoy and Mis servicios). On wide, the button stays
    where it is today, and there is no "+".
2.  **(owner)** With 0 active attributes of a scope, the client form, the appointment form and the
    client detail say so and say where to create them.
3.  **(owner)** A copy shows "Copiado" in two places: the button (as today) and the app's toast.
4.  **(owner)** New copy buttons: the message on each "Para enviar" card, the template preview, and
    the client's phone.

## Non-Goals

*   No new create action. Only the place of the existing ones moves.
*   No change on wide (1440 px) apart from the new copy buttons and the attributes hint.
*   The empty-state action "Nueva clienta" (Clientas with 0 clients) stays: it is the empty state's
    own action, not the screen's primary button **(agent)**.

## Technical Plan

### 1. The floating "+" on every create screen (stage S1)

Today, on the phone:

| Screen | Action | Today | File |
| --- | --- | --- | --- |
| Hoy | Nueva cita | floating "+" (link) | `web/src/routes/+page.svelte` |
| Servicios → Mis servicios | Nuevo servicio | floating "+" (sheet) | `web/src/routes/servicios/catalogo/+page.svelte` |
| Clientas | Nueva clienta | ghost `WriteLink` next to the `h1` | `web/src/lib/components/ClientList.svelte` |
| Ajustes → Plantillas | Nueva plantilla | strong `WriteLink` above the list | `web/src/routes/ajustes/(reglas)/plantillas/+layout.svelte` |
| Ajustes → Recordatorios | Nueva regla | strong `WriteButton` at the top | `web/src/routes/ajustes/(reglas)/recordatorios/+page.svelte` |
| Ajustes → Atributos | Nuevo atributo | strong `WriteButton` in the header | `web/src/routes/ajustes/atributos/+page.svelte` |
| Enviar → Respuestas | Nueva respuesta | strong `WriteButton` at the top | `web/src/routes/enviar/respuestas/+page.svelte` |

For the last five, on compact (`layout.current === 'compact'`):

*   The top button is not drawn.
*   The floating "+" is drawn with the same label (`clients_new`, `templates_new`, `rules_new`,
    `attributes_new`, `replies_new`), so its accessible name does not change.
*   A link action (Clientas, Plantillas) passes `href`; a sheet action (Reglas, Atributos,
    Respuestas) passes `onclick`, as Mis servicios does.
*   Offline it is `inert` (faded); the offline banner says why (release-1 "Offline").
*   Master-detail screens (Clientas, Plantillas) show the "+" only while the list is the screen, not
    over a client or a template **(agent)**. For Clientas the "+" goes in
    `routes/clientas/+layout.svelte`, which already knows `onList` and `wide`. Plantillas keeps
    its top button over a template: on the phone the list pane is hidden then, and on a compact
    window wide enough for both panes (its container query) it is the only way to a new
    template **(agent)**.
*   The root layout's `pb-32` already keeps the last row clear of the "+".

A shared wrapper is not added: the wrapper is one `div` with fixed classes, and Hoy and Mis servicios
already repeat it. If jscpd flags the repeat, a `FloatingAdd.svelte` that takes `label`, `href` or
`onclick`, and `inert` replaces all seven **(agent)**.

### 2. The "no attributes yet" hint (stage S2)

Each place already filters the attribute definitions by scope:

*   `ClientForm.svelte`: `defs` (client, not archived).
*   `AppointmentForm.svelte`: `defs` (appointment, not archived).
*   `routes/clientas/[id]/+page.svelte`: `defs` (client).

When that list is empty, the place shows a one-line hint in a `Callout`:

*   **Forms:** text only, no link **(agent)**. A link would leave the form, and editing a client keeps
    no draft, so her typed changes would be lost.
*   **Client detail:** the same text inside the facts card, plus a link to `/ajustes/atributos`.
*   For the client detail, "empty" means 0 **active** client attributes **(agent)**. An archived
    attribute with an old value still shows its fact, as today.

The hint is computed on read from the cached attribute list. Nothing is stored.

### 3. Copy feedback and new copy buttons (stage S3)

Today `lib/copy.ts` holds `copyText` and `copiedFlag`. `QuoteForm.svelte` and
`routes/enviar/respuestas/+page.svelte` each keep their own `copied` state, their own failure
`Callout` and their own `sr-only` live region.

**New component `web/src/lib/components/CopyButton.svelte` (agent).** Five callers, so one owner:

*   Props: `text`, `variant` (a `Button` variant), optional `label` (default "Copiar").
*   On tap: `copyText(text, …)` inside the tap, nothing awaited before it (iOS rule, quick-quotes
    "Copiar").
*   On success: the button shows the check icon and "Copiado" for `COPIED_MS`, and
    `toaster.say('copied')`.
*   On failure: `toaster.say('copy_failed')` ("No se pudo copiar. Mantén presionado el texto"), and
    the log line `code=clipboard_failed` as today.
*   The `Toast` is `role="status"`, so the `sr-only` live regions of QuoteForm and Respuestas are
    removed. Two announcements of the same word would be noise.

**Failure moves to the toast (agent).** Today a failure shows a `Callout` on the page. The new places
(a due card, the phone row) have no message slot, and one place for both outcomes keeps the five
the same. `QuoteForm`'s `message` union loses `copy_failed`.

**Callers:**

| Place | Copies | Look | File |
| --- | --- | --- | --- |
| Cotizar | the quote text | `strong` (as today) | `QuoteForm.svelte` |
| Respuestas rápidas | the reply body | `glass` (as today) | `routes/enviar/respuestas/+page.svelte` |
| Para enviar card (new) | `item.text`, both kinds (reminder and booking prompt) | `glass`, in the actions row before Omitir | `components/ui/DueCard.svelte` |
| Template preview (new) | the rendered preview text, shown only when there is one | `glass`, under the preview | `TemplatePreview.svelte` |
| Client phone (new) | `phone_e164` (works pasted in WhatsApp search and in other apps) | `ghost`, in the phone row | `routes/clientas/[id]/+page.svelte` |

A card whose client has no phone (it shows "Agregar teléfono") also gets Copiar: she can still paste
the message by hand **(agent)**.

### 4. Texts (`web/src/lib/i18n/es.json`)

*   Renamed, because they are no longer only for the quote **(agent)**: `quote_copy` → `copy`
    ("Copiar"), `quote_copied` → `copied` ("Copiado"), `quote_copy_failed` → `copy_failed`.
*   New **(agent)**, to be checked against `.claude/rules/spanish-texts.md`:
    *   `attributes_none_client`: "Aún no tienes atributos fijos. Créalos en Ajustes › Atributos".
    *   `attributes_none_appointment`: "Aún no tienes atributos por cita. Créalos en Ajustes ›
        Atributos".
    *   `attributes_none_link`: "Crear atributos".

### 5. Design file (stage S0)

`.claude/rules/ui.md` stage 1: the design comes first. Done in `design/soft-glass.pen` on
2026-11-16:

*   **Clientas** (393): already had the floating "+" and no top button. The code had drifted from
    it.
*   **Enviar · Respuestas · 393** and **Recordatorios · Reglas · 393**: the top button row is gone;
    `IconButton/Add` sits at the bottom right, as on Hoy.
*   **`DueCard/Message` and `DueCard/BookingPrompt`** (components): a `Button/Glass` "Copiar"
    between the send button and Omitir.
*   **Servicios · Cotizar · copiado** (393 and 1440): a toast "Copiado" at the top center
    (`GlassCard/Solid`, pill, 44 px high), with the button still showing "✓ Copiado".
*   **Servicios · Cotizar · no se pudo copiar** (393 and 1440): the page `Callout` is gone; the
    toast says "No se pudo copiar. Mantén presionado el texto".
*   **Nueva cita · sin atributos** (new, 393): the Forma field is gone and a `Callout` with
    `attributes_none_appointment` sits where the attributes go, before Notas.

The design file has no frame for Plantillas, Atributos, the client detail, the client form or the
template preview. Their changes follow the patterns above: the same "+", the same `Callout` hint,
the same "Copiar" button.

The toast in the design has even padding (8 px top and bottom, 20 px left and right). `Toast.svelte`
uses `Card` pad `trail` (`pl-5 pr-2`), made for a toast with "Deshacer" on the right. S3 gives a toast
with no action pad `message` (`px-5 py-3`) and keeps `trail` when there is an action **(agent)**.

No new token, primitive or icon: `IconButton` `add`, `Button`, `Callout`, `Card` `solid` and the
`plus` and `check` icons exist.

### 6. Docs this change makes false

Fixed in the stage that changes the behaviour:

*   `release-1.md`: "Offline" (the floating "+" was only on Hoy), the Clientas list ("Nueva clienta"
    opens the form), Plantillas; add the general rule of Goal 1 under "Screen layouts".
*   `quick-quotes.md`: "Every tap ends in something she can see" and "Copiar" say "no toast"; the
    text keys `quote_copy*`; the S3/S4 checks that read "Copiado" on the button.
*   `reminders-and-replies.md`: the place of "Nueva regla" and "Nueva plantilla"; the Respuestas copy
    feedback.
*   `CLAUDE.md`: this doc in the "Architecture source" list (done with this doc).
*   A search for `quote_copy`, `copyFailed`, `Copiado` and each `*_new` key after the change finds no
    stale comment.

## Risks

*   **The "+" hides the last row.** The root `pb-32` covers it today on Hoy and Mis servicios; the
    `ui-probe` layout probe at 393 px checks every route.
*   **A toast covers another toast.** The app has one toast (`toaster`); a copy replaces an "Omitido"
    toast with its "Deshacer". Copying on Para enviar right after Omitir drops that undo **(agent:
    accepted; the skip already goes through after its delay)**.
*   **Pencil is needed for S0.** No access to the design file → stop and ask (ui.md).

## Alternatives Considered

### A1: Keep the failure `Callout` on the page

*   **Reason for Rejection**: the due card and the phone row have no message slot; two failure
    styles for one action would confuse her. The toast text already says how to copy by hand.

### A2: A copy icon button instead of a text "Copiar"

*   **Reason for Rejection**: needs a new icon in the `.pen` and `paths.ts`, and "Copiar" is the word
    she already knows from Cotizar and Respuestas.

## Detailed Implementation Plan

Each stage is one commit, behaviour and its tests and docs together. The gate in `CLAUDE.md` runs
before each is called done, plus `make e2e-docker` locally.

### Stage S0: design (done 2026-11-16)

*   `design/soft-glass.pen` as in section 5. Guards `design-tokens` and `design-parity` stay green
    (to run with S1).

### Stage S1: the floating "+"

*   The five screens of section 1, and the docs of section 6 for it.
*   Tests: one e2e per screen at 393 px (the "+" is shown with its label; tapping it opens the form
    or sheet; no top button) and at 1440 px (the header button, no "+"). Offline: the "+" is inert.
    `navigation.spec.ts` keeps finding `clients_new` by role and name.

### Stage S2: the attributes hint

*   Section 2 and its texts.
*   Tests: e2e with 0 attributes (the hint in both forms and on the client detail; the detail link
    opens Atributos), then with one active attribute (the field shows, no hint), then with only an
    archived one (the hint again).

### Stage S3: copy

*   Refactor commit first, no behaviour change: `CopyButton.svelte` used by QuoteForm and Respuestas
    with today's look, key renames.
*   Then the behaviour: toast on success and failure, the `Callout` and `sr-only` regions removed, the
    toast padding of section 5, the three new places, the docs of section 6.
*   Tests: e2e with a stubbed clipboard (pattern of `quote.spec.ts`) in each of the five places:
    success shows "Copiado" on the button and in the toast, and the clipboard holds the expected
    text; the three failure cases (missing API, synchronous throw, rejection) show `copy_failed` in
    the toast.

### Verification Plan

*   The gate, `make e2e-docker`, and the owner's check on the phone (a `TASKS.md` entry, NOT RUN
    until she does it).
