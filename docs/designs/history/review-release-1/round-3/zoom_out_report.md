# Zoom Out Report: Stage 5 (round 3)

Target: `docs/designs/release-1.md`, full file (lines 1–2274).
Reviewer: `egm-zoom-out`, in a fresh context.
Standard: `zoom-out-standard.md`.

Verdict: pass with changes. No Blockers, 3 Major, 5 Nit.
- The repo has no code yet, so there was nothing to reuse.
- The design already uses standard libraries (phonenumbers, x/text/currency, restic, archive/zip, Dexie, goose).
- The root fixes are sound: the due list is computed when read, and "done" is computed instead of stored.

## 🟡 Major

**1. Client search is built twice, on the server and on the phone.**
- Scope: L340–341, L379–380, L595–600, L1638–1645, items 47, 48, 69, 78.
- Rationale:
  - `/bootstrap` already sends every active client to the phone.
  - The server search still adds `pg_trgm`, `unaccent`, the `f_unaccent` wrapper, two GIN indexes, escaping rules and `word_similarity` ranking.
  - A JS copy of the search also runs offline. The two can't rank the same way, so L600 promises only the "same set".
- Fix:
  - Search only on the phone, over the cache.
  - `GET /clients` becomes a plain list with no `q`.
  - Drop the extensions, the wrapper, the indexes, the search clauses and the DB search tests.

**2. Export paging contradicts itself.**
- L675 says pages of 500 by `id`. L1665 says one streamed query per table and no paging.
- Fix: keep one streamed query per table and delete the L675 sentence.

**3. The Technical Plan repeats implementation detail.**
- Scope:
  - L281 (timer);
  - L554 (offline detection);
  - L562 (service-worker routing);
  - L575 (cookie);
  - L711–735 (backup steps and status table, again in item 85);
  - L743–763 (upgrade and restore steps, again in item 86);
  - L631–632 (version plumbing);
  - L675 (export paging).
- Rationale: every edit has to change both copies. #2 shows the copies have already drifted apart.
- Fix: keep 1–3 bullets of *what* and *why* in the Technical Plan. Move the step lists and tables into items 34, 65, 66, 85 and 86.

## 🟢 Nit

4. **The option cycle check can never fire** (L369–371, items 45–46). `parent_id` must already exist when an option is created and can never change, so a cycle can't happen. Drop the check and its test.
5. **Preview lives in `reminders`** (L467, items 32, 53). Say why in "Message templates", or make the shared loader its own named piece.
6. **One Go interface per module** (L325, item 32). That is 12 interfaces with one implementation each. Use concrete types, and add an interface only where a test needs a fake. Shrink the method table.
7. **The 20:00 cutoff is baked into the DDL CHECK** (L281, L353, item 58). Name it as a product constant in one place, or list it in Goal 7 as an accepted fixed value.
8. **Item 14 is out of date.** `round-1/` and `round-2/` both exist, and the root holds no reports. Reword it to "the next free round number".
