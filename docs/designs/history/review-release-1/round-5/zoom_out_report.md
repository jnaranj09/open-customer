# Zoom Out Report: Stage 5 (round 5)

Target: `docs/designs/release-1.md`, full file (lines 1–2349). Reviewer: `egm-zoom-out`, fresh context. Standard: `zoom-out-standard.md`. Owner decisions given as constraints.

Zoom out (Stage 5, round 5): passes. 0 Blocker, 1 Major, 6 Nit. Context is sufficient, it targets the root problem, YAGNI is strong, and there is no code to reuse.

M1 (337, 646, 1757–1763): writes inside one module have no transaction. DBTX has no Begin, and handlers pass the pool, so multi-statement writes (appointment + appointment_services; PATCH service_ids delete+insert+update; clients duplicate check+insert; Validate + write) commit each statement separately. A crash midway leaves partial data. Fix: every non-GET handler wraps its module call in db.WithTx (read-write); GET handlers pass the pool. Add a test in item 37. Simplest change: yes.

N1 (408–410, 593, 617, 1538, 1567): media.client_id can be NOT NULL now that ownership always fills it; drop the CHECK; make Media.client_id required.
N2 (708, 1716): export paging is premature; use one :many per table in the same tx. [Reopens round-3 R4 and round-4's "do not defer export paging".]
N3 (289, 436, 716, 1545): last_push_date is written even on 0-count days, so it means "checked", yet the status screen shows it as "last push date". Fix: rename it push_checked_date; the status screen shows last_push.date; drop it from Status.
N4 (340, 1328): exporter does not need to import reminders; remove it from the allow list and depguard.
N5 (668, 1591): GET /settings has no caller; drop it or name a caller.
N6 (747–773, 797–801): shell-level flags in the Technical Plan Backups/Upgrades; move them to items 85/86. Optional.
