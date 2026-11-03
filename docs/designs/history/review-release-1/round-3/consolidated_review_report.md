# Consolidated Review Report: release-1.md (round 3)

Target: `docs/designs/release-1.md` (2274 lines), after the round-2 fixes (C1–C27).
Mode: Consolidated report, all five stages, no extra context. Every reviewer ran in a fresh context.

## Stage reports

| Stage | Report | Result |
| --- | --- | --- |
| 5 Initial Zoom Out | `zoom_out_report.md` | 0 / 3 / 5 |
| 6 Comprehension | `comprehension_report.md` | 0 / 2 / 13 |
| 7 Critic (design, structural, style) | `critic_report.md` | 0 / 6 / 26 |
| 8 Intermediate Zoom Out | `intermediate_zoom_out_report.md` | 0 / 8 / 9 |
| 9 Implementation Readiness (IR, CD, style) | `implementation_readiness_report.md` | 2 / 19 / 34 |

Counts are Blocker / Major / Nit, before merging duplicates.

## Verdict

**Not ready for implementation, but close.**
- Round 2 had 3 Blockers spread across the whole plan. Round 3 has 2, and both are narrow:
  - the module rules miss some needed reads;
  - the binary path is not on PATH.
- The feature spec passes. Every reviewer could paraphrase it, and no design Blocker was found.
- Most Majors are contract gaps in the plan, not design problems.
- Two fixes from round 2 (my edits) introduced new problems:
  - C27 replaced export paging with "streaming", which sqlc cannot do (R4).
  - C16 declared `X-App-Version` in the spec, which breaks the middleware header (R6).
- Three items reopen or need an owner decision: R5, R8 and R17.

## Action items

### 🔴 Blockers

- **R1: Add the missing module edges.**
  - The module reads that are needed but not allowed:
    - `appointments` → `services`;
    - `templates` → `attributes`;
    - `reminders` → the attribute definitions and options;
    - `auth.CompleteSetup` → `settings` (inject a settings creator, like the seeder);
    - `status` → `push`, `scheduler`, the goose version (add `status.sql` or the read functions).
  - Mirror them in the depguard rules (item 5).
  - Sources: IR-B1, CD-M6.
- **R2: The binary path.**
  - `ENTRYPOINT ["/open-customer"]` is not on PATH, so the healthcheck and `upgrade.sh` step 1 fail.
  - Fix: install to `/usr/local/bin/open-customer`.
  - Source: CD-B1.

### 🟡 Majors

**Build and phases**
- **R3: P1 depends on P2.**
  - Split item 34 by phase: steps 1–4 in P1, steps 5–7 in P2.
  - Move `auth.SetupNeeded` and the `GET /setup/status` handler into P1.
  - Move the overlap test to P3.
  - `GET /bootstrap` goes in P4.
  - The release workflow builds the backup image only from P7.
  - Sources: IR-M1, CD-M11, CD-N13.

**Data and queries**
- **R4: Export paging.**
  - The plan contradicts itself (L675 vs item 29).
  - sqlc `:many` loads every row into a slice, so "streaming" is false.
  - Keep the 500-row paging and fix item 29. Stage 8 suggested deleting the paging; CD shows that paging is the option that works.
  - Sources: Stage 5 #2, Stage 8 #2, IR-M6, CD-M5.
- **R5: Search on the server and on the phone. This reopens D2 and needs the owner.**
  - Stages 5 and 8 propose phone-only search. It would drop `pg_trgm`, `unaccent`, `f_unaccent`, the GIN indexes, the server search and its DB tests.
  - IR shows the two copies can't rank the same way.
  - D2 (round 1) chose to fix the server search, so cutting it is a change of decision.
  - If the server search stays: define "starts a word" exactly, and set the offline order to word-start, then name. Also state the top-20 cap as the one exception to "no paging".
  - Sources: Stage 5 #1, Stage 8 #1, IR-M4, ST-M1.
- **R7: `pgx.Batch` on `DBTX`.**
  - The sqlc `DBTX` has no `SendBatch`.
  - Fix: run the `DueList` queries one after another on `q`, which reads one snapshot inside the tx.
  - Also state `type DBTX = sqlc.DBTX` in `db.go`.
  - Sources: CD-M4, CD-N2.
- **R10: Bootstrap includes archived attributes, options, services, templates and rules** (with `archived: true`). Clients are active only.
  - Source: IR-M5.

**Templates and reminders**
- **R8: Template data and preview. Part of this is the owner's call.**
  - Define `templates.Data` with values already formatted.
  - Add one `attributes.Format(def, options, raw, locale, target)`, called by both `reminders` and `exporter`.
  - State that `{{appointment.*}}` in a come-back message is the past visit.
  - Owner's call: preview against the sample client only (Stage 8: drop `client_id`/`appointment_id`, move `Preview` to `templates`), or also against a real client's latest appointment.
  - Sources: IR-M2, Comprehension #1, Stage 5 #5, Stage 8 #4.
- **R9: "Enviados" needs `client_id` and `client_name` in the log response.** `date` filters on `acted_at` as a local day.
  - Source: IR-M3.
- **R17: A new or edited come-back rule lists old visits as late. This needs the owner to decide the intent.**
  - (a) Document it as intended. The list is bounded by the 60-day look-back and by rebooking.
  - (b) Or add the rule's `created_at` to the created-day guard.
  - No new `effective_at` column.
  - Sources: design-3-1, Stage 8 #7.

**API contract (strict server)**
- **R6: Headers and errors.**
  - `X-App-Version` is set only in the middleware, not in the spec.
  - Declare `Content-Disposition` (export) and `Cache-Control` (media).
  - `problem.Write` sets `Retry-After`.
  - Mount with `StdHTTPServerOptions{BaseURL: "/api/v1", BaseRouter, ErrorHandlerFunc}`, so binding errors are problem JSON.
  - Sources: CD-M1, CD-M2, CD-M3.

**PWA**
- **R11: The manifest is not precached** (or is network-first), because the wizard changes it.
  - Source: CD-M7.
- **R15: Backup failure shows on "Hoy".**
  - One red line, "Respaldo con problema — avisa a <owner>", reusing the existing `red` value.
  - No author alert.
  - Sources: design-3-2, Stage 8 #5.
- **R14: CUJ 1 wizard order.** Username and password come first, then the optional icon (as in the design).
  - Sources: Comprehension #2, Stage 8 #3.

**Tooling**
- **R12: e2e database and pnpm.**
  - `make e2e` uses `E2E_DATABASE_URL` when it is set; CI sets it to the service container.
  - `pnpm/action-setup` gets `package_json_file: web/package.json`.
  - Sources: CD-M8, IR-N3, CD-M9.

**Docs**
- **R13: The item 12 PRD edit list.** Add FR-P3, FR-P5, NFR-3 and NFR-11.
  - Source: CD-M10.
- **R16: Rewrite the "she keeps her data through the export" risk honestly.**
  - Optionally, a sealed restic password goes into the handover checklist.
  - No nudge.
  - Sources: design-3-3, Stage 8 #6.
- **R18: Name the two reminder kinds once.** Then use only "before" and "come-back" in prose.
  - Source: ST-M2.
- **R19: Trim only the drifted Technical Plan paragraphs** (L554, L562, and the export).
  - Do not do a full move of the mechanics.
  - Sources: Stage 5 #3, Stage 8 #8.
- **R20: Downgraded to Nit by Stage 8.**
  - The `restore` service gets a one-line mention in Goal 8 and Deployment.
  - The kept SAD ADRs get one link to the ADR table in `history/sad-original.md`.
  - The M1 claims are deleted.
  - Sources: structural-3-1, structural-3-2, design-n4, style-3-1.

### 🟢 Nits

The full lists are in the stage reports. Groups:
- **Tooling and libraries:**
  - the `go.mod` deps;
  - the sqlc pointer override;
  - busybox `flock` and dates;
  - Dependabot compose;
  - `serviceWorkers: 'block'`;
  - golangci-lint paths;
  - debian13;
  - one `PG_IMAGE`;
  - the eslint and prettier configs.

  Sources: CD-N1, N3, N6, N8–N12; IR-N1.
- **Plan details:**
  - column types;
  - a recovery step and `MaxBytesReader`;
  - appointment input checks;
  - manifest fields;
  - fallback escaping;
  - the chip list source;
  - push count = `len(DueList)`;
  - half-open overlap;
  - icon checks;
  - color format;
  - CSV formats;
  - the export file name zone.

  Sources: CD-N4, N5; IR-N2, N4–N9.
- **Doc hygiene:**
  - item 14 vs the disk (CD-N7, Stage 5 #8);
  - define "item N" and P0–P7 in Terms;
  - delete the EGM entry;
  - the heading name at L283;
  - Goal 3 "four checks";
  - Goal 5 wording;
  - the 20:00 cutoff as a fixed value;
  - delete the cycle check;
  - interfaces only where a fake is needed;
  - the banner text "Sin conexión con el sistema";
  - error sheet wording;
  - interview questions (come-back reset);
  - delete the subjective phrases;
  - CUJ layout and blank lines;
  - the `text` code fence;
  - she/owner wording;
  - the spike deadline;
  - the ADR column sentence;
  - the `sad.md` rule stated once;
  - statement vs line coverage.

  Sources: Comprehension #3–15, structural-n1–n8, style-n1–n12, ST-N1–N12, Stage 5 #4, #6, #7, Stage 8 #9–#17.
- **Rejected by Stage 8 (do not do):**
  - `effective_at`;
  - an export nudge;
  - an author alert;
  - a "falta confirmar" state;
  - a full mechanics move;
  - a risks table;
  - linking every "item N";
  - reordering sections;
  - a VPS alternative.

## Traceability matrix

| Raw finding | → Item |
| --- | --- |
| IR-B1, CD-M6 | R1 |
| CD-B1 | R2 |
| IR-M1, CD-M11, CD-N13 | R3 |
| Stage 5 #2, Stage 8 #2, IR-M6, CD-M5 | R4 |
| Stage 5 #1, Stage 8 #1, IR-M4, ST-M1 | R5 |
| CD-M1, CD-M2, CD-M3 | R6 |
| CD-M4, CD-N2 | R7 |
| IR-M2, Comprehension #1, Stage 5 #5, Stage 8 #4 | R8 |
| IR-M3 | R9 |
| IR-M5 | R10 |
| CD-M7 | R11 |
| CD-M8, CD-M9, IR-N3 | R12 |
| CD-M10 | R13 |
| Comprehension #2, Stage 8 #3 | R14 |
| design-3-2, Stage 8 #5 | R15 |
| design-3-3, Stage 8 #6 | R16 |
| design-3-1, Stage 8 #7 | R17 |
| ST-M2 | R18 |
| Stage 5 #3, Stage 8 #8, style-n7 | R19 |
| structural-3-1, structural-3-2, design-n4, Comprehension #13, style-3-1 | R20 |
| All other -n / N findings | Nit groups |

---

## Resolution (2026-11-02)

The owner decided R5 (phone-only search), R8 (preview also against a real client) and R17 (ask "Solo de hoy en adelante" / "Incluir atrasadas", like a calendar's "this event or all"). The agent applied every other item with its recommendations, and most Nits. The edits are in `docs/designs/release-1.md`. The findings above refer to the document **before** these edits; their line numbers no longer match.

| ID | Outcome |
| --- | --- |
| R1 | New allowed imports: `appointments` → `services.GetMany`, `templates` → `attributes.List`, `reminders` → `attributes` (`List`, `Format`), `status` as an orchestrator calling `push.CountSubscriptions`, `scheduler.State`, `db.Version`. `auth` imports no domain module; `cmd` injects `settings.Create` and the seeder. Depguard: one rule per package. Modules are concrete types; interfaces only for fakes. |
| R2 | Binary at `/usr/local/bin/open-customer`. |
| R3 | P1 adds the `GET /setup/status` handler, `SetupNeeded`, middleware steps 1–5; P2 steps 6–8; the overlap test in P3; bootstrap in P4; the backup-image release step in P7. A panic-recovery step was added (CD-N5). |
| R4 | Export keeps 500-row keyset paging; item 29 corrected (sqlc `:many` cannot stream). |
| R5 | Server search removed: no `pg_trgm`, `unaccent`, `f_unaccent`, GIN indexes, `clients.Search` or DB search tests. `search.ts` is the only search; word-start defined; archived clients via `GET /clients?archived=true`. Alternative G5 records the rejected server search. PRD NFR-3 kept, met by local search. |
| R6 | `X-App-Version` only in middleware; `Content-Disposition` and `Cache-Control` declared; `Retry-After` from `problem.Write`; mount with `StdHTTPServerOptions{BaseURL, BaseRouter, ErrorHandlerFunc}`; body limits through `MaxBytesReader`. |
| R7 | No `pgx.Batch`; `DueList` runs its queries in one read-only `REPEATABLE READ` transaction. `type DBTX = sqlc.DBTX`. |
| R8 | Preview: sample client by default (with a placeholder per attribute type), or a real client with her latest active appointment. `client_id?` only. Preview stays in `reminders`, with the reason stated. `templates.Data` defined; `attributes.Format` is the one formatter for `reminders` and `exporter`. `appointment.*` meaning stated. Chip list sent as `template_vars[]`. Fallback text cannot contain `"`. |
| R9 | `LogItem` with `client_id`, `client_name`, `kind`; `date` filters `acted_at` as a local day; `LogForDate`. |
| R10 | Bootstrap includes archived attributes, options, services, templates and rules; clients active only. |
| R11 | The manifest is not precached or cached by the service worker. Manifest fields listed. |
| R12 | `make e2e` uses an existing `E2E_DATABASE_URL`; `pnpm/action-setup` with `package_json_file`. |
| R13 | PRD edits added for FR-P3, FR-P5, NFR-3, NFR-11 (IDs checked in the root PRD). |
| R14 | CUJ 1: username and password, then the optional icon. |
| R15 | `backup_red` in Bootstrap; a red line on "Hoy" links to the status screen. No author alert. A dead push subscription listed as an accepted risk. |
| R16 | The "one person and one PC" risk rewritten; the handover checklist gives her a sealed copy of `RESTIC_PASSWORD`. |
| R17 | `reminder_rules.applies_from`; `include_overdue` on create and on changes that can add items; the dialog in `recordatorios`; tests. |
| R18 | The two kind names defined in Terms; prose uses "before" and "come-back". |
| R19 | Offline-detection and service-worker paragraphs trimmed to what and why. |
| R20 | `restore` named in Goal 8 and Deployment; a link to the SAD ADRs for kept choices; the M1 claims removed. |
| Nits applied | Terms for the author, "item N", P0–P7 and "Enviados"; the cycle check removed; `Format` tests; Goal 3, 5 and 7 wording; the interview question on refill skipping; overlap boundary; push count = `len(DueList)`; appointment input checks; icon checks; color format; CSV date, phone and ZIP-name formats; `client-errors` limit; `go.mod` deps; column types; lint config files; Dependabot directories; the `flock` package check and jq time math; `serviceWorkers: 'block'`; items 14 and 87 match the disk; banner text; failed "Sí" wording; the spike deadline; the ADR column sentence; statement coverage; subjective phrases removed. |
| Nits not applied | The sqlc pointer syntax (CD-N3, not confirmed), golangci-lint paths (CD-N11), debian13 (CD-N12), she/owner wording in plan items (ST-N4), the `sad.md` rule repeated (ST-N8), section order and layout (structural-n1, n2, n5–n8), and the remaining subjective rationales (style-n4). Stage 8 advised against the reorder and layout changes. |
