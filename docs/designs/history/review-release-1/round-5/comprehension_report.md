# Comprehension Report: Stage 6 (round 5)

Target: `docs/designs/release-1.md`, FeatureRange lines 1–1218, DetailedRange lines 1219–2349.
Reviewer: `egm-comprehension`, fresh context, no document standard, linked predecessor files not opened.

Verdict: 0 Blocker, 4 Major, 13 Nit. The reviewer could paraphrase every section.

## What this proposes

Release 1 of open-customer is a self-hosted app for one non-technical nail-studio owner. She uses it as an installed PWA on her iPhone to keep clients, appointments, her own custom attributes, and reminder messages built from those attributes. She sends each WhatsApp message by hand through a `wa.me` link and then confirms she sent it. The server works out the "to send today" list each time it is read, from two rule kinds. A "before" rule is business-wide and N days before an appointment. A "come-back" rule belongs to a service and fires N days after a visit that is "done". "Done" is computed, never stored. A visit with no other later appointment produces a come-back item. One push per day tells her how many messages are waiting. The stack is one Go binary with the SvelteKit app embedded, plus PostgreSQL 18, a Cloudflare Tunnel and a restic backup container to a USB disk, all in one Compose file on the author's home PC. The detailed plan lists 87 files, built in phases P0–P7, each with its own exit check.

## How the existing system works, per this document

Today she keeps clients and reminders across WhatsApp chats, a notebook and her memory, and nothing tells her what to send each day. The earlier design (the SAD) used a stored reminders queue with a `SKIP LOCKED` worker, a manual "Completar" tap to start come-back reminders, per-service "before" rules, Caddy with Let's Encrypt, and delta sync. The review found that its skip rule counted the appointment itself as a "future appointment", so every 24 h reminder was skipped. This document replaces the architecture parts of the SAD. The SAD becomes a Spanish ADR index, and the PRD is updated to match.

## Findings

### 🟡 Major

1. **Who opens the transaction for the due list (lines 337, 517, 1761).**
   - Line 337 says "Handlers pass the pool" and that only three flows open a transaction (bootstrap, setup, export).
   - Lines 517 and 1761 say every `DueList` caller passes a read-only `REPEATABLE READ` transaction, and name `GET /reminders/due` and the timer.
   - Fix: name every `DueList` caller in line 337.
2. **The unknown-variable test contradicts the design (line 1967 vs 477).** The design says the error names the token as written, because no label exists. Item 54 says "naming the label". Fix: make item 54 match.
3. **`main.go` (item 17) is never scheduled after P0 (lines 1257–1259, 1615–1618).** `serve` (P1) and the admin subcommands (P2) need its dispatch. Fix: list item 17 in P1 and P2, or say that each phase adds its own subcommands.
4. **The "Activar notificaciones" button has no implementing item (line 565 vs items 72 and 77).** Fix: add the button to items 72 and 77 `estado`, with its state from `push.ts`.

### 🟢 Nit

5. Line 681: "without `q`" suggests that a `q` parameter exists.
6. Lines 468–473, 1969: the exact output of `appointment.date` (with the weekday) and `appointment.time` is not stated in the FeatureRange.
7. Line 499: the come-back "Shown until" column leaves out the 60-day drop-off.
8. Lines 45, 192, 2160: "Enviar" is both a tab and a per-item button. Fix: list the four tabs in Terms.
9. Lines 226, 712: the path to the status screen (Ajustes → Estado del sistema) is not given.
10. Line 626: does phone search strip non-digits from the query first?
11. Lines 1431, 1450–1458: where the new ADRs 018–025 live (only as index rows?).
12. Line 477 vs 1974: the unmatched-`{{` save error is not in the FeatureRange "Saving" bullet.
13. Line 514 vs 1989: which make target runs `due_test.go` (`compute` cases vs `db`-tagged cases).
14. Lines 188–190: CUJ 4's Given leaves out the two other items behind "3 mensajes".
15. Line 289: `DueList` is named before it is defined. Fix: add a pointer.
16. Line 158: "SINPE" vs "SINPE Móvil".
17. Line 218: "'Enviar', 'Omitir' are disabled". Fix: "'Enviar' and 'Omitir'".
