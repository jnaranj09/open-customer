# Comprehension Report — Stage 6

*   **Target:** `docs/designs/release-1.md` (FeatureRange 1–1052, DetailedRange 1053–1879)
*   **Reviewer:** `egm-comprehension` — completed
*   **Severity breakdown:** 0 Blocker, 7 Major, 8 Nit

## What this proposes

Release 1 of open-customer is a self-hosted app for one non-technical nail-studio owner who works from her iPhone. It has one Go binary with an embedded SvelteKit web app (installed on the Home Screen as a PWA), PostgreSQL 18, cloudflared and a backup container. They run in a single Compose stack on the author's home PC, which is reached through a Cloudflare Tunnel and is sometimes switched off. She sets up her own services, custom client and appointment attributes, and message templates. The server works out, when asked, which WhatsApp reminders are due today ("before the appointment" and "come back after N days"). She sends each one by hand through a `wa.me` link and confirms it, which writes a reminder log. One push notification per day tells her how many messages are waiting. When the server is off, the app is read-only from a phone cache, and the only write it queues is the "sent" confirmation. Backups are a daily `pg_dump` plus restic to a USB disk, and the app refuses to run database migrations without a recent backup. The Detailed Implementation section lists about 88 files to create, from repo tooling and CI through every Go module, web route and deploy script, and also rewrites and moves the PRD and SAD into `docs/`.

## How the existing system works, per this document

There is no software yet. Today she keeps clients, appointments and reminders in WhatsApp chats, a notebook and her memory, so reminders depend on her remembering. Before this document there were a PRD and a SAD. The SAD's design used a stored reminders queue with a background worker, reminder rules per service, come-back reminders started by a manual "Completar" tap, Caddy with Let's Encrypt, and a deeper attribute model. This document replaces the SAD's architecture for release 1 and lists the PRD changes needed to match it.

## Findings

### 1. `come_back_hour` vs the daily push — Severity: Major
*   **Affected scope:** lines 159-161, 260, 320, 449, 1583.
*   **Description:** Come-back items count as due only from `come_back_hour`, which defaults to 9. The push fires at `push_hour`, which defaults to 8, and counts the items due at that moment.
    *   With the defaults, the 8:00 push never counts that day's come-back items.
    *   On a quiet day where only come-backs are due, the count is 0 and no push goes out.
    *   The document never says whether today's date is still recorded when the count is 0, so a reader cannot tell whether a later tick would send the push after 9:00.
*   **Rationale:** This works against the stated reason for having push at all (Alternative F1). The document never explains why `come_back_hour` exists or how it relates to `push_hour`.

### 2. What offline "Para enviar hoy" shows on a new day — Severity: Major
*   **Affected scope:** CUJ 6 (lines 180-189), lines 511-523, Alternative E8, item 74.
*   **Description:** The due list is rendered only on the server, and the phone caches the last list. On a new day with the server off, the document does not say what that list contains:
    *   yesterday's items, including late ones;
    *   or nothing that became due today.
*   **Rationale:** It also does not say whether the phone hides a cached "before" item once `starts_at` has passed. If it does not, she could send a reminder after the appointment started, which breaks a Goal 3 check.

### 3. PRD changes given only as IDs — Severity: Major
*   **Affected scope:** lines 1159-1179 (item 12), line 1048.
*   **Description:** Item 12 refers to FR-A2, FR-A3, FR-A6, FR-R1, FR-R7, FR-R8, FR-C4, FR-I1 to FR-I5, NFR-1, NFR-4, NFR-5 and NFR-9 by number only.
*   **Rationale:** A reader cannot check the planned changes without opening the PRD. Each ID needs a short gloss.

### 4. ADR table uses numbers only, and its sections contradict each other — Severity: Major
*   **Affected scope:** lines 763, 775, 1187-1205 (item 13).
*   **Description:**
    *   The kept ADRs and the superseded ADRs are listed by number, with no titles.
    *   "009/015 kept" sits under the "Superseded" heading.
    *   010, 016 and 017 show no replacement ADR.
*   **Rationale:** A reader cannot tell which decisions are kept and which are replaced.

### 5. Phase 2 and Phase 3 are never defined — Severity: Major
*   **Affected scope:** lines 79, 106, 771, 979-981, 1047, 1177.
*   **Description:** Several items are deferred "to the Phase 3 gate", but the document never says what the phases or their gates are.
*   **Rationale:** A reader cannot judge when the deferred items come back.

### 6. `owner_task` has no stated purpose or example — Severity: Major
*   **Affected scope:** lines 375, 380, 446-450, 470, 498, 726.
*   **Description:** No CUJ, goal or example covers `owner_task`.
*   **Rationale:** A reader can see how it works but not why it exists.

### 7. `starter` and `testclock` packages vs the module rules — Severity: Major
*   **Affected scope:** lines 265-301 vs items 40, 43, 80.
*   **Description:**
    *   `internal/starter` and `internal/testclock` are missing from the code layout.
    *   `starter.Seed` writes into three other modules' tables.
    *   `auth` calls `starter`, which looks like it breaks the rule that modules do not import each other's internals.
*   **Rationale:** A reader cannot tell whether this is allowed or a violation of the design's own rules.

### 8. Migration guard condition worded two ways — Severity: Nit
*   **Affected scope:** line 674 vs line 1304.
*   **Description:** The Technical Plan says "the database is not empty". The detailed plan says "any user exists".
*   **Rationale:** These are different conditions. Pick one.

### 9. Goal 6 vs the optimistic reminder-log outbox — Severity: Nit
*   **Affected scope:** lines 85-88 vs line 522.
*   **Description:** Goal 6 says a change is never shown as saved when it was not. The reminder-log outbox shows an offline "Sí" tap as sent before the server has it.
*   **Rationale:** State the exception in the goal, or a reader will see a contradiction.

### 10. Sample client described as seeded data — Severity: Nit
*   **Affected scope:** CUJ 2 (line 139) vs items 43, 53.
*   **Description:** CUJ 2 lists the sample client as starter content. Items 43 and 53 show it lives only in code (`sample.go`).
*   **Rationale:** Say plainly that it is not stored, and never appears in the client list or the export.

### 11. `postgres:18` vs `postgres:18-alpine` — Severity: Nit
*   **Affected scope:** line 219 vs line 640.
*   **Description:** The diagram and the Deployment table name different images.
*   **Rationale:** A reader cannot tell which image the data-path open question applies to.

### 12. Undefined local terms — Severity: Nit
*   **Affected scope:** lines 139, 576, 1179, 1042.
*   **Description:** SINPE Móvil, "manual asistido", "bandeja" and "slot mapping" are not defined in this document.
*   **Rationale:** They are specific to the project or to Costa Rica, and SINPE Móvil ships as a starter option.

### 13. "After rules / after items" undefined — Severity: Nit
*   **Affected scope:** lines 386, 567-568.
*   **Description:** The phrase is never defined.
*   **Rationale:** From context it means `come_back` + `owner_task`. Say so once.

### 14. "The review" and "the SAD bug" cited without context — Severity: Nit
*   **Affected scope:** lines 792, 1556.
*   **Description:** Neither reference is named or explained where it appears.
*   **Rationale:** One clause of explanation in each place makes them readable on their own.

### 15. "EGM", item 11 marked "Create", and stray report files — Severity: Nit
*   **Affected scope:** line 1157, line 1879.
*   **Description:**
    *   "EGM" and "Step 6" are not explained.
    *   Item 11 marks the file being read as "Create".
    *   No item covers the review reports in the repo root (for example `zoom_out_report.md`).
*   **Rationale:** A reader cannot tell what these references mean or what happens to the report files.
