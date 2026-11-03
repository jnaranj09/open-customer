# Zoom Out Report: Stage 5 (round 4)

Target: `docs/designs/release-1.md`, full file (lines 1–2308).
Reviewer: `egm-zoom-out`, in a fresh context. Standard: `zoom-out-standard.md`.

Verdict: approve with changes. No Blockers, 5 Majors, 5 Nits.
- There is no code to reuse and no root README to align with. Alignment was checked against the doc's own priorities and the PRD.
- Strong points: the due list is computed when read, "done" is computed, search is phone-only, there is one daily push, offline is read-only, and the non-goals are tight.

Synthesizer notes:
- #2 reopens the owner's round-3 R17 decision.
- #6 reopens the owner's D5.1 decision (option trees of any depth).
- #7 goes against the round-3 finding that sqlc `:many` loads every row into memory.

## 🟡 Major

1. **The oapi-codegen strict server causes a chain of workarounds** (341, 1463–1472, 1588, 1646, 1706–1707).
   - Strict mode forces all of these:
     - headers declared in the spec;
     - the `X-App-Version` caveat;
     - the P1 multipart/binary spike, with a hand-written fallback;
     - two error hooks;
     - the body-limit wrapping.
   - Fix: generate `std-http-server` without `strict-server`. Handlers get `(w, r)` plus typed params. The TS client is unchanged. Or state why strict mode is worth the cost.
2. **The "Solo de hoy en adelante / Incluir atrasadas" prompt adds a lot for a rare case** (416, 495–498, 1542, 1561, 1946, 1964, 2172).
   - The cost: a column, a request field required only on some edits, a list of triggering edits, a sheet, and tests.
   - It also asks a non-technical user a question about how rules work.
   - Fix: always behave as "Solo de hoy en adelante" with no question. Add the choice later if needed.
3. **The Technical Plan still repeats the implementation items.** Six topics are written twice:
   - the timer;
   - the cookie;
   - the backup steps and `status.json`;
   - upgrade and restore;
   - offline detection;
   - `PGDATA` and volume ownership.

   Fix: keep what and why in the Technical Plan; put the steps, paths and flags only in the items.
4. **Photos are a full subsystem with no goal, CUJ or check** (405–407, 602–607, 1896–1905, 2095).
   - Photos are in the PRD (FR-C3, FR-P4), but no goal or CUJ verifies them.
   - Fix: add a CUJ ("attach a photo of the finished set") and fold photos into Goal 2's check. Or defer photos until the interview with her shows a need.
5. **The per-package depguard allow-list is an ongoing maintenance cost** (323–339, 1300). Fix: enforce only the coarse rules in lint (no domain → `httpapi`/`gen`; `auth` imports no domain module). Keep the detailed list as guidance.

## 🟢 Nit

6. Cap option depth at 2 (her example is shape → length; the brief said one sub-level).
7. Export paging is an early optimization at this size.
8. "avisa a <owner>" puts install data in a translation file. Use generic text, or a settings value.
9. Repo housekeeping (items 11–14, 87) takes up space in the design. Shrink it to one item.
10. `POST /client-errors` is extra surface with little payoff for one user. Optional; defer it.
