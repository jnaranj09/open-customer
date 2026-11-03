# Intermediate Zoom Out Report: Stage 8 (round 3)

Target: `docs/designs/release-1.md`, full file (lines 1–2274).
Reviewer: `egm-zoom-out`, in a fresh context. It read the Stage 5, 6 and 7 reports.
Standard: `zoom-out-standard.md`.

Verdict: pass with changes. No Blockers, 8 Major, 9 Nit. Most fixes are needed. Several would add scope where cutting text or rewording is enough.

## 🟡 Major

1. **Search only on the phone (Stage 5 #1). Accept it and cut the server search.**
   - Delete: `pg_trgm`, `unaccent`, `f_unaccent`, the GIN indexes, `clients.sql` search, `Search`, and the DB search tests.
   - `GET /clients?archived=true` stays a plain list.
   - The `search.ts` tests become the only search tests.
2. **Export paging contradicts itself (Stage 5 #2).** Delete the "pages of 500" sentence at L675.
3. **CUJ 1 wizard order (Comprehension #2).** Edit the CUJ, not the design: "…accent color, then username and password; then an optional icon."
4. **What `{{appointment.*}}` means (Comprehension #1, Stage 5 #5). Fix it by deleting.**
   - Preview renders only against the sample client. Drop `client_id?` and `appointment_id?`.
   - Move `Preview` into `templates`.
   - Add one sentence: "in a come-back message, `appointment.*` is the past visit that started it".
   - If real-client preview is kept, use her latest appointment. No picker.
5. **Backup failures are hidden (design-3-2). Needed, in its smallest form.**
   - Add one red line on "Hoy" when backup `red` is true, reusing the existing value.
   - No author alert: it needs new config and an outside service (priority 4).
6. **"She keeps her data through the export" (design-3-3).** Rewrite the risk honestly instead of adding a nudge. Optionally, a sealed restic password goes into the handover checklist.
7. **Late come-back items after a rule change (design-3-1). Decide the intent first.**
   - The flood is bounded: the 60-day look-back caps it, and clients who already rebooked drop out.
   - (a) Document it as intended.
   - (b) Or add the existing `reminder_rules.created_at` to the created-day guard. No new `effective_at` column.
   - Add the question to the interview.
8. **Moving all detail out of the Technical Plan (Stage 5 #3) is too big.**
   - Fix the export drift (#2).
   - Cut L554 and L562 to *what* and *why*.
   - Leave the backup, upgrade and restore steps where they are.

## 🟢 Nit

9. Delete the option cycle check (it can't happen) and its test.
10. Kept SAD ADRs (structural-3-1): one link to the ADR table in `history/sad-original.md` is enough. Downgraded from Major.
11. `restore` service: reword Goal 8 to "four services, plus a `restore` profile". Downgraded from Major.
12. Conflicting small fixes; pick the cheaper side:
    - Define "item N" once in Terms instead of adding ~20 links. Fix the heading name at L283.
    - Delete the EGM Terms entry.
    - Add a P0–P7 Terms line.
13. Banner text: "Sin conexión con el sistema", with no `navigator.onLine` branch.
14. Double send (design-n2): reword the error sheet, and add no new state.
    - design-n6 is already covered by the version prompt.
    - design-n3: add it to the interview.
    - Skip the VPS alternative.
15. Delete instead of rewording:
    - CUJ 8's `download` detail;
    - "They get no new mitigation";
    - the Meta/slot-mapping terms;
    - "tidy job", "costs little now", "easy to add";
    - the M1 size claims. This is a Nit, not a Major.
16. Change L325 to "concrete types; an interface only where a test needs a fake". Keep the item 32 table as the contract.
17. Small edits:
    - the 20:00 cutoff as a fixed product value in Goal 7;
    - item 14: "next free number";
    - Goal 3: "the last four checks";
    - Goal 5: "at most 24 h apart while the PC is on".
    - Skip: reordering sections, moving J3/J4, the CUJ 5 layout, and rewriting the style-n4 rationales.

## Simplest combined change

- **Delete:**
  - the server search;
  - the export paging sentence;
  - real-client preview (`Preview` moves to `templates`);
  - the cycle check;
  - the subjective phrases;
  - the EGM entry;
  - CUJ 8's implementation detail.
- **Edit:**
  - CUJ 1;
  - the export risk;
  - the Goal 8 wording;
  - the banner text;
  - trim L554 and L562;
  - Goals 3 and 5;
  - item 14.
- **Add (one sentence each):**
  - `appointment.*` means the past visit;
  - the late-item intent (or the `created_at` guard);
  - Terms for "item N" and P0–P7;
  - the SAD ADR link;
  - the interview question.
- **Only new behavior:** one red backup line on "Hoy".
- **Do not do:**
  - `effective_at`;
  - the export nudge;
  - an author alert;
  - a "falta confirmar" state;
  - a full move of the mechanics;
  - a risks table;
  - linking every "item N";
  - section reordering;
  - a VPS alternative.
