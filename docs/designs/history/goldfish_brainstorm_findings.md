# Brainstorm findings: open-customer (client + appointment app for service businesses)

* **Mode:** BRAINSTORM (6-lens Goldfish shootdown)
* **Date:** 2026-10-29
* **Targets:**
  * `PRD — Sistema de citas para emprendimientos.md` (150 lines), cited as `PRD:<line>`
  * `SAD — Sistema de citas para emprendimientos.md` (435 lines), cited as `SAD:<line>`
* **Fixed context given to every reviewer:** the first user's needs come first and open source second; first install is the author's home PC behind a Cloudflare Tunnel, and downtime is accepted; the studio is already open and data is migrated by hand; the first release is an empty app she configures herself on an iPhone; one business per install, small volume.

Lens tags: `design` = egm-design, `details` = egm-codebase-details, `zoom` = egm-zoom-out, `comp` = egm-comprehension, `ready` = egm-implementation-readiness, `struct` = egm-structural.

"Unverified" means at least one reviewer flagged the claim as outside what it could confirm without browsing or a device. Those claims need checking on her iPhone or against current vendor docs before anything relies on them.

## Reviewer status

| Reviewer | Role title used | Status |
| --- | --- | --- |
| `egm-design` | Skeptical Architect Brainstormer | completed |
| `egm-codebase-details` | Technical Accuracy Brainstormer | completed (no codebase; checked PRD/SAD consistency and technical claims) |
| `egm-zoom-out` | Contrarian / YAGNI Brainstormer | completed |
| `egm-comprehension` | UX & Ergonomics Brainstormer | completed |
| `egm-implementation-readiness` | Migration & Compatibility Brainstormer | completed |
| `egm-structural` | Maintainability & Tech Debt Brainstormer | completed |

Severity breakdown after deduplication: 9 Blocker, 31 Major, 14 Nit.

## Overall verdict

All six lenses agree that the core idea is worth building: a phone-first app where she puts her own client attributes inside WhatsApp messages. The building case rests on two things only, that gap and the portfolio motive. The docs as written are not ready to build from. They have four main problems:

1. They describe an always-on VPS with Caddy, not a home PC behind a tunnel that is sometimes off.
2. The release-1 scope contradicts itself across the PRD, the SAD and the phase diagram.
3. The reminder engine has several correctness bugs. One of them would suppress every 24-hour reminder.
4. The first-run path a non-technical owner needs to configure an empty app is missing.

About half of the SAD (Cloud API, S3, api/worker split, delta sync, latency gates, on-device AI) can leave the first release.

---

## Blockers

### B1. Release-1 scope is undefined and contradicts itself
* **Lenses:** comp, ready, struct, details, zoom, design (all six)
* **Where:**
  * PRD:87 (FR-R8 Web Push, no phase) vs PRD:124 (Phase 2) vs SAD:269 and SAD:73 (push built into the manual flow).
  * PRD:111 (NFR-7 export, no phase) vs PRD:124 (Phase 2) vs SAD:248 (`GET /export` in the API).
  * PRD:110 (NFR-6 backups, no phase) vs SAD:412 (restore is Phase 2) vs SAD:422 (the risk table cites "restore probado en CI" as a current mitigation).
  * PRD:124 ("modo offline ampliado" in Phase 2 implies a Phase 1 offline level, which SAD:434 leaves open).
  * Cloud API, S3, api/worker modes, Prometheus, multi-arch images and the EXPLAIN gate are designed in detail with no phase.
  * PRD:119 ("lo mínimo para que deje la libreta") vs the stated goal of an empty app she configures herself.
* **Problem:** Phases are recorded in three places (the PRD diagram, inline "(Fase N)" tags, SAD:89 and SAD:412), and they already disagree. An engineer cannot tell what release 1 contains.
* **Recommendation:** Make the PRD the single source of scope. Add one table that maps every FR/NFR ID to In, Out or Stub for release 1. The SAD should reference IDs only and never restate phases. Use the in-scope list at the end of this file as the starting proposal.

### B2. The deployment design does not match the real install: Caddy and Let's Encrypt cannot work behind the tunnel
* **Lenses:** struct, details, ready, design, zoom, comp
* **Where:** SAD:10, 23, 31, 41-43, 59 ("VPS chico"), 69 (ADR 011), 297, 333 (HSTS from Caddy), 367-373 (caddy publishes `80:80`, `443:443`), 387-393 (Caddyfile), 413, 422; PRD:109 (NFR-5), PRD:112 (NFR-8 "TLS automático").
* **Problem:**
  * With no inbound 80/443, Caddy's ACME challenges fail and the certificate is never issued. The `details` lens rates this verified-from-knowledge.
  * TLS ends at Cloudflare's edge. Publishing 80/443 on a home LAN adds attack surface for no benefit.
  * There is no `cloudflared` service and no tunnel token.
  * No ADR records the tunnel, the home PC, or the fact that Cloudflare sees all traffic decrypted (client phones, photos).
  * Through the tunnel, a server that is off returns Cloudflare HTML 502/530 pages, not network errors.
  * Proxied uploads are capped at 100 MB on the free plan. A response that has not started after about 100 s gets a 524, so `/export` must stream. The `details` lens rates both verified-from-knowledge; check current limits.
* **Recommendation:** Add an ingress ADR (for example 018: "Cloudflare Tunnel for the reference install"). The reference compose becomes `app + postgres + cloudflared` with no published ports. Keep Caddy plus Let's Encrypt as an optional `vps` profile for other self-hosters. HSTS and compression move to Cloudflare or the app. Reword NFR-8 as "HTTPS end to end via the chosen ingress". Replace the VPS risks with home-PC risks: disk failure, power loss, PC off, backups on the same disk.

### B3. `skip_if_future_booking` would cancel each appointment's own 24-hour reminder, and its matching rule is wrong for nails
* **Lenses:** design (raised the core bug); comp, ready, struct, details (scope and matching)
* **Where:** SAD:214 (`DEFAULT true` on every rule, including `before_start`), SAD:267 (step 4), PRD:86 (FR-R7).
* **Problem:**
  * Every `before_start` rule has the flag set to true by default. When a "24 h before" reminder comes due, the appointment it belongs to is itself "a future appointment of that service", so the reminder is skipped. Implemented as written, no pre-appointment reminder is ever sent. I confirmed this reading of SAD:214 and SAD:267.
  * "Of that service" misses the common nail case. The rule sits on "acrílico nuevo", but the next booking is "relleno" or a switch to gel, so a client who has already rebooked still gets a refill nudge.
  * The skip is terminal. If the future appointment that suppressed the refill is later cancelled, the refill reminder is lost.
* **Recommendation:**
  * Make the skip apply only to `after_completed` rules. A property of the anchor works better than a per-rule column.
  * Exclude the reminder's own appointment, and exclude `cancelled`/`no_show` appointments.
  * Default the matching to "any future active appointment". Optionally let her pick a set of services.
  * Defer instead of skip: re-check at wake-up and keep the row pending while a future booking exists, or re-arm it when that booking is cancelled.
  * Confirm the real service list with her (PRD:146 is still open).

### B4. Reminders that came due while the PC was off flood in as stale messages
* **Lenses:** all six
* **Where:** SAD:264-270 (step 3: `due_at <= now()` has no upper bound), SAD:82, PRD:138.
* **Problem:**
  * After a day or a weekend off, every "24 h before" reminder for an appointment that has already happened, or starts in an hour, becomes `ready` at once.
  * Each one fires its own push.
  * Each one carries frozen text that is now wrong ("mañana a las 3").
  * While the PC is off, the tray is silently empty and no push arrives.
  * Back-dated appointments entered during the manual migration hit the same flood (see M21).
* **Recommendation:**
  * Add a lateness policy per anchor. A `before_start` reminder expires at `starts_at` (new status `expired`, or `skipped` with a reason), and so does any reminder with less than a minimum lead time left. `after_completed` reminders are still delivered late and marked "late".
  * Send one summary push per catch-up run.
  * Run the catch-up at startup, not only on the 60-second tick.
  * Decide whether a same-day booking gets a "24 h" reminder at all.
  * Add a test that starts the worker with N overdue rows for both anchors.
  * See M9. A tray computed by query removes most of this class of bug.

### B5. What she sees when the server is off is undefined, and optimistic UI can lose her work silently
* **Lenses:** comp, ready, design, struct, zoom
* **Where:** SAD:434 (offline level open), SAD:286-288 (cache first, optimistic UI), SAD:81 (no background sync on iOS), SAD:113 (UUIDv7 "safe to create offline"), PRD:124.
* **Problem:**
  * Downtime is guaranteed. Optimistic UI shows a new appointment as saved before the server confirms it, so with the PC off the booking either disappears or looks saved when it is not. She double-books or loses a booking made in front of the client.
  * A fetch layer that does not know the tunnel behavior may cache Cloudflare 530 HTML pages, or try to parse them as JSON.
  * If she cannot work while the PC is off, she goes back to the notebook and fails the 4-week gate (PRD:129).
* **Recommendation:** Decide now and close SAD:434. The reviewers converge on this for release 1:
  * Read-only offline from the cache.
  * A permanent banner in plain Spanish: "El sistema está apagado. Mostrando datos de las 9:14."
  * Writes are blocked with that message and are never shown as saved.
  * Half-filled forms are kept as local drafts with a one-tap retry.
  * "Mark as sent" is queued locally.
  * 5xx/530 responses and non-JSON bodies count as offline and are never cached.
  * `ready` disagrees on one point: it favors a small outbox for creates only, replayed on app open using the client-generated ID. See the "Disputed" list.

### B6. Backups sit on the same disk, are opt-in, cannot authenticate, and restore is untested while real data already lives there
* **Lenses:** all six
* **Where:** SAD:375-383 (`./backups` bind mount, `profiles: ["backup"]`, no DB credentials, no scheduler described), SAD:406, 410-412, 422; PRD:110 (NFR-6 "automático"), PRD:124.
* **Problem:**
  * On a home PC, one disk failure destroys both the data and the backups.
  * A daily job that runs while the PC is off is simply skipped.
  * `upgrade.sh` takes a dump only if the backup profile is active.
  * The backup container has no `env_file` or PG* variables.
  * VAPID keys stored in the DB are lost on a partial restore.
  * Restore is deferred to Phase 2, yet the risk table claims it as a current mitigation.
* **Recommendation:** For the reference install:
  * Backups are always on, with an off-machine encrypted copy (restic or rclone to a cloud target, or a USB disk).
  * Run a backup at startup if the last one is older than 24 h.
  * Take a mandatory dump before every migration.
  * Back up media incrementally.
  * Show a "last backup OK at" status.
  * Do one documented manual restore drill on a clean install before she enters real data. CI restore tests can wait for Phase 2.

### B7. Refill reminders depend on a "complete" tap she will forget, and are timed from the tap
* **Lenses:** comp (Blocker), design, ready, details
* **Where:** PRD:20 (Goal 2), PRD:80 (FR-R1); SAD:210, 243 (`complete` triggers `after_completed`), 264; `appointments` has no `completed_at` (SAD:178-187).
* **Problem:** The 21-day refill reminder is the core value for this user, and it only exists if she taps "complete" between clients with a brush in hand. If she forgets, nothing is created and nothing shows that it is missing. If she closes appointments in a batch days later, the 21 days are counted from the tap.
* **Recommendation:**
  * Anchor `after_completed` on `ends_at`.
  * Auto-complete past `scheduled`/`confirmed` appointments, for example at the end of the day, and let her mark the exceptions as `no_show`. Alternatively, show a daily "close the day" prompt.
  * Un-completing an appointment skips its unsent `after_completed` rows.
  * Add `completed_at`.
  * State the chosen rule in the PRD.

### B8. There is no first-run path for a non-technical owner to receive and configure an empty app
* **Lenses:** comp (Blocker), struct (Blocker), ready, design, details
* **Where:** SAD:327 (setup link printed in Docker logs), SAD:329 (the installed PWA has its own cookie jar), SAD:311-316 (`email` is required but no email is ever sent), SAD:82 (onboarding covers only the PWA install), SAD:121-127 (`business_settings.name NOT NULL` with no seed); PRD:44-97 (no onboarding or settings FR group); PRD:145 (assumes the author designs her schema for her).
* **Problem:**
  * Only the author can read the setup link.
  * If she uses the link in Safari, her session stays in Safari. She then has to log in again inside the installed app, and the order of steps is never stated.
  * There is no password reset, so a forgotten password locks her out until the author runs a command that is not defined.
  * Nothing creates the business row.
  * There are no empty states or starter content. She cannot create an appointment without a service, and template preview needs a client that does not exist yet.
* **Recommendation:**
  * Add an FR-O* group ("Onboarding y configuración") with acceptance criteria.
  * The author does an in-person handover:
    1. The author opens the setup link and creates the business row and her user.
    2. The app shows a one-time code or QR code.
    3. She installs the PWA from Safari.
    4. She enters the code inside the installed app.
  * Provide CLI commands `create-owner` and `reset-password`.
  * The first-run wizard covers business name, time zone, currency, country code and branding (see M11), then the install guide.
  * Add a small Spanish starter kit: one editable 24 h reminder, one-tap attribute suggestions, preset payment options, and a built-in sample client for previews.
  * Treat `email` as a username, or use her phone number.
  * Consider passkeys with Face ID later. Their behavior in an iOS home-screen app is unverified.

### B9. The attribute model was designed before her real attributes are known, and the value contract and evolution rules are missing
* **Lenses:** zoom (Blocker), ready (Blocker), design, comp, struct, details
* **Where:** PRD:58-62 (FR-A2 to FR-A6), PRD:145-146 (open questions not answered); SAD:114-116, 129-148, 172-176, 241, 277, 423.
* **Problem:**
  * There are seven types, option trees of any depth, and attributes limited to services, but no list of what she actually tracks. The flagship example "Forma → Almendra → Corta/Larga" is really two flat attributes (shape and length).
  * The JSON shape per type is undefined: an array means a path for a select, but a set for a multiselect, which is ambiguous with sub-options. `date`, `number`, `image` and `payment_method` have no defined shape. `image` has no link to `media`.
  * The examples use `opt_*` IDs, but the schema says `uuid`.
  * The evolution rules are undefined:
    * whether `type` or `scope` can change;
    * what happens to stored values when an option or attribute is archived;
    * what happens to templates that reference an archived attribute (validated only at save time, SAD:277);
    * whether a key can be reused (it cannot, because UNIQUE includes archived rows);
    * whether a key can collide with fixed fields such as `name` or `phone`;
    * what happens when a required attribute is added after records exist. Every old record then becomes invalid, and a PATCH that validates the whole `attrs` object blocks unrelated edits.
  * Changing any of this after real data exists means a JSONB rewrite.
* **Recommendation:**
  1. Run the interview first: one hour with her on attributes and services (PRD:145-146).
  2. Plan for flat types by default (text, number, date, select, multiselect). Allow at most one level of sub-options, and only if the interview shows she needs it.
  3. Drop the `payment_method` type in favor of a seeded select. Drop the `image` type, because client and appointment photos already cover it.
  4. Write a per-type value table: JSON shape, validation, template rendering, CSV column.
  5. Make `key`, `type` and `scope` immutable. Generate the key from the label and reserve fixed-field names.
  6. Enforce "required" only on create and on fields that are touched. Preserve archived values as read-only.
  7. Warn and list affected templates when an attribute is archived.
  8. Send every write path (API, import, scripts) through one validator. Add a DB CHECK `jsonb_typeof(attrs) = 'object'`.

---

## Major

### M1. The stated drivers rank performance and stranger self-hosting above the first user
* **Lenses:** zoom, struct, design, ready
* **Where:** PRD:101 ("Performance y facilidad de self-hosting... mandan sobre cualquier feature"), PRD:22 (Goal 4), PRD:108-109, 113 (NFR-4, NFR-5, NFR-9), PRD:141; SAD:7-12.
* **Problem:** This contradicts the fixed priority, and it will steer effort toward S3, multi-arch images and the 10-minute install before onboarding. Her actual goal ("she sets everything up herself on an iPhone") is not a goal or metric anywhere.
* **Recommendation:** Reorder the drivers: (1) her daily workflow and self-setup on the iPhone, (2) data safety, (3) reminder correctness, (4) maintainable by one person, (5) generic and self-hostable by others. Add metrics such as "she creates 5 attributes, her services and one template unaided in under 20 minutes" and "the app is in her hands by date X". Move Goal 4 and NFR-4/5/9 to the Phase 3 gate (PRD:129).

### M2. Login rate limiting "per IP" becomes one global bucket behind the tunnel
* **Lenses:** zoom, design, details, struct, ready
* **Where:** SAD:330.
* **Problem:** Every request reaches the app from cloudflared's address, so one bad actor can lock her out.
* **Recommendation:** Read `CF-Connecting-IP` only from the trusted tunnel hop, and make the trusted proxy a configuration item. Add per-account backoff.

### M3. Rule edits and appointment changes leave reminders inconsistent
* **Lenses:** ready, design, details, struct
* **Where:** SAD:242 (`PUT /services/{id}/reminder-rules`, "se reemplazan en bloque"), SAD:219 (`rule_id NOT NULL REFERENCES`, no ON DELETE), SAD:226, SAD:265, 268-269; PRD:76 (FR-P5).
* **Problem:**
  * Replacing rules in bulk either fails on the foreign key or, if cascaded, wipes the sent history. Regenerated rule IDs defeat `UNIQUE(rule_id, appointment_id)`, so duplicates become possible.
  * Rescheduling updates only `pending` rows. A `ready` row keeps the old date in `rendered_body`, so she sends the wrong time.
  * Cancelling leaves `ready` rows in the tray.
  * After a reschedule, the uniqueness rule blocks a new reminder if the old one was already sent.
  * Adding or removing a service on an appointment does not touch its reminders.
  * New rules do not backfill existing future appointments.
* **Recommendation:**
  * Give rules stable IDs, upsert them, and archive instead of deleting.
  * Write one recalculation function, called on every change to time, status or services. It resets `pending` and `ready` rows, re-renders them, and treats sent rows as immutable.
  * Include the anchor time in a reminder's identity.
  * Snapshot what was sent (body, phone, `sent_at`).
  * Most of this disappears with M9.

### M4. Message templates are not their own records
* **Lenses:** design, struct, ready, details, comp
* **Where:** SAD:213 (`reminder_rules.template text`), SAD:246 (only `POST /templates/preview`), SAD:278 (the Meta template name and slot mapping have no column); PRD:82, PRD:125.
* **Problem:** She has to retype the same "24 h before" text for every service and keep the copies in sync. There is nowhere to put the Cloud API mapping. Inline text also blocks the Phase 3 per-business presets.
* **Recommendation:** Add a `message_templates` table (id, name, body, locale, recipient kind, archived_at, nullable provider fields). Rules reference it with `template_id`. Add `/templates` CRUD and a template-management FR. Define whether a template edit re-renders unsent `ready` rows.

### M5. An appointment with several services sends duplicate "see you tomorrow" messages
* **Lenses:** zoom, comp, ready, design, details
* **Where:** PRD:67-69 (FR-S2, FR-S3), SAD:207-215, 226, 264.
* **Problem:** Rules belong to services, so two services that each have a 24 h rule send two messages to the same client.
* **Recommendation:** Make "before the appointment" one business-wide rule (one message listing `{{services}}`). Keep per-service rules only for "after" or rebooking reminders. Alternatively, deduplicate per (appointment, anchor, offset, recipient).

### M6. The template and attribute editors expose developer concepts to a non-technical iPhone user
* **Lenses:** comp, zoom, ready, struct
* **Where:** PRD:57, 61, 82; SAD:71 (ADR 013), 114, 131-133, 210-212 (`anchor`, `offset_min`, "30240 = 21 d"), 274-277.
* **Problem:**
  * Typing `{{client.bebida_favorita | "un cafecito"}}` means hunting through the iOS symbol keyboards. iOS smart punctuation turns `"` into curly quotes by default, so a strict parser fails.
  * "Key", "scope", "anchor" and "offset" are jargon to her.
  * The error "nearest suggested key" is written for developers.
* **Recommendation:**
  * Use a text box with tap-to-insert variable chips that always show the current label, plus a live preview against a real or sample client.
  * Set the fallback by tapping a chip ("Si no tiene: ___").
  * Keep raw syntax internal, and make the parser accept curly quotes.
  * Generate keys automatically and never show them.
  * Ask scope as a question ("¿Es igual siempre para la clienta, o cambia en cada cita?").
  * Build rules as a sentence: "Enviar [1] [día ▾] [antes de la cita ▾]".
  * Write errors in plain Spanish.

### M7. The WhatsApp send loop marks a message "sent" without proof, and its iOS behavior is unverified
* **Lenses:** all six
* **Where:** SAD:270 ("al volver, la app marca sent") vs PRD:85 (FR-R6, "marcar como enviado con un tap").
* **Problem:**
  * The two documents disagree.
  * `visibilitychange` can detect that she returned to the app, but not whether she pressed send.
  * iOS may unload the PWA while it is in the background.
  * `wa.me` needs the number without "+", but `phone_e164` stores "+" (`details` rates this verified).
  * Unverified: whether `wa.me` opened from a standalone PWA on iOS 26 goes straight to WhatsApp or through Safari, which app wins when both WhatsApp and WhatsApp Business are installed, and practical URL length limits.
  * This loop is the core of the product.
* **Recommendation:**
  * Spend one hour on a test page on her phone, served through the tunnel. Test accents, emoji and newlines, and compare `whatsapp://send` with `wa.me`.
  * On return, show a large bottom sheet: "¿Lo enviaste a Ana? Sí / Todavía no". Alternatively, mark it sent on tap with an undo.
  * Then advance to the next item.
  * Store confirmations locally when the server is off.
  * A missing phone number becomes a visible "add a phone" fix in the tray.

### M8. Reminders addressed to the owner do not fit the WhatsApp flow
* **Lenses:** comp, ready, design, details, struct
* **Where:** PRD:81 (FR-R2 "llamar a Ana"), SAD:212, 269-270; `users` has no phone.
* **Problem:** The send button would open WhatsApp to her own number, or to no number at all.
* **Recommendation:** Model owner reminders as in-app tasks ("Llamar a Ana") with call, WhatsApp-the-client and "done" actions. They are a separate reminder kind with no channel requirement.

### M9. The reminder outbox and worker are overbuilt for manual mode and cause most of the reminder bugs
* **Lenses:** zoom, struct, design (alternative A), ready (alternative), comp (day grouping)
* **Where:** SAD:64 (ADR 006), 217-228, 258-270, 32 (worker).
* **Problem:**
  * In manual mode the server sends nothing. The outbox, `SKIP LOCKED`, the 60-second poll, `attempts`, backoff and `pending → ready` exist only for Cloud API and push.
  * The tray depends on the worker's state changes. It is empty when the worker is not running, and she cannot batch-send in the morning the reminders due later today.
  * Materialized `due_at` values go stale on every reschedule, rule edit or service change (M3), and after downtime (B4).
* **Recommendation:**
  * In release 1, compute the "Para enviar hoy" tray with a query: reminders due today in the business time zone that are not expired, grouped by day. Use `due_at` as "not before".
  * Store only an append-only log of what was sent or skipped, keyed by (rule, appointment, anchor time).
  * Pre-rendered or cached bodies let her send from the cache even while the server is off.
  * Bring in the worker and outbox with Cloud API in Phase 2. River (riverqueue) is an option to weigh then.
  * Do not render on the client. A second renderer in TypeScript would drift from the Go one (`struct`).

### M10. The `/sync` delta endpoint cannot work with the current schema
* **Lenses:** details (Blocker if kept), design, zoom, struct, ready
* **Where:** SAD:247, 252, 286, 427, 113; the tables at SAD:121-227.
* **Problem:**
  * Only `clients` has `updated_at`, and nothing updates it on UPDATE.
  * Nothing produces tombstones.
  * A UUIDv7 cursor orders rows by creation, not update.
  * A timestamp cursor misses rows when transactions commit out of order.
  * `appointment_services` changes are invisible to sync.
* **Recommendation:**
  * For release 1, drop delta sync and refresh a date window (for example -60 to +120 days) plus the catalogs on app open and on focus. At this volume that is about 200 KB gzipped (`design` estimate).
  * If deltas are needed later, add a global `bigint` change sequence set by a trigger, plus tombstones, under its own ADR.
  * `struct` disagrees and wants that design now, so that read-only offline is robust. See "Disputed".

### M11. Branding and business settings are missing; iOS freezes the icon at install time
* **Lenses:** design, ready, details, struct
* **Where:** SAD:121-127 (name only), SAD:66 (static `adapter-static` build), SAD:400 (`PUBLIC_URL` drives the manifest); there are no settings endpoints (SAD:237-248).
* **Problem:**
  * There is no logo, accent color, app name or icon, even though "branding stays generic" is a goal and `{{business.*}}` variables are promised.
  * The embedded manifest is static.
  * iOS captures the home-screen name and icon when she adds the app, and changing them likely needs a reinstall (unverified on iOS 26).
  * There is no currency field, so `price_cents` has no currency. There is no default phone country code.
* **Recommendation:**
  * Add `GET/PATCH /settings`.
  * Add branding, currency and country-code fields.
  * Serve `manifest.webmanifest` and the apple-touch-icon dynamically.
  * Put branding in the first-run wizard before the install step.

### M12. The PWA origin must be final before she installs
* **Lenses:** ready, struct, comp, zoom
* **Where:** SAD:431 (an open question that the fixed context has already answered), SAD:400.
* **Problem:** The origin determines the PWA's identity, IndexedDB, the session cookie and push subscriptions. Changing it later means reinstalling the app, logging in again and re-subscribing to push.
* **Recommendation:** Close the question with an ADR. Use a dedicated subdomain on the author's domain that can later be handed over or pointed elsewhere without changing the origin, and document that the origin must never change. Export (M24) is her portability guarantee, because her data lives on someone else's PC. This partly applies to PRD:12 as well.

### M13. The photo pipeline conflicts with a pure-Go distroless binary, and HEIC and EXIF are not addressed
* **Lenses:** details, design, zoom, ready
* **Where:** SAD:34, 197-205, 244, 289, 298, 334, 413.
* **Problem:**
  * `golang.org/x/image/webp` only decodes. Encoding WebP usually needs cgo (libwebp or libvips), which breaks the static distroless and multi-arch story. `details` rates this verified-from-knowledge; `design` marks the pure-Go port status unverified.
  * iPhone photos are HEIC by default. Whether Safari's file input always converts them to JPEG is unverified.
  * EXIF GPS data in client photos is a privacy leak, and EXIF orientation is not handled.
  * `thumb_key NOT NULL` conflicts with generating thumbnails asynchronously.
  * Originals at 3-5 MB grow storage and backups to roughly 100-250 GB (`design` estimate).
* **Recommendation:**
  * Resize on the client before upload (canvas to JPEG at about 2048 px plus a thumbnail at about 400 px), so the server just stores the bytes.
  * If the server does any processing, use JPEG thumbnails with a pure-Go library and strip EXIF.
  * Add `bytes`, `width`, `height` and `archived_at` to `media`, and a CHECK that `client_id` or `appointment_id` is set.

### M14. `go:embed` silently drops SvelteKit's `_app/` directory
* **Lenses:** details
* **Where:** SAD:32, 66 (ADR 008).
* **Problem:** Embedding a directory skips entries whose names start with `_` or `.`. `adapter-static` puts all JS and CSS under `_app/`, so the binary would serve a blank app (verified-from-knowledge).
* **Recommendation:**
  * Use `//go:embed all:build`, or change `kit.appDir`.
  * Specify an SPA fallback to `index.html`.
  * Serve `service-worker.js` and the manifest with `Cache-Control: no-cache`, and never mark them immutable. Cloudflare caches `.js` files by extension.

### M15. OpenAPI 3.1 with oapi-codegen, and the "one spec generates the TS client" claim
* **Lenses:** details (accuracy), zoom (ceremony)
* **Where:** SAD:62 (ADR 004), 235; PRD:113 (NFR-9), PRD:40.
* **Problem:** oapi-codegen sits on kin-openapi, which `details` reports supports OpenAPI 3.0.x, not 3.1. Check the current release. oapi-codegen generates Go only, so a TS generator is unnamed. `zoom` adds that spec-first codegen plus a CI drift gate slows every change during the phase when the UI changes most.
* **Recommendation:**
  * Write the spec as 3.0.3, or pick a generator confirmed to handle 3.1, such as ogen.
  * Name the TS tool (openapi-typescript plus openapi-fetch, orval, or hey-api).
  * Either defer the CI drift gate until the API settles, or keep the whole approach knowingly as a portfolio item.

### M16. Appointment states, transitions and side effects are undefined
* **Lenses:** ready, struct, details, design, comp
* **Where:** PRD:73-76 (FR-P2, FR-P3, FR-P5); SAD:183 (no CHECK on status, unlike other enums), 188, 222, 224, 243, 315.
* **Problem:**
  * Who sets `confirmed`, and how? The client replies "sí" in WhatsApp.
  * Can `completed` or `no_show` be reverted?
  * Does `cancelled → scheduled` re-create reminders?
  * `reminders.status` includes `failed`, but nothing ever sets it.
  * Does a clash block the booking or only warn? She may overlap appointments on purpose.
  * Without a defined state machine, transition logic scatters across handlers.
* **Recommendation:**
  * Add a transition table to the SAD.
  * Use CHECK constraints or enums on all status, role and channel columns.
  * Implement each entity's transitions in one domain function that owns the reminder side effects.
  * Put a one-tap "Confirmada" button on the tray item and the day view.
  * Clashes warn and offer "Agendar igual".
  * Map 409/412 errors to plain Spanish text.

### M17. Per-appointment price and payment amount have nowhere to live
* **Lenses:** design, ready, details, struct
* **Where:** SAD:163-170, 191-195; PRD:30, 95 (FR-I2 fills the "monto").
* **Problem:**
  * Price exists only in the service catalog, so changing it rewrites history.
  * `appointment_services` has no snapshot, and its primary key forbids the same service twice.
  * There is no amount column for FR-I2 to fill.
  * The meaning of `payment_method` is undefined: single or split payments, with or without an amount, client or appointment scope.
* **Recommendation:**
  * Snapshot `price_cents` and `duration_min` into `appointment_services`.
  * Add `appointments.total_cents` and optionally `paid_cents`.
  * Make payment method a seeded select (SINPE, efectivo, tarjeta). See B9.

### M18. The success metrics cannot be measured, and requirements have no acceptance criteria
* **Lenses:** ready, struct, design, details, comp
* **Where:** PRD:135-141, 48-97, 129; SAD:217-227 (no `created_at`, `ready_at`, `sent_at`, `skip_reason` or `last_error`), SAD:178-187 (no `created_at`, `updated_at` or `completed_at`).
* **Problem:**
  * "> 95% refills on time" has no data source, and "on time" is undefined.
  * It also measures her behavior and is distorted by downtime.
  * "6 of 6 days of use" and "< 30 s per appointment" have no instrumentation.
  * Without acceptance criteria, the 4-week gate cannot be judged.
* **Recommendation:**
  * Add the audit columns and a minimal events or usage counter.
  * Define the query behind each metric.
  * Restate the reminder metric as "> 95% of due reminders appear in the tray on their due day".
  * Write Given/When/Then cases for: an appointment in under 30 s; reschedule re-renders; cancel clears the tray; complete or auto-complete creates the refill; catch-up after downtime; FR-R7 skip and re-arm; template fallback; an archived option still renders; offline behavior; a backup and restore drill; an upgrade on a seeded copy of real data.

### M19. Nobody can see that the unattended home server is broken
* **Lenses:** details (observability), struct
* **Where:** SAD:300-304 (Prometheus with no consumer), SAD:414-415, 423 (the orphan-values job has no output).
* **Problem:** Neither she nor the author learns that the PC is off, the worker is stuck, the tunnel is down, a backup failed, or an error happened on her iPhone. `/metrics` may also be exposed through the tunnel; the docs say nothing about it.
* **Recommendation:**
  * Record a worker heartbeat and the last backup time in `/readyz`, and show them on an in-app status screen.
  * Show a "last connected" time in the PWA.
  * Add an optional external dead-man ping (for example, the Healthchecks.io free tier).
  * Add a small `POST /client-errors` endpoint that logs to slog.
  * Log the request ID and `CF-Ray`.
  * Bind `/metrics` and `/readyz` to the internal network.
  * Drop Prometheus from release 1.

### M20. Upgrades and version skew between the PWA and the server are not handled
* **Lenses:** ready, details
* **Where:** SAD:344 (`:latest`), 286, 290, 410-412, 433.
* **Problem:**
  * `:latest` combined with `pull && up` means uncontrolled upgrades.
  * There is no rollback. A failed migration leaves the app unreachable.
  * A cached old SPA can talk to a new API.
  * The IndexedDB schema has no version.
  * Postgres 17 vs 18 is undecided. A major-version upgrade on live data soon after launch needs dump/restore or `pg_upgrade`, and the PG 18 image changed its data path layout (`details`, moderate confidence).
* **Recommendation:**
  * Pin image tags.
  * Use forward-only migrations, with rollback defined as "restore the pre-upgrade dump plus the previous tag". Rehearse it once.
  * The SPA checks the server's build version and prompts a reload.
  * Version IndexedDB, and on a version change clear the cache and refresh.
  * Keep API changes additive within v1.
  * Decide the Postgres version now. Native `uuidv7()` does not matter, since Go generates the IDs.

### M21. Nothing supports entering the existing data by hand
* **Lenses:** ready, comp, details
* **Where:** PRD:50, 93 (OCR is Phase 2), PRD:119; SAD:153.
* **Problem:**
  * She or the author will re-enter clients and future bookings by hand, and nothing makes that fast.
  * She will type numbers like `8888-7777` without `+506`.
  * Back-dated "completed" appointments created to seed refill reminders hit the B4 flood.
  * There is no duplicate detection.
* **Recommendation:**
  * Add a quick-add client form (name and phone).
  * Normalize phones to E.164 using the business country code, and validate as she types.
  * Warn on duplicate phones.
  * Add a per-client "última visita / próximo relleno" entry that creates one reminder directly.
  * Add an `imported` flag that suppresses past `before_start` reminders.
  * Optionally give the author a CLI `import clients.csv`.

### M22. Time zone and DST semantics are undefined
* **Lenses:** ready, design
* **Where:** PRD:115; SAD:118, 124, 211, 276.
* **Problem:**
  * `offset_min = 30240` is an absolute duration, so it shifts under DST in other countries and inherits the time of day she completed the appointment.
  * "Today" in the views and the tray does not say whose time zone it uses.
  * Changing `business_settings.timezone` does not recompute materialized rows.
* **Recommendation:**
  * Hour offsets are absolute.
  * Day offsets are business-local calendar days at a configurable send hour.
  * All views use the business time zone.
  * The time zone is set once in onboarding.
  * Import `time/tzdata`. Go has no localized month or day names, so use your own tables or a library such as goodsign/monday.

### M23. Search will not match accented Spanish names, and phone search needs normalization
* **Lenses:** design, details
* **Where:** SAD:153, 160, 240, 288; PRD:51, 107.
* **Problem:**
  * `pg_trgm` on `name` does not match "Maria" to "María".
  * `unaccent()` is not IMMUTABLE, so an index on it needs a wrapper function.
  * Trigrams add little for phone search.
  * Both extensions need a migration.
* **Recommendation:**
  * Use an immutable `unaccent` wrapper in both the index and the query.
  * Normalize to NFD for local JavaScript search.
  * Add a digits-only phone column matched by substring.
  * Add a partial unique index or a warning for duplicate phones.

### M24. The lifecycle and privacy of client data are undefined
* **Lenses:** struct, details, ready, design
* **Where:** PRD:53 (FR-C4 hard delete); SAD:117, 197-205, 308, 406; no archive or delete endpoints exist (SAD:237-248).
* **Problem:**
  * No endpoint archives or deletes anything, and no foreign key defines ON DELETE behavior, so a "borrado definitivo" fails.
  * Media files on disk are never cleaned up.
  * Deleted data stays in backups for 14 days, undisclosed.
  * There is no WhatsApp opt-in record. The Cloud API requires opt-in under general Meta policy; the exact current terms are unverified.
  * Cloudflare sees decrypted traffic.
  * Costa Rica Ley 8968 obligations are unverified.
* **Recommendation:**
  * Add a "Datos personales y ciclo de vida" section to both docs.
  * Define the hard-delete cascade, including files, or defer hard delete out of release 1.
  * Disclose backup retention.
  * Add `clients.messaging_opt_in_at`.
  * Treat export as the portability guarantee and ship a minimal one in release 1. `zoom` disputes this; see "Disputed".

### M25. API gaps for features the PRD promises
* **Lenses:** details, ready, design
* **Where:** SAD:237-248; PRD:52, 75, 53.
* **Problem:** There are no endpoints for:
  * settings;
  * renaming, reordering or archiving options and sub-options (only `POST /attributes/{id}/options` exists);
  * listing a client's appointment history or photo gallery (FR-C3, FR-P4);
  * archive or delete;
  * per-device session revocation (SAD:329).

  There is also no `push_subscriptions` table, and `POST /push/subscriptions` is hidden in a Notes cell.
* **Recommendation:** Add the endpoints and tables, or explicitly defer the FRs.

### M26. Overbuilt items for one owner on a home PC
* **Lenses:** zoom, design, details, struct, ready
* **Where:**
  * PRD:21, 105, 140 (p95 < 50 ms)
  * SAD:9, 296 (in-memory catalog cache), 300-304 (`/metrics`, EXPLAIN ANALYZE in CI on a 100k seed)
  * SAD:63, 338 (api/worker modes), 68, 402 (S3), 413 (multi-arch)
  * SAD:161 (GIN on `attrs`, which no FR queries; ADR 002 lists FTS, which nothing uses)
  * SAD:255 (ETag/304), 256 (Idempotency-Key)
  * PRD:109 (NFR-5)
* **Problem:**
  * At this volume indexed reads take under a millisecond. Over the tunnel, what she feels is network round-trip time, so a 50 ms server p95 says nothing about her experience. NFR-1 also does not say whether it means the server or the device.
  * An EXPLAIN gate with a 10 ms limit will be flaky.
  * The api/worker split contradicts the "one instance" cache (SAD:296): a separate worker would render stale labels.
  * Cloudflare weakens strong ETags when it compresses (`details`, moderate confidence).
* **Recommendation:**
  * Replace the latency targets with one test: "Hoy opens in under 1 s on her iPhone over LTE through the tunnel".
  * Ship only the `serve` mode and only a local blob store. Keep the `BlobStore` interface but ship one implementation.
  * Use one image.
  * Drop the GIN index, the in-memory cache, ETags and `/metrics`.

### M27. The Cloud API is woven into the core design, and Meta's rules conflict with templates she edits
* **Lenses:** zoom (Major), details (Nit now, Major in Phase 2)
* **Where:** PRD:85, 124, 147; SAD:18, 126, 224-225, 269, 278, 403-404, 421.
* **Problem:**
  * This is speculative Phase 2 design inside release 1.
  * Every template edit needs Meta re-approval, which conflicts with FR-R3.
  * Parameters cannot contain newlines (`details`, verified-from-knowledge).
  * Meta moved to per-template-message pricing in mid-2025 (`details`, verified-from-knowledge), and a refill nudge may be classed as marketing.
  * "Ventana de conversación" is outdated wording; the current term is the 24-hour customer service window.
  * Webhooks are missed while the PC is off.
  * Coexistence with the WhatsApp Business app on the same number is unverified.
* **Recommendation:** Cut it to a one-paragraph future note, keeping `sent_via` and the status enum compatible. Record these constraints against PRD:147.

### M28. On-device AI takes up an ADR, and iOS already covers most of it
* **Lenses:** zoom, details
* **Where:** PRD:89-97 (FR-I1 to FR-I5); SAD:85-105 (ADR 017).
* **Problem:**
  * The data is migrated by hand, so FR-I1 (scanning the notebook) has no purpose.
  * iOS already provides Spanish keyboard dictation (FR-I3), Live Text for SINPE receipts (roughly FR-I2) and subject lift in Photos (FR-I4).
  * runonweb is at v0.1.0, and its maturity is unverified.
  * Models of 30 to 548 MB are likely to hit PWA memory limits on iOS.
* **Recommendation:** Move it to a "future ideas" appendix. Do not design the `Ocr`, `Dictation` or `BackgroundRemover` interfaces now. If it comes back, re-justify each FR-I against what iOS already does. Transformers.js and Tesseract.js are fallbacks if runonweb stalls.

### M29. Web Push: the phase is undecided and the key storage contradicts the secrets rule
* **Lenses:** details, design, struct, zoom
* **Where:** PRD:87 vs PRD:124; SAD:73, 245, 269, 332 ("secretos nunca en la DB"), 405 (VAPID keys stored in the DB).
* **Problem:**
  * There is no table for the VAPID keys or for subscriptions.
  * If the keys are regenerated, every subscription dies silently.
  * VAPID needs a `subject`, and there is no `VAPID_SUBJECT` variable. Apple reportedly rejects invalid subjects (unverified).
  * iOS requires every push to show a visible notification.
  * Push cannot fire while the PC is off.
* **Recommendation:**
  * Decide the phase (see "Disputed").
  * If push stays: pick one place for the keys, add `VAPID_SUBJECT` and a subscriptions table, prune on 404/410, and send one summary push per run.
  * Reuse SherClockHolmes/webpush-go.

### M30. Document structure: the ADR index, missing ADRs, missing sections, and an inaccurate state-of-the-art claim
* **Lenses:** struct, zoom, comp, design, details
* **Where:** SAD:57-73 (the table jumps from 014 to 016; 015 and 017 exist only as sections; 014 is a pending question, not a decision, and is duplicated at SAD:432 and PRD:148); SAD:7 ("monolito modular" with no module map); PRD:9-13; the PRD as a whole.
* **Problem:**
  * The ADR table has no status or date. Superseded plans (SwiftUI, VPS/Caddy) are not tracked.
  * Several decisions have no ADR: ingress, template storage, the tray model, offline/sync, the PWA origin, the backup target.
  * PRD:13 says "no hay una opción gratuita que cualquiera pueda hospedar". That is false: Easy!Appointments, Cal.com and Odoo Community exist. The real gap is attribute-driven messages plus a phone-first setup she does herself.
  * Missing per the Heilmeier and feature-spec standards: cost, timeline, a mid-term check, product risks (the biggest one is that she stops using it and goes back to the notebook), a glossary, doc status and version, and a SAD module map with ownership and allowed dependencies.
* **Recommendation:**
  * Make one ADR index with ID, decision, status, date and link.
  * Add the missing ADRs.
  * Restate the justification honestly and add an "alternatives considered" section.
  * Add cost and timeline, with a target date for release 1 and a week-2 check ("she enters real appointments for 3 days").
  * Add the adoption risk with its mitigation (in-person setup, weekly check-ins).
  * Add a glossary: relleno, SINPE Móvil, bandeja, manual asistido, owner/emprendedora, anchor, the 24-hour window, xtool, and the two meanings of "template".
  * Add a module map.

### M31. Nail and Costa Rica specifics leak into the generic schema
* **Lenses:** struct, comp
* **Where:**
  * SAD:124-125 (`America/Costa_Rica`, `es-CR` as DDL defaults), 153, 166 (no currency), 357 (`POSTGRES_DB: citas`)
  * PRD:20, 80, 86, 94, 138 ("relleno", SINPE)
  * The template namespace mixes English and Spanish.
* **Problem:** Defaults baked into migrations become every self-hoster's data. The missing currency and country code will force a schema change later.
* **Recommendation:**
  * Set locale, time zone, currency and country code in the wizard, not in the DDL.
  * Write the generic requirements in generic terms ("rebooking reminder"), with "relleno" as the worked example.
  * Make SINPE parsing a pluggable receipt parser if it ever ships.
  * Let her exported configuration become the first "nail studio" preset, stored as data rather than code.

---

## Nits

| # | Finding | Lenses | Where | Recommendation |
| --- | --- | --- | --- | --- |
| N1 | Login returns the token in the body and also sets it in a cookie. A body token is readable by JS, so HttpOnly protects nothing. NFR-8 says "auth por token". | details, design | SAD:239 vs 328; PRD:112 | Use the cookie only. |
| N2 | `SameSite=Strict` drops the cookie on top-level navigations from WhatsApp, the setup link and maybe push taps (unverified on iOS). The separate cookie jar for installed PWAs needs re-checking on iOS 26. | details, design | SAD:328-329 | Use `Lax` plus an Origin check on writes. |
| N3 | A failed `If-Match` should return 412 (RFC 9110), not 409, with a quoted ETag. Only `appointments` has `version`. `zoom` says last-write-wins is enough for one user. | details, zoom | SAD:186, 253 | Fix the status code, or defer optimistic locking to multi-staff. |
| N4 | ID generation contradicts itself: "generated in Go" and "safe to create offline from the PWA". A UUIDv7 cursor does not match name or `starts_at` sorting, and device clocks skew. The Idempotency-Key store is missing. | details, design, zoom | SAD:113, 252, 256 | The client generates IDs, inserts use `ON CONFLICT (id) DO NOTHING`, and drop Idempotency-Key. Paginate by (sort key, id). |
| N5 | `whatsapp_mode` is in both the DB and env with undefined precedence. VAPID keys can come from env or the DB. | struct, zoom, design, details, ready | SAD:126 vs 403; 405 | Env holds infrastructure and secrets only; the DB holds owner-editable behavior. |
| N6 | `.env` and compose do not line up: `DB_PASSWORD` is unlisted, `DATABASE_URL` repeats the password, the port is hardcoded, and there is no trusted-proxy or `VAPID_SUBJECT` setting. distroless has no shell for a healthcheck and runs as non-root, so the media volume ownership needs setting. | details | SAD:340-406 | Align them and add a healthcheck binary subcommand. |
| N7 | "Tres contenedores" becomes four or five with backup and cloudflared. | details | SAD:23, 55 | Update the count after B2. |
| N8 | The template examples use `{bebida favorita}` in one place and `{{client.bebida_favorita}}` in another. The namespace for `{{services}}` and for appointment attributes, and the separator for multiselect values, are undefined. | comp | PRD:11 vs 82; SAD:274 | Use one form and define the variable namespace. |
| N9 | "A single `docker compose up`" vs "copy two files, edit the domain, plus `.env`". | comp | PRD:7 vs SAD:338 | State the real steps once. |
| N10 | Schema gaps: `media` can have both foreign keys NULL; FR-A6 "no rows" semantics (should mean all services); a key that is unique across scopes; archived keys can never be reused; no business hours. | struct, details, design | SAD:172, 191, 197-205, 133 | Add CHECKs and write down the rules. |
| N11 | The anchor enum is closed at two values. | struct | SAD:210 | Keep the CHECK, resolve anchors behind one Go interface, and avoid a rule DSL. |
| N12 | Information sits in the wrong document: the PRD holds the stack, iOS versions and default values. | struct | PRD:7, 87, 91, 115; SAD:89 | The PRD says what and why; the SAD holds how and the defaults. |
| N13 | The license is undecided. | design, struct | SAD:72, 432; PRD:148 | Decide before the repo goes public, not before her first day. Keep one copy of the question. |
| N14 | Sessions update `last_seen_at` on every request. | details | SAD:323, 329 | Throttle the writes. |

---

## (a) Alternatives to weigh or reject

| Alternative | Lenses | Recommendation |
| --- | --- | --- |
| SaaS (AgendaPro, Fresha, Booksy) | zoom | Reject, as the PRD already does. They lack custom attributes in messages and the data lives elsewhere. |
| Easy!Appointments, Cal.com, Odoo Appointments | zoom, details, struct | Reject as the product: they are built around public booking, are heavy (Odoo may be Enterprise-only, unverified), and are weak on custom fields in messages. They disprove PRD:13, so name them in the PRD. |
| Twenty or EspoCRM | zoom, details | Reject. General CRMs, heavy for 1 GB (Twenty needs Redis and workers), not tuned for appointments or the iPhone. |
| NocoDB or Baserow with a formula that builds wa.me links | zoom | Reject. She would edit formulas, there is no rebooking logic without n8n, and URL encoding in the formula is unverified. |
| Google Sheet + Apps Script, or iOS Contacts + Calendar + Shortcuts | zoom | Weigh as a stopgap she could use this week while the app is built. Reject as the product. |
| A read-only ICS calendar feed (secret-token URL) subscribed in iOS Calendar | zoom | Weigh as an add-on. iOS caches subscribed calendars, so she sees her agenda while the PC is off (unverified for long downtimes). It may replace much of the offline work. |
| A tray computed by query plus a sent log, instead of an outbox and worker | zoom, design, struct, ready | Weigh seriously for release 1 (M9). Bring the outbox back with Cloud API. |
| SQLite plus Litestream instead of Postgres | zoom, design (weigh); ready (reject) | Split. It fixes off-site backup and removes a container. Postgres is defensible for familiarity and portfolio reasons; if kept, state that as the reason rather than SKIP LOCKED or GIN. |
| Dropping Caddy for the tunnel install | all | Adopt: cloudflared points at `app:8080`, and Caddy stays an optional VPS profile. Reject Go autocert. |
| Cloudflare Access in front of the app | zoom, struct, design | Reject for release 1. Its interaction with standalone PWA cookies and push is unverified. Possibly use it only for `/setup`. |
| Passkeys / WebAuthn | comp, design | Weigh later, once tested on her iOS version. Password plus CLI reset for now. |
| Full window refresh instead of delta sync | design, zoom, ready, details | Adopt for release 1 (M10). `struct` dissents. |
| Client-side image resize instead of server-side WebP | design, zoom, details | Adopt (M13). Reject govips/libvips because of cgo. |
| Auto-complete or a "close the day" prompt vs relying on the complete tap | comp, design, ready, details | Adopt (B7). |
| A business-wide "before" rule vs per-service rules | zoom, comp, ready, design | Adopt (M5). |
| A message template library vs inline text per rule | design, struct, ready, details | Adopt (M4). |
| Offline writes with conflict merge | all | Reject for release 1. A create-only outbox is disputed. |
| Rendering templates on the client while the server is down | struct | Reject: two renderers would drift. |
| A rule DSL or expression engine | struct | Reject (YAGNI, and she is non-technical). |
| `pg_jsonschema` or EAV tables | struct | Reject. Keep JSONB plus one validator package. |
| api/worker split, S3, Prometheus, Idempotency-Key storage, in-memory cache | design, zoom, struct, details | Reject for release 1 (M26). |
| Unofficial WhatsApp libraries (whatsmeow, Baileys) | details | Reject. They break Meta's terms and risk a ban on her business number. |
| Libraries to reuse | details | goose with embed.FS (verified), webpush-go, x/crypto/argon2 or alexedwards/argon2id, x/time/rate, disintegration/imaging, goodsign/monday, prodrigestivill/postgres-backup-local plus restic or rclone, cloudflare/cloudflared, @vite-pwa/sveltekit, Dexie.js, openapi-typescript plus openapi-fetch or ogen. River (riverqueue) for the Phase 2 Cloud API. Check whether scs stores session tokens unhashed (unverified) before adopting it. |

---

## (b) Release-1 scope (consolidated proposal)

### In scope
* **Handover and auth:** in-person handover by the author (setup link, then a one-time code entered inside the installed PWA); cookie sessions with sliding expiry; CLI `create-owner` and `reset-password`; a login rate limit that works behind the tunnel.
* **First-run wizard:** business name, time zone, currency, country code, and minimal branding (name, icon, color) feeding a dynamic manifest, then the iOS install guide. A small Spanish starter kit with a sample client for previews. Empty states that point to the next action.
* **Clients:** quick add, phone normalization to +506, duplicate-phone warning, accent-insensitive search, archive, photo gallery and history.
* **Attributes:** shaped by the interview with her. Flat types, at most one level of sub-options, keys hidden and generated, scope asked as a plain question, a written value contract, evolution rules, and "required" enforced only on create.
* **Services and appointments:** services with price and duration; appointments with several services, a price snapshot, payment method as a select, and an amount; day view and a simple week view; clash warnings; a defined state machine with CHECKs; auto-complete or a "close the day" prompt.
* **Photos:** resized on the client, stored on a local volume, served only with a session.
* **Templates and rules:** a template library with a chip editor and live preview. One business-wide "before" reminder. Per-service rebooking rules that skip when any future booking exists, and re-arm when that booking is cancelled. Lateness and expiry rules.
* **Sending:** the "Para enviar hoy" tray computed by query, grouped by day; wa.me with a "¿Lo enviaste?" confirmation and step to the next; owner reminders as tasks; a sent log with `sent_at`.
* **Offline:** read-only from a window refresh cache, a "server off" banner with the last-sync time, local drafts, and a local queue for "sent" confirmations.
* **Deploy and operations:** app, postgres, cloudflared and a mandatory backup service; off-site encrypted copies; backup at startup when the last is more than 24 h old; a dump before each migration; one manual restore drill before real data; pinned tags; worker heartbeat and last-backup status in the app; a client error log.
* **Metrics and tests:** audit columns for the metrics, and acceptance tests from M18.
* **Documents:** an ADR index including ingress, templates, the tray model, offline and the origin. The PWA origin fixed permanently.

### Non-goals for release 1
* WhatsApp Cloud API (keep `sent_via` and the status enum compatible).
* Offline creation or editing of appointments; offline photos; conflict merging; delta `/sync`.
* All on-device AI (FR-I1 to FR-I5).
* S3, the api/worker split, Prometheus `/metrics`, the EXPLAIN ANALYZE CI gate, the p95 SLO, multi-arch images, NFR-5 VPS sizing, ETag/304, the JSONB GIN index, FTS, Idempotency-Key, the in-memory catalog cache.
* Caddy plus Let's Encrypt, except as an optional profile for other self-hosters.
* The image attribute type, multi-level option trees, and service-scoped attributes (unless the interview needs them).
* Meta slot mapping. The template fallback syntax only if the interview shows no need.
* Hard delete (unless the cascade is fully specified).
* Staff and roles, public booking, several professionals, English UI (keep strings external if cheap), reports, per-business-type template packs.
* The "stranger installs in 10 minutes" target and contributor readiness (NFR-4, NFR-5, NFR-9), which move to the Phase 3 gate. OpenAPI codegen and its CI gate are optional, as a portfolio choice.
* Restore testing in CI (Phase 2; release 1 has the manual drill).
* Choosing the license (decide before the repo goes public).

### Disputed between lenses: the author must decide
1. **Web Push in release 1.**
   * `design`: include it, with one summary push per run, because the tray only works if she is prompted.
   * `zoom`, `comp`, `ready`, `struct`: defer it. She opens the app daily, the tray badge or an iOS Shortcuts automation at a fixed morning time is enough, and push cannot fire while the PC is off.
   * `details`: decide explicitly; it is cheap.
2. **Offline writes.**
   * Majority: read-only with blocked writes and local drafts.
   * `ready`: an outbox for creates only, replayed on app open using client-generated IDs. Its reasoning is that downtime during studio hours pushes her back to the notebook and threatens the 4-week gate.
3. **Sync mechanism.**
   * `design`, `zoom`, `ready`, `details`: a window refresh is enough.
   * `struct`: build the `updated_at`, tombstone and change-sequence design now, under an ADR.
4. **Export in release 1.**
   * `design`, `struct`, `ready`: ship a minimal JSON/CSV export as her portability guarantee.
   * `zoom`: pg_dump covers her.
   * `details`: align PRD:111 (JSON plus CSV) with SAD:248 (which adds photos).
5. **Sub-options.**
   * `zoom`: drop them.
   * `comp`: at most one level.
   * `design`, `ready`, `struct`: keep them with a value contract.
   * The interview with her settles this.
6. **Database.**
   * `zoom`, `design`: weigh SQLite plus Litestream.
   * `ready`: keep Postgres.
   * Whichever is chosen, restate the reason honestly.

---

## Claims flagged as unverified (check on her iPhone or against current docs)
* How `wa.me` opens from a standalone PWA on iOS 26, and which app wins when both WhatsApp and WhatsApp Business are installed. Practical URL and text length limits.
* Whether iOS updates the home-screen icon and name after install. Whether iOS 26 opens home-screen additions as web apps by default. Persistent Storage from Safari 17. The separate cookie jar for installed PWAs. The current "Add to Home Screen" path.
* `SameSite=Strict` behavior after a push tap.
* Whether Safari's file input always converts HEIC to JPEG. The state of pure-Go WebP encoding.
* oapi-codegen support for OpenAPI 3.1 in its current release.
* Cloudflare free-plan upload limit (100 MB), the 524 timeout (about 100 s), ETag weakening, and default caching or Rocket Loader effects on the service worker.
* Meta Cloud API current pricing and template rules. Coexistence with the WhatsApp Business app. Opt-in terms.
* runonweb maturity, and model memory use in an iOS PWA.
* The PG 18 Docker image data-path change.
* Costa Rica Ley 8968 obligations.
* Whether she offers separate "new" and "refill" services (PRD:146, still open).
