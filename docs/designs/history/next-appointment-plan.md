# Agendar próxima cita: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** A button on the appointment page opens a sheet of date choices taken from the come-back
rules, and the picked one opens Nueva cita already filled.

**Architecture:** Phone only. One pure function (`nextVisit`) turns the appointment and the cached
rules into date choices with their services. A feature sheet shows them and hands the picked one to
Nueva cita through the existing `cita:new` draft, written by one shared hand-off function.

**Tech Stack:** SvelteKit (Svelte 5 runes), TypeScript strict, Zod, Dexie, Vitest, Playwright
WebKit.

**Spec:** [next-appointment.md](../next-appointment.md). Read it first; section numbers below are
its sections.

**As built (2026-11-12).** This plan is a record. The code differs from it in these places; the
spec has the current text:

*   The date choices are `glass` `Button`s with two lines, not `ListRow`s (contrast on the glass
    sheet at 1440 px).
*   The services of "Otra fecha" are the `hint` of the date field, not a line above it.
*   Nueva cita's header arrow stays a link, with `data-sveltekit-replacestate` on its header; it is
    not a button with `goto`.
*   The reset, client and clean-up hooks of the two rule specs live in `ruleSpec(test)` in
    `web/tests/e2e/seed.ts` (jscpd flagged the copied preamble).
*   Added after the code review: the toast "Cita agendada · {date}" after a save in Nueva cita
    (`appointment_created`), the message slot scrolled into view, and two more e2e tests (the
    question in view under three choices; "Reemplazar" books the choice tapped last).
*   The texts `next_choice_one` and `next_choice_other` landed in Task 2 and the other three with
    the tests of Task 3.

## Global Constraints

*   No file under `api/`, `internal/` or `cmd/` changes (spec Goal 5).
*   Every UI text is Spanish, lives in `web/src/lib/i18n/es.json` and is read with `t()`; wording
    rules: `.claude/rules/spanish-texts.md`. The texts are fixed by spec section 8.
*   UI rules: `.claude/rules/ui.md`. No new primitive, token or `.pen` component. Surface and
    radius classes only in `web/src/lib/components/ui/`. No `<style>` block, no arbitrary value.
*   Lint fails on: an export nothing imports (knip), a copy of 10 lines or more (jscpd, e2e specs
    included), a disabled `Button` with no reason (types).
*   Never skip, focus or loosen a test to get green (`.claude/rules/testing.md`). e2e locators:
    role and label first.
*   Public repo: made-up names and phones only; no home path, e-mail or employer name anywhere.
*   Node is the pinned one (`make node-check`). Go and web commands go through `make`.
*   Gate before "done", output reported:
    `make check-pins lint test vuln build check-generated test-db cover test-deploy secrets test-sec config-scan sast deps-scan`
    and `make e2e-docker`. A check that could not run is reported as NOT RUN.
*   Branch: `next-appointment`, cut from `origin/main` after `git fetch` (the untracked design
    files come along). Commit steps run only if the owner allowed commits for this build; push
    never without being asked.

## Review Focus

Inputs the spec implies and its CUJs do not exercise. Each has its test in the task named.

1.  A visit that ends after midnight (23:00 to 00:30): the days count from the local day of
    `ends_at`, the next day. Task 2, test `counts from the local day the visit ends`.
2.  "Reemplazar" after the question: it books the choice she tapped, not an older one. Task 3, e2e
    `CUJ N4: Reemplazar`.
3.  The "Otra fecha" field emptied again after a real day: "Usar esta fecha" is off again with
    "Elige el día". Task 3, e2e `CUJ N3`.
4.  An appointment with no services: the sheet says "Sin servicios" and Nueva cita opens with no
    line. Task 2, test `an appointment with no services`.
5.  Six services on one choice at 393 px: the subtitle wraps, nothing is cut. Task 4, UI review
    (the layout probe only sees the probe's one service).

---

### Task 1 (S1): one hand-off function, one replace question

No behaviour change. Cotizar works as before.

**Files:**
- Move: `web/src/lib/cache/quoteHandoff.ts` → `web/src/lib/cache/citaHandoff.ts` (`git mv`)
- Move: `web/src/lib/cache/quoteHandoff.test.ts` → `web/src/lib/cache/citaHandoff.test.ts`
- Create: `web/src/lib/components/ReplaceDraftPrompt.svelte`
- Modify: `web/src/lib/components/QuoteForm.svelte` (import, the `convert` body, the `notice`
  snippet)
- Modify: `web/src/lib/i18n/es.json` (three keys renamed, texts unchanged)
- Modify: `web/tests/e2e/quote.spec.ts` (keys and log code)
- Modify: `docs/designs/quick-quotes.md`, `docs/designs/reminders-and-replies.md`, `TASKS.md`
  (every old name; files under `docs/designs/history/` are records and stay)

**Interfaces:**
- Produces: `handOff(form: Form, replace: boolean): Promise<'needs_confirm' | 'ok' | 'failed'>`
  in `web/src/lib/cache/citaHandoff.ts`. Same steps as `convertQuote`; the draft written is
  `FormSchema.parse(form)`; the log line is `code=cita_handoff_failed`.
- Produces: `ReplaceDraftPrompt.svelte`, props
  `{ busy: boolean; onreplace: () => void; oncancel: () => void }`. It is the `confirm_replace`
  block of `QuoteForm.svelte` moved as it is (a `Callout role="status"` with the question, a
  `strong` "Reemplazar" and a `glass` "Cancelar", both `busy`).
- Produces: text keys `draft_replace_prompt`, `draft_replace`, `cita_handoff_failed` (old:
  `quote_replace_prompt`, `quote_replace`, `quote_convert_failed`).

- [ ] **Step 1: Move the two files and change the test first**

In `citaHandoff.test.ts`: import `handOff` from `./citaHandoff`; `describe('handOff', …)`; every
`convertQuote(lines, TODAY, x)` becomes `handOff({ ...blankForm(TODAY), lines }, x)`; the expected
log is `'code=cita_handoff_failed'`. Add:

```ts
it('writes the whole form, not only the lines', async () => {
	const form = { ...blankForm('2026-12-03'), clientId: 'client-ana', lines, time: '14:30' };

	expect(await handOff(form, false)).toBe('ok');

	expect(await getDraft(KEY, FormSchema)).toEqual(form);
});

it('a form that fails its schema returns failed and writes nothing', async () => {
	const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
	const bad = { ...blankForm(TODAY), lines: [{ service_id: 'svc-a', count: 0 }] };

	expect(await handOff(bad, true)).toBe('failed');

	expect(warn).toHaveBeenCalledWith('code=cita_handoff_failed');
	expect(await getDraft(KEY, FormSchema)).toBeNull();
});
```

- [ ] **Step 2: Run it, see it fail**

Run: `cd web && pnpm exec vitest run src/lib/cache/citaHandoff.test.ts`
Expected: FAIL, `handOff` is not exported.

- [ ] **Step 3: Write `handOff` in `citaHandoff.ts`**

Rename and change the signature; the header comment names both callers (Cotizar and Agendar próxima
cita) and both design docs.

- [ ] **Step 4: Run it, see it pass**

Run: `cd web && pnpm exec vitest run src/lib/cache/citaHandoff.test.ts`
Expected: PASS, 7 tests.

- [ ] **Step 5: `ReplaceDraftPrompt.svelte`, `QuoteForm.svelte`, `es.json`, `quote.spec.ts`**

`QuoteForm` calls `handOff({ ...blankForm(today), lines: visible }, replace)` and renders
`<ReplaceDraftPrompt busy={converting} onreplace={…} oncancel={…} />` in its `notice` snippet.
Rename the three keys in `es.json` (same place, same texts) and in `quote.spec.ts`
(`es.draft_replace_prompt`, `es.draft_replace`, `es.cita_handoff_failed`,
`'code=cita_handoff_failed'`).

- [ ] **Step 6: The docs**

`grep -rn "quoteHandoff\|convertQuote\|quote_replace\|quote_convert_failed" docs/designs/*.md TASKS.md web/`
must print nothing afterwards. `quick-quotes.md` section 4 gets one sentence: the function was
generalized for a second caller, with a link to `next-appointment.md` section 5.

- [ ] **Step 7: Lint and unit tests**

Run: `make lint test`
Expected: both green.

- [ ] **Step 8: Commit** (if allowed)

`git add -A web docs/designs TASKS.md` and commit: `web: one hand-off into Nueva cita (handOff)`.
This first commit also adds the design, the plan and the brief.

---

### Task 2 (S2): `nextVisit`, the pure part

**Files:**
- Create: `web/src/lib/nextVisit.ts`, `web/src/lib/nextVisit.test.ts`
- Modify: `web/src/lib/rules.ts` (`canSend` → exported `ruleCanSend`, three call sites; export
  `effectiveReturn`)
- Modify: `web/src/lib/booking.ts` (export `linesOf` and `validDay`; `formFrom` calls `linesOf`)
- Modify: `web/src/lib/booking.test.ts` (one test for `linesOf`, one for `validDay`)

**Interfaces:**
- Consumes: `ruleCanSend(rule: Rule, parts: { templates: readonly Template[]; services: readonly Service[] }): boolean`
  and `effectiveReturn(rule): string | null` from `rules.ts`; `addDays`, `todayIn`, `shortDay`
  from `time.ts`; `activeServiceIds` from `booking.ts`.
- Produces, in `booking.ts`:
  `linesOf(saved: Appointment['services']): Line[]` (one line per service, in first-seen order,
  `count` = its copies) and `validDay(day: string): boolean` (today's private function, exported).
- Produces, in `nextVisit.ts`:

```ts
export type Choice = { days: number; day: string; lines: Line[] };
export type NextVisit = { choices: Choice[]; otherDate: Line[] };

export function nextVisit(
	appointment: Pick<Appointment, 'ends_at' | 'services'>,
	rules: readonly Rule[],
	templates: readonly Template[],
	services: readonly Service[],
	zone: string,
	today: string
): NextVisit;

/** "En 21 días · 3 dic"; "En 1 día · 4 dic" for one day. */
export function choiceTitle(choice: Choice): string;

/** "Relleno ×2, Pedicure"; "Sin servicios" for no line. A name not in `services` is left out. */
export function linesText(lines: readonly Line[], services: readonly Service[]): string;

/** The button shows: the status is scheduled or confirmed, and the client is in `clients`. */
export function canBookNext(
	appointment: Pick<Appointment, 'status' | 'client_id'>,
	clients: readonly { id: string }[]
): boolean;
```

**The algorithm of `nextVisit`** (spec sections 1 to 3):

1.  `sending` = the `come_back` rules for which `ruleCanSend` is true.
2.  `specific` = those of `sending` whose `service_id` is on the appointment. `counted` =
    `specific`, or, when it is empty, the general ones of `sending` (`service_id === null`).
3.  `base` = `todayIn(zone, new Date(appointment.ends_at))`.
4.  For each different `offset_days` of `counted`, rising: a choice
    `{ days, day: addDays(base, days), lines: linesFor(days) }`; drop it when `day < today`.
5.  `ruleFor(serviceId, days)`: among `specific` with that `service_id`, sorted by `offset_days`
    then `id`: the first with `offset_days === days` when `days` is not null and one exists, else
    the first.
6.  `linesFor(days)`: for each line of `linesOf(appointment.services)`, the target is
    `effectiveReturn(ruleFor(…))`, or the line's own service with no rule; keep it only when the
    target is in `activeServiceIds(services)`; two lines with the same target become one, at the
    place of the first, with the larger count.
7.  `otherDate` = `linesFor(null)`.

- [ ] **Step 1: Write `nextVisit.test.ts`**

Fixtures as in `rules.test.ts` (`rule()`, `service()`, `template()` builders; ids `ACRYLIC`,
`REFILL`, `PEDICURE`, `REMINDER`). `ZONE = 'America/Costa_Rica'`, `TODAY = '2026-11-12'`, visit
`ends_at: '2026-11-12T22:00:00Z'` (16:00 local) unless a test says otherwise. `appt(...names)`
builds `{ ends_at, services }` with one entry per name. One `it` per line:

| Test | Given | Expect |
| --- | --- | --- |
| `a before rule gives no choice` | only a `before_appointment` rule; visit Acrílico | `choices` `[]`, `otherDate` `[Acrílico ×1]` |
| `a rule for a service on the visit` | Acrílico 21 → Relleno; visit Acrílico | `[{ days: 21, day: '2026-12-03', lines: [Relleno ×1] }]`; `otherDate` `[Relleno ×1]` |
| `a rule for another service does not count` | Pedicure 30; visit Acrílico | `choices` `[]` |
| `a general rule alone` | general 30; visit Pedicure | `[{ days: 30, day: '2026-12-12', lines: [Pedicure ×1] }]` |
| `a general rule is left out when a specific one counts` | general 30, Acrílico 21 → Relleno; visit Acrílico | one choice, `days` 21 |
| `a rule that cannot send does not count` (`it.each`: rule archived, template archived, service archived, return service archived) | Acrílico 21 → Relleno with that part archived, plus general 30 | one choice, `days` 30 |
| `CUJ N1: two rules, one appointment` | Acrílico 21 → Relleno, Pedicure 30; visit Acrílico, Pedicure | days `[21, 30]`; both `lines` `[Relleno ×1, Pedicure ×1]` |
| `two rules with the same days give one choice` | Acrílico 21 → Relleno, Pedicure 21 | one choice |
| `CUJ N3b: the services follow the picked days` | Acrílico 21 → Relleno, Acrílico 60 → own; visit Acrílico | 21: `[Relleno ×1]`; 60: `[Acrílico ×1]`; `otherDate` `[Relleno ×1]` |
| `the fewest days decide for a service with no rule of the picked days` | Acrílico 21 → Relleno, Acrílico 60 → own, Pedicure 30; visit Acrílico, Pedicure | 30: `[Relleno ×1, Pedicure ×1]` |
| `two rules of the same days: the lower id decides` | Acrílico 21 → Relleno (id `…2`), Acrílico 21 → Pedicure (id `…1`) | 21: `[Pedicure ×1]` |
| `the count is kept` | Acrílico 21 → Relleno; visit Acrílico, Acrílico | `[Relleno ×2]` |
| `two lines that become one service keep the larger count` | Acrílico 21 → Relleno; visit Acrílico, Acrílico, Relleno | `[Relleno ×2]` |
| `an archived service is left out` | no rule; visit Acrílico, Pedicure; Pedicure archived | `otherDate` `[Acrílico ×1]` |
| `a choice before today is left out, today stays` | Acrílico 21 → Relleno; `today` `'2026-12-04'`, then `'2026-12-03'` | `[]`, then one choice |
| `counts from the local day the visit ends` | Acrílico 21 → Relleno; `ends_at` `'2026-11-13T03:30:00Z'` (21:30 on the 12th), then `'2026-11-13T06:30:00Z'` (00:30 on the 13th) | `'2026-12-03'`, then `'2026-12-04'` |
| `applies_from and handover_from are not read` | Acrílico 21 → Relleno with both set to `'2027-01-01T00:00:00Z'` | one choice |
| `an appointment with no services` | general 30; visit with `services: []` | one choice with `lines` `[]`; `otherDate` `[]` |

Plus: `choiceTitle` (`{ days: 21, day: '2026-12-03' }` → `'En 21 días · 3 dic'`;
`{ days: 1, day: '2026-12-04' }` → `'En 1 día · 4 dic'`), `linesText`
(`[Relleno ×2, Pedicure ×1]` → `'Relleno ×2, Pedicure'`; `[]` → `'Sin servicios'`), `canBookNext`
(`it.each` over the four statuses: true for `scheduled` and `confirmed`; false when the client is
not in the list). In `booking.test.ts`: `linesOf` (A, B, A → `[A ×2, B ×1]`) and `validDay`
(`'2026-12-03'` true; `''`, `'2026-02-30'`, `'03/12/2026'` false).

- [ ] **Step 2: Run, see it fail**

Run: `cd web && pnpm exec vitest run src/lib/nextVisit.test.ts src/lib/booking.test.ts`
Expected: FAIL, the module and the exports do not exist.

- [ ] **Step 3: The exports in `rules.ts` and `booking.ts`, then `nextVisit.ts`**

`nextVisit.ts` starts with a header comment: what it is, the spec, and that it computes nothing
about what is due.

- [ ] **Step 4: Run, see it pass**

Run: `cd web && pnpm exec vitest run src/lib/nextVisit.test.ts src/lib/booking.test.ts src/lib/rules.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit** (if allowed): `web: nextVisit, the date choices of a visit`

knip is red until Task 3 imports `nextVisit`; run `make lint` at the end of Task 3, not here.

---

### Task 3 (S2): the sheet, the button, the back arrow, and their e2e

**Files:**
- Create: `web/src/lib/components/NextAppointmentSheet.svelte`
- Create: `web/tests/e2e/next-appointment.spec.ts`
- Modify: `web/src/routes/citas/[id]/+page.svelte` (the button and the sheet)
- Modify: `web/src/routes/citas/nueva/+page.svelte` (the header arrow; its comment)
- Modify: `web/src/lib/i18n/es.json` (five new keys)
- Modify: `web/tests/e2e/seed.ts` (`serviceIdOf`), `web/tests/e2e/steps.ts` (`failDraftWrite`),
  `web/tests/e2e/quote.spec.ts` (uses `failDraftWrite`), `web/tests/e2e/offline.spec.ts` (N5),
  `web/tests/e2e/ui-probe.spec.ts` (sheet entry)
- Modify: `docs/designs/release-1.md`, `CLAUDE.md`, `docs/prd.md`

**Interfaces:**
- Consumes: `nextVisit`, `choiceTitle`, `linesText`, `canBookNext`, `Choice` (Task 2); `handOff`,
  `ReplaceDraftPrompt` (Task 1); `blankForm`, `validDay`, `Line` from `booking.ts`;
  `formatTime`; `clock.today(zone)`; `submitOnce`.
- Produces: `NextAppointmentSheet.svelte`, props
  `{ open: boolean; appointment: Appointment; data: CacheSnapshot; onclose: () => void }`.
- Produces, test helpers: `serviceIdOf(name: string): Promise<string>` in `seed.ts` (the id of
  the active service with that name; throws when there is none);
  `failDraftWrite(page: Page, key: string): Promise<void>` in `steps.ts` (the `addInitScript` of
  `quote.spec.ts`'s failed-write test, moved; `put` throws `QuotaExceededError` for that draft
  key).

**New texts** (spec section 8): `appointment_next` "Agendar próxima cita", `next_choice_one`
"En 1 día · {date}", `next_choice_other` "En {n} días · {date}", `next_other_date` "Otra fecha",
`next_use_date` "Usar esta fecha".

**The sheet** (spec section 4), top to bottom inside
`<BottomSheet {open} title={t('appointment_next')} onclose={close}>`:

1.  Per choice: `<ListRow title={choiceTitle(choice)} subtitle={linesText(choice.lines, data.services)} onclick={…} />`.
2.  A `<p>` with `linesText(plan.otherDate, data.services)`, then
    `<TextField label={t('next_other_date')} type="date" bind:value={otherDay} />`, then a `strong`
    `Button` "Usar esta fecha", `busy`, and while `!validDay(otherDay)`:
    `disabled` with `disabledReason={t('appointment_day_needed')}`.
3.  The message: `ReplaceDraftPrompt` (`confirm_replace`), or
    `<Callout role="alert" class="text-sm">{t('cita_handoff_failed')}</Callout>` (`failed`).
4.  A `glass` `Button` "Cancelar" (`cancel`), not rendered while the question shows.

State: `otherDay = $state('')`, `message = $state<'none' | 'confirm_replace' | 'failed'>('none')`,
`asked = $state<{ day: string; lines: Line[] } | null>(null)`, `busy`. `plan` is
`$derived(nextVisit(appointment, data.rules, data.templates, data.services, zone, clock.today(zone)))`
with `zone = data.settings.timezone`. `close()` sets `message` to `'none'`, `otherDay` to `''`,
then calls `onclose`.

One `submitOnce` function `pick(day, lines, replace)`: sets `asked`, calls
`handOff({ ...blankForm(day), clientId: appointment.client_id, lines, time: formatTime(appointment.starts_at, zone) }, replace)`;
`'needs_confirm'` → `message = 'confirm_replace'`; `'failed'` → `message = 'failed'`; `'ok'` →

```ts
await goto(resolve(`/citas/nueva?volver=/citas/${appointment.id}` as const), { replaceState: true });
```

(the `resolve(… as const)` form `routes/+page.svelte` uses for `?dia=`). "Reemplazar" runs
`pick(asked.day, asked.lines, true)`; the question's "Cancelar" sets `message = 'none'`.

**The page** (spec section 6): `let booking = $state(false)`; in the action row, after the status
buttons and before "Editar":

```svelte
{#if canBookNext(appt, data.clients)}
	<ActionButton variant="glass" onclick={() => { booking = true; }}>
		{t('appointment_next')}
	</ActionButton>
{/if}
```

and, beside `OverlapSheet`, the sheet inside `{#if appt !== null && data !== null}`.

**Nueva cita** (spec section 4, "History"): the header `IconButton` loses `href` and gets
`onclick={() => { void goto(back, { replaceState: true }); }}`. The page comment says Save, Cancel
and the arrow all replace this form's history entry, and why (the appointment page goes back with
`history.back()`).

- [ ] **Step 1: The helpers, then `next-appointment.spec.ts`**

Shape as `general-rules.spec.ts`: `test.use({ serviceWorkers: 'block' })`, `resetData` and one
client "Ana Rojas" in `beforeEach`, `deleteSpecRows(added)` in `afterEach`, `closePool` in
`afterAll`. Setup data already has Acrílico (90 min), Relleno, Pedicure and the rule "21 días
después de Acrílico → Relleno". A rule added with `insertComeBackRule` has no return service, so it
returns its own service. Helpers in the spec: `sheet(page)` =
`page.getByRole('dialog', { name: es.appointment_next })`; `choice(page, days)` = the button in the
sheet whose name starts with `En ${days} días`; `openSheet(page, appointmentId)` = open
`/clientas/<ana>`, tap the appointment's link, tap the button `es.appointment_next`. Nueva cita is
checked with the labels `quote.spec.ts` uses (`es.appointment_day`, `es.appointment_time`, the
"remove service" buttons, the client's name).

| Test | Steps | Expect |
| --- | --- | --- |
| `CUJ N1: the rule names the next service, and its reminder goes` | Add rule Pedicure 30. Ana: Acrílico + Pedicure at `at(-21, '14:30')`, `createdAt: at(-22, '09:00')`. Marta: Acrílico at the same time (her item proves the list is the server's). `/enviar`: Ana's item with the text "ya toca tu relleno" is there. Open the sheet. | Choices 21 and 30, each with "Relleno, Pedicure". Tap 21: URL `/citas/nueva?volver=…`; Ana, Relleno and Pedicure (no Acrílico), day `businessDate(0)`, time `14:30`, notes empty. Save: the visit's page again (`/citas/<visit>`). Its back arrow: Ana's client page. `/enviar`: Marta's item is there, and no item of Ana has "ya toca tu relleno". |
| `CUJ N2: only the general rule` | Add a general rule of 30 days. Ana: Pedicure at `at(1, '10:00')`. Open the sheet. | One choice, 30, with "Pedicure". Tap: Pedicure, day `businessDate(31)`, time `10:00`. |
| `CUJ N3: no rule, another date` | Ana: Pedicure at `at(1, '10:00')`. Open the sheet. | No `En … días` button; "Pedicure" above the field; "Usar esta fecha" is disabled with the description `es.appointment_day_needed`. Fill `businessDate(10)`: enabled. Clear it: disabled again. Fill again, tap: day `businessDate(10)`. The header arrow (a button now): the appointment page. `page.goBack()`: Ana's client page, never the heading `es.appointment_new`. |
| `CUJ N3b: the services follow the picked days` | Add rule Acrílico 60. Ana: Acrílico at `at(1, '09:00')`. Open the sheet. | Choice 21 has "Relleno" and not "Acrílico"; choice 60 has "Acrílico". Tap 60: Acrílico, no Relleno, day `businessDate(61)`. |
| `CUJ N4: Cancelar keeps the unsaved booking` | Start a Nueva cita with Marta and Pedicure, wait for `draftStored(page, 'cita')`. Ana: Acrílico at `at(1, '09:00')`. Open the sheet, tap 21. | `es.draft_replace_prompt` in the sheet, and only one "Cancelar" button. Tap it: the question goes, the sheet stays open. `/citas/nueva`: Marta and Pedicure. |
| `CUJ N4: Reemplazar` | Same start. Tap 21, then `es.draft_replace`. | Nueva cita: Ana, Relleno, day `businessDate(22)`, no Pedicure. |
| `a failed write says so and the sheet stays` | `failDraftWrite(page, 'cita:new')` before the first `goto`; collect console warnings. Ana: Acrílico at `at(1, '09:00')`. Open the sheet, tap 21. | An alert with `es.cita_handoff_failed` in the sheet; the URL is still the appointment's; the warnings hold `code=cita_handoff_failed` and no client or service name. |
| `a cancelled appointment has no button` | Ana: Acrílico at `at(1, '09:00')`, `status: 'cancelled'`. Open its page. | "Reactivar" is there; no button `es.appointment_next`. |

In `quote.spec.ts`, the failed-write test calls `failDraftWrite(page, 'cita:new')` instead of its
own script. In `offline.spec.ts`, `CUJ N5`: open Ana's appointment page online, `serverOff`,
reload; the button `es.appointment_next` is disabled with the accessible description
`es.offline_cannot_save` (as the "Agregar foto" test does). In `ui-probe.spec.ts`, the
`'/citas/[id]'` entry gets `sheets: [{ name: 'next appointment', open: button(es.appointment_next) }]`.

- [ ] **Step 2: Run, see it fail**

Run: `make e2e-docker`
Expected: FAIL in `next-appointment.spec.ts`, the `offline.spec.ts` N5 test and the probe's new
sheet (no button `Agendar próxima cita`); every other spec green, `quote.spec.ts` included.

- [ ] **Step 3: The five texts, the sheet, the page, the Nueva cita arrow**

- [ ] **Step 4: Run, see it pass**

Run: `make lint test` then `make e2e-docker`
Expected: all green. `ui-probe.allow.ts` gets no new entry: a finding is fixed in the component.

- [ ] **Step 5: The docs this change makes false**

*   `docs/designs/release-1.md`, "Appointments": one bullet, "**Next appointment:** the button
    'Agendar próxima cita' on a scheduled or confirmed appointment opens Nueva cita filled from
    the come-back rules; phone only", with a link to `next-appointment.md`.
*   `CLAUDE.md`, "Architecture source": one sentence naming `docs/designs/next-appointment.md` for
    the next-appointment button, built in its stages S1 and S2.
*   `docs/prd.md`, "Citas": `**FR-P6** Agendar la próxima cita desde una cita: la app propone las
    fechas de las reglas de "volver" y abre la cita nueva ya llena (ver
    [docs/designs/next-appointment.md](designs/next-appointment.md)).`

- [ ] **Step 6: Commit** (if allowed): `web: Agendar próxima cita on the appointment page`

---

### Task 4: gate, UI review, hand-over

**Files:**
- Modify: `TASKS.md` (one section, as "Quick quotes S4": what ran, what is NOT RUN)
- Modify: `docs/designs/next-appointment.md` (Status → built, with the date)
- Move: `docs/designs/next-appointment-plan.md` → `docs/designs/history/`

- [ ] **Step 1: The gate**

Run: `make check-pins lint test vuln build check-generated test-db cover test-deploy secrets test-sec config-scan sast deps-scan`
Expected: every target green. Keep the output for the report.

- [ ] **Step 2: Goal 5**

Run: `git diff --stat origin/main -- api internal cmd`
Expected: no output.

- [ ] **Step 3: Fault injection** (spec, Verification Plan)

One at a time, break and restore: "a service uses its rule of the picked days", "the fewest days
decide otherwise", "general rules only without a specific rule", "a choice before today is left
out". Each must turn at least one test of `nextVisit.test.ts` red. Report which test caught which.

- [ ] **Step 4: UI review** (`.claude/rules/ui.md`, stage 6)

Run the `web-design-guidelines` skill on the UI diff. Then, in the running app (`make dev`, made-up
data) with `agent-browser`: open the sheet at 393 × 852 and 1440 × 900 with (a) two choices, (b) no
choice, (c) the replace question, (d) an appointment with six services. Check: nothing cut or
overlapping, focus moves into the sheet and back to the button on close, the subtitle wraps. A
screenshot of each goes in the report (scratchpad, not the repo).

- [ ] **Step 5: `code-reviewer` subagent**

A fresh reviewer re-runs the gate and fault-injects the guards the change relies on. Fix what it
finds; re-run Steps 1 and 2 after any fix.

- [ ] **Step 6: `TASKS.md`, the design's Status, move this plan; commit** (if allowed)

The `TASKS.md` section lists as NOT RUN: the owner's check on the phone (open an appointment, tap
the button, pick a date, save), and CI on the branch.
