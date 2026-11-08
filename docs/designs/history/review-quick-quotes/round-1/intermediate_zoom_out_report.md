# Stage 8: Intermediate Zoom Out Review

**Target:** `docs/designs/quick-quotes.md` (full file, unchanged since Stages 5-7). Reviewer:
`egm-zoom-out` (completed). Inputs: Stage 5-7 reports. 0 new Blocker, 10 Major, 9 Nit.

Verdict: the ~60 earlier recommendations, applied as written, grow the design. Two conflict
(M-A, M-C/M-D), about a dozen repeat each other. The change set at the end resolves the Stage 7
Blocker and every Major, and removes more than it adds.

## Major

* **M-A. Convert in a lib function, but the quote clear stays in the route.** A direct Dexie delete
  cannot cancel Cotizar's pending 500 ms autosave timer (it survives `goto`; only the autosave
  handle's `clear()` calls `clearTimeout`, `drafts.ts:30-33`). Fix: `convertQuote(lines, today,
  replace): Promise<'needs_confirm' | 'ok' | 'failed'>` in `lib/cache`, reading and writing only
  `cita:new` via `getDraft`/`putDraft`; on `ok` the route calls the quote's `formDraft` `clear()`,
  then `goto`. Removes `readDraft`, `writeDraft`, `formFromLines`.
* **M-B. One `formDraft` rule:** nothing is saved or cleared until `load()` finishes; after that, a
  form equal to `initial` clears the draft and any other form saves. No user-change flag. Tests:
  "stored draft + fresh mount at initial → restored and still stored"; "change then revert → gone".
* **M-C. Ask whenever a parsed `cita:new` draft exists.** Drop `isBlankForm` (and the Critic's
  by-value definition). Extract `blankForm(day)` from `AppointmentForm.svelte:60`.
* **M-D. Tab links from `Card`** (`as="a"`, `radius="pill"`, `selected`, `aria-current`) in a
  labelled `<nav>`; no `NavItem` variant, no new `Segmented` primitive. Hoy unchanged.
* **M-E. The text block is `Card as="p"`** with `whitespace-pre-line select-text`; no new primitive.
* **M-F. One inline `Callout` slot** above the text, a tagged union `none | confirm_replace |
  copy_failed | convert_failed`, `role="alert"` for failures, reset on the next pick, copy or
  convert. Toast only for "Copiado". No Toast variant.
* **M-G. A short Risks H2:** Accepted (one quote at a time; Nueva cita's Total counts unpriced as 0;
  "Recuperamos..." after convert; press-and-hold fallback), each tagged by who decided; Handled as
  one line each pointing at its section. No five-column table.
* **M-H. Move the catalog once** to `/servicios/catalogo` (nav item points there until Cotizar
  lands, then `/servicios`); delete the line-392 claim; rename stages S0-S4 in run order; tag each
  goal's check with its stage.
* **M-I. Export one `quoteText(lines, services, settings): string`;** rows private; `minor` not
  `Cents`; skip archived/missing only in the builder; drop the zero-duration branch (the catalog
  requires 1-1440 minutes).
* **M-J. Drop T100.** State once in NFR: the quote reads no client data; a failed copy logs only a
  code (guard `catch-blocks`); e2e clipboard stubs cover missing API, rejection and synchronous
  throw.

## Nit

* N-1 One sentence defining "place" vs "tab"; fix line 339; drop "Goldfish".
* N-2 Add one Pro and Con to owner alternatives B2, B3 only.
* N-3 A visible "Nueva cotización" button when there are lines (`startOver()`); no restore note on
  Cotizar.
* N-4 One relative link to the brief's post-move path.
* N-5 Additional Resources and Future Improvements become H2.
* N-6 Convert uses `submitOnce` (busy state, no double tap).
* N-7 Cite symbols instead of line numbers where S1a edits the file.
* N-8 Wording nits in passing; drop "about a minute".
* N-9 Put Cotizar first, or tag the order "(agent)" with a reason.

## Smallest change set

1. `formDraft` load-gate rule (M-B).
2. `convertQuote` in `lib/cache`; the route clears the quote through `formDraft` (M-A, M-C); add
   `blankForm(day)`.
3. One `Callout` slot with a tagged union (M-F).
4. `Card` for the tab links and the text block (M-D, M-E).
5. One catalog move; stages S0-S4; goal checks tagged (M-H).
6. One exported `quoteText` (M-I).
7. Drop T100; add the synchronous-throw stub (M-J).
8. Short Risks section (M-G).
9. Doc fixes: place/tab sentence, B2/B3 cons, brief link, two H2s, "Nueva cotización" button.
