# Stage 6: Comprehension Test

**Target:** `docs/designs/quick-quotes.md` (FeatureRange 1-604, DetailedRange 605-916). Reviewer:
`egm-comprehension` (completed). 0 Blocker, 5 Major, 6 Nit.

## What this proposes

The design adds a phone-only quote page called Cotizar. The owner taps services from her catalog,
and the page builds a fixed Spanish reply: one line per service, with a ×N form for repeats and
"precio a consultar" for services with no price, then a total that admits what it leaves out, then
a duration line. One function builds the text for both the screen and the clipboard. Copiar writes
it to the clipboard within the same tap. If the phone refuses, the on-screen text can be pressed
and held to copy it by hand. "Convertir en cita" writes a full Nueva cita draft into Dexie and
waits for the write to finish. If an unsaved booking is already there, she is asked before it is
replaced. Only after the write succeeds is the quote cleared and Nueva cita opened, with no
`volver`, so both Save and Back land on Hoy. Cotizar and the moved service catalog ("Mis
servicios") live in a new fifth top-level place, Servicios, at `/servicios` and
`/servicios/catalogo`. Two pre-existing draft bugs are fixed first. The work ships in stages S0,
S1a, S1n, S1b and S1c, with no server, API or database change.

## How the existing system works, per this document

The PWA keeps a Dexie copy of what `/bootstrap` returns: services with name, price and duration,
and settings with business name, currency and locale. Two forms, new appointment and new client,
keep local drafts under `<form>:new` keys through a shared `formDraft` lifecycle. It saves 500 ms
after any change that differs from the form's start, but it never clears the draft when the form
returns to its start. Nueva cita reads its draft once when it opens and restores it with
`restoreForm`. Its own total treats a missing price as 0. Its Save and Back both go to a `volver`
parameter, or to Hoy when there is none. The service catalog sits under Ajustes → Servicios. The
shell has four places (Hoy, Enviar, Clientas, Ajustes), rendered from `NAV_ITEMS` by `BottomNav`
and `Sidebar`. Each tab gets about 42 px of label width at 393 px. There is one app-wide toast
(`toaster.say`).

## Findings

1. **Major. Section 9 and item 15: "no build ever ships a Servicios place with only one tab" is
   false.** In S1n the layout renders only the h1 and the catalog sits alone at `/servicios`, so
   S1n ships a one-page place with no tabs. It is the only reason given for moving the page twice.
2. **Major. Section 1 and item 24: `quoteText(quote)` cannot call `formatMoney`.** `formatMoney`
   needs currency and locale (`web/src/lib/money.ts:13`); `Quote` carries neither. Also `*Cents`
   names suggest exponent 2; `formatMoney` takes minor units whose exponent depends on the currency.
3. **Major. Section 4, item 29 and section 7: the quote's pending 500 ms autosave at clear time is
   not addressed.** A save pending from her last tap could rewrite `cotizacion:new` after convert
   clears it, breaking CUJ Q2 ("Cotizar is empty the next time").
4. **Major. Goal 6 promises an e2e test for a failing draft write that no item adds.** Item 30
   covers only CUJ Q2 and Q3. Item 25 lists a synchronous-throw clipboard case that item 27 does not
   test.
5. **Major. Item 5 (T100): `quote.test.ts` cannot cover "the copy failure logs a code only".** The
   handler lives in the route; a pure-builder Vitest test cannot see its `console.warn`.
6. **Nit.** The repeat-line template differs between section 1 (`×{n}`) and item 20 (`{count}`,
   filled by `appointment_service_count`). Say so once.
7. **Nit.** "S1n" is never explained, and alphabetically it reads as after S1c.
8. **Nit.** `concepts_brief.md` is ambiguous (there is also `docs/designs/history/concepts_brief.md`)
   and goes stale after item 31; give the post-move path.
9. **Nit.** "Today in the business time zone" names no helper; say where Cotizar computes `today`
   (release-1 has `todayIn`).
10. **Nit.** "Seen failing once" is the fault-injection step (technical-taste canon item 2); say so.
11. **Nit.** The format list should say the total line is omitted when no line has a price, as the
    duration line says for 0.
