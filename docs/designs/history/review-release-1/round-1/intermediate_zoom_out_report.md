# Goldfish Review Report: Intermediate Zoom Out (Stage 8)

**Target:** `docs/designs/release-1.md` (lines 1-1879)

**Prior findings read:** `zoom_out_report.md` (Stage 5), `comprehension_report.md` (Stage 6) and `critic_report.md` (Stage 7). The document has not been edited since those reports.

**Code reuse check:** not possible. This is a greenfield design, and no codebase exists.

## Reviewer Status

| Reviewer | Status |
| --- | --- |
| `egm-zoom-out` | completed |

**Severity breakdown:** 0 Blocker, 9 Major, 7 Nit.

**Overall:** Stages 6 and 7 found real gaps. Most of the fixes they suggest would make the design bigger: outbox hardening, a due window that covers several days, a skip rule based on `created_at` plus a "historical" flag, faster backup polling, a full Risks table, a gloss for every ID, and a daily snapshot of the due list. Most of these findings come from a small number of features. Cutting or merging those features closes several findings at once.

## Prioritized Findings (Blockers -> Major -> Nits)

### 1. Drop sending while offline, so offline mode is read-only everywhere - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 88, 98, 186-189, 305, 391, 500, 513, 516, 522, 1658, 1667, 1729, 1764
* **Related:** Critic M10, M11 and N33. Comprehension 2 and 9. Stage 5 #10 (this finding gives it more weight).
* **Discrepancy Description**: The reminder-log outbox is the only thing the app writes while offline. It is the cause of several findings:
  * M10: on a new day the cached list is stale, and "before" items can still show after the appointment has started.
  * M11: the outbox can be lost when the cache schema is reset, or it can retry forever when the server rejects a replay.
  * Comprehension 9: it breaks Goal 6, the promise that the app never shows a change as saved when it was not.
  * Comprehension 2: the design does not say what the cached list shows on a new day.

  Each suggested fix adds new work: send several days of items with `shown_until`, keep the outbox out of the schema reset, call `navigator.storage.persist()`, and add a new conflict target. The feature also gives little benefit. Offline sending mostly helps only when the server goes down in the middle of a day.
* **Rationale & Recommendation**:
  * While offline, make "Para enviar hoy" read-only: show the list from the last refresh with "Datos de HH:MM", and disable "Enviar" with the existing "Necesitas conexión" message.
  * Delete the outbox, the optimistic-UI exception, the step in `client.ts` that sends queued items after a successful request, and the outbox tests.
  * For M10, add one display rule: hide cached "before" items whose `starts_at` has passed.
  * Generate IDs on the server. Use a plain `ON CONFLICT DO NOTHING` on the log's unique key. This replaces the reliance on `ON CONFLICT (id)` and also handles a double tap.
  * Keep the `late` flag. It covers catching up after the server was off.

### 2. Use one "not before creation" rule instead of two separate guards - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 448-449, 463-464, 472, 1556-1566
* **Related:** Critic M4 and M5.
* **Discrepancy Description**: The Critic suggests two mechanisms. The first skips a "before" item when `created_at > starts_at - offset`. The second is a "historical entry" flag for visits copied from the notebook, which needs a new column, new UI and onboarding steps.
* **Rationale & Recommendation**: Use one condition for every rule kind: generate an item only if its due day is on or after the local day of the appointment's `created_at`. This one condition handles both cases, needs no flag, and uses a column that already exists. Also consider allowing only one business-wide "before" rule in release 1, instead of "one or more" (line 448). With only one rule, one booking cannot send two messages (M4).

### 3. Make `upgrade.sh` run the backup directly, and remove the request file and the migration guard - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 193-199, 654-681, 1027, 1304, 1309, 1835-1847
* **Related:** Critic M7, M8, M9, N32 and N33. Comprehension 8. Stage 5 #11 and #13 (raised from Nit to Major).
* **Discrepancy Description**: Five findings come from one design. That design is the `request-now` file, the hourly loop, the 10-minute poll, and a migration guard in `serve` that reads the backup container's `status.json`. The findings are:
  * M7: the backup request is not picked up in time, and the app ends up down.
  * M8: a rollback runs the migrations forward again.
  * Comprehension 8: the guard condition is worded in two ways.
  * N32: "last successful backup" is ambiguous.
  * Stage 5 #11: the guard ties the two containers together.

  The suggested fixes add even more moving parts.
* **Rationale & Recommendation**:
  * Make `upgrade.sh` run the backup synchronously (for example `docker compose exec backup /backup.sh --now`). It stops if the backup fails, and runs `up -d` only after the backup succeeds.
  * Delete `request-now`, the polling, the guard in `serve`, `ALLOW_MIGRATE_WITHOUT_BACKUP` and the guard tests. The status screen still reads `status.json`.
  * `restore.sh` takes the target version as a required argument and sets `APP_VERSION` before it starts the app. This fixes M8.
  * For M9, state that the backup disk is always mounted from boot. If a USB disk that is plugged in later must be supported, add the one-line `bind.propagation: rslave`.
  * For N33, accept in one sentence that the photos and the database dump can be slightly out of step. Media files never change and are keyed by ID, so an orphan file does no harm.
  * If a guard is still wanted, keep it with one documented condition.

### 4. Remove `come_back_hour`, and do not add a setting for the 20:00 cutoff - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 260, 319-320, 449, 1583, 1750
* **Related:** Stage 5 #1, Comprehension 1, Critic M3 and N28. All three stages raised this.
* **Discrepancy Description**: The suggested fixes keep `come_back_hour` and add constraints or special cases. N28 also pushes toward a new setting for the cutoff.
* **Rationale & Recommendation**: Remove `come_back_hour`, so come-back items are due from the start of their local day. Treat 20:00 as a fixed quiet-hours rule of the product, not as business data. Add `CHECK (push_hour < 20)`. State that the day is recorded even when the count is 0.

### 5. Remove the daily "completed" tidy job - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 261, 395, 564-572, 1498, 1584
* **Related:** Stage 5 #7 and #9 (this finding gives them more weight). Critic N17 and N27.
* **Discrepancy Description**: N27 shows that the timer stops a completed appointment from being cancelled. It may also cause unexpected 412 errors if the timer changes `version`. The suggested fixes add a new state transition and new rules for versions.
* **Rationale & Recommendation**: Never store `completed`. Derive the done state from `IsDone`. Remove `TidyCompleted`, its `scheduler_state` row and its tests. This closes N27 and N17. Dropping `If-Match` (Stage 5 #9) also removes the 412 risk.

### 6. Cut `owner_task` instead of explaining it - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 375-382, 386, 446-450, 470, 498, 567-568, 1726-1727
* **Related:** Stage 5 #12 (raised from Nit to Major). Comprehension 6 and 13. Critic M5 and N12.
* **Discrepancy Description**: The suggested fix adds a purpose, an example and a user journey. "After" rules are unclear partly because two rule kinds share the name.
* **Rationale & Recommendation**: Ship two rule kinds and rename "after" to `come_back`. Remove `task_text`, "Tareas", "Hecho" and the `done` action. Add it back later only if user interviews show a need.

### 7. Make local search the only search - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 308, 345-346, 521, 554-558, 594, 1323, 1471, 1481
* **Related:** Stage 5 #5 (this finding gives it more weight). Critic M6.
* **Discrepancy Description**: M6 shows two defects in server search:
  * An empty `digits(q)` matches every client.
  * At the default trigram threshold, "ana" does not match longer names, so the example in CUJ 3 can fail.

  The suggested fixes would leave two search engines that must give the same results.
* **Rationale & Recommendation**: Cut server search: the extensions, `f_unaccent`, the GIN indexes and the `q` parameter. Search on the phone with a simple substring match on normalized text and digits.

### 8. Cut the SAD rewrite and the PRD edit list instead of glossing every ID - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 36-37, 79, 106, 771, 1154-1218, 1879
* **Related:** Stage 5 #4 (this finding gives it more weight). Comprehension 3, 4, 5 and 15. Critic N1.
* **Discrepancy Description**: If every FR, NFR and ADR ID gets a gloss, items 12 and 13 would grow into a third document inside this one.
* **Rationale & Recommendation**:
  * Replace item 13 with a link and an ADR index table (number, title, status, link).
  * Shrink item 12 to one sentence that says to update the PRD to match this document. The ID-by-ID edits belong in the PRD commit.
  * Define Phase 2 and Phase 3 in one line each, or replace "Phase 3 gate" with "after release 1".
  * Remove "EGM Step 6" from item 11, and mark item 11 as Modify.

### 9. Measure Goal 3 with checks that already exist, without a daily snapshot - Severity: Major
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 72-77, 86-87, 1018, 1868, 1874
* **Related:** Critic M13.
* **Discrepancy Description**: M13's fix adds stored data only to measure a goal. The due list is computed when it is read, and the same input always gives the same list. It can only miss items while the server is off, and those days are already excluded from the goal. So the 95% figure measures almost nothing.
* **Rationale & Recommendation**: Replace the percentage with the checks the document already has: the bullets backed by `due_test.go`, and the weekly comparison at line 1874. If a number is still wanted, write the count the daily push already computes to a log line. Define "Hoy opens" as the first render from the cache.

### 10. Risks section: a short list of accepted risks, not a mitigation table - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 110-118, 472, 664, 889, 895, 1011-1029
* **Related:** Critic M1 and M12.
* **Rationale & Recommendation**: Add a short "Accepted risks" H2 with one line per risk, each linking to the existing text. Include the author as a single point of failure. Add no new mitigations. A table with Likelihood and Owner columns invites new features such as a second login factor, password reset, or Cloudflare Access, which D2 already rejected.

### 11. Ley 8968: a go-live check, but no consent or deletion features - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 118, 717, 1016
* **Related:** Critic M12.
* **Rationale & Recommendation**: Add one item to the go-live checklist, answered by one consultation. Design nothing for it until that answer requires it.

### 12. Remove `testclock`, and have the setup handler do the `starter` package's job - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 265-301, 1412, 1434-1442, 1772, 1779-1782
* **Related:** Comprehension 7. Stage 5 #16.
* **Rationale & Recommendation**: Cut `testclock`, and create e2e test data relative to the real clock. Have the setup handler call the existing create functions of each module inside one `db.WithTx`. Then no `starter` package and no exception to the module rules are needed.

### 13. React alternative: drop the unmeasured size claim - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 987-989
* **Related:** Critic M14.
* **Rationale & Recommendation**: Give the reason as author preference plus less code per screen, and delete the 40 KB figure. Do not run a build experiment to measure it.

### 14. Shrink the NFR section to links - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 1009-1029
* **Related:** Critic M15 and N5. Stage 5 #14.
* **Rationale & Recommendation**: Make each NFR bullet one line that links to its section in the Technical Plan. Drop the claim about "one or two indexed queries".

### 15. Export: rely on the CSV library, and leave archived attributes out of the CSV - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 615-620
* **Related:** Critic N31.
* **Rationale & Recommendation**: Go's `encoding/csv` already quotes values that contain commas. Leave archived attributes out of the CSV, because `datos.json` keeps everything.

### 16. Push health: replace a status row instead of adding one - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out`
* **Affected Scope:** Lines 507-508, 627
* **Related:** Critic N29.
* **Rationale & Recommendation**: Change "last push date" to "last push delivered", using the latest `last_ok_at`. This shows a broken push subscription without new fields.

## Net Effect

Adopting findings 1-8 removes these parts of the design:
* the outbox
* `come_back_hour`
* the tidy job
* `owner_task`
* server search
* the backup request file and the migration guard
* `testclock` and `starter`
* most of items 12 and 13

This closes about 20 prior findings. The only additions are:
* the due-day condition based on `created_at`
* the rule that hides cached "before" items after the appointment starts
* a version argument for `restore.sh`
* a CHECK on `push_hour`
