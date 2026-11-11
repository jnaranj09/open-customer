# Reminders, quick replies and service descriptions

* **Author:** jnaranj09
* **Date:** 2026-11-09
* **Status:** brainstorm round 1 done (six-lens review); all owner questions decided (D1-D21; D22-D23 were decided in the design review and live in `../reminders-and-replies.md`)
* **Archive as:** `docs/designs/history/<doc-name>-brief.md` (not `history/concepts_brief.md`: that
  name is release-1's brief and would be overwritten)

## Overview

The owner runs a small service business and keeps clients, appointments and WhatsApp reminders in
the app (iPhone PWA, Spanish UI). Today she still uses a paper notebook. **Success: she stops using
the notebook and uses only the app.**

This brief covers six changes, separate from the password design (`docs/designs/password-policy.md`):

1. **A general "after any service" reminder rule.** One rule covers every client; she adds a rule
   for a service only where that service needs a different gap.
2. **A second message by a second general rule** (for example 30 and 45 days after the last
   visit), each with its own template. No separate follow-up feature (D3).
3. **A sent counter** on each reminder: how many messages she sent this client since the last
   visit.
4. **A service description**, shown at the end of the Cotizar text.
5. **Quick replies**: saved plain-text messages she copies with one tap, in their own tab of
   "Para enviar".
6. **Visible rules**: short lines that say what the app does with each reminder (until when it is
   shown, what removes it), so nothing happens without an explanation.

Names used here: a **rule** is what she sets up in Ajustes (es.json already calls it
"Recordatorio"). A **message** is one item in "Para enviar" for one client.

## Analysis

Six review lenses read the concept against the code. The central facts were checked again by the
main session (`00001_init.sql:151` and `:164`, `due.go:393`, `reminders.sql:100`).

### How reminders work today (checked)

* A come-back item belongs to **one appointment**, not to the client. Each done appointment that
  holds the rule's service gets one item.
* It is shown from its due day until she marks it sent or skipped, or for 60 days
  (`lookBackDays = 60`, `internal/reminders/due.go:26`). A handled item never comes back: the log
  key is `(rule_id, appointment_id, anchor_at)` (`00001_init.sql:164`).
* Before it is due, it is skipped if a later appointment holds the rule's **return service**. A later
  visit without that service does not skip it.
* A come-back rule must have a service: the database check `(kind = 'before_appointment') =
  (service_id IS NULL)` (`00001_init.sql:151`), the server (`rules.go:56-60`) and the web form
  (`rules.ts:103-105`) all require it. Today a rule without a service would make **no items and no
  error** (`due.go:393`).

### Blockers

Blockers 1 and 2 of round 1 (the follow-up's log key and its load window) are gone: the owner
dropped the follow-up (D3). Each general rule is a normal rule, so the log key
`(rule, appointment, anchor)` stays unique per message.

3. **An older app on her phone can break.** A new app version waits until she taps "update", so an
   old app can run against a new server for days. A new rule `kind` value would make the old app
   refuse `/bootstrap` (strict enums in `schemas.ts`). A come-back rule with no service may also
   fail the old app's schema (not checked). Defense: keep the two rule kinds, and ship the web
   part that accepts a NULL service one release before the server allows it.
4. **The general rule needs real code.** It is not enough to allow a NULL service: `compute` must get
   its own branch for it.
5. **One planned text would be false.** "Hasta las 15:00" reads as today for a message sent the day
   before the visit; it must carry the day ("Hasta mañana a las 15:00").
20. **Two general rules can show the same client twice** (new, from D3). With rules at 30 and 45
    days, an unsent 30-day message is still shown on day 45 (it lasts 60 days), so the 45-day
    message joins it: two rows for one client. Decided (D11): only the latest due general message
    shows; the earlier unsent one leaves the list.

### Major

6. **"Specific wins" needs an exact meaning.** If "active rule" means only "not archived", a specific
   rule that is paused (its template is archived) still blocks the general rule, and the client gets
   nothing. Also: archiving a specific rule later brings old general-rule messages back as late, and
   archiving asks no question.
7. **"Last appointment" needs an exact meaning.** Today "later" means "starts after this one ends".
   Two visits back to back on one day would both count as the last, so one client gets two general
   messages. Proposed: "later" means a later start time (id breaks a tie), not cancelled or no-show.
8. **A mixed history still gives two messages.** Nails on day 1 (a 21-day nails rule) and eyebrows
   on day 10 (no eyebrows rule, a 30-day general rule): the nails message comes on day 22 and the
   general one on day 40. "Specific wins" is decided per appointment, so both appear. Decided (D12): both stay, each row names its visit.
9-11. (Round 1 follow-up points: removed with the follow-up, D3.)
12. **The counter is not fully defined.** Not defined yet: is the number the count sent, or the
    number of the next one? Hidden at zero? Do "before" messages count? Does a booked future visit
    reset it?
21. **The second message counts from the visit, not from the send** (cost of D3). If she sends the
    30-day message late, on day 42, the 45-day message comes 3 days later. The row chip
    (`General · 45 días · 2º mensaje`) makes that visible; she can skip it.
13. **The word "Recordatorios" would mean two things.** In Ajustes it means rules; a "Para enviar"
    tab with the same name would mean today's messages. Decided (D17): rules become "Reglas".
14. **Three words for "goes away".** "Vence", "Desaparece" and "Se cancela". "Vence en 12 días" can
    read as "not due yet", and "cancela" clashes with a cancelled appointment. Proposed: a date,
    "Hasta el 21/12", plus "Se quita si agenda Relleno". A date also stays right on a list read
    offline.
15. **Quick replies are a full new feature:**
    * a new module that owns its table;
    * create, edit, archive and reorder (PATCH `position`, since a new verb fails a guard test);
    * sent in `/bootstrap` and cached on the phone, so copy works offline;
    * in the export, plus threat-model and UI-probe entries.

    WhatsApp Business already has "Respuestas rápidas". If she uses it, the app's list may repeat
    it, and the same name may make her think they sync (not checked).
16. **Tabs now have a third and fourth user** (Servicios, Para enviar, Ajustes). The project rule
    says to make one shared tab component, designed in `soft-glass.pen` first. Every moved route
    needs its UI-probe, title, navigation and service-worker entries.
17. **The rules table must stay in one place.** Change release-1 "Reminder rules" in place, in the
    stage that builds each rule. Record that the general rule brings back "any later visit removes
    it" for its own kind (release-1 rejected that for specific rules, alternative E5).
18. **Migration numbers clash.** The password design already claims 00003. Name migrations by
    purpose and take the next free number when each stage lands.
19. **The export changes:** `datos.json` gains the description and quick replies (version 2); a
    general rule exports with an empty service. `servicios.csv` gains `descripcion`, through the existing
    formula-safety function.

### Smaller points

* The 60 days appear in Go and in an es.json text, with no test that ties them. The server should
  send the date.
* Offline past midnight, a countdown can go wrong; hide a message past its last day.
* "Pausado" sounds like she paused it. Proposed: "No envía mensajes: la plantilla está archivada".
* The text pair "¿Incluir recordatorios atrasados?" / "Incluir atrasadas" mixes masculine and
  feminine.
* A general rule with "Incluir atrasadas" can bring back one message per recent client at once.
* The service description is itself a hidden rule; its field needs the hint "Se agrega a tus
  cotizaciones".
* Already shipped and only needs rewording: the archived-part warnings, the late-messages
  explanation, and "para agendar X" in the rule sentence.

### Making the rules visible without paragraphs (UX research)

The problem: the rules are clear in code but hidden in the app. Even the owner of the code had
trouble with them. So the fix is not more text: the app should show each rule's effect where it
happens.

What other products do:

* **Show the state, not the rule.** Gmail nudges put one short tag next to an email: "Received
  3 days ago. Reply?". Nobody reads how nudges work; the tag says enough.
* **Recognition over recall** (Nielsen heuristic 6): the reason must be on screen, so she never
  has to remember a rule from the settings page.
* **The reason travels with the item** ("why am I seeing this?"): a row says which rule made it.
* **Say the consequence when she acts**, in the undo toast, not before.
* **Details on tap** (progressive disclosure): one line visible, the rest one tap away.
* **Preview with real data** in the rule editor: show who the rule hits, not how it works.

Applied here. Hard limit: one short line per element, numbers and dates instead of words, no
paragraphs anywhere.

| Place | Today | Proposed (example text) |
| :--- | :--- | :--- |
| Row in "Para enviar" | name + message text | adds one chip naming the visit that made it: `Cita 1 nov · Uñas · quedan 38`; tapping it opens that appointment (D12) |
| Row, last week | same | chip turns warning color: `Último día` / `Quedan 3 días` |
| General rule's row | (new) | chip: `Cita 10 nov · Cejas · General 45 días` |
| Toast after "Sí, lo envié" | "Deshacer" | `Enviado a Ana · siguiente el 15 dic · Deshacer` (when another general rule is still ahead) |
| Toast after "Omitir" | `Omitido: Ana` | `Omitido: Ana · no se mostrará de nuevo · Deshacer` |
| Rule editor | sentence (already exists) | sentence + preview: `Hoy: 4 clientas · próxima: Ana, 23 nov` |
| Rule list | sentence | sentence + count: `12 mensajes en espera` |
| Client page | nothing | one line: `Próximo mensaje: 23 nov (Uñas)` or `Sin mensajes pendientes` |
| Empty "Para enviar" | `Nada para enviar hoy` | `Nada para hoy · próximo: Ana, jueves` |
| Tap on a chip | (new) | 2 lines max: which rule, why it ends (`Sale si agenda o a los 60 días`) |

Why this matters for the open choices:

* With the chip on every row, the second general rule (chosen in D3) is understandable: the row
  says `General · 45 días`.
* The mixed-history case (two messages to one client) stays visible instead of a surprise: the
  client page shows both planned messages, and she can skip one.
* The 60-day expiry needs no explanation text: `quedan 38` counts down on the row.

Cost: the chip and toast need data the server already has (visit date, rule, days). The server
sends the last day as a date; the app only formats it. The client-page line and the editor
preview need one new read each (computed on read, never stored).

### Wording pass on every UI text (owner request)

The owner asked to rewrite every text that reads like the attribute question
"¿Es igual siempre para la clienta, o cambia en cada cita?". Rules for the pass:

* A label beats a question. A name beats a sentence.
* About 6 words or fewer for labels, chips, toasts. One short line for hints.
* Say the result, not the mechanism. No "el sistema", "latido", "temporizador".
* No "volver a salir"; the verb for a list item is "mostrar" ("No se mostrará de nuevo").
* Delete a hint the label already says.

The e2e tests read the texts from `es.json`, so changing a text does not break a test.

| Key | Today | Proposed |
| :--- | :--- | :--- |
| `attribute_scope_question` | ¿Es igual siempre para la clienta, o cambia en cada cita? | Tipo de atributo |
| `attribute_scope_client` | Es igual siempre para la clienta | Atributo fijo |
| `attribute_scope_appointment` | Cambia en cada cita | Atributo por cita |
| `attribute_fixed_note` | El tipo y si es de la clienta o de la cita no se pueden cambiar. Si necesitas otro, crea un atributo nuevo y archiva este. | No se puede cambiar después de crearlo |
| `attribute_required` | Obligatorio al guardar | Obligatorio |
| `option_add_inside` | Agregar opciones dentro | Agregar subopciones |
| `offline_banner` and 9 more with "con el sistema" | Sin conexión con el sistema. Datos {when}. | Sin conexión · datos {when} |
| `error_response_invalid` | El sistema respondió algo que no se entiende. Intenta de nuevo | Error del servidor. Intenta de nuevo |
| `due_confirm_failed` | No se guardó. Ya lo enviaste: cuando haya conexión, toca Sí otra vez | No se guardó. Con conexión, toca «Sí, lo envié» |
| `due_skip_wait` | Espera a que pase el aviso anterior | Espera unos segundos |
| `appointment_end_auto` | Calcular de nuevo | Calcular hora de fin |
| `appointment_service_remove` | Quitar una copia de {name} | Quitar 1 {name} |
| `clients_pick_body` | Toca una clienta de la lista para ver sus citas, sus fotos y sus recordatorios. | Toca una clienta para ver sus citas y fotos |
| `client_archive_body` | Deja de salir en la lista y en las búsquedas. Sus citas y fotos se conservan y la puedes restaurar en Ver archivadas. | Se oculta de la lista. Sus citas y fotos se guardan |
| `photo_archive_body` | La foto deja de verse en la cita y en la galería de la clienta. | Se oculta de la cita y de la galería |
| `templates_pick_body` | Para cambiarla, o crea una nueva. | O crea una nueva |
| `template_chip_hint` | Toca para cambiar lo que se escribe si no tiene el dato | Toca para elegir el texto si falta el dato |
| `template_fallback_hint` | Lo que se escribe si la clienta no tiene este dato. Si lo dejas vacío, no se escribe nada. | Vacío: no se escribe nada |
| `template_fallback_quote` | Este texto no puede llevar comillas rectas (") | Quita las comillas (") |
| `template_archive_body` | Los recordatorios que la usan dejan de enviar mensajes hasta que elijas otra plantilla. | Sus recordatorios se pausan hasta que elijas otra |
| `rules_templates_body` | Un recordatorio envía una plantilla. Crea una para empezar. | Primero crea una plantilla |
| `rule_return` | Para agendar | Servicio a ofrecer |
| `rule_archive_body` | Deja de crear mensajes por enviar. Los que ya enviaste quedan en el historial. | Deja de crear mensajes. El historial se guarda |
| `business_push_hint` | A esta hora te avisamos de los mensajes por enviar. De 00:00 a 19:00. | Aviso diario de mensajes (00:00 a 19:00) |
| `business_icon_hint` | El ícono nuevo se verá en la pantalla de inicio solo después de que quites la app y la vuelvas a agregar. | Para verlo, quita la app y agrégala otra vez |
| `setup_icon_hint` | Elige una imagen cuadrada o un logo. Se verá en la pantalla de inicio de tu teléfono. Puedes omitirlo. | Imagen cuadrada para tu pantalla de inicio. Opcional |
| `quote_empty` | Toca los servicios que te preguntaron para armar la cotización. | Toca los servicios para cotizar |
| `quote_copy_failed` | No se pudo copiar. Mantén presionado el texto de abajo para copiarlo. | No se pudo copiar. Mantén presionado el texto |
| `quote_replace_prompt` | Ya tienes una cita nueva sin guardar. ¿La reemplazamos con estos servicios? | Ya tienes una cita sin guardar. ¿Reemplazarla? |
| `quote_convert_failed` | No se pudo pasar a una cita nueva. Tu cotización sigue aquí; inténtalo de nuevo. | No se pudo crear la cita. Intenta de nuevo |
| `status_timer` / `status_timer_none` | Temporizador diario / Aún sin latido | Tarea diaria / Aún no ha corrido |
| `overdue_*` | (see Smaller points) | rewrite with the same rules |

Stage: this is its own small stage (texts only, no logic), shipped first. It touches
`es.json` (and any copy of the same text in `design/soft-glass.pen`, not checked yet), so it
carries no migration risk.

Sources: [Gmail nudges](https://mailmeteor.com/blog/gmail-nudges),
[9to5Google on nudges](https://9to5google.com/2018/05/14/new-gmail-web-nudge-ai-reminders/),
[Nielsen heuristics](https://ixdf.org/literature/article/user-interface-design-guidelines-10-rules-of-thumb).

## Goals vs. Non-Goals

* **In-Scope Goals:**
  * A general come-back rule with no service, counting from the client's latest visit, beaten by a
    specific rule.
  * Several general rules allowed, each on its own.
  * A second (or third) message comes from another general rule with a later gap and its own
    template; only the latest due one shows per client (D11).
  * A sent counter on each message, computed from the log.
  * Service description in the catalog, the Cotizar text and the export.
  * Quick replies: plain text, her own order, search, one-tap copy, edited in the same tab.
  * "Para enviar" with two tabs; Ajustes rules and templates in one entry with two tabs.
  * One short line for each hidden rule, shipped in the stage that adds the rule.
* **Out-of-Scope Non-Goals:**
  * The password policy (its own design).
  * Variables in quick replies.
  * A follow-up that counts from the send day (dropped, D3).
  * Sending messages automatically (still `wa.me` links she taps).
  * Undo of a "sent" mark.

## Alternatives Weighed

* **General rule as a new rule kind (`come_back_any`).** Every switch in the code would find each
  place to change. Rejected for now: a new kind breaks an older app on her phone (Blocker 3). The
  lenses prefer a NULL `service_id` with a call-site checklist and tests.
* **A real follow-up that counts from the send day** (columns on the rule, a `step` in the log and
  its unique key, its own item id and load window). Rejected by the owner (D3): it needs a schema
  change to the log, a two-release rollout for the phone, and it was the hardest part to explain.
  Chosen instead: a second general rule at a later gap. Cost: it counts from the visit, not the
  send (Major 21).
* **Follow-up as a child rule** or **stored in the log's `anchor_at`**: dropped with the follow-up.
  (`anchor_at` was also refused by the server on a come-back row, `log.go:81`.)
* **No sent counter**; the row chip's `2º mensaje` carries the number. Offered to the owner.
* **Quick replies inside `message_templates`.** Rejected: templates parse variables, and one table
  per module stays clearer.
* **No quick replies in the app; use WhatsApp Business "/" replies.** She has WhatsApp Business.
  Rejected by the owner (D16).
* **Description inside the Cotizar price line, or on a line under it.** Rejected by the owner: long
  lines or the name twice, and every price-line format and test would change.
* **Amending `quick-quotes.md` for all of this.** Rejected: that doc is built; one new staged doc.

## Key Decisions & Staging for Next Steps

### Decided (owner)

* D1. General rule counts only from the latest visit; a specific rule wins.
* D2. Several general rules allowed.
* D3. No follow-up feature. A second message is a second general rule at a later gap (for example
  30 and 45 days), with its own template, counted from the last visit.
* D4. Counter of messages sent since the last visit, reset by a new visit.
* D5. Service description:
  * optional, one line, 1-200 characters after trimming; empty means none; a line break is 422;
  * column `services.description text NULL` with a CHECK of 1-200 characters, its own migration;
  * API: `Service.description` (required, nullable); optional and nullable in `ServiceInput` and
    `ServicePatch`;
  * export: a `servicios.csv` column after the name, and `datos.json`;
  * edited in `ServiceSheet.svelte`: "Descripción (opcional)" under the name;
  * Cotizar text: after the duration line and one blank line, one `quote_detail_line`
    "{name}: {description}" per picked service that has one, once per service, in price-line
    order; with no described service the text is exactly as today; `quick-quotes.md` "Text format"
    is updated.
* D6. Quick replies: plain text, fixed order, search, one-tap copy, own table, edited in the same
  tab.
* D7. Ajustes: one entry with tabs "Reglas" and "Plantillas" (entry name in D17).
* D8. Visible rules: enough lines to make the app understandable.
* D9. Visible-rule UI approved as in "Making the rules visible without paragraphs": a chip on
  every due row, consequence in the toast, preview in the rule editor, next message on the
  client page, details on tap. No paragraphs.
* D10. Wording pass approved as in "Wording pass on every UI text", with "Atributo fijo" /
  "Atributo por cita". Its own texts-only stage, shipped first.
* D11. Several general rules: only the latest due message shows per client. An earlier unsent one
  leaves the list when the next one is due (computed on read, nothing stored).
* D12. D11 covers only general rules. Mixed history (nails with its own rule, then eyebrows under
  the general rule) gives both messages. Every row names the appointment that made it (date and
  services) and opens it on tap, so two rows for one client are explained by their two visits.
* D13. "Specific wins" only while the specific rule can send. A paused specific rule (template
  archived) or an archived one does not count; the general rule covers that visit.
* D14. Archiving a specific rule, or the template that pauses it, asks the existing question
  "Solo de hoy en adelante / Incluir atrasadas" (reworded per D10) when a general rule would take
  over its visits. Asked in both places: the rule archive and the template archive.
* D15. Counter (refines D4): the row shows the number of this message (`2º mensaje`) = come-back
  messages sent to this client since her last visit + 1. Hidden on the 1st. "Antes de la cita"
  messages do not count. Computed from the log on read.
* D16. Quick replies stay (D6), even though she uses WhatsApp Business, which has its own. Copy
  works offline (cached from `/bootstrap`). Cost accepted: a second list she keeps by hand.
* D17. Names. "Enviar" screen tabs: "Recordatorios" (the items to send) and "Respuestas" (quick
  replies). Ajustes entry: "Reglas de recordatorio". So a **recordatorio** is an item to send and
  a **regla** is what she sets up. This replaces P4's word choice. Texts that call a rule
  "recordatorio" change in the wording pass (D10): `settings_rules`, `rules_title`, `rules_new`,
  `rule_new_title`, `rule_edit_title`, `rule_save`, `rule_saved`, `rule_created`, `rule_archive`,
  `rule_archived`, `rule_restore`, `rule_restored`, `rules_loading`, `due_no_rules_title`,
  `due_no_rules_action` ("Regla", "Nueva regla", "Aún no tienes reglas", …).
* D18. The row shows a countdown (`quedan 38`), worked out on the phone from the server's
  `shown_until` date (P3) and today in the business time zone. Hidden while offline, because
  cached data can be stale; the row then shows no countdown at all.
* D19. (Confirms P1.) A general rule stays kind `come_back` with `service_id` NULL. In the rule
  form, the "Después de este servicio" picker gets a first option "Todos los servicios" that means
  NULL. No new kind, so an old app keeps loading (`schemas.ts:73` already accepts a NULL service).
* D20. (Confirms P3.) Each due item gains server-computed fields: `shown_until` (day), the source
  visit (start time and service names) and `message_number`. The phone only formats them. The
  60 days stay only in Go. An old app ignores the new fields (Zod strips unknown keys).
* D21. (Confirms P5; fixed by `.claude/rules/ui.md` "One component per job".) The Servicios tab
  nav (`web/src/routes/servicios/+layout.svelte`: one URL per tab, a `nav` with `aria-current`,
  pills on the phone, underline on wide) becomes one `ui/` primitive, designed first in
  `design/soft-glass.pen`, and Servicios, Enviar and Reglas de recordatorio use it.

### Proposed by the review (owner to confirm)

* P1. General rule: `service_id` NULL, no return service, no booking prompt. "Later" means a later
  start time, not cancelled or no-show.
* P2. (Dropped with the follow-up, D3.)
* P3. The server sends each message's last shown day (`shown_until`); the phone only formats it.
* P4. Words: replaced by D17 and D18.
* P5. One shared tab component, designed first in `soft-glass.pen`; existing URLs kept.

### Staging (review proposal)

| Stage | Ships |
| --- | --- |
| S0 | Design doc, archived brief, threat-model rows, `.pen` tab component and frames |
| S1 | Service description |
| S0b | Wording pass (D10): texts only |
| S2 | Wire prep: `shown_until`, the status line and chips, the phone accepts a rule with no service |
| S3 | General rule |
| S5 | Sent counter |
| S6 | Tab component, Ajustes merge, "Para enviar" tabs, quick replies |

Each stage is its own release, so a rollback (a snapshot restore) loses only that stage.

### Open questions (next round)

None. All round-1 questions are decided (D3, D11-D18).
