# Comprehension Report: Stage 6 (round 3)

Target: `docs/designs/release-1.md`. FeatureRange: lines 1–1172. DetailedRange: lines 1173–2274.
Reviewer: `egm-comprehension` (fresh context). No document standard.

## What this proposes

- Release 1 of open-customer is a self-hosted app for one non-technical nail-studio owner who uses an iPhone. She manages clients, appointments and WhatsApp reminders from it.
- Generic things are data she sets in the app, so another kind of business could run the same code: attributes, branding, templates, time zone, currency, country code.
- **Stack:**
  - one Go binary with an embedded SvelteKit PWA;
  - PostgreSQL 18;
  - cloudflared;
  - a restic backup container.

  All of it runs in one Compose file on the author's home PC, behind a Cloudflare Tunnel. No container publishes a port.
- **Reminders** are computed when read, with no queue. There are two rule kinds:
  - a business-wide "before the appointment" rule;
  - a per-service "come-back" rule, triggered by a computed "done" state.
- **Sending:**
  - She sends each reminder by hand through a `wa.me` link, then confirms it. The confirmation writes a reminder-log row.
  - One push a day tells her how many messages are waiting.
- **Offline** is read-only, from a Dexie cache that `/bootstrap` refreshes over a date window.
- **Data safety:**
  - a daily restic backup to a USB disk that has a marker file;
  - a migration guard that needs a backup from the last 60 minutes;
  - restore scripts that also restore the matching app version;
  - a CSV + JSON export she can download herself.
- **Build plan:** 87 file items, built in phases P0–P7. Each phase has an exit check.

## How the existing system works, per this document

- No code exists yet; every item is "Create".
- Today she runs the business from WhatsApp chats, a notebook and memory. Nothing tells her which messages to send, and her client details are never used in messages.
- An earlier PRD and SAD exist at the repo root. The SAD designed:
  - a stored reminders queue with a `SKIP LOCKED` worker;
  - per-service "before" rules;
  - a manual "Completar" tap to start come-back reminders;
  - Caddy with Let's Encrypt;
  - delta sync;
  - 7 attribute types.
- A brainstorm review found reminder bugs in the SAD. The main one: an appointment counted as its own "future appointment", so every 24 h reminder was skipped.
- This document replaces the SAD's architecture:
  - The SAD becomes a Spanish ADR index.
  - The PRD is edited to match.
- The SAD's design can only be pieced together from the "(changed)" and "(new)" marks and the Alternatives. There is no summary of it.

## Findings

1. 🟡 **Major: the meaning of `{{appointment.*}}` in come-back messages and in preview is not stated** (457–458, 467, 1509, 1696).
   - A come-back item belongs to the past visit. The reader has to guess that `{{appointment.date}}` and `{{appointment.services}}` show that past visit.
   - Preview "against a real client she picks" does not say which appointment fills the `appointment.*` variables, or what happens when the client has none.
   - The UI flow for picking an appointment is not described.
2. 🟡 **Major: CUJ 1 and the design give different wizard orders** (140 vs 569 and 2102).
   - CUJ 1 asks for the icon before the username and password.
   - The Auth section and item 73 upload the icon after `POST /setup`.
   - `setup.spec.ts` automates CUJ 1, so the two must agree.
3. 🟢 Nit: "item N" references start at line 36, but the doc never says they are Detailed Implementation Plan entries. Add it to Terms.
4. 🟢 Nit: "Phase" means two things: the product Phases 1–3 (line 41) and the build phases P0–P7. Only the first is defined.
5. 🟢 Nit: the "it" at line 485 is unclear. It follows the archived-client sentence, but it means the archived rule.
6. 🟢 Nit: Goal 3 says "the four checks below", but five bullets follow.
7. 🟢 Nit: "Enviados" is used in CUJ 4 (188) but defined only in item 74.
8. 🟢 Nit: "bootstrap" is used as a transaction flow at 326, before `/bootstrap` is defined (552).
9. 🟢 Nit: "the author" (the operator role) is not in Terms.
10. 🟢 Nit: the doc never says how the author runs `reset-password` or `setup-link` (`docker compose exec app open-customer …`).
11. 🟢 Nit: "slot mapping" and "Meta-approved templates" (1160) and "template (message) vs template (Meta)" (1366) are not defined.
12. 🟢 Nit: "EGM" (50) has no link. Item 14 depends on its six-report structure.
13. 🟢 Nit: Goal 8 and the Deployment table list four services. Restore uses a fifth, profile-only `restore` service.
14. 🟢 Nit: in the select example (441), "as typed" is unclear (original casing?).
15. 🟢 Nit: the new ADRs 018–025 are "New", but the doc does not say where their text lives.

Overall: every section could be paraphrased without guessing. No Blockers. The two Majors to fix before handover:
- what the `appointment.*` variables mean in come-back messages and in preview;
- the wizard order in CUJ 1.
