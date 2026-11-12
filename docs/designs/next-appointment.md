# Book the next appointment ("Agendar próxima cita")

*   **Author:** jnaranj09
*   **Status:** Approved by the owner on 2026-11-12 (owner brainstorm, one builder check; the
    findings of the check and of the planning pass are in the text)
*   **Date:** 2026-11-11
*   **Build plan:** [next-appointment-plan.md](next-appointment-plan.md) (tasks, tests and exact
    names; it moves to `history/` when the feature is built)
*   **Predecessor:** [feature-ideas-brief.md](feature-ideas-brief.md) (the research round this idea
    came from)
*   **Builds on:** [release-1.md](release-1.md) (the app as built),
    [quick-quotes.md](quick-quotes.md) (the hand-off into Nueva cita) and
    [reminders-and-replies.md](reminders-and-replies.md) (come-back rules, general rules)

Decisions are marked **(owner)** when the owner made them in the brainstorm and **(agent)** when the
agent chose and the owner can challenge them.

--------------------------------------------------------------------------------

## Overview

open-customer serves one owner of a small service business on an iPhone PWA. Her come-back rules
already say when a client should return and for which service ("21 días después de Acrílico →
Relleno"). Today that knowledge is only used to remind the client later.

This design adds one button to the appointment page, **Agendar próxima cita**. It opens a sheet
with one date choice per come-back rule that applies ("En 21 días · 3 dic"), each with the services
it books, and a field for another date. Picking a date opens Nueva cita already filled: the same
client, the services her rules say come next, the chosen day and the same start time. She checks it
and saves there.

The feature runs on the phone. It changes no server code, no database table and no API route.

## The Problem

A client finishes her visit, or writes later, and wants the next one. To book it today the owner
opens Nueva cita, searches the client, picks the services again, works out the date from memory
("three weeks from her last visit") and picks the time. The app already holds every one of those
values: the client and services on the last appointment, and the number of days and the next
service on her rules.

When the next visit is not booked, the app falls back on a reminder weeks later, which she sends by
hand and the client may not answer.

Evidence for the need is medium and comes from English-language sources only (App Store reviews of
salon apps that ask for booking the next visit from the finished one). No Spanish-language source
and no controlled study was found. See the brief.

## Goals

1.  **The next visit is booked in a few taps.**
    Check: from an appointment page, with one rule that applies, she reaches a filled Nueva cita in
    two taps (the button, the date choice) and saves with a third.
2.  **The date choices are the ones her rules give.**
    Check: the unit tests of the choice function have a passing case for every row of the table in
    "Which rules count" and every bullet of "The date choices".
3.  **The new appointment carries what the rules say comes next.**
    Check: unit tests for every bullet of "The services", and one e2e case where the rule names a
    different return service.
4.  **Booking this way does not leave a reminder that asks for what she just booked.**
    Check: an e2e case books the next visit, on a later day, for a visit that has a come-back item
    due, and the item of the rule that decided the service is gone from Enviar afterwards. The two
    cases where a reminder stays are named in "Effect on reminders".
5.  **No server change.**
    Check: the change touches no file under `api/`, `internal/` or `cmd/`.

## Non-Goals

*   **Repeating appointments** (a series every N weeks). One tap books one visit.
*   **A prompt that appears by itself** when a visit ends. She opens the appointment and taps;
    when she does it does not matter **(owner)**.
*   **Booking without the form.** The button never creates an appointment by itself; Nueva cita
    does, with its overlap check.
*   **Finding a free time.** The start time is copied; an overlap is answered by the existing
    "Agendar igual" sheet.
*   **Copying notes or per-appointment attributes.** They describe one visit **(owner)**.
*   **The button on a cancelled or no-show appointment** **(agent)**: its day is not a visit to
    count from, and "Reactivar" is already there.
*   **New rule fields** (a "suggested gap" that is separate from the come-back rule). The rule is
    the one owner of the number of days.
*   **A server route for the choices.** See Alternative A4.

## Open Questions

None.

## Critical User Journeys (CUJs)

Names, services and numbers are made up.

### CUJ N1: The rule names the next service

Rules: "21 días después de Acrílico → Relleno" and "30 días después de Pedicure". Ana's
appointment of 12 nov has Acrílico and Pedicure.

1.  She opens that appointment and taps **Agendar próxima cita**.
2.  The sheet shows two choices, "En 21 días · 3 dic" and "En 30 días · 12 dic", each with
    "Relleno, Pedicure" under it, and the field "Otra fecha".
3.  She taps "En 21 días · 3 dic". Nueva cita opens with Ana, Relleno and Pedicure, 3 dic and the
    start time of the 12 nov appointment. Notes and per-appointment attributes are empty.
4.  She saves. The app returns to the 12 nov appointment, and its back arrow goes where it went
    before she tapped the button.
5.  On 3 dic, Enviar shows no come-back reminder for Ana's Acrílico: her new booking has Relleno.

### CUJ N2: Only the general rule

The only come-back rule is "Todos los servicios, 30 días". Eva's appointment has Corte.

1.  The sheet shows one choice, "En 30 días · …" with "Corte" under it, plus "Otra fecha".
2.  She picks it. Nueva cita opens with Eva and Corte.

### CUJ N3: No rule, or another date

No come-back rule applies. The sheet shows only the "Otra fecha" field, with the services under
it. She picks
a date and taps "Usar esta fecha". Nueva cita opens with that day. The same field is there in N1
and N2 for a client who wants a different day.

### CUJ N3b: Two rules on one service

Rules: "21 días después de Acrílico → Relleno" and "60 días después de Acrílico → Acrílico". Luz's
appointment has Acrílico.

1.  The sheet shows "En 21 días · …" with "Relleno" under it and "En 60 días · …" with "Acrílico"
    under it.
2.  She picks "En 60 días". Nueva cita opens with Acrílico.

### CUJ N4: A booking is half written

She has an unsaved Nueva cita. She taps a date choice. The sheet asks "Ya tienes una cita sin
guardar. ¿Reemplazarla?" with "Reemplazar" and "Cancelar", as Cotizar does. "Reemplazar" goes on;
"Cancelar" keeps her draft and the sheet stays open.

### CUJ N5: Offline

The server is off. The button is off and says "Sin conexión: no se puede guardar", like the other
buttons of that row.

## Technical Plan (Architecture)

### What changes, in one picture

```
/citas/[id]  ──tap──▶  NextAppointmentSheet
                          │  nextVisit(appointment, rules, templates, services, zone, today)
                          │      → { choices, otherDate }                (lib/nextVisit.ts, new)
                          │  she picks a day
                          ▼
                       handOff(form, replace)                            (lib/cache/citaHandoff.ts)
                          │  writes the `cita:new` draft (asks before replacing one)
                          ▼
                       /citas/nueva?volver=/citas/<id>                   (unchanged)
```

Every input is already on the phone: the appointment (loaded by the page), and the rules,
templates, services and clients of the cache snapshot.

### 1. Which rules count

One pure function in `web/src/lib/nextVisit.ts` (new):

`nextVisit(appointment, rules, templates, services, zone, today): { choices: Choice[]; otherDate: Line[] }`

with `Choice = { days: number; day: string; lines: Line[] }` (`day` is a local `YYYY-MM-DD`).
`otherDate` holds the services for a day she types in "Otra fecha".

| Rule | Counts? |
| --- | --- |
| `before_appointment` | No |
| `come_back` for a service on the appointment, and it can send | Yes: a **specific** rule |
| `come_back` for a service that is not on the appointment | No |
| `come_back` with no service (a general rule), and it can send | Only when no specific rule counts |
| Any rule that cannot send (it, its template, its service or its return service is archived) | No |

"Can send" is the existing `canSend` in `web/src/lib/rules.ts`, the phone's copy of
`ruleRow.canSend` in `internal/reminders/due.go`. It and `effectiveReturn` are exported from
`rules.ts` for this second caller; no second definition is written. It is exported under the name
`ruleCanSend` **(agent)**: `web/src/lib/pending.ts` already exports another `canSend`, for a due
item. **(agent)** A rule that cannot
send is left out so the phone and the reminders agree on which rules are in force.

`applies_from` and `handover_from` are not read **(agent)**: they limit which reminders are shown,
not when a client should return.

### 2. The date choices

*   The days are counted from the local day of the appointment's `ends_at`, the day the come-back
    reminder counts from ("Reminder rules", `release-1.md`).
*   One choice per different `offset_days` among the rules that count, in rising order. Two rules
    with the same number of days give one choice.
*   A choice whose day is before `today` is left out **(agent)**. A choice that falls today stays.
*   With no rule that counts, or when every choice is in the past, `choices` is empty and the sheet
    shows only "Otra fecha".
*   `today` is `clock.today(zone)`, as Nueva cita computes it. The function takes it as an
    argument; it never reads the clock.

A come-back rule has 1 to 365 days, so there is no "same day" choice.

### 3. The services

*   Each service on the appointment is replaced by what its rule says comes next **(owner)**: the
    effective return service of its specific rule. A service with no specific rule is copied as it
    is. A general rule has no return service, so it never replaces one.
*   **The services follow the date she picks (owner).** For the choice of N days, a service uses
    its specific rule of N days when it has one (two such rules: the lower id). A service with no
    rule of N days uses its specific rule with the fewest days (then the lower id) **(agent)**.
    Example: "Acrílico, 21 → Relleno" and "Acrílico, 60 → Acrílico" give Relleno under "En 21
    días" and Acrílico under "En 60 días".
*   For "Otra fecha" no number of days was picked, so each service uses its specific rule with the
    fewest days (then the lower id) **(agent)**.
*   All the services go into one appointment, whatever date she picks **(owner)**: a service is
    never left out because its rule has other days.
*   The lines start from the appointment's services grouped into counts, the grouping `formFrom`
    does today; it becomes one exported `linesOf` in `booking.ts` that both use.
*   The count of a line is kept ("Relleno ×2"). When two lines end up as the same service, they
    become one line with the larger of the two counts **(agent)**: a full set and a repair on the
    same visit are followed by one refill, not two.
*   A service that is archived is left out, because the server refuses it on a new appointment.
    A return service is never archived here: such a rule cannot send and does not count.
*   A list can be empty (an appointment with no services, or all archived). The sheet then shows
    "Sin servicios" in its place and Nueva cita asks for the end time, as it does today.

### 4. The sheet (`web/src/lib/components/NextAppointmentSheet.svelte`, new)

A feature component made of existing primitives (`BottomSheet`, `Button`, `TextField`, `Callout`),
like `OverlapSheet`. It needs no new primitive, token or `.pen` component.

Top to bottom:

1.  Title: "Agendar próxima cita".
2.  One choice per date: "En 21 días · 3 dic" ("En 1 día · …" for one day; the date uses
    `shortDay`), and under it the names of that choice's services joined with commas ("Relleno,
    Pedicure"; a count above one reads "Relleno ×2", with the existing
    `appointment_service_count`), or "Sin servicios". The whole choice is one control she taps: a
    `glass` `Button` with the date on its first line and the services on its second **(agent)**.
    It shows the busy spinner while the draft is written. A `ListRow` was tried first: its
    subtitle is in the muted ink, and on the glass sheet at 1440 px axe measured 4.02:1 for it
    over a dark button of the page behind (the probe needs 4.5:1). A button has its own fill and
    the normal ink. A new primitive is not allowed without the `.pen` file (`rules/ui.md`, stage
    1).
3.  A date field labelled "Otra fecha", with the services of `otherDate` as its hint line (the
    `hint` of `TextField`, read with the field by a screen reader) **(agent)**.
4.  A button "Usar esta fecha". Until the field holds a real
    day the button is off with the reason "Elige el día" (the existing `appointment_day_needed`; a
    `Button` cannot be off without a reason). Any day is accepted, a past one too: she can type in
    a visit from the notebook, as Nueva cita allows.
5.  The message slot: the replace question (CUJ N4) or the write error. The question is the
    block Cotizar shows today; it becomes one feature component, `ReplaceDraftPrompt.svelte`, that
    both use (a copy fails jscpd).
6.  "Cancelar". It is not rendered while the question shows **(agent)**, so only one "Cancelar"
    is on screen: the question's.

Picking a choice or "Usar esta fecha" runs, inside `submitOnce`:

`handOff({ ...blankForm(day), clientId, lines, time }, replace)`

where `lines` is the picked choice's (or `otherDate`), `time` is the appointment's start time in
the business zone (`formatTime(starts_at, zone)`), `end` stays `null` (it follows the services),
and `notes` and `attrs` stay empty.

*   `'needs_confirm'`: the slot shows the question; "Reemplazar" calls again with `replace` true for
    the same day.
*   `'ok'`: `goto` to `/citas/nueva?volver=/citas/<id>` with `replaceState: true` (see "History").
*   `'failed'`: the slot shows "No se pudo crear la cita. Intenta de nuevo" as an alert; the sheet
    stays open.

Nueva cita opens, finds its draft, restores it with `restoreForm` and shows its usual "Recuperamos
lo que habías escrito." note with "Empezar de nuevo", as after Cotizar. Save, Cancel and the header
arrow all go to `volver`, the appointment she came from **(agent)**; see Alternative A7.

**History (agent).** This is the first caller that opens Nueva cita with `volver`, and the
appointment page's back arrow is `history.back()`. Two things keep Back from looping:

*   The sheet opens Nueva cita with `replaceState`, so Nueva cita takes the appointment page's
    history entry. Save and Cancel already replace Nueva cita's entry with `volver`, so one entry
    for the appointment is left, and its back arrow goes to the page she was on before.
*   Nueva cita's header arrow is a link today, so it adds an entry, and Back on the appointment
    page would then open Nueva cita again with the draft restored. The arrow becomes a button that
    does what Cancel does: `goto(back, { replaceState: true })`. This is the one change to
    `routes/citas/nueva/+page.svelte`, and it holds for every caller.

### 5. The hand-off (`web/src/lib/cache/quoteHandoff.ts` → `citaHandoff.ts`)

`convertQuote(lines, today, replace)` becomes `handOff(form, replace)` with the same steps (read
the `cita:new` draft; ask when one exists; parse with `FormSchema`; write with `putDraft`, awaited)
and the same tagged result. Cotizar passes `{ ...blankForm(today), lines }`. The file and its test
are renamed, and the log code `quote_convert_failed` becomes `cita_handoff_failed`.

`quick-quotes.md` rejected a generic draft helper because it had one caller (its Alternative C3).
There are two callers now with the same obligation, so one function serves both
(`rules/maintainability.md`, "Search before you write").

### 6. The button (`web/src/routes/citas/[id]/+page.svelte`)

An `ActionButton` (variant `glass`) in the action row, after the status buttons and before
"Editar". `ActionButton` wraps `WriteButton`, so offline it is off and says why (CUJ N5).

It is rendered only when all of these hold; otherwise it is not rendered
(`rules/ui.md`, "A control she must not use is not rendered"):

*   the appointment's status is `scheduled` or `confirmed`;
*   the cache snapshot is loaded;
*   the client is in the snapshot's client list (an archived client cannot be booked).

The page owns `open` for the sheet and passes the appointment and the snapshot.

### 7. Effect on reminders

None is built. The existing rules already give the wanted result:

*   A specific come-back item is held back by an appointment that starts after the visit ends and
    includes the rule's return service (`internal/reminders/due.go`, the later-appointment check).
    The new appointment includes the return service of the rule that decided each service
    (section 3), and that is the rule of the date she picked when the service has one.
*   A general item is removed by any later appointment of the client.

Two cases where a reminder stays, both accepted **(agent)**:

*   **Another rule on the same service with a different return service** ("21 días → Relleno" and
    "60 días → Acrílico"). She picks "En 60 días" and books Acrílico; the 21-day rule still gives
    its item on day 21, because no later booking has Relleno. That rule is hers; the owner's words
    for this kind of case: if the rule is there on purpose, it reminds her what comes next.
*   **"Otra fecha" on a day before the visit ends.** Such a booking is not a later appointment, so
    it holds nothing back.

The phone computes nothing about what is due. It reads two fields of a rule (`offset_days` and the
return service). The rule "the due list is never computed on the phone" (`rules/ui.md`) is kept.

### 8. Texts (`web/src/lib/i18n/es.json`)

| Key | Text |
| --- | --- |
| `appointment_next` | Agendar próxima cita **(owner)** |
| `next_choice_one` | En 1 día · {date} |
| `next_choice_other` | En {n} días · {date} |
| `next_other_date` | Otra fecha |
| `next_use_date` | Usar esta fecha |
| `draft_replace_prompt` (renamed from `quote_replace_prompt`) | Ya tienes una cita sin guardar. ¿Reemplazarla? |
| `draft_replace` (renamed from `quote_replace`) | Reemplazar |
| `cita_handoff_failed` (renamed from `quote_convert_failed`) | No se pudo crear la cita. Intenta de nuevo |

"Sin servicios" reuses `appointment_no_services`; "Cancelar" reuses `cancel`.

### 9. Rollout and rollback

Web only. It ships with the next image. A phone with the old web keeps working: nothing it calls
changes. Rollback is the previous image; no data was written in a new shape (the `cita:new` draft
keeps `FormSchema`).

### 10. Stages

| Stage | What | Behaviour change |
| --- | --- | --- |
| S1 | Rename and generalize the hand-off; the replace question as one component; rename three text keys | None |
| S2 | `nextVisit` (with the two exports from `rules.ts` and `linesOf`), the sheet, the button, texts, tests, docs | The feature |

The two exports from `rules.ts` land in S2, with their first outside user: knip fails an export
nothing imports.

## Risks

| Risk | Handling |
| --- | --- |
| The phone and the server disagree on which rules are in force | One `canSend` on the phone, already the copy of the server's for the handover; `nextVisit` adds no rule logic of its own beyond picking by service |
| The cached rules are stale | The cache refreshes as on every screen; the worst result is a date choice from a rule she just changed, and she sees the date before she saves |
| She loses a half-written Nueva cita | The replace question (CUJ N4) |
| The copied start time is taken | The existing overlap answer and "Agendar igual" |
| "Recuperamos lo que habías escrito." reads oddly after the button | Accepted, as for Cotizar (`quick-quotes.md`, Risks) |
| She leaves Nueva cita without saving and the filled draft stays; the next "+" on Hoy opens it | Accepted, as for Cotizar: the form says "Recuperamos lo que habías escrito." and has "Empezar de nuevo"; the next tap on the button asks before replacing it |
| Back loops between the appointment and Nueva cita | Section 4, "History"; an e2e step taps Back after Save and after the header arrow |

Two facts the builder check confirmed in the code: the bootstrap lists only active clients
(`internal/httpapi/handlers_bootstrap.go`, `List(ctx, tx, false)`), so "the client is in the
snapshot" means "not archived"; and the cached appointment and `GET /appointments/{id}` share
`AppointmentSchema`, whose `services` has one entry per copy, so `nextVisit` groups them into
counts as `formFrom` does.

## Alternatives Considered

### A1: Copy the exact same services (owner's first wording)

*   **Cons**: with a rule "Acrílico → Relleno", the new booking would hold Acrílico, and on day 21
    the app would still ask "¿Agregar Relleno?".
*   **Reason for Rejection**: the owner chose what the rule says. If the result surprises her, the
    rule is what she fixes.

### A2: Each date choice carries only the services of its own rule

*   **Cons**: Acrílico + Pedicure would need two taps and two bookings; a service with no rule
    would be on none of them.
*   **Reason for Rejection**: owner decision, one appointment with all the services.

### A3: The rule with the fewest days always decides the services

With two rules on one service (21 days → Relleno, 60 days → Acrílico), every date choice would book
Relleno, and the services would be listed once at the top of the sheet.

*   **Pros**: less logic; one list in the sheet.
*   **Cons**: picking "En 60 días" would book Relleno, and on day 60 the app would still ask
    "¿Agregar Acrílico?" (found by the builder check).
*   **Reason for Rejection**: owner decision, the services follow the date she picks.

### A4: The server computes the choices

*   **Pros**: one definition of "can send".
*   **Cons**: a new route, spec and handler for two fields the phone already holds; it would not
    work from the cache.
*   **Reason for Rejection**: unlike the row label (`reminders-and-replies.md`, Alternative A8),
    nothing here needs the log, the 60-day lifetime or old visits.

### A5: Create the appointment straight from the sheet

*   **Cons**: a second place that creates appointments, with its own overlap sheet and errors, and
    no chance to change the time.
*   **Reason for Rejection**: Nueva cita already does all of it.

### A6: Pass the values in the URL

*   **Reason for Rejection**: rejected for Cotizar (`quick-quotes.md`, Alternative C1); the draft is
    the form's one input.

### A7: After saving, open the new appointment

*   **Cons**: Nueva cita sends Save, Cancel and the header arrow to the same `volver`, so they
    would need two targets; a larger change to a shared page for one caller.
*   **Reason for Rejection**: return to where she started. Listed in Future Improvements.

### A8: A date field only, no rule choices

*   **Reason for Rejection**: she would work out "21 days from the 12th" by hand, the step the rule
    already knows.

## Future Improvements

*   The same button on a come-back row in Enviar.
*   Open the new appointment after saving (A7).
*   A choice from the client's own usual gap between visits.

## Detailed Implementation Plan

Each stage ends with the gate of `CLAUDE.md`. S2 also runs `make e2e-docker`.

### Stage S1: refactor, no behaviour change

1.  `web/src/lib/cache/quoteHandoff.ts` → `citaHandoff.ts`, `quoteHandoff.test.ts` →
    `citaHandoff.test.ts` (Move, Modify): `handOff(form, replace)`; code `cita_handoff_failed`.
2.  `web/src/lib/components/ReplaceDraftPrompt.svelte` (Create): the replace question, moved out
    of `QuoteForm.svelte`.
    `web/src/lib/components/QuoteForm.svelte` (Modify): call `handOff` with
    `{ ...blankForm(today), lines: visible }`; use the new component and the renamed text keys.
3.  `web/src/lib/i18n/es.json` (Modify): the three renamed keys.
4.  `web/tests/e2e/quote.spec.ts` (Modify): the renamed keys and log code.
5.  `docs/designs/quick-quotes.md`, `docs/designs/reminders-and-replies.md` and `TASKS.md`
    (Modify): the renamed function, file, keys and code, wherever they name the old ones.
6.  (moved to S2, item 7: the exports from `rules.ts`.)

### Stage S2: the feature

7.  `web/src/lib/nextVisit.ts` and `nextVisit.test.ts` (Create): sections 1 to 3, one test case
    per table row and bullet, with a fixed `today`. `web/src/lib/rules.ts` (Modify): export
    `ruleCanSend` (today's `canSend`) and `effectiveReturn`. `web/src/lib/booking.ts` (Modify):
    export `linesOf` and `validDay`.
8.  `web/src/lib/components/NextAppointmentSheet.svelte` (Create): section 4.
9.  `web/src/routes/citas/[id]/+page.svelte` (Modify): section 6.
    `web/src/routes/citas/nueva/+page.svelte` (Modify): the header arrow replaces the history entry
    (section 4, "History"), and its comment says so.
10. `web/src/lib/i18n/es.json` (Modify): the five new keys of section 8.
11. `web/tests/e2e/ui-probe.spec.ts` (Modify): a `sheets` entry for `/citas/[id]`.
12. `web/tests/e2e/next-appointment.spec.ts` (Create): CUJ N1 (with the reminder check of Goal 4
    and Back after Save), N2, N3 (with Back after the header arrow), N3b and N4;
    `web/tests/e2e/offline.spec.ts` (Modify): CUJ N5.
13. `docs/designs/release-1.md` (Modify): one bullet in "Appointments" that names the button and
    links here. `CLAUDE.md` (Modify): this document in "Architecture source". `docs/prd.md`
    (Modify): the requirement, in Spanish, beside the booking ones.
14. Move `feature-ideas-brief.md` to `history/` only when its three parked ideas are designed or
    dropped; until then it stays where it is.

### Verification Plan

*   The gate, with its output reported.
*   `make e2e-docker`: the new spec, `quote.spec.ts`, `offline.spec.ts` and `ui-probe`.
*   Fault injection: break "the services follow the picked days", "the fewest days decide
    otherwise", "general rules only without a specific rule" and "a past choice is left out" one
    at a time, and see a unit test go red each time.
*   UI review at 393 × 852 and 1440 × 900 with a screenshot of the sheet in each (`rules/ui.md`,
    stage 6).
