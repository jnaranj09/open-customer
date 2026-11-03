# Goldfish Review Report

**Target:** `docs/designs/release-1.md` (lines 1-1052, from the top through the end of "## Non-Functional Requirements")

Stage 7 (Critic Review), one round, REVIEW mode. Nothing was resolved. The main agent consolidates later.

## Reviewer Status

| Reviewer | Status |
| --- | --- |
| `egm-design` | completed |
| `egm-structural` | completed |
| `egm-style` | completed |

## Severity Breakdown (after deduplication)

| Severity | Count |
| --- | --- |
| Blocker | 0 |
| Major | 15 |
| Nit | 33 |

How the findings were merged:
- The design and structural reviewers both flagged the missing Risks & Mitigations section. That is one finding here.
- The structural reviewer (Major) and the style reviewer (Nit) both flagged the H3 headings at lines 1033 and 1040. That is one finding here, kept as Major.
- The structural and style reviewers both flagged the wrong section name at line 37. That is one Nit.
- The structural and style reviewers both flagged the NFR section repeating earlier text. That is one Nit.

The synthesizer checked these lines against the target: 260, 319-320, 449, 558, 657, 665, 1003-1053.

---

## Prioritized Findings (Blockers -> Major -> Nits)

No Blockers were reported.

### M1. No Risks & Mitigations section - Severity: Major
* **Reviewer Lens:** `egm-design`, `egm-structural`
* **Affected Scope:** Whole document. The risk content is spread across lines 110-118, 472, 664, 889, 895 and 1011-1029.
* **Discrepancy Description**: The feature-spec standard requires a Risks & Mitigations area. Today, risks are split across Open Questions, "accepted risk" notes inside Alternatives (lines 889 and 895), and the NFR bullets. Some risks are never stated anywhere:
  - The home PC and the author are a single point of failure. Only he can reset passwords, handle deletions, upgrade, restore, or reissue the setup link.
  - The app is on the public internet with password-only login and no second factor.
  - The tunnel is a path into the home network.
  - Reminders older than the 60-day look-back drop off (line 472).
  - If RESTIC_PASSWORD is lost, the backups cannot be read (line 664).
  - Both backup copies sit at home, so one event can destroy both.
  - The design depends on Cloudflare's free plan and its upload limits.
  - The bootstrap payload grows with the number of clients (line 514).
* **Rationale & Recommendation**: Add a `## Risks & Mitigations` H2 after Technical Plan or after the NFRs. Use a table with columns Risk / Likelihood / Impact / Mitigation or explicit acceptance / Owner, and pull the scattered items into it.

### M2. "Additional Resources" and "Future Improvements" are nested under the NFRs - Severity: Major
* **Reviewer Lens:** `egm-structural`, `egm-style`
* **Affected Scope:** Lines 1033, 1040
* **Discrepancy Description**: Both headings are H3 and come after `## Non-Functional Requirements` (line 1009). The horizontal rule at line 1031 does not end the parent section, so the outline files both under the NFRs.
* **Rationale & Recommendation**: Make both of them H2.

### M3. With default settings, the daily push never counts come-back items - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 260, 319-320, 449, 843
* **Discrepancy Description**: `push_hour` defaults to 8 and `come_back_hour` defaults to 9. Come-back items only count as due from `come_back_hour`, so the single push at 8:00 leaves them out.
  - On a day that has only come-back items, the count is 0 and no push is sent.
  - On other days, the number in the push is too low.

  Line 843 justifies push with exactly this case ("Come-back reminders often land on quiet days").
* **Rationale & Recommendation**: Pick one of these:
  - Count come-back items due today regardless of the hour.
  - Require `come_back_hour <= push_hour`.
  - Leave `come_back_hour` out of the push count.

### M4. "Before" reminders fire for appointments booked after their due moment - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 448, 463
* **Discrepancy Description**: Take a same-day booking made at 10:00 for 15:00 under the 24 h rule. Its due moment has already passed, so it shows up at once as "late". If a 2 h rule also exists, the client gets two reminders for a booking she just made.
* **Rationale & Recommendation**: Add a skip condition, for example `created_at > starts_at - offset_hours`. Put it in the rule table and in the Goal 3 checks.

### M5. Backfilled past appointments flood the come-back list - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 449, 464, 472
* **Discrepancy Description**: Under the done rule, any past appointment counts as done. So history entered from her notebook during onboarding instantly creates late come-back and owner-task items, even for clients who have since come back but whose visits are not recorded in the app.
* **Rationale & Recommendation**: Skip these items when `created_at` is after the item's due day, or add a "historical entry" flag that suppresses them.

### M6. Server search returns unrelated clients and misses short name matches - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Line 558; CUJ 3 at line 150; line 521
* **Discrepancy Description**: The search has two problems:
  - For a name-only query, `digits(q)` is empty, so `phone_digits LIKE '%' || '' || '%'` matches every client that has a phone.
  - `%` uses a similarity threshold of 0.3 by default. "ana" against "ana maria rodriguez" scores below that, so the CUJ 3 example can fail.
* **Rationale & Recommendation**:
  - Apply the phone clause only when the query contains at least 3-4 digits.
  - Use `word_similarity` (`<%`), or a prefix/ILIKE match, for short names.
  - Make the offline search at line 521 follow the same rules.

### M7. `backup-now.sh` cannot reliably trigger the hourly loop, and `upgrade.sh` does not stop when it fails - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 654-657, 665, 672-674
* **Discrepancy Description**: The backup loop checks for `request-now` once an hour, but `backup-now.sh` waits at most 10 minutes. So the request is usually not picked up before the timeout.
  - `upgrade.sh` does not say it stops when the backup fails. It goes on to `up -d`.
  - The migration guard then refuses to start, and the app stays down.
* **Rationale & Recommendation**:
  - Have the loop poll for `request-now` every few seconds, or have `backup-now.sh` signal the container directly.
  - Keep the hourly cadence only for the 24 h check.
  - Make `upgrade.sh` abort before `up -d` if the backup did not succeed.

### M8. Rollback can re-apply the migration it is meant to undo - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 675-681, 1028
* **Discrepancy Description**: `restore.sh` starts `app` at its step 5. If `.env` still holds the new `APP_VERSION`, the new binary sees pending migrations. The backup was just taken, so the guard passes and the restored database is migrated forward again.
* **Rationale & Recommendation**: Have `restore.sh` take the target `APP_VERSION` and set it before step 5, or ask the operator to confirm the version before starting `app`. Also state that data entered after the upgrade is lost on rollback.

### M9. The backup container may not see the USB disk being mounted or unmounted later - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 193, 642, 656
* **Discrepancy Description**: Docker bind mounts use private propagation by default. A USB disk mounted on the host after the container starts is invisible inside it, so the marker check fails until the container restarts. CUJ 7 and Goal 5 depend on this disk being reachable.
* **Rationale & Recommendation**: Use `bind.propagation: rslave` on `/target`, or document that the container must be restarted. Add a "disk not visible" hint to the status screen.

### M10. The offline "Para enviar hoy" list is the list from the last refresh - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 74, 185-189, 470, 514
* **Discrepancy Description**: The cached list uses "today" as of the last refresh.
  - Items that became due since then are missing.
  - It can still show "before" items for appointments that have already started or were cancelled. That breaks the Goal 3 check "never shown after the appointment has started".
* **Rationale & Recommendation**:
  - Have the server send items for the next N days, each with its due day and its `shown_until` time.
  - Have the phone filter them by its own clock.
  - Label offline items with the time of the last refresh.

### M11. The offline outbox can be lost or get stuck retrying - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 305, 391, 500, 513, 516
* **Discrepancy Description**: Two problems:
  - Line 305 says inserts use `ON CONFLICT (id) DO NOTHING`. A replay with a new id but the same `(rule_id, appointment_id, anchor_at)` hits a different unique key and raises a violation. If that comes back as a 5xx, the fetch wrapper reads it as "server off", retries forever, and shows a false banner.
  - Line 513 clears Dexie when the cache schema changes. That drops unsent "Sí, lo envié" confirmations, so she may send the same WhatsApp message twice.
* **Rationale & Recommendation**:
  - Use `ON CONFLICT` on the unique key, or a target-less `ON CONFLICT DO NOTHING`, and return 200 or 409.
  - Keep the outbox out of the schema reset, or flush it first.
  - Request `navigator.storage.persist()`.

### M12. Ley 8968 and the data-processor question are not tied to the go-live gate - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 118, 717, 1013, 1016
* **Discrepancy Description**: The open question covers only deletion. It leaves out:
  - Cloudflare decrypting traffic as a third-party processor.
  - Deleted data staying in backups for 14 days.
  - Client consent for storing their data and photos.

  Nothing stops real data from being loaded before the question is answered.
* **Rationale & Recommendation**: Make the Ley 8968 answer a gate before real data is loaded, like the restore drill, and widen the question to cover the points above.

### M13. Some goal checks cannot be measured as written - Severity: Major
* **Reviewer Lens:** `egm-design`
* **Affected Scope:** Lines 73, 79, 87 (and 1018)
* **Discrepancy Description**: Two checks have no clear way to measure them:
  - "More than 95% of due reminders appear on their due day" has no data source. The due list is computed when it is read, and `reminder_log` records only her actions.
  - "Hoy opens in under 1 s over LTE" is ambiguous when the screen renders from the cache first.
* **Rationale & Recommendation**: Say how each check is measured. For example, have the timer record the due list or its count every day. Define "opens" as either the first render from the cache or fresh data arriving from the server.

### M14. The React rejection rests on a size figure nobody measured - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Lines 988-989
* **Discrepancy Description**: Line 988 gives "About 40 KB more ... (from memory, not measured)". Line 989 then uses the smaller first load as a reason for the decision.
* **Rationale & Recommendation**: Measure the difference with two minimal builds and compare the gzipped JS, or drop the size argument and keep only reasons that can be checked.

### M15. An NFR contradicts the design body - Severity: Major
* **Reviewer Lens:** `egm-style`
* **Affected Scope:** Line 1020 vs lines 459-466, 514
* **Discrepancy Description**: Line 1020 says "every screen is one or two indexed queries". But `DueList` runs settings and rules queries plus a four-group batch, and `/bootstrap` loads eight data sets.
* **Rationale & Recommendation**: Restate it, for example: "a fixed number of indexed queries per request (no N+1); the largest is `/bootstrap` at about N". Or drop the query count and rely on Goal 6.

---

### Nits

Structure:
- **N1** (`egm-structural`, `egm-style`), line 37: "see Detailed Implementation" should be the exact heading "Detailed Implementation Plan", preferably as an anchor link.
- **N2** (`egm-structural`), lines 56-61: "Proposed solution, in short" sits under "The Problem". Move it to Overview or to the top of Technical Plan.
- **N3** (`egm-structural`), line 208: rename "Technical Plan (Architecture)" to "Proposed Architecture" to match the standard.
- **N4** (`egm-structural`), lines 1003-1005: "Left open" is not an alternative and repeats line 1045. Remove it, or rewrite it with Pros, Cons and Reason for Rejection like the other entries.
- **N5** (`egm-structural`, `egm-style`), lines 714-719 vs 1011-1016, and 667-682 vs 1025-1029: the NFRs repeat the Security summary and the Upgrades text almost word for word. Shorten each NFR bullet and link back to its subsection.
- **N6** (`egm-structural`), line 307: the `sql` fence holds shorthand, not valid DDL. Relabel it `text` or add a note that it is shorthand.
- **N7** (`egm-structural`), lines 140 and others: the CUJ "When" steps are formatted in two ways. Use a numbered list whenever there is more than one action.

Style:
- **N8** (`egm-style`), lines 26, 85: "behave in a clear way" and "It feels fast" are subjective. Replace them with the measurable behavior.
- **N9** (`egm-style`), line 51: "Heavy to run" and "weak at custom fields" have no support. Give the specific reason.
- **N10** (`egm-style`), lines 50, 730: the same claim is hedged as "usually cannot" in one place and "mostly cannot" in the other, with no source. Use one wording and name the products.
- **N11** (`egm-style`), line 514 vs 869: the payload size is an estimate at line 514 but a fact at line 869. Mark both as estimates, or measure it.
- **N12** (`egm-style`), lines 386, 567-568: "after rules" and "after items" are never defined. Define them next to the table at line 446.
- **N13** (`egm-style`), document-wide: several concepts have more than one name. Come-back, fill and relleno are one concept. "Before" and `before_appointment` are one. "Server off", "offline" and "PC is off" are used interchangeably. Add a glossary or pick one term for each.
- **N14** (`egm-style`), line 166 vs 189 (and 178): the button is "Sí" in CUJ 4 and "Sí, lo envié" in CUJ 6. Line 178 also switches from "she" to "the owner". Use one of each.
- **N15** (`egm-style`), lines 104, 927: the non-goal is conditional on the interview. Make it unconditional and track the interview under Open Questions.
- **N16** (`egm-style`), line 114: "A reviewer reported" names no source. Link the finding or the changelog.
- **N17** (`egm-style`), lines 261, 395: "tidy" and "last tidy date" are vague. Name what is actually kept in sync.
- **N18** (`egm-style`), line 298: "small Go interface" does not say what small means. Delete "small" or define the limit.
- **N19** (`egm-style`), line 501: "a one-line change" predicts code that does not exist yet. Say the switch is contained in one function.
- **N20** (`egm-style`), line 792: "The review found most reminder bugs" has no count or source. Give N of M and link it.
- **N21** (`egm-style`), lines 817, 843, 849, 863: these rejection reasons are unchecked claims about her business or are opinions. Tie each one to the interview or to a stated priority.
- **N22** (`egm-style`), line 883: "a long support window" is vague. Give the PostgreSQL 18 end-of-life date.
- **N23** (`egm-style`), line 959: "too important to risk" is a judgment. State the consequence: a ban would cut off her main channel to clients.
- **N24** (`egm-style`), line 967: "approved" does not say who approved it or when.
- **N25** (`egm-style`), lines 995, 1000: "faster to style with" and "reads as generic" are opinions. State the actual trade-off.
- **N26** (`egm-style`), line 1024: the due list does not "catch up", because it is computed when read. Say that missed items appear with the `late` flag.

Design:
- **N27** (`egm-design`), lines 261, 567-569: once the daily timer has marked a past appointment completed, it can no longer be cancelled, so what she can do depends on when the timer ran. Also, it is not stated whether the timer's update bumps `version`, which could cause a confusing 412. Allow `completed -> cancelled` and state how the timer handles `version`.
- **N28** (`egm-design`), lines 260, 319-320: the 20:00 cutoff is hardcoded, which goes against Goal 7. If `push_hour` is 20 or later, no push is ever sent. `come_back_hour` has no CHECK constraint.
- **N29** (`egm-design`), lines 507-508, 622-631: if the push subscription expires, push stops with no visible sign. Show the subscription count and `last_ok_at` on the status screen.
- **N30** (`egm-design`), lines 84, 205-206, 611: exporting a ZIP from the installed iOS PWA is unverified. Add it to the iPhone test in Open Questions.
- **N31** (`egm-design`), lines 206, 414, 615-616: an archived attribute and its replacement can share a label, which gives duplicate CSV columns. Decimal values with a comma (such as "12,5") need quoting.
- **N32** (`egm-design`), lines 220 vs 640, 527 vs 689, 657: three small inconsistencies.
  - Line 220 uses `postgres:18` but line 640 uses `postgres:18-alpine`.
  - Line 527 adds `https://` to `PUBLIC_URL`, which already includes it.
  - Line 657 says "last successful backup" without saying whether that means the local dump or the USB copy.
- **N33** (`egm-design`), lines 658-659 and 514: two smaller issues.
  - `pg_dump` and the media snapshot are taken at different times, so a restore can have mismatched photos. Document this or handle orphans.
  - The app does a full refresh on every focus, including each return from WhatsApp. Debounce it, or skip it while a confirmation is pending.
