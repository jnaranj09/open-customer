# Reminder rules, quick replies and service descriptions

*   **Author:** jnaranj09
*   **Status:** Approved (review rounds 1 and 2 resolved)
*   **Date:** 2026-11-10
*   **Predecessor:** [history/reminders-and-replies-brief.md](history/reminders-and-replies-brief.md),
    the brainstorm brief. Its "Decided (owner)" list holds decisions D1-D21, cited here by number.
    Two more owner decisions came from the design review: **D22** (the look-ahead is a separate
    pass, see "Look-ahead pass") and **D23** (archiving a service asks the handover question, see
    "Handover").
*   **Builds on:** [release-1.md](release-1.md) (the app as built) and
    [quick-quotes.md](quick-quotes.md) (Servicios and Cotizar)
*   **Separate from:** [password-policy.md](password-policy.md)

--------------------------------------------------------------------------------

## Overview

open-customer serves one owner of a small service business. She keeps clients, appointments and
hand-sent WhatsApp reminders in the app, on her iPhone. She still keeps a paper notebook next to
the app. This design changes six things so she can drop the notebook:

1.  **A general come-back rule.** One rule for "Todos los servicios" covers every client. She adds
    a rule for one service only where that service needs a different gap. A second general rule
    with a later gap (for example 30 and 45 days) gives a second message.
2.  **Visible rules.** Each reminder in the list carries a short label (see "Row label"): the
    appointment it comes from and, where they apply, the rule's days, the message number and the
    days left. The toast after an action says what happens next. The rule editor shows how many
    clients a rule reaches today and the next one.
3.  **A wording pass.** 45 texts become short labels ("Atributo fijo" / "Atributo por cita"), and
    a rule is called a "regla" everywhere (Detailed Implementation Plan, item 5).
4.  **A service description**, added at the end of the Cotizar text.
5.  **Quick replies ("Respuestas").** Saved plain-text messages she copies with one tap, offline
    too.
6.  **New tabs.** "Enviar" gets the tabs "Recordatorios" and "Respuestas". Ajustes gets one entry,
    "Recordatorios", with the tabs "Reglas" and "Plantillas".

The principle behind items 2 and 3: show each rule's effect where it happens, not help text about
the rule.

### Terminology

*   **Rule** ("regla"): what she sets up in Ajustes. A **come-back rule** sends a message some days
    after a visit. A **specific rule** is a come-back rule for one service. A **general rule** is a
    come-back rule for "Todos los servicios". A **"before" rule** sends a message some days before
    an appointment.
*   **Reminder** ("recordatorio"): one row in "Enviar" for one client. In the API it is a **due
    item**.
*   **Visit**: a done appointment. A **candidate visit** is a visit inside the due list's load
    window (its end is within the largest come-back gap plus 60 days before today).
*   **Source appointment**: the appointment a reminder comes from: the visit for a come-back
    reminder, the upcoming appointment for a "before" reminder.
*   **Can send** (a rule): the rule, its template, its service and its return service are all not
    archived.
*   **Handover**: a specific rule stops being able to send, so the general rule takes over its
    visits. The two answers to the handover question are **from today** ("Solo de hoy en
    adelante") and **include late** ("Incluir atrasados").
*   **Old web**: the previous app version still cached on her phone until she taps "Actualizar".
*   **Today**: the business-local day. Existing code is described as "currently".
*   **She** is always the owner; **the client** gets the message. **The author** is the developer
    who runs upgrades on the server.

## Problem Statement

The reminder rules work, but she cannot see them working.

*   **One rule per service.** A come-back rule must name a service
    (`internal/db/migrations/00001_init.sql:151`). A business with many services needs many rules,
    and a service with no rule sends nothing. She covers the gaps in her notebook.
*   **Hidden lifetimes.** A reminder stays in the list from its due day until she marks it sent or
    skipped, or for 60 days. The 60 days live only in the server code
    (`internal/reminders/due.go:26`); no screen shows them, so a reminder leaves the list with no
    sign of why.
*   **No source.** A row shows the client's name and the message text. With two rows for one
    client, nothing says which visit made each one.
*   **The client page says nothing about what comes next.** She cannot see when a client will get
    her next message, so she keeps it in the notebook.
*   **Texts that explain instead of label.** For example, "¿Es igual siempre para la clienta, o
    cambia en cada cita?" where two short labels say the same. And "Recordatorios" names the rules
    in Ajustes, so it cannot also name the list of reminders.
*   **Cotizar has no detail.** A quote lists names and prices only; she adds what each service
    includes by hand.
*   **Retyped messages.** She writes the same answers (address, prices, care tips) many times.
    WhatsApp Business has its own quick replies; she wants them in the app too, next to her
    reminders (D16).

## Goals

*   **G1. General rule.** A come-back rule can apply to all services. Its reminder counts from the
    client's latest visit. Any appointment after that visit that is not cancelled or no-show
    removes it.
*   **G2. Specific wins while it can send.** A visit holding a service whose specific rule can send
    gets that rule's reminder, not the general one. Cost accepted: such a rule blocks the general
    rule for that visit even when it makes no reminder itself (for example, its gap is longer, or
    the visit is past its 60 days).
*   **G3. Several general rules, one row.** With general rules at 30 and 45 days, the client shows
    once: the latest due general reminder. The earlier unsent one leaves the list.
*   **G4. Mixed history stays two rows.** A specific reminder (nails, day 1) and a general reminder
    (eyebrows, day 10) for one client both show. Each names its visit.
*   **G5. Every row says where it comes from**, with the label grammar in "Row label". Online, the
    label opens the source appointment; offline it is plain text and has no countdown.
*   **G6. Every action says what happens.** The sent and skip toasts say the consequence in one
    line ("no se mostrará de nuevo", "siguiente el 15 nov").
*   **G7. The rule editor previews; the client page shows what is next.** The editor shows how many
    clients a come-back rule reaches today and the next one. The client page shows the client's
    next planned message.
*   **G8. No surprise flood, no second message.** When a specific rule stops being able to send
    (its rule, template, service or return service is archived) and a general rule would take
    over, the app asks the handover question (D14, D23). With no answer, it means "from today".
    Restoring asks the same question. A specific reminder never shows for a visit whose general
    reminder was already sent.
*   **G9. Short texts.** The texts in Detailed Implementation Plan, item 5 ship with those values.
*   **G10. Service description** in the catalog, the Cotizar text and the export.
*   **G11. Quick replies**: create, edit, archive, restore, her own order, search, one-tap copy;
    copy works offline.
*   **G12. Tabs** as in Overview item 6, with one shared tab component; existing URLs keep working.
*   **G13. Safe upgrade.** The old web keeps working against the new server. Each stage is its own
    release.

## Non-Goals

*   The password policy (its own design).
*   A follow-up that counts from the day she sent the first message (dropped, D3).
*   Variables in quick replies (plain text only).
*   Syncing with WhatsApp Business quick replies.
*   Sending messages automatically (still `wa.me` links she taps).
*   Undo of a "sent" mark (the current app has none; this design adds none).
*   Changing the 60-day lifetime, or making it a setting.
*   A message number on "before" reminders.
*   A preview for "before" rules (it would need every future appointment loaded).
*   Predicting come-back messages from a booked, not yet done, appointment. A client with a
    booking and no "before" rule shows `Sin mensajes pendientes`; her appointments list on the same
    page shows the booking.
*   Three surfaces from the brief's UX table that D9 did not list: a count on each rule in the rule
    list, "próximo" in the empty Enviar state, and a details popup on the label. The label already
    opens the appointment.

## Open Questions

*   None.

## Critical User Journeys (CUJs)

Labels follow "Row label". Dates are in the business time zone.

### CUJ R1: One general rule covers everyone

*   **Given**: general rule "Todos los servicios, 30 días"; Ana had nails on 1 Oct and nothing
    after; no nails rule.
*   **When**: she opens "Enviar" on 31 Oct.
*   **Then**: Ana shows once: `Cita 1 oct · Uñas · General 30 días · quedan 60`. If Ana has any
    appointment after 1 Oct that is not cancelled or no-show (even one booked for December), the
    reminder does not show. If that appointment is cancelled, the reminder comes back.

### CUJ R2: Specific rule, then mixed history

*   **Given**: rule "Uñas, 21 días"; general rule 30 días; Ana had nails on 1 Nov and eyebrows on
    10 Nov; no eyebrows rule.
*   **When**: she opens "Enviar" on 22 Nov and on 10 Dec.
*   **Then**: on 22 Nov a row `Cita 1 nov · Uñas · quedan 60`. On 10 Dec a second row
    `Cita 10 nov · Cejas · General 30 días · quedan 60`; if the first is still unsent, both show.
    Online, tapping a label opens that appointment.

### CUJ R3: Two general rules

*   **Given**: general rules at 30 and 45 days; Ana's last visit 1 Oct, nails.
*   **When**: she opens "Enviar" on 31 Oct, sends it, and opens "Enviar" again on 15 Nov.
*   **Then**: on 31 Oct `Cita 1 oct · Uñas · General 30 días · quedan 14` (it shows until 14 Nov,
    the day before the 45-day reminder is due). On 15 Nov one row,
    `Cita 1 oct · Uñas · General 45 días · 2º mensaje · quedan 60`. If she had not sent the 30-day
    one, on 15 Nov only the 45-day row shows, without `2º mensaje`.

### CUJ R4: A specific rule stops sending, then comes back

*   **Given**: rule "Uñas, 21 días" and a general rule; 12 nails clients in the last 60 days.
*   **When**: she archives the nails template (or the nails rule, or the nails service); later she
    restores it.
*   **Then**: on archive the app asks "Solo de hoy en adelante / Incluir atrasados". With "from
    today" no old reminder appears; with "include late", the general reminders for those visits
    appear as late. On restore it asks again for the nails reminders. A client whose general
    reminder was already sent gets no nails reminder for that visit.

### CUJ R5: Acting on rows

*   **Given**: today 31 Oct, online. Ana's 30-day general row (visit 1 Oct) with a 45-day general
    rule ahead; Bea's general row (visit 1 Oct, only the 30-day rule).
*   **When**: she sends Ana's and skips Bea's.
*   **Then**: toasts `Enviado a Ana · siguiente el 15 nov` and
    `Omitido: Bea · no se mostrará de nuevo · Deshacer`. With no general rule ahead, the sent toast
    is `Enviado a Ana`.

### CUJ R6: Offline

*   **Given**: the phone has no connection; the list was loaded yesterday.
*   **When**: she opens "Enviar".
*   **Then**: rows show their labels as plain text with no countdown. She opens "Respuestas", finds
    "Dirección" by typing "direc", and copies it.

### CUJ R7: Rule preview

*   **Given**: today 2 Nov; general rule "30 días"; four clients whose last visit was 12 Oct, Carla
    whose last visit was 20 Oct, Ana whose last visit was 3 Oct.
*   **When**: she edits the rule and changes the days to 21.
*   **Then**: under the rule sentence, `Hoy: 4 clientas · próxima: Carla, 10 nov`. Ana's reminder
    (due 24 Oct) is late, so it is not counted.

### CUJ R8: Cotizar with descriptions

*   **Given**: "Uñas acrílicas" has the description "Incluye limado y esmalte".
*   **When**: she quotes it with another service that has no description.
*   **Then**: the text ends with a blank line and `Uñas acrílicas: Incluye limado y esmalte`. With
    no described service the text is exactly as today.

### CUJ R9: Upgrade with the old web open

*   **Given**: the server is updated to S3; her phone still runs the old web; a general rule exists.
*   **When**: she opens the app before tapping "Actualizar", looks at the rules, saves a change to
    the general rule, and archives the nails rule.
*   **Then**: it loads; the rule list shows the general rule with "…" for its service; saving it
    shows "Elige un servicio" and sends nothing; archiving the nails rule hands its visits over
    "from today" (no flood).

### CUJ R10: Client page

*   **Given**: Ana's last visit 1 Oct; general rule 30 días.
*   **When**: she opens Ana's page on 20 Oct, on 31 Oct and on 5 Nov (not sent).
*   **Then**: `Próximo mensaje: 31 oct (Uñas)`, then `Mensaje para hoy (Uñas)`, then
    `Mensaje atrasado desde 31 oct (Uñas)`. With nothing planned: `Sin mensajes pendientes`.
    Offline the line is not shown.

### CUJ R11: Quick replies

*   **Given**: no quick replies.
*   **When**: she opens Enviar › Respuestas, creates "Dirección" and "Precios", moves "Precios" up,
    edits its text, archives "Dirección", opens "Archivadas (1)" and restores it.
*   **Then**: the list shows her order; search "prec" finds "Precios"; "Copiar" puts the text on the
    clipboard and shows "Copiado".

### CUJ R12: Tabs and old URLs

*   **Given**: bookmarks to `/ajustes/plantillas` and `/ajustes/recordatorios`.
*   **When**: she opens each, and taps "Recordatorios" in Ajustes.
*   **Then**: each URL opens its own tab inside "Recordatorios"; Ajustes shows one entry,
    marked open on both; the tab bar moves between them.

## Technical Plan (Architecture)

### Where each piece lives

| Piece | Module (owns its tables) | Web |
| :--- | :--- | :--- |
| General rule, one row per client for general reminders (D11), handover, look-ahead, preview, next message | `internal/reminders` (`reminder_rules`, `reminder_log`) | `RuleSheet.svelte`, `rules.ts`, `due.ts`, `DueItemRow.svelte`, `ui/DueCard.svelte`, client page |
| Service description | `internal/services` (`services`) | `ServiceSheet.svelte`, `quote.ts` |
| Quick replies | new `internal/replies` (`quick_replies`) | new route `/enviar/respuestas` |
| Tabs | none | new `ui/TabNav.svelte` from the Servicios nav |
| Row label | none | new `ui/LabelChip.svelte` |
| Wording | none (server texts in `internal/platform/problem/messages.go`) | `es.json` |

The web is embedded in the server binary, so a new web never runs against an old server. The only
version skew is the **old web against a new server**. Every API change below is additive for that
reason: new fields, new endpoints, no new enum values, no removed field. Zod objects on the phone
drop unknown keys, so the old web ignores new fields.

### Data model (one migration per schema-changing stage)

Each migration takes the next free number when its stage lands. `password-policy.md` also adds a
migration; whichever lands first takes `00003`. Tests never hard-code the number (see the
`internal/status` test in the plan).

`services_description` (S2):

```sql
ALTER TABLE services ADD COLUMN description text NULL
  CHECK (description IS NULL OR char_length(description) BETWEEN 1 AND 200);
```

`reminder_general_rule` (S3):

```sql
-- 00001 has an unnamed CHECK ((kind = 'before_appointment') = (service_id IS NULL)).
-- Find it by its definition and fail unless exactly one matches.
DO $$
DECLARE names text[];
BEGIN
  SELECT array_agg(conname) INTO names FROM pg_constraint
  WHERE conrelid = 'reminder_rules'::regclass AND contype = 'c'
    AND pg_get_constraintdef(oid) LIKE '%before_appointment%service_id IS NULL%';
  IF coalesce(array_length(names, 1), 0) <> 1 THEN
    RAISE EXCEPTION 'expected one service CHECK on reminder_rules, found %', names;
  END IF;
  EXECUTE format('ALTER TABLE reminder_rules DROP CONSTRAINT %I', names[1]);
END $$;
ALTER TABLE reminder_rules ADD CONSTRAINT reminder_rules_service_check
  CHECK (kind = 'come_back' OR service_id IS NULL);
ALTER TABLE reminder_rules ADD CONSTRAINT reminder_rules_general_return_check
  CHECK (service_id IS NOT NULL OR return_service_id IS NULL);
ALTER TABLE reminder_rules ADD COLUMN handover_from timestamptz NULL;
ALTER TABLE reminder_rules ADD CONSTRAINT reminder_rules_handover_check
  CHECK (handover_from IS NULL OR (kind = 'come_back' AND service_id IS NOT NULL));
```

*   A **general rule** is `kind = 'come_back'` with `service_id IS NULL` (D19). It has no return
    service and never makes a booking prompt.
*   `handover_from` lives only on a specific rule. It is her answer to the handover question, not
    a derived value, so storing it keeps the rule that derived values are never stored
    ([`CLAUDE.md`](../../CLAUDE.md), load-bearing constraint 4).

`quick_replies` (S5):

```sql
CREATE TABLE quick_replies (
    id          uuid PRIMARY KEY,
    title       text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 60),
    body        text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    position    int NOT NULL,
    archived_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
```

The title is what the list and the search show; the body is what she copies. `position` works as
`services.position` does (`internal/platform/position`): 0 to `position.Max`, not unique, a new
row gets the current maximum + 1 (`position.Next`).

`reminder_log` does not change. Each general rule is its own rule, so the key
`(rule_id, appointment_id, anchor_at)` stays unique per reminder.

### Due-list rules (`compute`, `internal/reminders/due.go`)

Currently `comeBackItem` returns nothing for a rule with no service (`due.go:393`). A new
function `generalItem` runs once per client, not per (rule, visit).

**Where "can send" is checked.** Currently: rule archived in SQL (`ListActiveRulesForDue`,
`internal/db/queries/reminders.sql:48`); template and service archived at `due.go:345`; return
service archived at `due.go:356`. S3 loads archived specific rules too (for the handover), so it
adds an explicit "rule archived" check in Go, next to the one at `due.go:345`.

**`allowed` splits in two.** Currently `allowed` (`due.go:442-447`) refuses a due day after today
and a due day before the visit's `created_at` or the rule's `applies_from`. It becomes:

*   `dueReached(due, today)`: the due day is today or earlier;
*   `allowedFrom(d, r, a, due)`: the `created_at` and `applies_from` checks.

`compute` uses both. `next_due_date` and the look-ahead pass use only `allowedFrom`.

```text
for each client with a candidate visit:
  v = the client's latest candidate visit: the latest starts_at; on a tie, the higher id
      (ids are UUIDv7, so the later-created) is later
  if the client has an appointment that is not cancelled or no-show and starts after v.starts_at,
      or at the same start with a higher id: no general reminder                           -- G1
  if a specific rule that can send covers v (v holds its service): no general reminder     -- G2
  cutoff = the latest handover_from among specific rules that cannot send and cover v;
           NULL when none of them has one (a non-NULL answer wins over an "include late" one)
  candidates = general rules g that can send, with due(g) = v's end day + g.offset_days:
      dueReached(due(g)), due(g) >= today - 60 days, allowedFrom(g, v, due(g)),
      cutoff is NULL or due(g) >= day(cutoff)                                              -- G8
  pick = the candidate with the latest due; ties: larger offset, then higher rule id       -- G3
  if pick is logged (sent or skipped): no general reminder (earlier candidates stay hidden)
  else: one reminder for (pick, v)
```

*   **Specific reminders** keep release-1's rule: only a later appointment with the rule's return
    service removes one, and a future booking without it turns it into a booking prompt
    (`release-1.md`, Alternative E5, chosen there in a modified form). New: a specific reminder for
    visit v is not shown when a general reminder for v is logged as sent (G8).
*   **"Later" compares start times.** Currently `laterAppointments` (`due.go:276-285`) loads from
    the earliest candidate `ends_at`. It changes to the earliest candidate `starts_at`, and
    `ListLaterAppointments` changes `starts_at > @after` to `>=`, so the same-start tie reaches
    `compute`.
*   **Loading.** `ListActiveRulesForDue` becomes `ListRulesForDue`: active rules plus archived
    specific come-back rules, with `archived_at` and `handover_from`. `ruleRow` gains `Archived`
    and `HandoverFrom`. Archived rows never make reminders and do not count in `maxComeBack`.
*   **Daily push.** The scheduler counts `DueList` (`internal/scheduler/scheduler.go:157`), so from
    S3 the daily notification counts general reminders too. That is intended.

### Look-ahead pass (D22)

Owner decision D22: the due list stays `compute(d, now)`. A separate function
`upcoming(d, now, horizon)` answers "what comes next":

*   It reuses `comeBackItem`, `generalItem`'s steps and `allowedFrom`, and replaces `dueReached`
    and the 60-day cut with "after today and on or before `horizon`". The log is not checked (a
    future reminder cannot be logged).
*   **Specific reminders**: from the same candidate visits. A future due day is at most
    `maxComeBack` days after a visit, so the current load window (`today - (maxComeBack + 60)`)
    already holds every visit that can have one.
*   **General reminders**: the same `v`, G1, G2 and cutoff checks; the result is the candidate with
    the **earliest** due after today (the next one she will see).
*   **"Before" reminders**: only for the client page, from that client's appointments in the next
    365 days (new query `ListClientUpcomingAppointments`: the client's `scheduled` or `confirmed`
    appointments with `starts_at` in `(now, now + 365 days]`, with their service lines).
*   `horizon` is today + 365 days (the largest allowed offset).

The two reads:

*   `POST /reminder-rules/preview` (operation `PreviewRule`), only for come-back rules. Body
    `RulePreviewInput`: the come-back rule form fields (`service_id` or null, `return_service_id`,
    `offset_days`, `template_id`) plus `rule_id` (the rule being edited, or absent). Checks: the
    same field checks and messages as creating a rule (422); an unknown `rule_id` → 404; a
    `rule_id` of a "before" rule → 422 on `rule_id`. The handler loads the due data in
    `s.snapshot`, with the draft's offset counted in `maxComeBack` before the windows are sized,
    puts the draft in place of the saved rule, and answers `RulePreviewResult`
    `{ today_count, next: { client_name, due_date } | null }`:
    *   `today_count` = clients with a reminder of that rule due exactly today (late ones are not
        counted, so the number does not depend on her handover answer);
    *   `next` = the earliest reminder of that rule from `upcoming`.

    It is a POST because it carries a draft, but it only reads, so it uses `s.snapshot`, not
    `s.inWriteTx`.
*   `GET /clients/{id}/next-reminder` (operation `GetClientNextReminder`): in `s.snapshot`, the
    earliest reminder of that client from `compute` (due today or late) or else from `upcoming`;
    ties by due day, then rule id. Answer `NextReminder` `{ due_date, source_services, general }`,
    or 204 when there is none, 404 for an unknown client (an archived client is allowed).

### New fields on each due item (D20)

| Field | Type | Value |
| :--- | :--- | :--- |
| `source_starts_at` | instant | the source appointment's start |
| `source_services` | string[] | its service names, in line order |
| `general` | boolean | true for a general rule's reminder |
| `offset_days` | int | the rule's days (the N of "General N días") |
| `shown_until` | day | specific: due + 60 days; general: the earlier of due + 60 days and the day before `next_due_date`; "before": the source appointment's day |
| `message_number` | int | come-back: the number of `sent` log rows of come-back rules (specific or general) for the same visit, + 1; skipped rows do not count; "before": 1 |
| `next_due_date` | day, nullable | general only: the earliest `due(g')` after the pick's due among general rules `g'` that can send and pass `allowedFrom` and the cutoff; NULL when none |

`message_number` follows D15 (messages sent to the client since that visit, reset by a new visit):
two specific rules on one visit give `2º mensaje` on the second. `general` and `offset_days` come
from the server, not the cached rule, because an archived rule may be missing from the phone's
cache.

`ListLogKeys` becomes:

```sql
SELECT l.rule_id, l.appointment_id, l.anchor_at, l.action, r.kind, (r.service_id IS NULL) AS general
FROM reminder_log l JOIN reminder_rules r ON r.id = l.rule_id
WHERE l.appointment_id = ANY(@ids::uuid[]);
```

`dueData.Logged` stays a set of keys. New: `dueData.SentByVisit map[uuid.UUID]int` (sent
come-back rows per appointment) and `dueData.GeneralSent map[uuid.UUID]bool` (a sent general row
exists for the appointment). All are filled in the same snapshot; nothing is stored.

### Row label

`dueLabel(item, today, online)` in `web/src/lib/due.ts` builds the label from the due-item fields
only. Segments, in this order, joined by " · ":

| Reminder | Segments |
| :--- | :--- |
| Specific | `Cita {d mmm}` · services · [`{k}º mensaje`] · [days left] |
| General | `Cita {d mmm}` · services · `General {N} días` · [`{k}º mensaje`] · [days left] |
| "Before" | `Cita {d mmm} {HH:mm}` · services |

*   Services: the first two names joined by ", ", then `+{n}` for the rest.
*   `{k}º mensaje` only when `message_number` ≥ 2.
*   Days left: `n` = `shown_until` − today. `quedan {n}` when n ≥ 2, `queda 1 día` when 1,
    `Último día` when 0. Shown only online (D18): offline the cached list may be a day old.
*   The label is drawn by a new primitive, `ui/LabelChip.svelte`
    (`LabelChipVariant = 'normal' | 'warning'`): an `<a>` to `/citas/{appointment_id}` when online,
    a `<span>` offline (the appointment may not be cached). `warning` when n ≤ 7. The existing
    `ui/Chip.svelte` is a toggle button and is not reused.
*   In `DueItemRow.svelte` and `ui/DueCard.svelte` the label sits next to the row's button, never
    inside it.
*   Dates use a new `shortDay(day)` in `web/src/lib/time.ts` ("15 nov"), shared with the toast.

### Handover (G8, D14, D23)

The existing API carries the handover answer as `include_overdue` (boolean; false or absent =
"from today", true = "include late") on `RuleInput` and `RulePatch`. The design reuses it and adds
it to `TemplatePatch` and `ServicePatch`.

**Archive.** Only rules that can send before the change and cannot after it are touched, so an
earlier answer on a rule that had already stopped is kept, and an archive that changes nothing
writes nothing.

*   Rule archive (`UpdateRule`, `internal/reminders/rules.go:107`): when `archived_at` goes from
    NULL to set on a specific rule that could send (template, service and return service not
    archived), write `handover_from` = the module clock's now ("from today") or NULL ("include
    late").
*   Template archive: `UpdateTemplate` runs in its existing `s.inWriteTx` callback, **before**
    `templates.Update`, `reminders.SetHandover(ctx, q, HandoverScope{TemplateID: id},
    includeOverdue)`. It writes the specific rules under that template that can send at that
    moment. Then `templates.Update` archives the template. An already archived template stops no
    rule, so nothing is written.
*   Service archive (D23): the same in `UpdateService`, with `HandoverScope{ServiceID: id}` (rules
    whose service or return service is that service).

**Restore.** Restoring lets stopped rules send again, which can bring back their old reminders.

*   Rule restore: the existing `UpdateRule` already asks the question and writes `applies_from`
    (`rules.go:179-186`). New: it also clears `handover_from` when the rule can send after the
    restore.
*   Template or service restore: the handler calls, **before** the update,
    `reminders.RestartRules(ctx, q, scope, includeOverdue)`. It selects the specific rules that
    cannot send now only because of that template or service and will after, and sets their
    `applies_from` (= now for "from today", NULL for "include late", as a rule restore does) and
    clears `handover_from`.
*   Whatever the answer, a specific reminder never shows for a visit whose general reminder was
    already sent (Due-list rules).

**When the web asks.** `handoverAsks(change, rules, templates, services)` in
`web/src/lib/rules.ts` decides, from cached data, for the five changes (rule, template or service
archive; template or service restore): it asks when an active general rule exists that can send
and the change stops or restarts at least one specific rule. Otherwise the web sends no answer,
which means "from today". The question is the existing overdue sheet in `RuleSheet.svelte`; the
template and service flows show the same sheet after their own archive or restore confirmation
(one `.pen` frame for the combined flow). The old web never sends the new answers, so its
archives mean "from today".

The clock is the module clock (`m.clock`, [`CLAUDE.md`](../../CLAUDE.md) constraint 10). Each
module writes only its own table; the handler orchestrates in one transaction.

### Service description (D5)

`Service.description` is always present and may be null. It is optional in `ServiceInput` and
`ServicePatch`, where `null` clears it. Validated by a new `text.Line(raw, max)` in
`internal/platform/text/text.go`: trimmed; empty means NULL; at most 200 characters; a line break
is refused. Cotizar adds, after the duration line and a blank line, one `quote_detail_line`
"{name}: {description}" per picked service that has a description; a service picked twice gets
one line; lines follow the price-line order. Export: `servicios.csv` column `descripcion` after
the name, through the export's formula-injection guard (`neutralize` in `internal/exporter`), and
`datos.json`.

### Quick replies (G11)

*   Module `internal/replies`: `List`, `Create`, `Update` (title, body, position, archived), owning
    `quick_replies`. Imports `internal/platform`, `internal/db` and its `sqlc` output only.
*   API: `GET /quick-replies` (`ListQuickReplies`), `POST /quick-replies` (`CreateQuickReply`),
    `PATCH /quick-replies/{id}` (`UpdateQuickReply`). `QuickReply` = `{ id, title, body, position,
    archived }`; `QuickReplyInput` = `{ title, body }`; `QuickReplyPatch` = any of
    `{ title, body, position, archived }`.
*   Rules: `title` through `text.Line(raw, 60)`; `body` trimmed at both ends, line breaks kept, 1-2000
    characters. A new reply gets the current maximum position + 1. A `position` change sets that
    one row; the web computes the moves with `reorder()` (`web/src/lib/attributes.ts:25`) and
    sends one PATCH per moved row, as `AttributeSheet.svelte` does.
*   `Bootstrap` gains `quick_replies`: all replies with `archived`, by position, as it sends all
    services. The phone caches them in a new Dexie table, so copy and the archived list work
    offline.
*   Web `/enviar/respuestas`: a search field that filters title and body with `normalize()` from
    `web/src/lib/search.ts` (accent-insensitive; a blank query shows all; her order kept), the
    list, a "Copiar" button per row, `ReplySheet.svelte` to create, edit, move up or down, archive
    and restore, and "Archivadas ({count})" for archived ones.
*   Copy feedback ("Copiado") and the up/down buttons currently live inside `QuoteForm.svelte` and
    `OptionTree.svelte`; S5 moves them to `web/src/lib/copy.ts` and
    `web/src/lib/components/MoveButtons.svelte`, used in all three places.
*   Export: `respuestas.csv` (columns `titulo,texto,orden,archivada`; `orden` = position order
    starting at 1; `archivada` uses the existing `Yes` text, "sí", or empty) and `quick_replies` in
    `datos.json` with the `QuickReply` shape.

### Export format version

Every release that changes the `datos.json` shape bumps `jsonVersion`
(`internal/exporter/json.go:146`) by one: S2 (description) to 2, S3 (`handover_from` on rules) to
3, S5 (`quick_replies`) to 4.

### Tabs and places (G12, D17, D21)

```text
Enviar (bottom nav "Enviar")             Ajustes > Recordatorios
  [Recordatorios] [Respuestas]             [Reglas] [Plantillas]
  /enviar          /enviar/respuestas      /ajustes/recordatorios   /ajustes/plantillas/…
```

*   `ui/TabNav.svelte`, drawn first in `design/soft-glass.pen`. Props: `label: string` (the nav's
    accessible name) and
    `tabs: readonly { href: ResolvedPathname; label: MessageKey; match: 'exact' | 'prefix' }[]`.
    A tab is current when the path equals `href` (`exact`), or equals it or starts with `href` +
    "/" (`prefix`). One URL per tab, a `nav` with `aria-current`, pills on the phone and an
    underline on wide, as the Servicios nav does now.
*   Enviar: a new `web/src/routes/enviar/+layout.svelte` holds the h1 `send_title` "Enviar" and the
    tabs (both `exact`). The Recordatorios page's current h1 (`due_today_heading`) becomes its h2.
*   Ajustes: the folders `recordatorios` and `plantillas` move into the route group
    `web/src/routes/ajustes/(reglas)/`, whose `+layout.svelte` holds the h1 `settings_rules` and
    the tabs (`/ajustes/recordatorios` `exact`, `/ajustes/plantillas` `prefix`). URLs do not change.
    `settings_rules` keeps its value "Recordatorios": after D17 the owner chose that name for the
    Ajustes entry and its page over "Reglas de recordatorio".
    The h1s of the recordatorios page and the plantillas layout are removed. "Nueva regla" stays at
    the top of the Reglas page body; "Nueva plantilla" moves from the plantillas layout header to
    the top of the Plantillas list.
*   The group changes `page.route.id` (for example `/ajustes/(reglas)/plantillas`):
    `web/src/lib/titles.ts` re-keys those routes, and the UI probe's `routesOnDisk` strips
    `(group)` segments.
*   `SettingsList.svelte`: a section's `prefix: string` becomes `prefixes: string[]`, so the one
    entry "Recordatorios" is marked open on both paths.

### Stages and releases

| Stage | Ships | Schema |
| :--- | :--- | :--- |
| S0 | Design doc, archived brief, `.pen` components and frames | none |
| S1 | Wording pass | none |
| S2 | Service description | `services_description` |
| S3 | General rule, one row per client for general reminders, handover, due-item fields, row label, toasts, upgrade test | `reminder_general_rule` |
| S4 | Look-ahead pass, rule preview, client next message | none |
| S5 | `TabNav`, Ajustes merge, Enviar tabs, quick replies | `quick_replies` |

Each stage is one release. Migrations only move forward, and a rollback is the restore of the
snapshot taken before the upgrade, which drops everything she entered after the upgrade
(`release-1.md`, "Upgrades and the migration guard"). So the author upgrades outside her working
hours, checks the stage that day, and fixes later problems with a new release, not a restore.

### Old web (CUJ R9, checked in the code)

*   A general rule reaches the old web as a `come_back` rule with `service_id: null`. The old Zod
    schema allows it (`web/src/lib/api/schemas.ts:73`); the old rule sentence shows "…" for the
    missing name (`nameOf` in `web/src/lib/rules.ts`); the old warnings skip it. Nothing throws.
*   Saving a general rule from the old web: the old form requires a service
    (`web/src/lib/rules.ts:103-104` sets `rule_service_required`), shows "Elige un servicio" and
    sends nothing.
*   Archiving from the old web sends no `include_overdue`, which means "from today": no flood.
*   New due-item fields, `quick_replies` in bootstrap and new endpoints are ignored by the old web.

## Alternatives Considered

### A1: General rule as a new rule kind (`come_back_any`)

*   **Pros**: the `exhaustive` lint and TypeScript point at every switch that must handle it.
*   **Cons**: the phone's kind enum is strict (`web/src/lib/api/schemas.ts:68`), so the old web
    fails `/bootstrap` until she updates; the web would have to ship one release earlier.
*   **Reason for Rejection**: a NULL service on `come_back` loads in the old web today (D19). Each
    place that reads `service_id` gets a test with a NULL service instead.

### A2: A real follow-up, counted from the day she sent the first message

*   **Pros**: the second message always comes N days after the first, even when she sent it late.
*   **Cons**: the log key `(rule, appointment, anchor)` cannot hold a second message per rule and
    visit (the insert is `ON CONFLICT DO NOTHING`, `internal/db/queries/reminders.sql:100`), so a
    follow-up would never leave the list. It needs a `step` column in the key, its own item id, its
    own load window (send + N + 60 days) and the web shipped one release first.
*   **Reason for Rejection**: a second general rule gives a second message with none of that
    (D3). Cost accepted: it counts from the visit, so a late first message can be followed soon by
    the second.

### A3: Show every due general reminder

*   **Pros**: no hidden reminder.
*   **Cons**: with rules at 30 and 45 days, an unsent 30-day reminder is still listed on day 45, so
    one client shows twice for one visit, with two texts that say almost the same.
*   **Reason for Rejection**: one row per client for general reminders (D11).

### A4: One row per client for every come-back rule (specific ones too)

*   **Pros**: never two rows for one client.
*   **Cons**: a nails reminder would vanish because of a later eyebrows visit, though the nails
    return service was never booked.
*   **Reason for Rejection**: mixed history stays two rows, each naming its visit (D12).

### A5: A specific rule that cannot send still blocks the general rule

*   **Pros**: nothing changes for visits of a stopped rule.
*   **Cons**: archiving a template would leave its clients with no come-back message at all.
*   **Reason for Rejection**: "specific wins only while it can send" (D13), with the handover
    question against floods.

### A6: Handover without a question

*   **Pros**: no extra sheet in archive and restore flows; on service archive, no reminders
    question on a screen about prices.
*   **Cons**: "always from today" hides reminders she might want; "always include late" can fill
    the list at once after archiving a template used by a busy rule.
*   **Reason for Rejection**: ask, as the app already asks for a new or restored rule (D14, D23).
    Storing the answer on the general rule's `applies_from` was also rejected: it would cut off all
    that rule's visits, not only the handed-over ones.

### A7: The look-ahead inside `compute` with an "until" date

*   **Pros**: one code path.
*   **Cons**: every check (`allowed`, one row per client, the 60 days, each load window) would be
    redefined against "until", and a mistake would change the due list too.
*   **Reason for Rejection**: a separate pass (D22) leaves the due list as it is. Dropping the
    look-ahead was rejected too: D9 asks for "próxima" and the client line.

### A8: The phone works out the row label

*   **Pros**: no API change.
*   **Cons**: the 60 days would live in Go and TypeScript; the phone cache may not hold an old visit
    or its log rows, so the label would be empty or wrong offline.
*   **Reason for Rejection**: server fields on each due item (D20).

### A9: A date instead of a countdown

*   **Pros**: always right, even on a list cached yesterday.
*   **Cons**: she has to work out the days left herself.
*   **Reason for Rejection**: a countdown, hidden offline (D18).

### A10: No quick replies; use WhatsApp Business "/" replies

*   **Pros**: no new table, API, cache table or export file; one tap inside the chat.
*   **Cons**: her messages live outside the app, with no search by text, and she cannot use them
    from the app's reminders screen.
*   **Reason for Rejection**: quick replies in the app with search, her own order and offline copy
    (D6, D16). Cost accepted: two lists she keeps by hand.

### A11: Quick replies inside `message_templates`

*   **Pros**: no new table.
*   **Cons**: templates are parsed for variables and checked on save, and rules point at them; a
    plain reply would need flags to skip both, and archiving rules would have to skip replies.
*   **Reason for Rejection**: a separate table keeps both modules free of those flags.

### A12: Service description inside the price line, or under it

*   **Pros**: the detail sits next to its price.
*   **Cons**: long lines, or the name twice; every price-line format and its tests would change.
*   **Reason for Rejection**: a detail block at the end (D5).

### A13: Explanatory help text for the rules

*   **Pros**: one place explains everything.
*   **Cons**: she reads it once; the rule's effect on a given row stays invisible.
*   **Reason for Rejection**: labels on rows, consequences in toasts and previews in the editor
    (D9).

## Risks & Mitigations

| Risk | Mitigation |
| :--- | :--- |
| A flood of late general reminders after an archive or restore | The handover question; "from today" when unanswered or from the old web; only rules whose state flips are touched (Handover) |
| A second come-back message for one visit after a restore | A specific reminder is hidden when a general one was sent for the visit (Due-list rules) |
| General reminders hidden by a specific rule that makes none itself | Accepted (G2) |
| The S3 migration cannot find the old CHECK | The `DO` block fails the migration before any change; the upgrade stops and `serve` does not start (`release-1.md`, "Upgrades and the migration guard") |
| A restore wipes data entered after an upgrade | Upgrade outside working hours, fix forward ("Stages and releases") |
| The old web edits or archives | Checked in "Old web"; skew check in the Verification Plan |
| The look-ahead and the due list disagree | Shared helpers; an agreement test (Detailed Implementation Plan, item 42) |
| An archive and its handover half-applied | One `inWriteTx` per request; a failing commit leaves neither (Detailed Implementation Plan, item 29) |
| The preview runs `compute` on each edit | The web sends at most one request per pause in typing (the template preview's delay) and drops stale answers; one business's rows |
| Stored text (descriptions, replies) shown or exported | Length CHECKs, Svelte escaping, `neutralize` on export; threat-model rows (Detailed Implementation Plan, items 13, 42, 58) |
| A late first message followed soon by the second (A2) and two lists of replies (A10) | Accepted by the owner (D3, D16) |

## Non-Functional Requirements

*   **Version skew**: every API change is additive ("Old web"). `make check-generated` keeps spec,
    Go and web types in step.
*   **Security and privacy**: quick replies and descriptions are her own text, shown only to her;
    see Risks & Mitigations. New endpoints sit behind the session and Origin check like the rest.
*   **Performance**: the look-ahead runs over the same rows the due list loads, plus one client's
    upcoming appointments for the client page. No new index is needed;
    `reminder_log_appointment_id_idx` serves the count.
*   **Rollout and rollback**: one release per stage; see "Stages and releases".

## Detailed Implementation Plan

Rules for every stage: contract first (`api/openapi.yaml`, then `make generate`); never edit
generated code; every UI text in `es.json`; UI changes start in `design/soft-glass.pen`; a change
that makes a doc false fixes the doc in the same change; the gate in `CLAUDE.md` runs before
"done".

Before S0: the Pencil app must be running so its MCP server connects. Without it, S0 and every UI
stage stop (`.claude/rules/ui.md`, UI pipeline step 1).

### Stage S0: design and docs

#### 1. `docs/designs/reminders-and-replies.md` (Create)

*   This design.

#### 2. `docs/designs/history/reminders-and-replies-brief.md` (Create)

*   The brief, moved from the repo root.

#### 3. `design/soft-glass.pen` (Modify)

*   Components: `TabNav` (variants `pill`, `underline`), `LabelChip` (variants `normal`,
    `warning`). The Respuestas row (a `ListRow` with a "Copiar" `Button/Glass`) and `MoveButtons`
    (two `IconButton/Glass`) are drawn in the frames, not as reusable components: they are feature
    components, and guard `design-parity` maps every reusable `.pen` component to a primitive in
    `ui/`. Frames: Enviar (both tabs), the reply sheet with `MoveButtons` (393 only), Reglas de
    recordatorio, the archive confirmation and then the handover question, at 393 and 1440 px.
*   `web/tests/guards/designParity.guard.test.ts`: `DESIGN_ONLY` gains the four new design
    components until their primitives land (`LabelChip` in S3, `TabNav` in S5); each stage removes
    its entries.

#### 4. `CLAUDE.md` (Modify)

*   "Architecture source" names this design and its stages S1-S5.

### Stage S1: wording pass

#### 5. `web/src/lib/i18n/es.json` (Modify)

Set these values (keys unchanged). 45 texts, written to `.claude/rules/spanish-texts.md`:

| Key | Value |
| :--- | :--- |
| `attribute_scope_question` | Tipo de atributo |
| `attribute_scope_client` | Atributo fijo |
| `attribute_scope_appointment` | Atributo por cita |
| `attribute_scope_client_short` | Fijo |
| `attribute_scope_appointment_short` | Por cita |
| `attribute_fixed_note` | No se puede cambiar después de crearlo |
| `attribute_required` | Obligatorio |
| `option_add_inside` | Agregar subopciones |
| `offline_banner` | Sin conexión · datos {when} |
| `offline_banner_no_data` | Sin conexión |
| `offline_cannot_save` | Sin conexión: no se puede guardar |
| `error_offline` | Sin conexión |
| `agenda_needs_connection` | Sin conexión: no se ven otros días |
| `photos_offline` | Sin conexión: no se ven las fotos |
| `clients_archived_offline` | Sin conexión: no se ven las archivadas |
| `client_history_more_offline` | Sin conexión: no se ven todas las citas |
| `client_reminders_offline` | Sin conexión: no se ven los recordatorios |
| `export_offline` | Sin conexión: no se puede descargar |
| `error_response_invalid` | Error del servidor. Intenta de nuevo |
| `due_confirm_failed` | No se guardó. Con conexión, toca «Sí, lo envié» |
| `due_skip_wait` | Espera unos segundos |
| `appointment_end_auto` | Calcular hora de fin |
| `appointment_service_remove` | Quitar 1 {name} |
| `clients_pick_body` | Toca una clienta para ver sus citas y fotos |
| `client_archive_body` | Se oculta de la lista. Sus citas y fotos se guardan |
| `photo_archive_body` | Se oculta de la cita y de la galería |
| `templates_pick_body` | O crea una nueva |
| `template_chip_hint` | Toca para elegir el texto si falta el dato |
| `template_fallback_hint` | Vacío: no se escribe nada |
| `template_fallback_quote` | Quita las comillas (") |
| `template_archive_body` | Sus reglas ya no enviarán hasta que elijas otra plantilla |
| `rules_templates_body` | Primero crea una plantilla |
| `rule_return` | Servicio a ofrecer |
| `rule_archive_body` | Ya no creará recordatorios. Los enviados quedan en el historial |
| `service_archive_body` | Ya no aparecerá al agendar. Las citas anteriores lo conservan |
| `attribute_archive_body` | Ya no aparecerá en los formularios. Los datos guardados se conservan |
| `business_push_hint` | Aviso diario de recordatorios (00:00 a 19:00) |
| `business_icon_hint` | Para verlo, quita la app y agrégala otra vez |
| `setup_icon_hint` | Imagen cuadrada para tu pantalla de inicio. Opcional |
| `quote_empty` | Toca los servicios para cotizar |
| `quote_copy_failed` | No se pudo copiar. Mantén presionado el texto |
| `quote_replace_prompt` | Ya tienes una cita sin guardar. ¿Reemplazarla? |
| `quote_convert_failed` | No se pudo crear la cita. Intenta de nuevo |
| `status_timer` | Tarea diaria |
| `status_timer_none` | Aún no ha corrido |

And the handover and rule texts (D17):

| Key | Value |
| :--- | :--- |
| `overdue_title` | ¿Incluir atrasados? |
| `overdue_body` | Puede traer recordatorios de visitas de los últimos 60 días |
| `overdue_from_today` | Solo de hoy en adelante |
| `overdue_include` | Incluir atrasados |
| `rules_title` | Reglas |
| `rules_new`, `rule_new_title` | Nueva regla |
| `rule_edit_title` | Editar regla |
| `rule_save` | Guardar regla |
| `rule_saved` | Regla guardada |
| `rule_created` | Regla creada |
| `rule_archive` | Archivar regla |
| `rule_archived` | Regla archivada |
| `rule_restore` | Restaurar regla |
| `rule_restored` | Regla restaurada |
| `rules_loading` | Cargando reglas… |
| `due_no_rules_title` | Aún no tienes reglas |
| `due_no_rules_action` | Crear una regla |
| `due_no_rules_body` | Crea una regla para saber a quién escribirle cada día. |
| `templates_empty_body` | Escribe tus mensajes una vez y úsalos en tus reglas. |

#### 6. `internal/platform/problem/messages.go` and the tests that compare these texts (Modify)

*   `MsgRuleKindInvalid` "Elige un tipo de regla", `MsgServiceNotAllowed` "Esta regla no lleva
    servicio", `MsgRuleUnknown` "Esa regla no existe". `MsgAnchorInvalid` keeps "recordatorio":
    there the word means the reminder, not the rule.

#### 7. `design/soft-glass.pen` (Modify)

*   The same texts where a frame shows them. The author checks this by eye: the design guards
    compare tokens and components, not text.

### Stage S2: service description

#### 8. `internal/db/migrations/<next>_services_description.sql` (Create)

*   The column and its CHECK ("Data model").

#### 9. `internal/db/queries/services.sql`, `internal/db/queries/export.sql` (Modify)

*   Select, insert and update `description`; `ExportServices` selects it.

#### 10. `internal/platform/text/text.go`, `text_test.go` (Modify)

*   `Line(raw string, maxRunes int) (value *string, msg string)`: trim; empty → nil; longer than
    `maxRunes` → `MsgTextTooLong`; a line break → `MsgTextOneLine`. Tests for each case.

#### 11. `internal/platform/problem/messages.go` (Modify)

*   `MsgTextTooLong` "Es muy largo", `MsgTextOneLine` "Escríbelo en una sola línea".

#### 12. `internal/services/services.go`, `services_db_test.go`, `internal/db/schema_test.go` (Modify)

*   `Service.Description *string`; `ServiceInput.Description`, `ServicePatch.Description`
    (nullable), validated with `text.Line(raw, 200)`. Tests: create, update, clear with `null`, 200
    characters accepted and 201 refused, a line break refused. `TestSchemaRejectsInvalidRows` gains
    a 201-character description.

#### 13. `api/openapi.yaml`, `internal/httpapi/handlers_services.go`, `handlers_services_test.go`, `docs/threat-model.md` (Modify)

*   Spec: `Service.description` (always present, may be null); optional and nullable in
    `ServiceInput` and `ServicePatch`; the `ServicePatch` and `UpdateService` texts list
    `description` as nullable. Handlers: `CreateService` passes `body.Description`;
    `UpdateService` calls `decodePatch(w, r, &body, "price_cents", "description")`; `toGenService`
    maps it. Tests: 422 with each message. Threat-model row: stored description text (length
    CHECK, escaped output, `neutralize` on export), naming `handlers_services_test.go` and
    `export_formula_db_test.go`.

#### 14. `internal/locale/es.go`, `locale.go`, `locale_test.go` (Modify)

*   `ServiceCols` gains `descripcion` after `nombre`.

#### 15. `internal/exporter/export.go`, `json.go`, `export_db_test.go`, `export_json_db_test.go`, `export_formula_db_test.go`, `internal/httpapi/export_schema_test.go` (Modify)

*   `servicios.csv` column; `datos.json` `description`; `jsonVersion` 2; a formula-looking
    description is neutralized; `TestExportJSONMatchesSchemas` covers it.

#### 16. `web/src/lib/api/schemas.ts`, `web/src/lib/cache/db.ts` (Modify), `db.upgrade-v2.test.ts` (Create)

*   `ServiceSchema.description: z.nullable(z.string())`. `SCHEMA_VERSION` 2 → 3; the upgrade drops
    the cached server data, services included (refilled by the next bootstrap), so a row without
    the field never reaches `clearCache()`, and drafts and pending confirmations survive. The test
    is its own file: the cache module opens its database on import, so each starting version
    needs a fresh module.

#### 17. `web/src/lib/components/ServiceSheet.svelte` (Modify)

*   "Descripción (opcional)" field under the name, hint "Se agrega a tus cotizaciones". An empty
    field sends `null` on edit and nothing on create.

#### 18. `web/src/lib/quote.ts`, `quote.test.ts` (Modify)

*   The detail block ("Service description").

#### 19. `web/src/lib/i18n/es.json` (Modify)

*   `service_description` "Descripción (opcional)", `service_description_hint` "Se agrega a tus
    cotizaciones", `quote_detail_line` "{name}: {description}".

#### 20. `web/tests/e2e/quote.spec.ts` (Modify)

*   CUJ R8.

#### 21. `internal/status/report_db_test.go` (Modify)

*   `TestReportMigrationVersionIs2` becomes `TestReportMigrationVersionIsLatest`: it compares the
    reported version with the highest migration number in the embedded migrations, so later
    stages and the password design need no change here.

#### 22. `docs/designs/quick-quotes.md` (Modify)

*   "Text format" gains the detail block.

### Stage S3: general rule and visible rules

#### 23. `internal/db/migrations/<next>_reminder_general_rule.sql` (Create)

*   The SQL in "Data model".

#### 24. `internal/db/queries/reminders.sql` (Modify)

*   `ListActiveRulesForDue` → `ListRulesForDue`:
    `WHERE r.archived_at IS NULL OR (r.kind = 'come_back' AND r.service_id IS NOT NULL)`, also
    returning `r.archived_at` and `r.handover_from`.
*   `ListLaterAppointments`: `starts_at >= @after`.
*   `ListLogKeys` as in "New fields on each due item".
*   `ListRules`, `GetRule` and `UpdateRule` read and write `handover_from` (a rule archive or
    restore sets it). New: `SetHandover` (the rules that can send under that template or service)
    and `RestartRules` (the rules that will send again), each taking a nullable `template_id` and
    `service_id`, exactly one set.

#### 25. `internal/reminders/rules.go`, `rules_db_test.go` (Modify)

*   `CreateRule` (`rules.go:56-60`): a come-back rule may have no service; with no service,
    `return_service_id` must be empty, else 422 `MsgGeneralNoReturn` "Una regla para todos los
    servicios no lleva servicio para agendar" (new in `messages.go`).
*   `UpdateRule`: on a rule with no service, a `return_service_id` gives the same 422. On archive
    and restore, `handover_from` per "Handover".
*   `SetHandover(ctx, q, scope HandoverScope, includeOverdue bool)` and
    `RestartRules(ctx, q, scope HandoverScope, includeOverdue bool)` with
    `HandoverScope{TemplateID, ServiceID uuid.NullUUID}`; both use `m.clock`.
*   `MsgServiceRequired` loses its last server use: delete it.
*   Tests: the case "come-back rule without a service gives `MsgServiceRequired`"
    (`rules_db_test.go:184-187`) becomes "a general rule is created"; a general rule with a return
    service is refused on create and update; archive "from today" writes now; archive "include
    late" writes NULL; archiving a rule and then its template keeps the rule's answer; an archived
    rule restored while its template is archived keeps `handover_from`; restore clears it.

#### 26. `internal/reminders/due.go` (Modify)

*   Split `allowed` into `dueReached` and `allowedFrom`.
*   Add the "rule archived" check next to `due.go:345`; skip archived rows in `maxComeBack`.
*   Add `generalItem` per the pseudocode; hide a specific reminder when `GeneralSent[v]`.
*   Load from the earliest candidate `starts_at` in `laterAppointments`.
*   Fill `SentByVisit`, `GeneralSent` and the new `DueItem` fields.

#### 27. `internal/reminders/due_test.go`, `due_comeback_test.go` (Modify), `due_general_test.go` (Create)

*   Pure tests of `compute`: R1-R3 (with `quedan 14` on 31 Oct); two visits with the same start
    (higher id wins); a later cancelled appointment brings the general reminder back; a specific
    rule that can send hides the general reminder; an archived rule, template, service or return
    service lets it show; the latest due wins, and a logged latest hides the earlier ones; cutoff
    NULL, set, and the latest of two; the specific reminder hidden after a sent general one;
    `shown_until` for specific, general with and without a next rule, and "before";
    `message_number` (sent counts, skipped does not, two specific rules on one visit);
    `next_due_date` with the next rule in the future, none, and two ahead.

#### 28. `internal/reminders/due_db_test.go`, `cuj_db_test.go`, `internal/db/schema_test.go` (Modify), `internal/db/upgrade_db_test.go` (Create)

*   `ListRulesForDue`, `ListLogKeys` and the handover queries against Postgres; CUJ R1-R4 end to
    end. `TestSchemaRejectsInvalidRows` keeps its "before" rule with a service and gains a general
    rule with a return service and `handover_from` on a general rule.
*   `TestMigrateFromRelease1Data`: migrate a fresh database to version 2 with goose `UpTo` over the
    embedded migrations, insert release-1 rows (a "before" rule, a specific come-back rule, an
    archived rule, their services and templates, a visit and a log row), migrate to the latest
    version, and check the rows are unchanged, `handover_from` is NULL, a general rule can be
    inserted, and a "before" rule with a service is refused.

#### 29. `api/openapi.yaml`, `internal/httpapi/handlers_reminders.go`, `handlers_templates.go`, `handlers_services.go` and their tests (Modify)

*   Spec: the new `DueItem` fields; `Rule.handover_from` (nullable; `TestExportJSONMatchesSchemas`
    keeps the export's rule equal to it); `RuleInput.service_id` is already nullable, its text now
    allows NULL for a come-back rule; `TemplatePatch.include_overdue` and
    `ServicePatch.include_overdue` (optional boolean, described as on `RulePatch`, read only on
    archive and restore).
*   Handlers: map the fields; template and service archive call `reminders.SetHandover`, and
    restore calls `reminders.RestartRules`, before the module update inside the existing
    `s.inWriteTx` callback.
*   Tests: with the failing-commit fake (`handlers_auth_test.go:442-462`), neither the archive nor
    `handover_from` is visible after the request.

#### 30. `web/src/lib/api/schemas.ts`, `web/src/lib/cache/db.ts` (Modify), `db.upgrade-v3.test.ts`, `upgradeSeed.ts` (Create)

*   `DueItemSchema` new fields, `RuleSchema.handover_from`. `SCHEMA_VERSION` 3 → 4; the upgrade
    drops the cached server data, the due list included. `upgradeSeed.ts` writes the older
    database for each `db.upgrade-v*.test.ts`.

#### 31. `web/src/lib/rules.ts`, `rules.test.ts` (Modify)

*   `ruleRequest` and the form values accept "Todos los servicios" (service NULL, no return
    service). `ruleSentence` for a general rule: `rule_sentence_general` "Enviar {template}
    {offset} de la última visita" (`{offset}` from `offsetText`). `sortRules`: general rules after
    specific ones, by days. `handoverAsks` per "Handover", with a test per change type.

#### 32. `web/src/lib/components/RuleSheet.svelte`, `TemplateForm.svelte`, `TemplatePane.svelte`, `ServiceSheet.svelte` (Modify)

*   Service picker first option `rule_service_all` "Todos los servicios"; the return-service field
    is hidden for it. Each archive and restore flow shows the handover question when
    `handoverAsks` says so and sends `include_overdue`.

#### 33. `web/src/lib/components/ui/LabelChip.svelte` (Create), `web/src/lib/time.ts`, `time.test.ts` (Modify)

*   The primitive per "Row label"; `shortDay`.

#### 34. `web/src/lib/due.ts`, `due.test.ts`, `web/src/lib/components/DueItemRow.svelte`, `web/src/lib/components/ui/DueCard.svelte` (Modify)

*   `dueLabel` per "Row label", tested for each segment and each days-left text. Both row
    components show `LabelChip` next to their button.

#### 35. `web/src/lib/pending.ts`, `web/src/lib/components/ConfirmSentSheet.svelte`, `web/src/routes/enviar/+page.svelte` (Modify)

*   The pending confirmation stores `next_due_date` (copied from the due item when she taps
    "Enviar"). After "Sí, lo envié", `toaster.say` shows `due_sent_next` "Enviado a {name} ·
    siguiente el {date}" (with `shortDay`) when it is set, else `due_sent` "Enviado a {name}". The
    skip toast `due_skipped` becomes "Omitido: {name} · no se mostrará de nuevo".
*   A skip toast and a sent toast never compete: "Enviar" already sends a waiting skip before it
    opens WhatsApp (`enviar/+page.svelte:85`).

#### 36. `web/src/lib/i18n/es.json` (Modify)

*   `rule_service_all` "Todos los servicios", `rule_sentence_general`, `due_label_visit` "Cita
    {date}", `due_label_visit_time` "Cita {date} {time}", `due_label_general` "General {n} días",
    `due_label_number` "{n}º mensaje", `due_label_left_other` "quedan {n}", `due_label_left_one`
    "queda 1 día", `due_label_last_day` "Último día", `due_label_more` "+{n}", `due_sent`,
    `due_sent_next`.

#### 37. `web/tests/e2e/reminders.spec.ts`, `offline.spec.ts`, `ui-probe.spec.ts` (Modify)

*   R1-R5 and R9 end to end; the handover question on each of the five changes; offline: no
    countdown and a plain-text label (R6). The probe checks the label link is not inside a button.

#### 38. `internal/exporter/json.go`, `export_json_db_test.go`, `internal/db/queries/export.sql`, `internal/httpapi/export_schema_test.go` (Modify)

*   Rules in `datos.json` gain `handover_from`; `jsonVersion` 3.

#### 39. `docs/designs/release-1.md` (Modify)

*   "Reminder rules" and "Due-list computation": the general rule, one row per client for general
    reminders, D13, handover. Note that any later appointment removes a general reminder, while a
    specific reminder keeps release-1's rule (Alternative E5 as chosen there).

### Stage S4: look-ahead, preview and next message

#### 40. `internal/reminders/upcoming.go` (Create)

*   `upcoming(d, now, horizon)` per "Look-ahead pass".

#### 41. `internal/reminders/preview.go`, `reminders.sql` (Modify), `next.go` (Create)

*   `RulePreview(ctx, q, now, draft, ruleID) (RulePreviewResult, error)` (the existing `Preview`
    type is the template preview; this one is separate). `NextForClient(ctx, q, now, clientID)`.
    `loadDue` takes the draft's offset into account before sizing windows. New query
    `ListClientUpcomingAppointments` in `internal/db/queries/reminders.sql`.

#### 42. `internal/reminders/upcoming_test.go`, `next_db_test.go` (Create), `preview_db_test.go`, `docs/threat-model.md` (Modify)

*   Agreement test: on a fixed data set with no log changes, for each day D from tomorrow to 90
    days ahead, the reminders `upcoming(now)` gives with due D equal the reminders
    `compute(d, at D)` gives with due D (same rule, visit and client). This covers can send, one
    row per client and the cutoff.
*   Preview: counts the clients due today; a draft of an existing rule is computed in place of the
    saved one and nothing is written; a draft offset larger than any saved rule still finds its
    visits; a new rule; no reminder ahead gives `next: null`; a "before" `rule_id` gives 422.
*   Next: today, late, future, none (204), unknown client (404), a "before" reminder from a booking.
*   Threat-model row for the two reads (session and Origin, no data beyond what the due list
    returns), naming these tests.

#### 43. `api/openapi.yaml` (Modify)

*   `PreviewRule` (`POST /reminder-rules/preview`, `RulePreviewInput` → `RulePreviewResult`) and
    `GetClientNextReminder` (`GET /clients/{id}/next-reminder`, `NextReminder` or 204).

#### 44. `internal/httpapi/handlers_reminders.go`, `handlers_clients.go`, `handlers_bootstrap_test.go` (Modify)

*   Both handlers use `s.snapshot`. `TestSnapshotReadersUseReadOnlyRepeatableRead` becomes a table
    of method, path, body and expected status, with two new rows: POST the preview with a
    come-back draft (200), GET next-reminder for a seeded client with no reminders (204). The
    Semgrep rule `non-get-handler-in-withtx` reports nothing for the POST handler, because it
    never uses `s.pool`.

#### 45. `web/src/lib/api/client.ts`, `client.test.ts`, `schemas.ts` (Modify)

*   `previewRule` and `nextReminder` (204 → null), with their schemas.

#### 46. `web/src/lib/components/RuleSheet.svelte` (Modify)

*   For come-back rules only: `rule_preview` "Hoy: {count} clientas · próxima: {name}, {date}"
    (`rule_preview_one` for one client; `rule_preview_none` "Hoy: {count} clientas" when `next` is
    null). Sent only while the form is valid, after the same pause as the template preview; a newer
    answer replaces an older one; on an error or offline the line is hidden.

#### 47. `web/src/routes/clientas/[id]/+page.svelte` (Modify)

*   In the reminders section: `client_next_reminder` "Próximo mensaje: {date} ({services})",
    `client_next_today` "Mensaje para hoy ({services})", `client_next_late` "Mensaje atrasado desde
    {date} ({services})", `client_next_none` "Sin mensajes pendientes". Services joined as in the
    row label. Hidden offline, while loading and on an error.

#### 48. `web/src/lib/i18n/es.json`, `web/tests/e2e/ui-probe.spec.ts`, `web/tests/e2e/reminders.spec.ts` (Modify)

*   The S4 texts. Probe data states: next-reminder answers 204 (shows `client_next_none`), 500
    (line hidden, no alert), never answers (line hidden); the preview answers `today_count: 0`
    and `next: null`, 500 (line hidden), never answers (line hidden). R7 and R10 end to end.

#### 49. `CLAUDE.md` (Modify)

*   Load-bearing constraint 9's list of snapshot readers gains the preview and next-reminder reads.

### Stage S5: tabs and quick replies

#### 50. `web/src/lib/components/ui/TabNav.svelte`, `TabNav.test.ts` (Create), `web/src/routes/servicios/+layout.svelte` (Modify)

*   Create the primitive per "Tabs and places" and test `exact` and `prefix` matching. Make the
    Servicios layout use it.

#### 51. `web/src/routes/enviar/+layout.svelte` (Create), `web/src/routes/enviar/+page.svelte` (Modify), `web/src/routes/enviar/respuestas/+page.svelte` (Create)

*   Put the h1 and the tabs in the new layout. Turn the Recordatorios page's h1 into an h2.
    Build the Respuestas page per "Quick replies".

#### 52. `web/src/routes/ajustes/(reglas)/+layout.svelte` (Create); `web/src/routes/ajustes/recordatorios/` and `web/src/routes/ajustes/plantillas/` (Move into `(reglas)/`, Modify)

*   Move both folders into the group, add the group layout, and remove the two old h1s, per "Tabs
    and places".

#### 53. `web/src/lib/components/SettingsList.svelte`, `web/src/lib/titles.ts`, `titles.test.ts` (Modify)

*   Change `prefix` to `prefixes: string[]` and merge the two entries into one. Re-key the titles
    for the group's route ids.

#### 54. `web/src/lib/components/ReplySheet.svelte`, `web/src/lib/copy.ts`, `copy.test.ts`, `web/src/lib/components/MoveButtons.svelte` (Create); `QuoteForm.svelte`, `OptionTree.svelte` (Modify)

*   Build the sheet. Move the copy feedback and the up/down buttons into shared files; `QuoteForm`
    and `OptionTree` use them
    with no change in behaviour (their existing tests stay green unchanged).

#### 55. `web/src/lib/replies.ts`, `replies.test.ts` (Create)

*   `filterReplies(list, query)` with `normalize()`: accents and case ignored, title and body
    searched, her order kept, a blank query shows all.

#### 56. `internal/db/migrations/<next>_quick_replies.sql`, `internal/db/queries/replies.sql` (Create), `internal/db/queries/export.sql` (Modify)

*   Create the table. Add the queries `ListReplies`, `MaxReplyPosition`, `InsertReply`,
    `UpdateReply` and `ExportQuickReplies`.

#### 57. `internal/replies/replies.go`, `replies_db_test.go` (Create), `internal/platform/problem/messages.go` (Modify)

*   Write the module per "Quick replies". Messages: title through `text.Line` (`MsgTextTooLong`,
    `MsgTextOneLine`), plus `MsgReplyTitleRequired` "Escribe un nombre" and `MsgReplyBodyLength`
    "Escribe un mensaje de 1 a 2000 caracteres". Tests: create appends at the end, trim, a title
    line break refused, length limits, a position change sets one row, archive and restore.

#### 58. `api/openapi.yaml`, `handlers_bootstrap.go`, `server.go`, `docs/threat-model.md` (Modify), `internal/httpapi/handlers_replies.go`, `handlers_replies_test.go` (Create)

*   Add the three operations and schemas, `Bootstrap.quick_replies`, and the routes. Add a
    threat-model row for stored reply text, naming `handlers_replies_test.go`.

#### 59. `cmd/open-customer/serve.go`, `cmd/open-customer/server_test.go`, `.golangci.yml` (Modify)

*   Build the `replies` module and pass it to the server. depguard: `internal/replies` joins the
    domain list of `auth-imports-no-domain-module`, and only `httpapi` and `cmd` import it.

#### 60. `web/src/lib/api/client.ts`, `client.test.ts`, `schemas.ts`, `web/src/lib/cache/db.ts`, `db.upgrade.test.ts`, `store.svelte.ts` (Modify)

*   Add client methods for the three operations, `QuickReplySchema`, and a `quick_replies` Dexie table
    added in `SCHEMA_VERSION` 5 and filled from bootstrap.

#### 61. `internal/locale/es.go`, `locale.go`, `locale_test.go`, `internal/exporter/export.go`, `json.go`, `export_db_test.go`, `export_json_db_test.go`, `internal/httpapi/export_schema_test.go` (Modify)

*   Add `RepliesFile` "respuestas.csv" and `ReplyCols`; write the file as the last CSV in the ZIP
    (`TestExportHasFourFilesInOrder` becomes `TestExportHasFiveFilesInOrder`); `datos.json`
    `quick_replies`; `jsonVersion` 4.

#### 62. `web/src/lib/i18n/es.json` (Modify)

*   Add `send_title` "Enviar", `send_tabs_label` "Secciones de Enviar", `tab_reminders`
    "Recordatorios", `tab_replies` "Respuestas", `rules_tabs_label` "Secciones de Reglas",
    `tab_rules` "Reglas", `tab_templates` "Plantillas", `replies_search` "Buscar respuesta",
    `replies_empty_title` "Aún no tienes respuestas", `replies_empty_body` "Guarda los mensajes que
    escribes seguido", `replies_no_results` "Sin resultados", `replies_new` "Nueva respuesta",
    `replies_archived_toggle` "Archivadas ({count})", `reply_title` "Nombre", `reply_body`
    "Mensaje", `reply_save` "Guardar respuesta", `reply_saved` "Respuesta guardada",
    `reply_created` "Respuesta creada", `reply_archive` "Archivar respuesta", `reply_archived`
    "Respuesta archivada", `reply_restore` "Restaurar respuesta", `reply_restored` "Respuesta
    restaurada". Copy and move reuse `quote_copy`, `quote_copied`, `option_up` and `option_down`.
    `settings_templates` is removed (no entry uses it).

#### 63. `web/tests/e2e/ui-probe.spec.ts`, `reminders.spec.ts`, `navigation.spec.ts`, `export.spec.ts`, `offline.spec.ts`, `setup.spec.ts` (Modify)

*   Add probe entries for `/enviar/respuestas` and the tabbed pages; make `routesOnDisk` strip `(group)`
    segments. R11 (create, edit, move, archive, restore, search), R12, copy offline (R6), export
    columns. `setup.spec.ts` reaches Plantillas through "Recordatorios" and its tab.

#### 64. `Makefile`, `CLAUDE.md` (Modify)

*   Add `internal/replies` to `COVER_PKGS` (90% floor) and to the `CLAUDE.md` text that lists them.

#### 65. `docs/designs/release-1.md` (Modify)

*   Add `replies/` and its imports to "Code layout and module rules", and the new files to the export
    format.

### Verification Plan

*   **Every stage, S1 on**: the gate in `CLAUDE.md` (`make check-pins lint test vuln build
    check-generated test-db cover test-deploy secrets test-sec config-scan sast deps-scan`), plus
    `make e2e` (or `make e2e-docker`) and `make image image-scan dast`, because every stage changes
    what the app serves (S1 changes the embedded texts).
*   **UI**: run the `web-design-guidelines` skill on the UI diff and take screenshots at 393 × 852
    and 1440 × 900 of each changed screen (`.claude/rules/ui.md`, UI pipeline step 6).
*   **Version skew (CUJ R9)**: before S3 ships, run the S2 web build against the S3 server with a
    general rule in the data. Check: bootstrap loads; the rule list shows "…"; saving the general
    rule shows "Elige un servicio" and sends nothing; archiving a specific rule writes
    `handover_from` = now; Enviar lists the general reminder.
*   **Upgrade**: `TestMigrateFromRelease1Data` (Detailed Implementation Plan, item 28) in
    `make test-db`.
