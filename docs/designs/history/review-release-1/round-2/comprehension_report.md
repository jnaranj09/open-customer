# Comprehension Report: Stage 6 (round 2)

Target: `docs/designs/release-1.md`. FeatureRange: lines 1–1113. DetailedRange: lines 1114–2088.
Reviewer: `egm-comprehension` (fresh context). No document standard.

## What this proposes

Release 1 of open-customer is a self-hosted app for one owner of a nail studio. She is not technical and uses an iPhone.

What she does in the app:
- She records clients and appointments.
- She records her own attributes, per client or per appointment, in five types, with option trees of any depth.
- She builds WhatsApp reminder templates from chips, so she never sees the template syntax.

How it is built:
- One Go binary with an embedded SvelteKit PWA.
- PostgreSQL 18 for storage.
- A Compose stack of four containers, on the author's home PC, behind a Cloudflare Tunnel.

How reminders work:
- Each time "Para enviar hoy" is read, the server works it out from two rule kinds:
  - a business-wide "before the appointment" rule, which is hidden once the appointment starts;
  - a per-service "come-back" rule, which uses a computed "done" state and is skipped if the client already has a later active appointment.
- She sends each message by hand through a `wa.me` link, then confirms it. The confirmation writes a reminder-log row.
- Once a day, a summary push tells her how many messages are waiting.

Other behavior:
- If the server is off, the app opens read-only from a Dexie cache.
- Restic backs up daily to a USB disk.
- An upgrade only runs after a fresh backup.
- She can export her data as CSV and JSON.

The Detailed Implementation Plan lists 87 numbered files and items, built in phases P0–P7. Each phase has an exit check.

## How the existing system works, per this document

There is no running system yet. Today she keeps everything in WhatsApp chats, a notebook and her memory.

An earlier PRD and SAD exist, in Spanish, in the repo root. The SAD described:
- a stored reminder queue, drained by a `SKIP LOCKED` worker;
- Caddy with Let's Encrypt;
- seven attribute types;
- per-service "before" rules;
- a manual "Completar" tap to start come-back reminders;
- delta sync.

A brainstorm review found many bugs in the SAD's design. The main one: the skip rule counted the appointment itself as a "future appointment". Because of that, every 24 h reminder was skipped.

This document replaces the SAD's architecture:
- The SAD becomes a Spanish ADR index that links here.
- The PRD is edited to match.

## Findings

1. 🟡 **Major: "Come-back time" setting is not defined** (line 1951, item 77, `ajustes/negocio`).
   - `business_settings` (335–346), the `Settings` schema (1369) and the time rules (764–770) only have `push_hour`.
   - Come-back items are due "from the start of that day" (473, 1764).
   - It is unclear whether the setting is missing from the model or is a leftover.

2. 🟡 **Major: the media upload shape contradicts itself** (line 1401 vs 579 and 1708).
   - The Technical Plan and item 52 say multipart parts `full` and `thumb`.
   - The request-body table says `file (JPEG), client_id?, appointment_id?`.
   - So item 15 (`api/openapi.yaml`) gets two contracts that don't match.

3. 🟡 **Major: what P1 puts in the spec is unclear** (line 1144 vs 646 and 1350).
   - Line 646 says `/healthz` and `/readyz` are not in `api/openapi.yaml`.
   - The P1 row still lists items 15–16 as "health only".
   - Line 1350 adds a one-endpoint spike in P1.
   - So it is unclear what `openapi.yaml` holds at the end of P1.

4. 🟡 **Major: the local developer setup is not stated** (740–756, 1445 item 20 vs 1144, 1189).
   - Item 20 requires every "yes" variable and an `https` `PUBLIC_URL`. That includes `TUNNEL_TOKEN`, `RESTIC_PASSWORD` and `BACKUP_TARGET`, which the app never uses.
   - The P1 exit check and `make dev` don't say which variables to set or what `PUBLIC_URL` is on localhost.
   - The Origin check compares against `PUBLIC_URL`, so this decides whether any write works in dev.

5. 🟡 **Major: "three" rule kinds** (line 899, E7; line 1742, item 55).
   - The data model (399), the rules table (468–473) and item 12 all say two kinds.

6. 🟡 **Major: "Enviados" offline cache does not exist** (line 1928, item 74).
   - The same sentence says "online only" and "offline it shows the last cached copy".
   - The Dexie schema (item 67, line 1865) has no reminder-log table.
   - `Bootstrap` (1378) carries no log rows.

7. 🟡 **Major: links the plan itself will break** (lines 6, 47, 126, 793, 1094–1096 vs item 14, line 1337, and item 87).
   - Items 12–14 move the PRD, SAD, brief and findings out of the root, and item 87 deletes the root copies.
   - Item 14 updates only the "Additional Resources" and "Terms" links.
   - The links in Predecessors (6), Open Questions (126) and the Alternatives intro (793) would break.

8. 🟢 **Nit:** the Spanish screen names "Hoy", "Clientas", "Ajustes" and "Enviar" (97, 167, 1906) are not in Terms. "Hoy" is explained only at line 1911.
9. 🟢 **Nit:** "EGM" (line 48) has no link and no explanation.
10. 🟢 **Nit:** the time for an unplugged USB disk to turn the status line red differs. Line 678 says "within about an hour"; line 2078 says "within 2 hours".
11. 🟢 **Nit:** the archived-rule test is said to be in `due_test.go` (1736). The `due_test.go` case list (1750–1764) does not include it.
12. 🟢 **Nit:** the ADR index text promises "a link to the design section" (1301). The table (1305–1330) has no such column.
13. 🟢 **Nit:** Goal 8 says "four services" (104). Compose also defines a profile-only `restore` service (1998).
14. 🟢 **Nit:** two gaps in the daily timer and `DueList`:
    - It is not said what the daily timer does before setup, when there is no `business_settings` row (280).
    - The two `DueList` signatures differ: `DueList(now)` at 484, `DueList(ctx, now)` at 1743.

Overall: the reviewer could paraphrase every section, and the two parts connect. No Blockers. The Majors are:
- internal contradictions (2, 3, 5, 6);
- an undefined setting (1);
- an unstated dev setup (4);
- links the plan itself breaks (7).
