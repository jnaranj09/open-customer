# Comprehension Report: Stage 6 (round 4)

Target: `docs/designs/release-1.md`. FeatureRange: lines 1–1196. DetailedRange: lines 1197–2308.
Reviewer: `egm-comprehension` (fresh context). No document standard.

## What this proposes

- Release 1 of open-customer is a self-hosted app for one non-technical nail-studio owner on an iPhone.
  - She keeps clients, appointments and her own custom attributes in it.
  - She sends personal WhatsApp reminders by hand: the app opens a `wa.me` link with the text filled in, she sends it, then confirms in the app.
- **Stack:** one Go binary with an embedded SvelteKit PWA, plus PostgreSQL 18, in one Compose file on the author's home PC. A Cloudflare Tunnel exposes it, and a restic backup container copies to a USB disk.
- **Reminders:**
  - There is no stored queue. The server works out today's list each time it is read.
  - There are two rule kinds: "before the appointment", and "come back after N days" per service.
  - A reminder log blocks repeats, and one daily push gives the count.
- **When the PC is off**, the phone shows cached data read-only.
- The doc also covers setup, upgrades behind a backup guard, restore, export, and a phased build plan (P0–P7, items 1–87).

## How the existing system works, per this document

- No code exists yet. Today she runs everything from WhatsApp chats, a paper notebook and memory. Reminders depend on her remembering, and client details are never used in messages.
- A PRD and a SAD existed before this doc.
  - The SAD's stored queue and worker had reminder bugs. For example, its skip rule skipped every 24 h reminder.
  - This doc replaces the SAD's architecture. The SAD becomes an ADR index, and the PRD is edited to match.
- Existing tools (SaaS booking apps, self-hosted booking tools, general CRMs) were rejected: none puts her own attributes into her own messages from a phone.

## Findings

1. 🟡 **Major: "Owner" means two people** (49, 1022, 1080). "The owner chose phone-only search in the round-3 review" means the author. Write "the author" where that is meant.
2. 🟡 **Major: "The author approved the change" says nothing** (50, 1120). The author is defined as the person who wrote the design, so the author approves their own change. Say who drafted it and who approved it.
3. 🟢 Nit: "EGM" and "round-N review" are not explained (55, 1022, 1439, 2308).
4. 🟢 Nit: PRD requirement IDs are used before being introduced (954, 1190, 1192). Line 954 says "SAD FR-R7", but it is a PRD ID.
5. 🟢 Nit: the doc never says how the author reads the setup link from the logs (`docker compose logs app`), or that `setup-link` runs through `docker compose exec` (144, 581).
6. 🟢 Nit: "largest `offset_days`" for come-back candidates is unclear (512). It should say the largest come-back offset.
7. 🟢 Nit: the Postgres pin count doesn't match (1205: five places; 1216: three).
8. 🟢 Nit: "avisa a <owner>" (2129). The Feature section never mentions the `backup_contact_name` string.

Overall: the doc stands on its own; every section can be paraphrased, including the due-list rules, the backup guard and offline mode. No Blockers. The two Majors are about who "owner" and "author" refer to.
