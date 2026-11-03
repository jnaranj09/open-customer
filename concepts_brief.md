# open-customer: client and appointment app for service businesses

* **Author:** jnaranj09
* **Date:** 2026-10-29
* **Sources:** `PRD — Sistema de citas para emprendimientos.md`, `SAD — Sistema de citas para emprendimientos.md`
* **Full review findings:** `goldfish_brainstorm_findings.md` (9 Blocker, 31 Major, 14 Nit, with PRD/SAD line references)

## Overview

An open-source, self-hosted app to manage clients, appointments and WhatsApp reminders for small service businesses.

The first real user is a nail studio that is already open. The owner is not technical and works from her iPhone.
She gets an empty app and sets up her own attributes (for example "favorite drink", "nail shape"), services and reminder messages without code.
Messages can use her client attributes, for example "Aquí te esperamos con tu café negro". No current tool she could use does this well. That is the core value.

Priorities, in order:

1. Her daily use and her ability to set it up herself.
2. Generic where it is cheap (attributes, branding, templates), so other businesses can run it.
3. Open source and portfolio value.

Stack: Go monolith, PostgreSQL 18, SvelteKit PWA embedded in the binary, Docker Compose.
First install: the author's home PC behind a Cloudflare Tunnel on the author's domain. The PC is sometimes off. The owner accepts that downtime.

## Analysis

Six review lenses read the PRD and SAD with the context above. All six think the idea is worth building. None think the documents are ready to build from. The problems fall into five groups.

### 1. Release-1 scope is not defined, and the priorities are in the wrong order

- Web Push, export, backups, restore and offline mode each get a different phase in the PRD text, the PRD phase diagram and the SAD.
- The PRD (line 101) and SAD drivers (lines 7–12) put API latency and a 10-minute install for strangers above her needs.
- Her real goal, "she configures the empty app herself on her iPhone", is not a goal or a metric anywhere.
- About half of the SAD can leave release 1: WhatsApp Cloud API, S3, the api/worker split, delta `/sync`, Prometheus, the EXPLAIN CI gate, multi-arch images, on-device AI.
- The p95 < 50 ms target measures the server. Over the tunnel, what she feels is network time. A better test: "Hoy opens in under 1 s on her iPhone over LTE".

### 2. The deployment in the SAD is a VPS, not a home PC behind a tunnel

- Caddy with Let's Encrypt needs inbound ports 80/443. Behind a Cloudflare Tunnel there are none, so no certificate is issued. TLS ends at Cloudflare.
- The compose file opens ports on the home network and has no `cloudflared` service.
- Login rate limit "5 per minute per IP" (SAD:330) becomes one shared limit, because every request comes from cloudflared. One attacker could lock her out. Use `CF-Connecting-IP` from the trusted hop.
- When the PC is off, the tunnel returns Cloudflare HTML error pages (502/530), not network errors. The PWA must treat them as "offline" and never cache them.
- Backups are optional, go to the same disk, and the backup container has no DB credentials. Restore is only tested in Phase 2, while her real data is already there. One disk failure loses everything.

### 3. The reminder engine has correctness bugs

- **Every 24 h reminder would be skipped.** `skip_if_future_booking` defaults to `true` on all rules (SAD:214). Step 4 (SAD:267) skips when the client has a future appointment of that service. For a "before" reminder, the appointment itself is that future appointment. Fix: the skip applies only to "after" rules and excludes the reminder's own appointment.
- **The skip matches the wrong service.** The rule is on "acrílico nuevo", but the next booking is "relleno". The client gets a refill nudge she does not need. Match any future active appointment.
- **Refill reminders depend on a "complete" tap.** If she forgets, no refill reminder is created and nothing shows it is missing. If she closes appointments days later, the 21 days start from the tap. Fix: anchor on `ends_at` and auto-complete past appointments, or show a daily "close the day" prompt.
- **Flood after downtime.** The worker picks up everything with `due_at <= now()`, no upper limit. After a weekend off, old "see you tomorrow" messages show up for appointments that already happened.
- **Duplicates.** An appointment with two services that each have a 24 h rule sends two messages.
- **Stale text.** Rescheduling updates only `pending` rows. A `ready` row keeps the old date in its text.
- **Rule edits break.** Replacing rules in bulk fails on the `reminders.rule_id` foreign key, or deletes the sent history.
- Several lenses suggest a simpler model for release 1: build the "Para enviar hoy" list with a query at read time, and keep only a log of what was sent. The queue and worker come back with the Cloud API. This removes most of the bugs above.

### 4. A non-technical owner cannot get started with an empty app

- The setup link is printed in Docker logs (SAD:327). Only the author can read it.
- No password reset, no setup wizard, no business settings endpoint, no branding fields, no currency or country code.
- iOS saves the home-screen name and icon when she installs. Branding must be set before she installs. (Changing it later probably needs a reinstall; not verified on iOS 26.)
- The PWA origin (domain) must be final before she installs. Changing it later means reinstall, new login and new push subscriptions.
- Templates expect her to type `{{client.bebida_favorita | "un cafecito"}}` on an iPhone. iOS turns `"` into curly quotes by default, so the parser fails. Words like key, scope, anchor and offset mean nothing to her.
- Fix: in-person handover with a one-time code typed inside the installed PWA; CLI `create-owner` and `reset-password`; a first-run wizard; tap-to-insert variable chips with a live preview; keys generated and hidden.

### 5. The attribute model is designed before her real data is known

- Seven types, option trees of any depth, attributes limited to some services. Nobody has listed what she actually tracks (PRD open questions, lines 145–146).
- The example "Forma → Almendra → Corta/Larga" is really two flat attributes: shape and length.
- The JSON shape per type is not defined. Rules for changes later are missing: can `type` change, what happens to values and templates when an attribute is archived, what happens when a new required attribute is added to old records.
- Once real data exists, changing this means rewriting JSONB.
- Fix: a one-hour interview with her first. Flat types by default, at most one level of sub-options if she needs it. Write a value table per type. Drop the `image` and `payment_method` types; use photos and a normal select.

### Smaller but real

- `go:embed build` skips folders that start with `_`. SvelteKit puts all JS and CSS in `_app/`, so the app would be blank. Use `//go:embed all:build`.
- oapi-codegen may not support OpenAPI 3.1 (not verified). It also generates Go only; the TypeScript generator is not named.
- WebP encoding in Go usually needs cgo, which conflicts with a static distroless binary. Resize photos on the phone before upload instead.
- `wa.me` needs the number without `+`; `phone_e164` stores `+`.
- Searching "Maria" will not find "María" with `pg_trgm` alone. Needs `unaccent`.
- Login returns the token in the body and also in an HttpOnly cookie. The body copy makes HttpOnly useless.
- PRD line 13 says no free self-hosted option exists. Easy!Appointments, Cal.com and Odoo exist. The real gap is attributes inside messages plus a phone-first setup.

## Goals vs. Non-Goals

Drivers, in priority order (replaces SAD lines 7–12 and PRD line 101):

1. Her daily use and self-setup on the iPhone.
2. Data safety.
3. Reminders are correct.
4. Maintainable by one person.
5. Generic and self-hostable by others.

* **In-Scope Goals (release 1):**
  1. **Ingress:** app + PostgreSQL 18 + cloudflared, no published ports. Caddy only as an optional VPS profile. Login rate limit keyed on `CF-Connecting-IP` from the trusted hop. 5xx/530 and non-JSON responses count as "offline" and are never cached.
  2. **Reminders:**
     * The "Para enviar hoy" list is computed by a query when read. Only a log of sent/skipped reminders is stored, keyed by (rule, appointment, anchor time).
     * One business-wide "before the appointment" reminder (lists `{{services}}`), not one per service.
     * Per-service come-back reminders (relleno). Past `scheduled`/`confirmed` appointments count as completed; the delay starts at `ends_at`. She marks no-shows ("No vino"), and the come-back reminder goes away.
     * "Skip if rebooked" applies only to come-back rules, matches any future active appointment, and re-arms if that appointment is cancelled.
     * A late "before" reminder expires at the appointment start.
     * Owner reminders are in-app tasks, not WhatsApp messages.
     * `wa.me` send (number without `+`), then a "¿Lo enviaste?" confirmation, then the next item.
  3. **Web Push:** one daily summary push at a time she picks. VAPID keys stored in one place, `VAPID_SUBJECT` setting, subscriptions table, pruning on 404/410. A small timer in `serve` sends it; no job queue.
  4. **Templates:** a `message_templates` library used by rules. Tap-to-insert variable chips, live preview against a sample client, fallback set by tapping. Parser accepts curly quotes. Keys generated and never shown.
  5. **Onboarding:** in-person handover; the setup link creates the business and her user, then a one-time code is typed inside the installed PWA. CLI `create-owner` and `reset-password`. First-run wizard (business name, time zone, currency, country code, name/icon/color) before the iOS install guide. Dynamic `manifest.webmanifest` and apple-touch-icon. Starter templates and a sample client. Empty states that say what to do next.
  6. **Attributes:** one-hour interview with her first. Flat types (text, number, date, select, multiselect); at most one level of sub-options, only if she needs it. No `image` or `payment_method` types. A value table per type (JSON shape, validation, rendering, CSV column). `key`, `type` and `scope` cannot change; reserved names blocked. "Required" checked only on create and on touched fields. One validator for every write path.
  7. **Appointments:** several services each, with price and duration copied into the appointment. Amount paid and payment method (a select). Day and week view. Clash = warning with "Agendar igual". A status table with CHECK constraints, implemented in one domain function per entity.
  8. **Clients:** quick add, phone normalized to E.164 using the business country code, digits-only phone column for search, duplicate-phone warning, accent-insensitive name search (immutable `unaccent` wrapper), archive, history and photo gallery.
  9. **Photos:** resized on the phone (JPEG ~2048 px + ~400 px thumbnail), EXIF removed, stored on a local volume, served only with a session.
  10. **Offline:** read-only. The PWA refreshes a date window and the catalogs when opened or focused; no delta `/sync`. Banner "El sistema está apagado. Datos de las 9:14." Writes blocked and never shown as saved; half-filled forms kept as drafts; "sent" confirmations queued on the phone.
  11. **Export:** a settings button that downloads CSV + JSON of clients, appointments, services and attributes. No photos.
  12. **Backups:** daily `pg_dump` + photos, encrypted, copied to a USB disk or another machine at home (not the Postgres disk). Runs at startup if the last one is older than 24 h, and before every migration. One manual restore drill on a clean install before her real data goes in.
  13. **Operations:** pinned image tags. Forward-only migrations; rollback = restore the pre-upgrade dump + previous tag. The SPA checks the server build version and asks to reload. Versioned IndexedDB. An in-app status screen with last backup (local and USB copy) and timer heartbeat. `POST /client-errors` to the server log.
  14. **API and auth:** spec-first OpenAPI 3.0.3, `oapi-codegen` for Go, `openapi-typescript` for the SPA, CI drift check. Session only in an HttpOnly cookie (no token in the body), `SameSite=Lax` + Origin check on writes. 412 on a failed `If-Match`. Phone-generated UUIDv7 IDs with `ON CONFLICT (id) DO NOTHING` in place of Idempotency-Key.
  15. **Build:** `//go:embed all:build`, SPA fallback to `index.html`, `service-worker.js` and the manifest served with `Cache-Control: no-cache`.
  16. **Domain:** a subdomain of the author's domain. Moving later costs her a reinstall and a new login; data is on the server.
  17. **Metrics:** audit columns (`created_at`, `updated_at`, `completed_at`, `sent_at`, skip reason). Reminder metric restated as "> 95% of due reminders appear in the list on their due day". Performance target: "Hoy opens in under 1 s on her iPhone over LTE through the tunnel". Acceptance tests from review item M18.
  18. **Docs:** PRD table marking each FR/NFR In/Out for release 1 (SAD references IDs only). ADR index with status and date. New ADRs: Cloudflare Tunnel ingress, template storage, reminder model, offline level, backups. Fix PRD line 13 (free self-hosted options do exist; the real gap is attributes inside messages + phone-first setup). Glossary.
* **Out-of-Scope Non-Goals (release 1):**
  * WhatsApp Cloud API and Meta slot mapping. Keep `sent_via` and statuses compatible.
  * The materialized `reminders` queue and SKIP LOCKED worker (returns with the Cloud API).
  * Offline creation or editing, conflict merge, delta `/sync`.
  * All on-device AI (FR-I1 to FR-I5).
  * S3, api/worker split, Prometheus `/metrics`, EXPLAIN CI gate, p95 < 50 ms SLO, multi-arch images, ETag/304, GIN index on `attrs`, FTS, in-memory catalog cache.
  * Caddy + Let's Encrypt for the reference install.
  * Photos in the export.
  * Hard delete. Archive only; a deletion request is handled by hand in SQL.
  * Image attribute type, deep option trees, service-limited attributes (unless the interview shows a need).
  * Staff roles, public booking, English UI, reports, business-type template packs.
  * NFR-4, NFR-5, NFR-9 ("stranger installs in 10 minutes", VPS sizing, contributor readiness). Moved to the Phase 3 gate.
  * Restore testing in CI (Phase 2; release 1 has the manual drill).
  * Choosing the license. Decide before the repo is public.

## Alternatives Weighed

| Alternative | Result | Reason |
| --- | --- | --- |
| SaaS (AgendaPro, Fresha, Booksy) | Rejected | Limited custom fields in messages; data on someone else's platform; monthly cost. |
| Easy!Appointments, Cal.com, Odoo Appointments | Rejected | Built around public booking, heavy, weak on custom fields inside messages. Name them in the PRD. |
| Twenty, EspoCRM | Rejected | General CRMs, heavy, not phone-first or appointment-first. |
| NocoDB / Baserow with formulas for `wa.me` links | Rejected | She would edit formulas; no rebooking logic. |
| Google Sheet or iOS Contacts + Calendar + Shortcuts | Stopgap only | Usable now while the app is built. Not the product. |
| Native iOS app (SwiftUI) | Rejected (ADR 015) | Needs Mac and Apple account; two clients to maintain. |
| Keep the SAD driver order (performance, stranger self-hosting first) | Rejected | Steers work to S3, multi-arch and latency gates before her onboarding. |
| Caddy + Let's Encrypt for the first install | Rejected | Does not work behind a Cloudflare Tunnel. Optional VPS profile only. |
| Cloudflare Access in front of the app | Rejected for release 1 | Effect on PWA cookies and push not verified. |
| Materialized reminders queue + worker | Rejected for release 1 | Causes most reminder bugs; protects nothing in manual mode. Returns with Cloud API. |
| Manual "Completar" tap or evening "close the day" prompt to start come-back reminders | Rejected | A forgotten tap or skipped prompt loses the reminder with no warning. Auto-complete chosen; no-shows have 21 days to be corrected. |
| No push | Rejected | On days without appointments she may not open the app, and come-back reminders wait. |
| One push per reminder (SAD) | Rejected | Noise, and bursts after downtime. Daily summary chosen. |
| Offline creates saved on the phone (create-only outbox) | Rejected for release 1 | Pending states, local-only clash check, loss if iOS clears storage. UUIDv7 IDs keep it possible later. |
| Full offline editing with merge | Rejected | Too complex. |
| Delta `/sync` | Rejected for release 1 | Schema cannot support it. Date-window refresh instead. |
| Full export with photos | Rejected for release 1 | Multi-GB ZIPs through the tunnel; more work. |
| No export | Rejected | Only the author could get her data out. |
| SQLite + Litestream | Rejected | Postgres kept: familiar to the author, better base for staff/booking later. |
| PostgreSQL 17 | Rejected | Oldest option, earliest forced upgrade. |
| PostgreSQL 19 | Rejected | General release planned Nov 19, 2026; too new. |
| Hourly dumps or WAL archiving | Rejected | Daily dump chosen; up to one day of data can be lost and re-entered. |
| Cloudflare R2, Backblaze B2, Google Drive for the off-site copy | Rejected | USB disk / home machine chosen; no cloud account. Accepted risk: fire, theft or surge can take both copies. |
| Code-first API, or OpenAPI 3.1 with ogen | Rejected | Spec-first 3.0.3 gives typed Go + TS and avoids the unverified 3.1 support question. |
| Her own domain now | Rejected for now | A subdomain is free; moving later is a 5-minute reinstall. Revisit before Phase 3 public booking. |
| Server-side WebP thumbnails | Rejected | Needs cgo. Resize on the phone. |
| Per-rule inline template text | Rejected | Retyping per service. Template library instead. |
| Per-service "before" reminders | Rejected | Duplicate messages for multi-service appointments. |
| Rule DSL, EAV tables, `pg_jsonschema` | Rejected | Too complex. JSONB plus one validator. |
| Unofficial WhatsApp libraries (whatsmeow, Baileys) | Rejected | Break Meta's terms; risk of a ban on her number. |
| Rendering templates in the browser | Rejected | Two renderers (Go and TS) would drift. |
| Read-only ICS calendar feed in iOS Calendar | Open (not in release 1) | Could show her agenda while the PC is off. Cache duration not verified. |

## Key Decisions & Staging for Next Steps

**Decided (2026-10-29)**

| # | Decision | Choice |
| --- | --- | --- |
| 0 | Priority when goals conflict | Her needs over open-source goals |
| 0 | First install | Home PC + Cloudflare Tunnel; downtime accepted |
| 0 | Release 1 | An empty app she configures herself; data entered by hand |
| 1 | Driver order | Her use → data safety → reminder correctness → one-person maintenance → generic/self-hostable |
| 2 | Reminder model | Query at read time + sent log |
| 3 | Come-back (relleno) trigger | Automatic completion after `ends_at`; she marks no-shows |
| 4 | Offline | Read-only |
| 5 | Web Push | One daily summary push |
| 6 | Export | Minimal CSV + JSON, no photos |
| 7 | Database | PostgreSQL |
| 8 | Postgres version | 18 |
| 8b | Backup depth | Daily `pg_dump` + photos |
| 8c | Off-site copy | USB disk or another machine at home |
| 9a | API contract | Spec-first OpenAPI 3.0.3, oapi-codegen + openapi-typescript |
| 9b | PWA domain | Subdomain of the author's domain |
| 9 | Release-1 scope | The In/Out lists above, accepted in full |

**Work before or during `/egm-draft`**

- One-hour interview with her: the attributes she tracks, her services, how often each client comes back (closes PRD lines 145–146). Settles sub-options.
- One-hour test page on her iPhone through the tunnel: `wa.me` vs `whatsapp://send`, accents, emoji, newlines, WhatsApp vs WhatsApp Business.
- Check the PG 18 Docker image data path before writing the compose file.
- Pick the exact subdomain.

**Claims not verified** (check on her iPhone or in current docs): `wa.me` from a standalone PWA on iOS 26; whether iOS updates the icon after install; HEIC conversion in Safari's file input; oapi-codegen and OpenAPI 3.1 (avoided by using 3.0.3); Cloudflare free-plan response timeout; iOS push behavior after a `SameSite` cookie change; Costa Rica Ley 8968 obligations (relevant to archive-only deletion).
