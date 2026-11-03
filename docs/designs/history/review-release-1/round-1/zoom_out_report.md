# Goldfish Review Report: Zoom Out (Stage 5)

**Target:** `docs/designs/release-1.md` (lines 1-1879)

## Reviewer Status

| Reviewer | Status |
| --- | --- |
| `egm-zoom-out` (instance A) | completed |
| `egm-zoom-out` (instance B) | completed |

Both instances read the full target, the zoom-out persona and standard, and `concepts_brief.md`. Neither read the SAD or `goldfish_brainstorm_findings.md` in full. Instance A searched the PRD for a few points. There is no codebase yet, so neither instance could run the code-reuse check.

**Severity breakdown:** 0 Blocker, 8 Major, 10 Nit.

**Overall:** The objective is clear. The overview, problem, priorities and goals match the brief. The core architecture is lean: one binary, a "Para enviar hoy" list worked out when it is read, a once-a-minute timer instead of a queue, read-only offline and four containers. Most remaining weight comes from features built "just in case", from two ways of doing the same thing, and from too much low-level detail. One Major finding (1) is a real correctness bug.

The synthesizer checked the line references for findings 1 and 3 against the target.

## Prioritized Findings (Blockers -> Major -> Nits)

### 1. The separate come-back hour makes the daily push miss today's come-back reminders - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (A; B raised `come_back_hour` as an unneeded setting)
* **Affected Scope:** Lines 260, 319-320, 449, 843, 1750
* **Discrepancy Description:**
  * `push_hour` defaults to 8 (line 319) and `come_back_hour` defaults to 9 (line 320).
  * The timer counts today's due items once, at push time (line 260).
  * Come-back items only count as due from `come_back_hour` (line 449).
  * With the defaults, the 8:00 push never includes come-backs due that day. The push is sent once a day, so these reminders are never announced on their due day. On a day with nothing else due, no push goes out at all.
  * This undoes the stated reason for the push (line 843): come-back reminders often fall on quiet days.
* **Rationale & Recommendation:** Remove `come_back_hour`. Make come-back items due from the start of their local day. If an earliest send time is really needed, use `push_hour` for both kinds.

### 2. Sub-options are built before the interview that decides whether they are needed - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (A, B)
* **Affected Scope:** Lines 104, 112, 336-337, 409, 1231, 1456, 1464, 1693
* **Discrepancy Description:**
  * The non-goals (line 104) and Open Questions (line 112) say the interview decides whether sub-options are needed.
  * The design builds them anyway:
    * `attribute_options.parent_id`
    * a select value that is either a string or a `[parent, child]` array
    * one-level checks
    * an OpenAPI `oneOf`
    * a two-step picker
    * tests
  * The array-or-string value makes the validator, renderer, CSV export and API schema more complex. It also changes the stored JSON shape, and the brief warned that rewriting stored JSON later is costly.
  * The brief's own example (shape, then length) turned out to be two flat attributes.
* **Rationale & Recommendation:** Ship a flat `select` (one option ID) and `multiselect`. Remove `parent_id`, the array form, the two-step picker and their tests. Add sub-options in a later migration only if the interview shows a need.

### 3. `create-owner` and `setup-link` are redundant, and `create-owner` leaves a broken install - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (A, B)
* **Affected Scope:** Lines 251-253, 527-528, 1262-1264, 1412, 1426-1427
* **Discrepancy Description:**
  * There are three ways to create the first user:
    * the setup link that `serve` logs on every start while no user exists (line 527)
    * the `setup-link` subcommand, which repeats that link
    * `create-owner`, for "installs without the setup link" (line 251). That case is not tied to any goal or user journey.
  * Only `CompleteSetup` creates the `business_settings` row and seeds the starter content (line 1412). Setup is refused once a user exists.
  * So an install made with `create-owner` has a user but no settings row. The app relies on that row for time zone, currency and country code.
* **Rationale & Recommendation:** Remove `create-owner`. Remove `setup-link` too, unless restarting the app to get a new link is not acceptable. Keep `reset-password`, which fills a real gap.

### 4. Two architecture documents that must be kept in sync - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (A, B)
* **Affected Scope:** Lines 36, 1154-1158, 1181-1218
* **Discrepancy Description:**
  * The plan rewrites the SAD in Spanish as `docs/sad.md`. It copies the data model, API and reminder sections "at summary level" and adds ADRs 018-025.
  * It then requires every design change to update both files in the same commit (lines 1157, 1213).
  * For one maintainer (priority 4, line 33), that means two architecture documents in two languages to keep in step, and they will drift.
  * Line 36 already says this design replaces the SAD's architecture parts.
* **Rationale & Recommendation:** Make `release-1.md` the only architecture source. Reduce `docs/sad.md` to a short pointer and an ADR index (number, title, status, link to the design section), or retire it. Keep the PRD update, which covers the "what" and "why".

### 5. Server-side client search duplicates the phone's local search - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (B)
* **Affected Scope:** Lines 308, 345-346, 514, 521, 554-558, 594, 1323, 1471, 1481, 1678, 1742
* **Discrepancy Description:**
  * `/bootstrap` sends every active client to the phone (line 514), and the phone already searches them without accents (lines 521, 1678).
  * The design also builds a server search:
    * `pg_trgm` and `unaccent`
    * an `f_unaccent` wrapper
    * two GIN indexes
    * `GET /clients?q=`
    * a database test
  * The design never says which screen uses the server search.
  * The two searches can return different results. For example, trigram matching at the default threshold can miss a short query like "ana" against a long full name.
* **Rationale & Recommendation:** Make local search the only search. Remove the extensions, the wrapper, the indexes and the `q` parameter. If archived clients must be searchable, add a simple `ILIKE` later. If server search stays, say which screen uses it and why local search is not enough.

### 6. `/bootstrap` plus separate list endpoints give two read paths for the same data - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (B)
* **Affected Scope:** Lines 298, 462, 514, 589-602, 1348-1349
* **Discrepancy Description:**
  * `/bootstrap` returns settings, attributes, services, templates, rules, clients, the appointment window and the due list.
  * The API table also lists separate GETs for most of the same data. Each needs a schema, a generated stub, a handler and tests, and nothing in the design uses them.
  * The bootstrap handler builds one cross-module `pgx.Batch` (line 1349). That conflicts with two rules in the design: handlers hold no business logic (line 1348), and modules expose small interfaces (line 298).
  * The batch, and the one in DueList (line 462), optimize a few small queries against a database on the same machine.
* **Rationale & Recommendation:** Keep `/bootstrap` as the only bulk read. Keep only the GETs that a screen or user journey actually uses, such as client history outside the window, media, the reminder log, status and export. Have bootstrap call module read functions one after another, and drop both `pgx.Batch` uses.

### 7. The stored "completed" status and the daily tidy job duplicate the derived "done" rule - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (A)
* **Affected Scope:** Lines 261, 438-442, 569, 1498, 1584
* **Discrepancy Description:**
  * Line 261 says the tidy job "only keeps stored data tidy" and that reminder logic never depends on it. "Done" is worked out by `IsDone` (lines 438-442).
  * The tidy job still adds a timer duty, a status row written by the system, `TidyCompleted` and scheduler tests.
  * The result is two meanings of "completed" that can disagree for up to a day, plus code whose output no logic reads.
* **Rationale & Recommendation:** Stop the timer from writing `completed`. Show "realizada" in the UI and export using `IsDone(a, now)`. Add a stored status later if a real need appears.

### 8. Goal 7 forces internationalization work for an English UI that is out of scope - Severity: Major
* **Reviewer Lens:** `egm-zoom-out` (A, who noted it could also be a Nit with the same fix)
* **Affected Scope:** Lines 90, 105, 315, 711-712, 1442, 1672
* **Discrepancy Description:**
  * Goal 7 says no Spanish-only value lives in code (line 90).
  * That brings in a `locale` column (line 315), a locale package ready to "add `en`" (line 711), a starter-content map keyed by locale (line 1442) and a lint rule against inline Spanish (line 1672).
  * An English UI is a non-goal (line 105), and "generic" is priority 5.
* **Rationale & Recommendation:** Limit Goal 7 to business data: time zone, currency, country, branding and catalog. Keep strings in `es.json` as a convention. Drop the `locale` column and the lint rule until a second language is actually needed.

### 9. Version checks on edits (`If-Match` / 412) for a single-user app - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A, B; B leaned toward Major)
* **Affected Scope:** Lines 344, 358, 574, 1228, 1470, 1484
* **Discrepancy Description:**
  * There is one owner, no staff accounts and no offline edits. A conflict could only happen between her Safari tab and the installed app.
  * Even so, every client and appointment PATCH carries a version. The SPA must track versions in Dexie, and 412 errors need their own UI copy and tests.
  * The brief listed 412 as in scope, so this was a deliberate choice.
* **Rationale & Recommendation:** Use last write wins for release 1, and add version checks together with staff accounts. A `version` column is easy to add later. If the checks stay, add one line that says which conflict they prevent.

### 10. Phone-generated IDs for creates, built for deferred offline creates - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A)
* **Affected Scope:** Lines 305, 857 (and the reminder log's unique key at 391, 500)
* **Discrepancy Description:**
  * Offline creates are a non-goal.
  * The only queued write is the reminder log, which its own unique key already de-duplicates.
  * Accepting client-made IDs with `ON CONFLICT (id) DO NOTHING` on every insert is justified only by "keeping G1 possible later" (line 857).
* **Rationale & Recommendation:** Have the server generate IDs in release 1. The UUIDv7 column already allows client IDs if offline creates arrive later.

### 11. The migration backup guard ties app startup to the backup container, and the missing-USB case is not stated - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A, B)
* **Affected Scope:** Lines 199, 656, 674, 1304, 1846
* **Discrepancy Description:**
  * `serve` reads the backup container's `status.json` and refuses to migrate without a dump from the last 60 minutes. That couples two containers through a shared file.
  * `backup.sh` skips the dump entirely when the USB marker is missing (line 656). So an upgrade with the USB disk unplugged leaves her without the app until the author steps in, which hurts priority 1.
  * `upgrade.sh` already takes a backup and stops if it fails (line 1846).
  * The design does not say whether this fail-closed behavior is intended.
* **Rationale & Recommendation:** The reviewers disagree on the fix:
  * A: rely on `upgrade.sh`, or turn the guard into a logged warning plus a status-screen alert, or allow a one-time override.
  * B: keep the guard, since data safety is priority 2, but state that a missing USB disk blocks upgrades on purpose, or let the local dump run even when the USB copy cannot.
  * Either way, the design should state the intended behavior.

### 12. The `owner_task` rule kind has no user journey or goal check - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A, B)
* **Affected Scope:** Lines 375-382, 450, 470, 498, 1726-1727
* **Discrepancy Description:**
  * This third rule kind traces back to PRD FR-R2.
  * It adds a `task_text` column, a "Tareas" group, a "Hecho" action, a `done` log action and template rendering.
  * None of the 8 user journeys and no goal check uses it.
* **Rationale & Recommendation:** Confirm in the interview that she has a concrete use for it. If not, ship two rule kinds and add `owner_task` later. That only needs a new CHECK value and a nullable column.

### 13. Backup-now uses a request file and polling - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A)
* **Affected Scope:** Lines 657, 661, 665, 1845
* **Discrepancy Description:**
  * Backup-now works indirectly: it writes a `request-now` flag file, the hourly loop picks it up, and the script polls `status.json` for up to 10 minutes.
  * The hourly loop waits for the next hour, so a manual request might not be picked up within the 10-minute poll.
* **Rationale & Recommendation:** Have `backup-now.sh` run the backup directly and wait for it to finish, for example with `docker compose exec backup /backup.sh --now`. Keep the hourly loop for routine runs.

### 14. The Technical Plan and Implementation Plan repeat each other and go down to code-level detail - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A, B)
* **Affected Scope:**
  * Repeated topics:
    * scheduler: 257-263 and 1578-1590
    * backups and upgrade: 654-681 and 1831-1847
    * status screen: 624-631 and 1597-1606
    * Argon2 parameters: 718 and 1396
  * Code-level detail: 534, 549, 1418-1419, 1458, 1549, 1692, 1782, 1829
* **Discrepancy Description:**
  * The same behavior is described twice at slightly different levels of detail, so the copies can drift apart.
  * Several entries go down to function signatures and tuning constants, for example:
    * the `last_seen_at` throttle
    * `createImageBitmap` options
    * rate-limiter burst and idle cleanup
    * the `Record(...)` and `Validate(...)` signatures
    * a 300 ms debounce
    * `apk add restic jq`
  * Line 1782's `go list -tags ''` check does not prove the test-clock endpoint is missing from the binary.
  * At 1879 lines, the document is costly to keep current.
* **Rationale & Recommendation:** Make the Technical Plan the single description of behavior, and have implementation items say "as in section X". Move constants and signatures into code comments or one short appendix.

### 15. An optional VPS/Caddy profile ships although self-hosting for others is deferred - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A)
* **Affected Scope:** Lines 101, 106, 644, 1795, 1819, 1821-1824
* **Discrepancy Description:** Self-hosting for other people is moved to the Phase 3 gate (line 106). Release 1 still ships a commented-out Caddy service, a Caddyfile and a README section that the reference install never uses or tests.
* **Rationale & Recommendation:** Leave all of it out until Phase 3, and mention the option in one sentence.

### 16. Small extras that serve no release-1 goal - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A, B)
* **Affected Scope:**
  * cursor pagination: 583, 594
  * free-space reporting: 629, 1603
  * `/readyz`: 606, 1602
  * Ajustes, Dispositivos: 537, 590, 1755
  * e2e test-clock endpoint: 1772, 1779-1782
* **Discrepancy Description:**
  * Cursor pagination covers a list that bootstrap sends in full and that search caps at 20.
  * `syscall.Statfs` free-space reporting is not tied to any goal.
  * Nothing calls the `/readyz` probe; Compose only uses `/healthz`.
  * The session-management screen is for one user on one phone.
  * The test-clock endpoint adds a route close to production code, plus a CI guard to protect it.
* **Rationale & Recommendation:**
  * Drop pagination, `/readyz` and the Dispositivos screen. Logout plus `reset-password` covers a lost phone.
  * Keep free-space reporting only if the author really watches it.
  * For e2e tests, seed data relative to the real clock. The `DueList` unit tests already use a fake clock (lines 474, 1552).

### 17. The template parser still handles iOS curly quotes, though she never types the syntax - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (B)
* **Affected Scope:** Lines 283, 422-427, 433, 1526, 1536, 1689-1692
* **Discrepancy Description:**
  * The brief needed curly-quote tolerance because she was going to type template syntax on an iPhone.
  * In this design, chips are turned into syntax by code (line 1692), and the fallback comes from a plain field (line 433). The lenient-quote parsing and its tests are left over from the earlier design.
  * `ChipEditor` also relies on `contenteditable` with inline chips on iOS Safari. That is one of the hardest UI parts to make reliable, and it serves one screen.
* **Rationale & Recommendation:**
  * Parse only what the serializer writes.
  * Consider storing template bodies as structured segments (text, and variable plus fallback) instead of a string that needs a tokenizer.
  * Consider a simpler editor, such as a list of text and chip blocks, instead of `contenteditable`.

### 18. Implementation item 11 marks the design file as "Create" although it already exists - Severity: Nit
* **Reviewer Lens:** `egm-zoom-out` (A)
* **Affected Scope:** Line 1154
* **Discrepancy Description:** `docs/designs/release-1.md` is listed as "Create", but the file already exists.
* **Rationale & Recommendation:** Change it to "Modify".
