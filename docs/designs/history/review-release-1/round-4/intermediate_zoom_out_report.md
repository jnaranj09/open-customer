# Intermediate Zoom Out Report: Stage 8 (round 4)

Target: `docs/designs/release-1.md`, full file (lines 1–2308).
Reviewer: `egm-zoom-out`, in a fresh context. It read the Stage 5, 6 and 7 reports.
Standard: `zoom-out-standard.md`. The owner's earlier decisions were given as constraints.

Verdict: approve with changes. No Blockers, 8 Majors, 13 Nits.
- About half of the proposed fixes are needed.
- About a quarter add scope or overturn the owner's decisions; turn those down.
- Most of the rest is best fixed by deleting or merging text.

## 🟡 Major

1. **Turn down the fixes that reopen decisions.**
   - The fixes:
     - Stage 5 #2 (drop the overdue prompt);
     - Stage 5 #6 (depth cap);
     - design-4-1 (offline "Enviar" and a cached tomorrow list);
     - design-4-5 (offline "+" and a draft list).
   - Each one overturns D1, D5.1 or R17. design-4-1 would also break Goal 6.
   - The loss in design-4-1 is narrower than the critic says. A "before" item stays listed (late) until the appointment starts, and today's appointments are cached, so she can still message the client directly.
2. **Keep the "¿Lo enviaste?" confirmation when iOS closes the app (design-4-2), in its small form.**
   - Store the pending item ID in Dexie, show the sheet on any app open, and clear it on Sí or Todavía no.
   - Skip the one-hour window, the "Abierto en WhatsApp" label and a killed-app e2e test; make it a manual check.
3. **Keep the badge in step with the list (design-4-3).** Call `setAppBadge(due.length)` or `clearAppBadge()` after each refresh and each log POST. Add one Vitest case.
4. **Handle downtime with wording, not features.**
   - Goals 3 and 4: days with the PC off don't count as misses and don't break the 4-week run.
   - Accepted risks:
     - a "before" reminder can be missed if the PC stays off until the appointment starts;
     - silence on a quiet day can also mean the PC is off, and the banner tells them apart.
   - No zero-count push.
5. **Generate the plain `std-http-server`, not `strict-server` (Stage 5 #1).**
   - This deletes:
     - the spike;
     - the header declarations;
     - the `X-App-Version` caveat;
     - one of the two error hooks.
   - Handlers get `(w, r, params)`.
   - Cost: Go response bodies are no longer type-checked at compile time. Handler tests and e2e cover that.
6. **Photos: make them testable or defer them, but decide (Stage 5 #4).**
   - The cheapest fix is one photo step in CUJ 3 plus a manual iPhone check.
   - If the interview shows she doesn't use photos, defer them. That removes the media table, three endpoints, the module, `photos.ts` and the media part of backup and restore.
7. **Owner vs author: delete text** (Stage 6 #1–2, style-4-2).
   - Delete the "round-3 review" sentence (1022) and "the author approved the change" (1120).
   - Line 1080 says "she asked for".
8. **Remove repeats only where they can drift (Stage 5 #3).**
   - Six topics, each with one owner and a pointer from the other place:
     - Backups vs item 85;
     - Upgrades and restore vs item 86;
     - timer vs item 58;
     - cookie vs items 34 and 39;
     - `PGDATA` and volume ownership vs items 81 and 84;
     - offline detection vs item 66.
   - The Technical Plan keeps the rules and reasons; the items keep paths and flags.

## 🟢 Nit

- G1: apply the naming rule (5 word edits).
- G2: merge items 11, 14 and 87 into one "move docs and rewrite links" item.
- G3: alternative labels:
  - M1 and H4 get "Author preference:";
  - A5, I1 and K2 use "Outcome:".
- G4: drop "avisa a <owner>" and `backup_contact_name`; show "Respaldo con problema".
- G5: lint only the coarse depguard rules (no domain → `httpapi`/`gen`; `auth` no domain; who may import `reminders` and `starter`). Keep the per-package list as guidance.
- G6: make `include_overdue` optional, defaulting to "Solo de hoy en adelante". This removes the conditional 422; the phone still asks.
- G7: add "Bot Fight Mode off" to the Cloudflare settings.
- G8: one sentence: a rollback drops data entered after the upgrade, so upgrade outside her working hours.
- G9: `autocapitalize="off"` and `autocomplete` attributes on the inputs; the server trims and lowercases the username.
- G10: a 0-day item gets no push when the appointment starts before the push hour. One sentence and one test row.
- G11: leave as is: export paging, `POST /client-errors`, no VPS entry.
- G12: mechanical fixes:
  - "PRD FR-R7", plus a Terms line for FR/NFR IDs;
  - "largest come-back `offset_days`";
  - the Pins row lists all five Postgres places;
  - the `text` code fence;
  - blank lines before lists;
  - Cloudflare stated once;
  - the icon timing stated once;
  - line 594 trimmed;
  - the calendar analogy and line 829 deleted;
  - line 1016 points to 566;
  - J3 and J4 moved;
  - `docker compose logs app` in CUJ 1;
  - the PG 18 release date.
- G13: skip:
  - "Mitigation:" leads;
  - Open Questions deadline fields;
  - moving Additional Resources;
  - rewording every opinion;
  - making "unverified" / "not measured" the same everywhere.

## Simplest combined change

1. **Don't apply:**
   - Stage 5 #2 and #6;
   - design-4-1 offline Enviar;
   - design-4-4 always-push;
   - design-4-5 offline "+".
2. **Delete:**
   - strict-server and its workarounds;
   - the owner/author clauses;
   - "avisa a <owner>";
   - the per-package depguard list;
   - the duplicate sentences.
3. **Merge:**
   - items 11, 14 and 87;
   - the six repeated topics;
   - the alternative labels.
4. **Add (small):**
   - the Dexie pending confirmation;
   - the badge sync;
   - the downtime wording;
   - a CUJ 3 photo step;
   - optional `include_overdue`;
   - G7–G10.
5. **Mechanical:** G1 and G12.

Net effect: the doc gets shorter. The only new behavior is on the phone (the pending confirmation and the badge), and neither touches D1.
