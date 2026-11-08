# Goldfish Review Report

**Target:** `docs/designs/quick-quotes.md` (lines 1-604, FeatureRange; the Detailed Implementation Plan from line 605 is out of scope)

**Stage:** 7, Critic Review. Standard for `egm-design` and `egm-structural`: `feature-spec-standard`. `egm-style`: no standard.

## Reviewer Status

| Reviewer | Status |
| --- | --- |
| `egm-design` | completed |
| `egm-structural` | completed |
| `egm-style` | completed |

**Severity counts (after deduplication):** 1 Blocker, 9 Major, 23 Nit.

Reviewers checked these code citations and found them correct: `formDraft.svelte.ts:28-35`, `AppointmentForm.svelte:56` and `:74`, `booking.ts:357-358`, `citas/nueva/+page.svelte:36-41`, the prefix match in `nav.ts`, and release-1 Alternative E8. No reviewer recommends reversing an "(owner)" decision.

## Prioritized Findings (Blockers -> Major -> Nits)

### Blocker

### No Risks section - Severity: Blocker
* **Reviewer Lens:** `egm-structural` (Blocker), `egm-design` (Major)
* **Affected Scope:** Whole document. It should sit between Technical Plan and Alternatives Considered (release-1.md has one at line 909).
* **Discrepancy Description**: The standard's "Risks & Limits / Mitigations" requirement is not met. The risks the design does cover are scattered: the totals not matching after convert (lines 249-251), the iOS clipboard and five-tab fit (Open Questions, lines 107-113, and section 6), a failed draft write (line 306), rollback (section 8), and security (NFR lines 576-580). Some risks appear nowhere:
  - Only one quote can be open at a time. A second asker's quote overwrites the first.
  - After convert, Nueva cita's `totalOf` counts a missing price as 0 (`booking.ts:62-68`), so she sees a lower Total with no hint.
  - The "Recuperamos..." note appears after convert (line 314).
  - Routes, probe rows, titles and e2e steps move twice, in S1n and again in S1b (lines 391-392).
  - The quote is lost if the Nueva cita draft is later emptied.
* **Rationale & Recommendation**: Add a "Risks & Mitigations" section in release-1's layout. It should have an "Accepted risks" list, each with a decision owner (the totals mismatch, the restored-draft note, the press-and-hold clipboard fallback, one quote at a time). It should also have a "Handled risks" table giving the trigger, impact, mitigation, check, and the section that holds the detail.

### Major

### No visible way to start a fresh quote - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 2 "The draft" (lines 261-264), CUJ Q1 Then (line 134), States (lines 267-272)
* **Discrepancy Description**: The design says "Empezar de nuevo" resets the quote "as on the other forms". On those forms the button appears only inside FormCard's restored-draft callout, when `restored && onstartover` (`FormCard.svelte:46-51`). `restored` becomes true only when a draft loads on mount (`formDraft.svelte.ts:19-26`). So after copying a quote for one asker, nothing on Cotizar clears it. The most common flow, one price question after another, starts from the last asker's lines. The design also does not say whether Cotizar shows the restored-draft note. If it does, the note appears on every tab switch.
* **Rationale & Recommendation**: Add a clear action (for example "Nueva cotización" or "Limpiar") that stays visible while there are lines. Add it to the States list, the S0 `.pen` frames, and an e2e step (copy, clear, quote again). State that Cotizar does not show the restored-draft note.

### The "error toast" does not exist, and a toast is the wrong place for the fallback - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 3 Copiar (lines 280-284), section 4 convert failure (lines 306-307), Goal 6, CUJ Q4
* **Discrepancy Description**: `toaster.svelte.ts` has one plain message type and no error variant. `Toast.svelte:23-29` is `role="status"` and hides itself after `UNDO_MS` = 5000 ms (`undo.ts:3`). So the "press and hold the text" instruction looks the same as "Copiado", is announced politely, and is gone in 5 seconds, which is exactly when she needs to read it while switching apps. The convert failure has the same problem.
* **Rationale & Recommendation**: Show a failed copy as a persistent inline `role="alert"` message next to the text block, as AppointmentForm already does (`AppointmentForm.svelte:222-224`). Clear it on the next successful copy or quote change. Use the toast only for "Copiado". If an error toast is still wanted, draw the new Toast variant in `.pen` first (ui.md stage 1) and list it in S0.

### "tab" and "place" are ambiguous, and there is no terms list - Severity: Major
* **Reviewer Lens:** `egm-style`, `egm-structural`
* **Affected Scope:** Lines 7, 19-21, 45, 72-75, 107-110, 316-356, 391-392, 499-502. Glossary missing after the Overview (release-1 lines 39-57).
* **Discrepancy Description**: "tab" means both a bottom-bar entry and each link inside Servicios. Line 339 shows the result: "The bottom tab 'Servicios' stays current on both tabs." "place" and "top-level place" are never defined. Other terms are also undefined for a new reader: "the gate", "`.pen` frame", "ui-probe", "draft", `volver`, the Spanish screen names, "c/u", "precio a consultar", and "Goldfish" (process jargon, line 6).
* **Rationale & Recommendation**: Add a "Terms used in this document" list under the Overview. Define "place" as a bottom-bar or sidebar entry and "tab" as a link inside Servicios, then use each term only in that sense. Replace "Goldfish" with plain words such as "design review rounds".

### Line 392 contradicts the stage table - Severity: Major
* **Reviewer Lens:** `egm-style` (Major), `egm-design` (Nit)
* **Affected Scope:** Section 9, lines 387-392
* **Discrepancy Description**: The doc says "no build ever ships a Servicios place with only one tab". But S1n ships `/servicios` with the catalog only, and the tab links arrive in S1b (line 388).
* **Rationale & Recommendation**: Say exactly what S1n ships ("Servicios with the catalog only, no tab control"). Alternatively, merge S1n and S1b so the catalog moves only once, which also removes the double-move risk.

### Goals are not mapped to stages, and the stage names are out of order - Severity: Major
* **Reviewer Lens:** `egm-style`, `egm-structural`
* **Affected Scope:** Line 49 against the stage table at lines 383-389
* **Discrepancy Description**: Line 49 says each goal's proving stage "is defined in the Technical Plan". The stage table names no goals; its Exit column mentions only CUJ Q2/Q3 and the 15-second check, so the reader cannot tell which stage proves goals 2-4 or 6-10. The order S0, S1a, S1n, S1b, S1c breaks the letter sequence, the "n" is never explained, and "S0 to S1c" reads as wrong.
* **Rationale & Recommendation**: Add a "Proves goals" column to the stage table, or tag each goal's Check with its stage. Rename the stages in run order (S1a-S1d, or S1-S4) and update line 49.

### Alternatives entries use different formats - Severity: Major
* **Reviewer Lens:** `egm-structural`
* **Affected Scope:** Alternatives Considered, lines 400-570
* **Discrepancy Description**: release-1 gives every alternative Pros, Cons, and a Reason for Rejection. Here only A1-A6 and B1 have all three. B2, B6, B8, D2 and D5 have only a Reason. B3, B5, B7, C1-C6, D1, D3 and D4 have no Pros. B4 and C4 add a description paragraph. B2 and B3 restate the owner's choice instead of recording its cost. D4 bundles three rejected options under one heading.
* **Rationale & Recommendation**: Give every alternative the same three parts. For owner decisions, record at least one Pro and one Con so the cost is written down (no reversal is asked for). Split D4 into three entries.

### `concepts_brief.md` references cannot be followed - Severity: Major
* **Reviewer Lens:** `egm-structural` (Major), `egm-style` (Nit)
* **Affected Scope:** Lines 6, 396, 591, and 592 onward
* **Discrepancy Description**: The brief is cited as a bare code span. No such file exists relative to `docs/designs/`. Copies exist at the repo root and at `docs/designs/history/concepts_brief.md`, and release-1.md links the history copy. Line 396 says some rejections were decided in the brief, so the design does not stand alone.
* **Rationale & Recommendation**: Link the intended file by its real relative path. Make every "(owner)" and "(agent)" reason complete inside this document. Make `CLAUDE.md` and `.claude/rules/ui.md` links too, to match line 589.

### "honest" is a judgement that hides the actual gap - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 54 (Goal 2), 250, 601
* **Discrepancy Description**: The doc says "correct and honest", "the honest total holds in the quote but not after convert", and "Honest total in Nueva cita". These judge instead of describe. Line 250 hides the concrete behaviour: Nueva cita's Total silently counts an unpriced service as 0.
* **Rationale & Recommendation**: Use concrete wording. Goal 2: "The copied text matches the catalog and marks unpriced lines." Line 250: "After convert, Nueva cita's Total counts an unpriced line as 0 and gives no 'a consultar' note." Line 601: "Unpriced lines in Nueva cita's total."

### Additional Resources and Future Improvements are at the wrong heading level - Severity: Major
* **Reviewer Lens:** `egm-structural`
* **Affected Scope:** Lines 587 and 595
* **Discrepancy Description**: Both are H3, so they read as subsections of Non-Functional Requirements. release-1.md makes them H2 (lines 1262 and 1269).
* **Rationale & Recommendation**: Promote both to H2.

### Nit

### Width numbers are for five tabs and are stated twice - Severity: Nit
* **Reviewer Lens:** `egm-design`, `egm-style`
* **Affected Scope:** Lines 107-110 and 347-349
* **Discrepancy Description**: "Today each tab gets about 66 px" is the five-tab figure: (393 - 32 - 16 - 16) / 5 is about 66, leaving about 42 px of label after `px-3`. With four tabs, each gets about 83 px. Open Questions repeats the same arithmetic and the same hedge.
* **Rationale & Recommendation**: Correct the wording, keep the numbers only in section 6, and point to them from Open Questions.

### Threat-model row stated as done, in two places that do not match - Severity: Nit
* **Reviewer Lens:** `egm-style`, `egm-structural`, `egm-design`
* **Affected Scope:** Lines 287 and 580
* **Discrepancy Description**: Line 287 says "`docs/threat-model.md` records this", but the file has no T100 or clipboard row yet. Line 580 calls it a new row, T100. IDs in the threat model are not in sequence (T90-T99 sit among T01-T19).
* **Rationale & Recommendation**: State it once, as future work ("S0 adds row T100"), naming the asset and the test that checks it, and point to it from the other place.

### Diagram borders are broken - Severity: Nit
* **Reviewer Lens:** `egm-style`, `egm-structural`
* **Affected Scope:** Lines 173-175
* **Discrepancy Description**: The tab-bar label overruns the phone box and collides with the "no change" server box, which has no left edge on that row.
* **Rationale & Recommendation**: Shorten the label or widen the box, and give the server box complete borders of its own.

### Repeated unpriced service and the zero-total branch - Severity: Nit
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 1 "Text format" (lines 217-220), the missing-price table, line 247
* **Discrepancy Description**: The doc does not say how "×2" prints when the service has no price, yet Goal 2 tests only the table's cases. "A total of 0 leaves the line out" can only happen with no lines, because the catalog requires 1-1440 minutes (`services.ts:37-39`).
* **Rationale & Recommendation**: Add the repeated-unpriced row to the table. Drop the zero-total branch or explain why it stays.

### Convert needs a busy state and double-tap protection - Severity: Nit
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 4, lines 291-304
* **Discrepancy Description**: Convert does an async read, may ask for confirmation, then awaits a write. ui.md requires async buttons to show busy and to block a second submit.
* **Rationale & Recommendation**: State that convert goes through `submitOnce` (`web/src/lib/submit`) and shows a busy state.

### Convert works offline, an unstated exception - Severity: Nit
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 2 "Offline" (line 272), against release-1.md:607
* **Discrepancy Description**: release-1 disables every link to a create screen offline (`WriteLink`). Convert stays enabled. That is harmless, since Save is disabled offline, but it is undocumented. Cotizar's empty-state "Configurar servicios" is a `WriteLink` (`EmptyState.svelte:31`), so it is disabled offline while the Mis servicios tab link is not.
* **Rationale & Recommendation**: Tag it as an "(agent)" decision and give the reason.

### Draft-clear fix needs a test against convert - Severity: Nit
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 7, first fix (lines 362-365), and section 4
* **Discrepancy Description**: The new rule clears the draft when the form is back at its start. That rule could fire on Nueva cita's first effect run, before the draft that convert just wrote has loaded (`formDraft.svelte.ts:19-35`). Today the order favours the load only by accident.
* **Rationale & Recommendation**: Add a `formDraft` unit test: a stored draft plus a mount that equals the initial values restores the draft and does not delete it.

### Define "blank" by value - Severity: Nit
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 4 step 2 (lines 295-297)
* **Discrepancy Description**: "No attributes" should mean every value is empty under `isEmpty` (`booking.ts:171-172`), not that `attrs` has no keys, because a cleared multiselect is stored as `[]`.
* **Rationale & Recommendation**: Define blank by value. Better still, after the section 7 fix, define "not blank" as "a `cita:new` row exists and differs from `formFromLines([], today)`", so emptiness has only one definition.

### Wide layout of Cotizar and the moved catalog header - Severity: Nit
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Section 5 "Layout" (lines 326-330)
* **Discrepancy Description**: The doc gives no intent for the wide layout, although the owner forbids a stretched phone layout. With the h1 moving to the layout, the doc does not say where "Nuevo servicio" goes (today it shares a `justify-between` row with the h1 in `ajustes/servicios/+page.svelte:55-67`).
* **Rationale & Recommendation**: Add one sentence of layout intent per width, so the S0 frame review has something to check.

### Draft schema written in mixed notation - Severity: Nit
* **Reviewer Lens:** `egm-structural`
* **Affected Scope:** Lines 262-263
* **Discrepancy Description**: `{ lines: LineSchema[] }` mixes TypeScript type notation with zod. Today the line schema is defined inline in `FormSchema` (`booking.ts:139`).
* **Rationale & Recommendation**: Write `z.object({ lines: z.array(LineSchema) })`.

### User-facing strings missing - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 164, 282, 295, 306
* **Discrepancy Description**: The copy-failure text, the replace prompt and the convert-failure text are described but not given, unlike line 216.
* **Rationale & Recommendation**: Give each new `es.json` key and its Spanish value, or say that the S0 `.pen` frames fix the wording.

### CUJs formatted inconsistently - Severity: Nit
* **Reviewer Lens:** `egm-structural`
* **Affected Scope:** Lines 119-165
* **Discrepancy Description**: Q1 and Q2 number their steps, while Q3 and Q4 do not. Q2's Then contains a second journey ("If she had backed out..."). No CUJ covers the catalog tab or first setup.
* **Rationale & Recommendation**: Use one format for every CUJ. Move the backed-out case into its own CUJ or into Goal 5.

### Goals formatted inconsistently - Severity: Nit
* **Reviewer Lens:** `egm-structural`, `egm-style`
* **Affected Scope:** Lines 51-82
* **Discrepancy Description**: Some goals add prose after the bold heading. Goal 7 reads "Check: e2e test: ...". Goal 1 says "in seconds" while its check says 15.
* **Rationale & Recommendation**: Use one layout for every goal: outcome, an optional line of explanation, then Check. Retitle Goal 1 as "... in under 15 seconds".

### Unmeasured baseline - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 28-34
* **Discrepancy Description**: "About a minute per question" is not measured, yet Goal 1's 15-second target is set against it.
* **Rationale & Recommendation**: Have the owner time one real reply in S0, or drop the number.

### Predictions stated as fact - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 41, 80, 365
* **Discrepancy Description**: "far more often than she edits it", "quoting makes that worse" and "she would hit this daily" are predictions.
* **Rationale & Recommendation**: Describe the mechanism instead, for example "each tab switch unmounts the form, so an emptied draft reloads on every return".

### Process jargon in a check - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 75, 353-354
* **Discrepancy Description**: "a new check, seen failing once" assumes the reader knows the repo's fault-injection habit.
* **Rationale & Recommendation**: Write instead: "shown to fail with five tabs at today's padding before the fix".

### Hedge on a checkable fact - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 224-225
* **Discrepancy Description**: "for CRC that likely includes decimals and non-breaking spaces" hedges on something checkable: `formatMoney` (`money.ts:13-18`) is a deterministic `Intl.NumberFormat`.
* **Rationale & Recommendation**: Run it once and state the output, or keep only "Illustrative; amounts are what `formatMoney` prints."

### Untagged, passive decision - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Line 314
* **Discrepancy Description**: "That note is accepted for v1" does not say who decided.
* **Rationale & Recommendation**: Tag it "(agent)" or "(owner)" and use the active voice. List it under Accepted risks.

### S1a rename has no stated reason - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Line 386
* **Discrepancy Description**: "Rename the local `quote` in `AppointmentForm.svelte:56`" gives no reason and no new name.
* **Rationale & Recommendation**: State both (presumably the name clashes with the new `quote.ts`).

### "At most a few dozen lines" contradicts "no line cap" - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Line 581 against line 100
* **Discrepancy Description**: The Performance section gives an upper bound that the Non-goals section rules out.
* **Rationale & Recommendation**: Write "Linear in the number of picked lines; no cap."

### Informal or subjective wording in Alternatives - Severity: Nit
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 426, 428, 490, 550
* **Discrepancy Description**: The Alternatives use "a list nobody reads", "No real use for the history", "for no reader" and "Nail wording".
* **Rationale & Recommendation**: Replace them with factual wording, for example "No planned screen reads a quote history" and "Fixed wording in the default text".

### Rollout section lacks a pointer intro - Severity: Nit
* **Reviewer Lens:** `egm-structural`
* **Affected Scope:** Section 8 (lines 370-377) and line 583
* **Discrepancy Description**: Rollout is described in two places. The NFR line points to section 8, but the doc lacks release-1's intro line "Each line points to the section that holds the detail."
* **Rationale & Recommendation**: Add the intro line so the pointer style stays consistent.
