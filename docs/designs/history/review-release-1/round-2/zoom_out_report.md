# Zoom Out Report — Stage 5 (round 2)

Target: `docs/designs/release-1.md`, lines 1–2088 (full file).
Reviewer: `egm-zoom-out` (fresh context). Standard: `zoom-out-standard.md`.
GoldfishContext: none beyond files the doc links (the reviewer read `concepts_brief.md`).

Codebase reuse: the repo has no code yet, so reuse of existing code could not be checked. The reviewer checked alignment with the brief instead.
Context sufficiency: passes.

Verdict: no Blockers. The read-time due list plus `reminder_log` fixes the root problem in its structure, not with a patch. 4 Major, 7 Nit.

---

### 🟡 Major 1: Option trees of any depth bring back a feature the brief deferred
- **Scope:** lines 360–362, 432–433, 441, 999–1003, 1279, 1647, 1655, 1731, 1893.
- **Rationale:** The brief allows "at most one level of sub-options, only if she needs it" (concepts_brief.md:106) and lists "deep option trees" as a non-goal (:128). The doc's Open Questions (line 124) leave the attribute shape to the interview.
  - Release 1 still ships nesting to any depth.
  - That brings a self-referencing `parent_id`, cycle checks, a `parent_id`-never-changes rule, paths that go through archived parents, path joins for CSV and templates, a drill-down picker, and tests for all of it.
  - Alternative I1 argues that depth does not change how data is stored. But the cost is in validation, rendering, export and the UI, not in storage.
  - Line 1002 itself says the main example (shape → length) is "really two flat attributes".
- **Fix:** ship flat select and multiselect. Add one level only if the interview shows she needs it.
  - Values store only option IDs, so adding depth later needs no data migration.
  - The doc already uses the same argument for the "group" type (line 1108).

### 🟡 Major 2: Two client-search implementations, which goes against the doc's own E8 reasoning
- **Scope:** lines 331–332, 370–371, 540, 547, 582–592, 644, 1488–1495, 1512, 1662, 1672–1676, 1878, 1966.
- **Rationale:** `/bootstrap` already sends all active clients to the phone (line 540). Offline search runs on that cache.
  - Online search uses a separate server path: `pg_trgm`, `unaccent`, an IMMUTABLE wrapper function, two GIN indexes, `word_similarity` ordering, LIKE escaping, and paged `GET /clients`.
  - Both paths must return the same set, so the rules are tested twice (items 48 and 78).
  - E8 rejects rendering templates in the browser because "two renderers would drift". The same reasoning applies to search.
- **Fix:** search active clients only from the cache, online and offline, with one TS implementation and one test set.
  - Keep a plain server query only for `archived=true`.
  - That drops both extensions, the wrapper function, the trigram indexes and the paging for client lists.

### 🟡 Major 3: The Technical Plan drops to implementation detail in several places
- **Scope:**
  - search SQL clauses (587–591);
  - the service-worker update handshake (624);
  - the precache list and the SvelteKit fallback quirk (549);
  - distroless volume ownership (691);
  - fstab `nofail` and `rslave` (737);
  - exporter paging SQL (667);
  - Argon2 parameters (776);
  - the order of the setup wizard and icon upload (556).
- **Rationale:** these mechanics are also in, or belong in, Detailed Plan items 35, 38, 59, 65, 81 and 85.
  - Writing them twice makes the architecture section hard to scan.
  - Both copies then have to be kept in sync.
- **Fix:** keep the decision and its reason in the Technical Plan. Move the mechanics to the numbered items.

### 🟡 Major 4: The device-sessions screen has no stated need
- **Scope:** 351 (public `sessions.id`), 565, 630 (`GET /sessions`, `DELETE /sessions/{id}`), 1368 (the `Session` schema), 1595, 1956 (`ajustes/dispositivos`).
- **Rationale:** one user has about two sessions, and no Goal or CUJ needs this screen.
  - A lost phone is already handled: a password change revokes the other sessions (1596), and `reset-password` revokes all of them.
  - The screen costs a second ID on sessions, two endpoints, a schema and a settings page.
- **Fix:** drop it from release 1 and add it back with staff accounts.

---

### 🟢 Nit 1: `users.role` exists only for staff accounts (line 350)
- The CHECK allows only one value.
- Staff accounts are a non-goal.
- Adding the column later takes a one-line migration.

### 🟢 Nit 2: The per-username delay map is built for many users (lines 564, 1610)
- The 10k entry cap with drop-oldest eviction is more than one user needs.
- Keep the per-IP limiter and a simple backoff that expires.

### 🟢 Nit 3: Export paging may be premature (lines 667, 1515)
- 500-row pages inside one REPEATABLE READ transaction is an unmeasured optimization for a few thousand rows.
- Streaming one query per table is enough.

### 🟢 Nit 4: "Come-back time" setting is not defined (line 1951)
- The `negocio` screen lists it, but no such field exists in `business_settings`, the `Settings` schema or the rules.
- Come-back items are due from the start of the day (line 473).

### 🟢 Nit 5: The coverage-gate list differs between sections (lines 1146–1147, 1188, 2070)
- The Verification Plan gates `internal/auth`; the build-order exit checks do not.
- Use one list. `make cover` could also be folded into `make test-db`.

### 🟢 Nit 6: The `/bootstrap` transaction is opened in the HTTP layer (lines 319–320, 1537–1538)
- That layer is described as having no business logic.
- Every module's read function then has to accept a transaction handle, and the module rules do not say so.
- Either say that in the module rules, or move bootstrap into a read-model function, as `exporter` does.

### 🟢 Nit 7: The doc drops `create-owner` without saying so (lines 567, 1039–1043)
- The brief listed a `create-owner` CLI (concepts_brief.md:105).
- Say that `setup-link` replaces it, so the PRD update (item 12) does not carry it over.

---

### Positive notes
- The read-time due list plus `reminder_log` has no queue, no worker and no stored "completed" status.
- The doc uses standard libraries instead of custom code: `nyaruka/phonenumbers`, `webpush-go`, goose, sqlc, restic and `archive/zip`.
- The non-goals list matches the stated priority order.
