# Intermediate Zoom Out Report: Stage 8 (round 5)

Target: `docs/designs/release-1.md`, full file (lines 1–2349).
Reviewer: `egm-zoom-out`, in a fresh context. It read the Stage 5, 6 and 7 reports.
Standard: `zoom-out-standard.md`. The owner's decisions were given as constraints.

Verdict: passes after small edits. 0 Blocker, 3 Major, 16 Nit.
- No fix needs a new component, endpoint or table.
- Most fixes are one sentence, and several are deletions.
- 13 proposals are turned down because they reopen a decision, add scope, or change text with no gain.

## 🟡 Major

**MA1. Writes have no transaction (Stage 5 M1, plus Comprehension #1).** Lines 337, 1677, 1742, 1752, 1761.
- **Problem:** handlers pass the pool, and `db.DBTX` has no `Begin`. So each multi-statement write commits one statement at a time:
  - appointment create (one insert plus the service lines);
  - `PATCH service_ids`;
  - the client duplicate check plus the insert.
- **Overlap:** line 337 also leaves out the `DueList` callers named at 517 and 1761 (Comprehension #1).
- **Fix:**
  - Every non-GET handler runs its module call in `db.WithTx` (read-write), and commits before writing the response. Otherwise a failed commit after a 201 breaks Goal 6.
  - Bootstrap, `GET /reminders/due`, the timer and export use a read-only `REPEATABLE READ` transaction.
  - Other GET handlers pass the pool.
- **What this simplifies:**
  - `CompleteSetup` takes `q`, and stops opening its own tx.
  - Item 25 lists the four read-only callers.
- **Test:** add a rollback test to item 51.

**MA2. Goal 7 fails against the starter content (Design M1).** Fix with option (b) only.
- Goal 7 gets one sentence: starter content (`internal/starter`) is the one exception, because it is editable data she can rename or archive.
- Line 2341 already treats `internal/starter/` that way.
- Option (a), a starter table per country, is YAGNI.

**MA3. "Activar notificaciones" (Comprehension #4, Design N10 line 149).**
- CUJ 1 step 5 becomes "She taps 'Activar notificaciones' and then Allow."
- `push.ts` reports "needs activation".
- "Hoy" and `estado` show the button.
- One mechanism covers both the first permission and re-subscribing.

## 🟢 Nit (needed, cheap)

- N1: `media.client_id NOT NULL`; drop the CHECK; `Media.client_id` required; line 593 wording.
- N2: drop `last_push_date?` from `Status`; show `last_push.date`. The key rename is optional.
- N3: the timer records nothing if the count fails, and retries the next minute (289).
- N4: remove `exporter` from the `reminders` importers (340, 1328).
- N5: delete `GET /settings` (668, 1591).
- N6: photo files are written and synced before the row is inserted (618). No orphan cleanup.
- N7: line 1967 says "token as written".
- N8: item 17 goes in the P1 and P2 rows.
- N9: the come-back "Shown until" cell gets "or 60 days after its due day".
- N10: phone search strips non-digits first; add one test in item 78.
- N11: line 20 says "tells her when to ask a client to come back for a refill (relleno)".
- N12: "the client's" at 482, 626, 1563 and 1750.
- N13: line 479 says "a chip in the text box".
- N14: soften the logout sentence (the HTTP cache may keep photos). No Clear-Site-Data.
- N15: Goal 5 says "at most 25 h apart".
- N16, one-word fixes:
  - 43: the colon outside the bold;
  - 158: "SINPE Móvil";
  - 188: "and two other items are due";
  - 218: "'Enviar' and 'Omitir'";
  - 289: a pointer to "Due-list computation";
  - 291: one sentence;
  - 471: the repeat deleted;
  - 477: an unmatched `{{` is rejected;
  - 528: the history wording;
  - 637: the "rebook skip" wording;
  - 681: "without `q`" deleted;
  - 712: the Ajustes path;
  - 728 and 731: `<owner>` becomes `jnaranj09`;
  - 1057: "November 19, 2026";
  - 1431: ADRs 018–025.

## Turned down

| Proposal | Why |
| --- | --- |
| Stage 5 N2: drop export paging | Reopens an owner decision kept in two rounds. |
| Stage 5 N6: move the shell flags to items 85 and 86 | Moving reviewed text risks drift and removes nothing. |
| Design M1 (a): a starter table per country | Adds scope. |
| Design N3: log a send after its "before" item expired | A new path for a few-minute edge case. Accept the risk. |
| Design N6: restore temp space | The temp restore runs before `dropdb`, so it already fails safely. |
| Design N8: `createImageBitmap` memory | Optional; at most add a full-resolution camera photo to the manual check. |
| Design N9: Excel phone display and formula injection | `datos.json` is the faithful copy; the data is her own. |
| Design N10 (503): show the count of disappearing items | Needs a dry-run count; adds scope to the prompt. |
| Structural M1: a concrete con for React | "Author preference" is honest; at most add "accepted trade-off: a smaller ecosystem". |
| Structural N2, N4 | Template formatting; no gain for the reader. |
| Style M2, N7, N9, N10 | Style churn; N9 conflicts with the author's own flow rule. |
| Comprehension #8, #13 | Already clear. |
| Structural N1, N3 | Optional. |

## Simplest combined change

1. MA1: the transaction rule at line 337, `CompleteSetup(q)`, item 25, and the item 51 rollback test.
2. MA2: one sentence in Goal 7.
3. MA3: one "Activar notificaciones" button.
4. Deletions:
   - the media CHECK;
   - `GET /settings`;
   - `exporter` from the `reminders` import lists;
   - `Status.last_push_date`;
   - "without `q`";
   - the repeated "she never types it".
5. One-clause additions:
   - timer retry;
   - the file write order;
   - the 60-day limit;
   - phone digits;
   - 25 h;
   - logout.
6. The wording fixes and N16.
7. Change nothing else.
