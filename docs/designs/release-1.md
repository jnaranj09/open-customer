# open-customer — Release 1

*   **Author:** jnaranj09
*   **Status:** Accepted. P0–P7 are built; the P7 exit is pending: CI on the pushed branch, the release workflow and upgrade.sh end to end (both need a v* tag), and the on-PC and iPhone checks in the Verification Plan. No owner interview is needed: the app ships empty and she sets up everything herself.
*   **Date:** 2026-10-30
*   **Predecessors:** [concepts_brief.md](history/concepts_brief.md), [PRD](../prd.md), [SAD](history/sad-original.md)

--------------------------------------------------------------------------------

## Overview

open-customer is an open-source, self-hosted app to manage clients, appointments and WhatsApp reminders for a small service business.

Release 1 is built for one real user: the owner of a nail studio that is already open.
She is not technical and works from her iPhone, often with one hand busy.
She gets an empty app, installs it on her Home Screen, and sets up her own services, client attributes and reminder messages without code.

The core value is personal reminders.
A message like "Aquí te esperamos con tu café negro" is built from attributes she defined herself.
It also tells her when to ask a client to come back for a refill (relleno), a set number of days after a visit.

Everything that can be generic stays generic: attributes, branding, templates, time zone, currency, country code.
Another business (barber, spa, tattoo studio) can run the same code with its own setup.

The first install runs on the author's home PC behind a Cloudflare Tunnel.
The PC is sometimes off. The owner accepts that downtime. While the server is off, the app must open from the phone's cache and say so on screen.

Priorities when goals conflict, in order:

1.  Her daily use and her ability to set it up herself.
2.  Data safety.
3.  Reminders are correct.
4.  Maintainable by one person.
5.  Generic and self-hostable by others.

This document replaces the architecture parts of the SAD for release 1. The SAD is reduced to a Spanish ADR index ([docs/sad.md](../sad.md)) that links here (item 13), so architecture detail lives in one place.
The PRD ([docs/prd.md](../prd.md)) keeps the "what" and "why", updated to match this document (see [Detailed Implementation Plan](#detailed-implementation-plan), item 12).

### Terms used in this document

*   **Phases** (from the PRD): Phase 1 is this release. Phase 2 adds automation (WhatsApp Cloud API, OCR, dictation); its gate is 4 weeks of daily use without the notebook. Phase 3 opens the project to others (public booking, presets, staff, English UI); its gate is one outside install done from the docs alone.
*   **Come-back**: the follow-up visit some services need after N days. For nails, that visit is a refill (relleno); this document always calls it "come-back". A `come_back` rule reminds her to message the client. It names a **return service**, the service the client should book next (empty means the rule's own service); the rule on "Acrílico" says "21 días después → Relleno".
*   **Booking prompt**: a come-back item shown to her instead of a client message, when the client already has a future appointment without the return service: "Ana tiene cita el 12/11 (Pedicure). ¿Agregar Relleno?" (see "Reminder rules").
*   **The two reminder kinds**: `before_appointment` (in prose: a "before" rule or item) and `come_back` (in prose: a "come-back" rule or item). Code spans use the enum values; prose uses only these two names.
*   **"Hoy"**: the home screen and the first tab. It shows the count of messages due today, which links to "Para enviar hoy", and the appointments for the chosen day or week.
*   **"Para enviar hoy"**: the screen with today's reminder messages, opened from the "Enviar" tab or from the count on "Hoy" (the PRD calls it "bandeja").
*   **"Clientas" and "Ajustes"**: the other two tabs, the client list and the settings.
*   **Manual asistido**: the app prepares each WhatsApp message and she sends it by hand through `wa.me`.
*   **SINPE Móvil**: Costa Rica's phone-number bank transfer, a common way clients pay. The app has no built-in payment-method field; she can create a select attribute for it if she wants one.
*   **Owner**: the business owner, the only user in release 1.
*   **The author**: the person who installs and runs the system (upgrades, restores, password resets). For the reference install, that is the person who wrote this design.
*   **"Item N"**: the numbered entry N in the [Detailed Implementation Plan](#detailed-implementation-plan).
*   **P0–P7**: the build phases in "Build order". They are not the product Phases 1–3 above.
*   **"Enviados"**: the list of today's sent messages, below "Para enviar hoy".
*   **The review / the SAD bug**: the brainstorm review of the PRD and SAD ([goldfish_brainstorm_findings.md](history/goldfish_brainstorm_findings.md)). Its main reminder bug: the SAD's skip rule counted the appointment itself as a "future appointment", so every 24 h reminder was skipped.
*   **EGM**: the design-review workflow used to write and review this document.
*   **FR-… and NFR-…**: functional and non-functional requirement IDs from the PRD.

## The Problem

Today she keeps clients, appointments and reminders across WhatsApp chats, a notebook and her memory.
Because of that:

*   Reminders depend on her remembering. Clients miss appointments, and come-back visits are not booked on time.
*   Details she knows about each client (favorite drink, nail shape, allergies) are not written anywhere she can search, and are never used in messages.
*   Nothing tells her which messages she should send today.

Existing tools do not fit:

*   **SaaS** (AgendaPro, Fresha, Booksy): custom fields are limited and usually cannot be used inside messages. Data lives on someone else's platform, with a monthly fee and paid SMS.
*   **Free self-hosted tools** (Easy!Appointments, Cal.com, Odoo Appointments): built around public booking pages, heavy to run, and weak at putting custom fields into messages.
*   **General CRMs** (Twenty, EspoCRM): not built for appointments or for one-hand use on an iPhone.

The gap is the combination: her own attributes, used inside her own messages, set up by her from a phone.

Proposed solution, in short:

*   A Go server with PostgreSQL 18, and a SvelteKit web app embedded in the same binary and installed as a PWA on her iPhone.
*   The server computes the list of messages to send today from her rules and appointments.
*   She sends each one with a tap that opens WhatsApp with the text and number filled in (`wa.me`), then confirms it was sent.
*   One push notification per day tells her how many messages are waiting.

## Goals

Each goal has a check. Release 1 is done when all checks pass.

1.  **She sets up the empty app herself.**
    Check: on her own iPhone, without help, she creates her services, at least 5 attributes and one reminder template in under 20 minutes.
2.  **Booking is fast.**
    Check: creating an appointment with client, service(s), date and attributes takes under 30 seconds for an existing client.
3.  **Reminders are correct.**
    Checks:
    *   The due-list tests in `internal/reminders` (`due_test.go`, `due_comeback_test.go` and the `DueList` database tests in `due_db_test.go`) have a passing case for every row and bullet of the rules table, including the last four checks below.
    *   During the 4-week gate (Goal 4), every reminder she expected but did not see is logged as a bug, and none is open at the end. A reminder missed because the PC was off is not a bug (see "Accepted risks").
    *   A "before" reminder is never shown after the appointment has started.
    *   A come-back reminder is never missing because she forgot to mark an appointment as done.
    *   A client with several services on one appointment gets one "before" message, not one per service.
    *   A client who already has a future booking that includes the come-back rule's return service does not get a come-back reminder. With a future booking without it, the owner gets a booking prompt instead of a client message.
4.  **She uses it every working day.**
    Check: 4 weeks of daily use without going back to the notebook (the PRD's gate to Phase 2). Days with the PC off do not count as misses and do not restart the 4 weeks.
5.  **Her data is safe and is hers.**
    Checks:
    *   While the PC is on, backups run at most 25 h apart (the hourly check runs a backup once the last one is 24 h old), and each copy lands on a second disk or machine. The first install keeps `BACKUP_TARGET` on a folder of the same laptop that runs the stack (owner decision), so this check is met only once `BACKUP_TARGET` is on another disk or machine.
    *   One full restore on a clean install succeeds before her real data goes in.
    *   She can download her data as CSV + JSON from the app, without the author.
6.  **"Hoy" opens in under 1 s, and the app shows when the server is off.**
    Checks:
    *   The "Hoy" screen opens in under 1 second on her iPhone over LTE through the tunnel.
    *   With the server off, "Hoy" opens from the phone's cache, shows a "server off" banner with the last sync time, and never shows a change as saved when it was not.
7.  **Nothing business-specific is hardcoded.**
    Check: no nail, Costa Rica or Spanish-only value lives in code or migrations as data. Time zone, currency, country code, business name, branding, services, attributes and templates are all set in the app. One fixed product value is accepted: no push is sent after 20:00 business time. The app ships with no starter content: every service, attribute, template and rule is data she creates.
8.  **One person can run it.**
    Check: the install is one Compose file with four always-running services (app, postgres, cloudflared, backup) plus a `restore` service that runs only during a restore, no published ports, pinned image tags, and upgrades that take a backup first.

## Non-Goals

*   **WhatsApp Cloud API** (automatic sending through Meta). Release 1 is manual send through `wa.me`. Data fields stay compatible with adding it later.
*   **A reminders queue and background worker.** The list is computed when read. A queue comes back with the Cloud API.
*   **Creating or editing data while the server is off.** Release 1 is read-only offline. No conflict merge, no delta sync.
*   **On-device AI** (notebook OCR, receipt reading, dictation, background removal, photo tags). iOS already covers dictation and text capture.
*   **Scale and multi-host features:** S3 storage, separate API and worker processes, Prometheus metrics, query-time gates in CI, a p95 latency target for the server, multi-architecture images, ETag caching, in-memory caches, JSONB indexes.
*   **Caddy and Let's Encrypt.** Neither the reference install nor release 1 ships them. A self-hoster without a tunnel can put any reverse proxy in front of `app:8080`; a tested VPS profile comes with the Phase 3 gate.
*   **Photos in the export.**
*   **Hard delete.** Records are archived. A deletion request is handled by the author with SQL.
*   **Image attribute type, typed values per option (a "group" type), attributes limited to some services.** Each can be added later, when a real need shows up in her daily use.
*   **Staff accounts, public booking, English UI, reports, preset packs per business type.**
*   **"A stranger installs it in 10 minutes"**, VPS sizing targets and contributor readiness. These move to the Phase 3 gate.
*   **Restore tests in CI.** Release 1 has one manual restore drill.
*   **Choosing the license.** Decided before the repo goes public.

## Open Questions

*   **Her real attributes and services** (closed by owner decision): there is no interview. The app ships empty, and she defines every service, attribute, template and rule herself (Goal 1, CUJ 2). The come-back question it held (should any later visit cancel a pending come-back?) is answered in "Reminder rules": only a later booking that includes the rule's return service skips it, and a future booking without it gives her a booking prompt.
*   **WhatsApp link behavior on her iPhone**: Does `wa.me` opened from the installed PWA go straight to WhatsApp or through Safari? Does `whatsapp://send` behave better? Do accents, emoji and line breaks survive? Which app opens if both WhatsApp and WhatsApp Business are installed? Source: a test page on her iPhone through the tunnel.
*   **Exact subdomain**: Which subdomain of the author's domain. Must be fixed before she installs the PWA.
*   **iOS behavior to confirm on her device**: whether iOS updates the Home Screen icon and name after install; whether Safari's photo picker always gives JPEG instead of HEIC; whether a `SameSite=Lax` session cookie is sent when she opens the app from a push notification; whether "Descargar mis datos" (a ZIP download) works from the installed PWA.
*   **Cloudflare limits**: the current free-plan upload size limit and response timeout, which affect photo upload and export.
*   **Costa Rica Ley 8968**: whether archive-only deletion meets her obligations for client personal data. Must be answered before her real data goes in (see "Risks").

## Critical User Journeys (CUJs)

### CUJ 1: Handover and first setup

*   **Given**: The author has started the Compose stack on his PC. The tunnel points the chosen subdomain at the app. The database is empty.
*   **When**:
    1.  The author copies the one-time setup link from the app logs (`docker compose logs app`) and opens it with her, on her iPhone in Safari.
    2.  The setup wizard asks for business name, time zone, currency, country code and accent color, then her username and password. After that it asks for an optional icon (the upload needs the session that setup creates).
    3.  The wizard shows the iOS install steps (Share → Add to Home Screen).
    4.  She opens the installed app. It shows the login screen. She logs in with her username and password.
    5.  She taps "Activar notificaciones", and then Allow in the iOS prompt (iOS asks only after a tap).
*   **Then**:
    *   The installed app shows her business name and icon.
    *   She has a session in the installed app (separate from Safari).
    *   One push subscription is stored.
    *   The setup link no longer works.

### CUJ 2: She configures the empty app

*   **Given**: Setup is done. The app is empty: no services, attributes, templates or rules. The only built-in data is a sample client used for previews. The sample client lives in code (`internal/templates/sample.go`); it is never stored, and never appears in the client list or the export.
*   **When**:
    1.  She creates services (name, duration, price, color), for example "Acrílico", "Relleno" and "Pedicure".
    2.  She creates client attributes ("Bebida favorita", text) and appointment attributes ("Forma", select).
    3.  She writes two messages by tapping variable chips ("+ Bebida favorita"): "Recordatorio de cita" and "Relleno". She adds a "before" rule that sends "Recordatorio de cita" 1 day before each appointment.
    4.  She adds a come-back rule to "Acrílico": "21 días después, enviar plantilla 'Relleno'", with "Relleno" as the return service.
*   **Then**:
    *   Before each first step, the screen that needs it shows an empty state that leads to it: no services yet → Ajustes → Servicios; no templates → Ajustes → Plantillas; no rules → Ajustes → Recordatorios. The texts live in `es.json`.
    *   Each attribute gets a hidden key generated from its label.
    *   The message editor shows a live preview against the sample client.
    *   She never sees `{{ }}` syntax, keys, "scope" or "offset".
    *   All of this works on her iPhone, one screen at a time.

### CUJ 3: Booking an appointment

*   **Given**: The client "Ana" and the services from CUJ 2 exist. The server is on.
*   **When**:
    1.  She opens "Hoy" and taps "+".
    2.  She searches "ana" (finds "Ána" too).
    3.  She picks "Acrílico" and "Pedicure".
    4.  She picks Saturday 3:00 p.m. and sets "Forma: Almendra".
    5.  After the appointment, she opens it and attaches a photo of the finished set from the camera or the photo library.
*   **Then**:
    *   The appointment is saved, with the two services' prices and durations copied into it.
    *   The end time is the sum of the durations.
    *   If it overlaps another appointment, she sees a warning and can choose "Agendar igual".
    *   Steps 1–4 take under 30 seconds.
    *   The photo shows on the appointment and in Ana's gallery, without GPS data (see "Photos").

### CUJ 4: Morning sending

*   **Given**: Ana's appointment is tomorrow at 3:00 p.m. The "before" rule is 1 day. Her push time is 8:00. Two other items are also due today. The server is on at 8:00.
*   **When**:
    1.  At 8:00 her phone shows "Tienes 3 mensajes para enviar hoy".
    2.  She taps it. The app opens on "Para enviar hoy". She taps "Enviar" on Ana's item.
    3.  WhatsApp opens with Ana's number and the rendered message.
    4.  She sends it and returns to the app.
*   **Then**:
    *   The app asks "¿Lo enviaste a Ana?".
    *   She taps "Sí". The item moves to "Enviados" with the exact text that was sent.
    *   The app moves to the next item.

### CUJ 5: Come-back reminder lifecycle

*   **Given**: Ana had "Acrílico" on October 1, ending at 4:00 p.m. "Acrílico" has a 21-day come-back rule whose return service is "Relleno". Nobody tapped anything after the appointment.
*   **When**: October 22 arrives.
*   **Then**:
    *   The come-back message for Ana appears in "Para enviar hoy", because the appointment time passed and it was not cancelled.
    *   If Ana had booked a later appointment that includes "Relleno", the message does not appear.
    *   If that later appointment is cancelled, the message appears again.
    *   If Ana's only later booking is a "Pedicure" on November 12, the owner sees a booking prompt instead of the message: "Ana tiene cita el 12/11 (Pedicure). ¿Agregar Relleno?". "Agregar a la cita" adds "Relleno" to that appointment and the item disappears; "Enviar mensaje" and "Omitir" work as for any item.
    *   If that "Pedicure" was on October 15, it is in the past on October 22, so the normal message appears.
    *   If Ana did not come on October 1 and the owner marks "No vino", the message never appears.

### CUJ 6: Server off

*   **Given**: The PC is off. She used the app yesterday.
*   **When**: She opens the app.
*   **Then**:
    *   "Hoy" shows yesterday's cached data at once.
    *   A banner says "Sin conexión con el sistema. Datos de las 18:42." (the same text whether her phone or the PC is offline).
    *   "+" and edit buttons explain that changes need the server.
    *   A form she had started is kept as a draft.
    *   She can read the cached "Para enviar hoy" list, but "Enviar" and "Omitir" are disabled with the same message. The items are still there when the server is back.
    *   With the app left open, the banner goes away by itself once the server answers again.

### CUJ 7: Backup, upgrade and restore

*   **Given**: The backup folder (a removable disk, if the owner uses one, mounted) exists and has the target marker file.
*   **When**: The PC has been off for two days and is turned on. Later, the author upgrades the app.
*   **Then**:
    *   As soon as the backup container starts, it sees that the last backup is older than 24 h and runs one.
    *   The app's status screen shows "Último respaldo" with the day and time of the last copy, for example "5/11 10:05" (the screen shows no disk label).
    *   During the upgrade, the upgrade script runs a backup before the new version starts.
    *   The new version will not run migrations unless a backup copy from the last 60 minutes exists.
    *   A rollback restores the snapshot taken before the upgrade and also puts back the app version that wrote it, so the old migrations are not re-run on a newer schema.
    *   A restore from any of the last 14 daily copies brings back the database and the photos.

### CUJ 8: Export

*   **Given**: The server is on.
*   **When**: She opens Ajustes → "Descargar mis datos" and taps the button. The button is a plain link to `/api/v1/export` with the `download` attribute, so the browser downloads the streamed ZIP itself (no fetch into memory).
*   **Then**: Her phone downloads a ZIP with clients, appointments, services, attributes, templates and rules as CSV and JSON. Client and appointment attributes appear as one column each, named by their label.

## Technical Plan (Architecture)

### System view

```mermaid
flowchart LR
  phone["<b>iPhone PWA</b><br/>SvelteKit SPA<br/>Dexie cache (IndexedDB)<br/>service worker"]
  edge["<b>Cloudflare edge</b><br/>TLS, HSTS"]
  subgraph pc["Author's PC · docker compose"]
    cfd["<b>cloudflared</b><br/>outbound tunnel only"]
    app["<b>app</b> · one Go binary<br/>HTTP API /api/v1 + SPA<br/>daily timer goroutine"]
    db["<b>postgres:18-alpine</b>"]
    bk["<b>backup</b><br/>postgres:18-alpine + restic<br/>check at start + hourly"]
    media[("media volume")]
    status[("backup-status volume")]
    usb[("Backup folder<br/>restic repository")]
  end
  push["Web Push service<br/>(Apple)"]
  wa["WhatsApp app<br/>on her phone"]

  phone -- HTTPS --> edge --> cfd --> app
  app --> db
  app --> media
  app -. reads .-> status
  app -- VAPID push --> push --> phone
  bk -- pg_dump --> db
  bk -- reads --> media
  bk -- writes --> status
  bk -- restic --> usb
  phone -- "wa.me link" --> wa
```

*   No container publishes a port. cloudflared opens an outbound connection to Cloudflare, and Cloudflare routes the subdomain to `http://app:8080`.
*   TLS ends at Cloudflare. Traffic between Cloudflare and the PC goes inside the tunnel.
*   WhatsApp messages leave from her phone, not from the server.

### Process model

One binary, `open-customer`, with subcommands:

| Subcommand | What it does |
| --- | --- |
| `serve` | Runs migrations (with the backup guard below), then starts the HTTP server and the daily timer. This is the container command. |
| `reset-password <username>` | Sets a new temporary password and prints it. She must change it at next login. |
| `setup-link` | Deletes any old setup token, creates a new one (valid 24 h) and prints the link. Works only while no user exists. |
| `healthcheck` | Calls `GET /healthz` on localhost and exits 0 or 1. Used by the Compose healthcheck, because the image has no shell. |
| `version` | Prints the build version. |

The **daily timer** is one goroutine in `serve`. Every minute it does two things:

1.  Writes a heartbeat time to `scheduler_state`.
2.  If the business-local time is past the push hour, and today's push has not been sent yet, and it is before 20:00, it counts the items `DueList` returns (see "Due-list computation"; late items included, the same list as "Para enviar hoy") and sends one summary push when the count is above 0. Then it records today's date in `scheduler_state`, even when the count was 0, so a quiet day does not re-check every minute and no second push goes out later that day. If the count fails (for example the database is not ready yet after boot), it records nothing and tries again the next minute. When it sends, it also records the last push result in `scheduler_state` (`last_push`: the date, how many subscriptions it tried, how many accepted it). If the PC was off at the push hour, the push goes out when the PC comes back, up to 20:00. Before setup there is no settings row, so the timer writes only the heartbeat. If the send fails before it tries any subscription, it records nothing and tries again the next minute; if it stops part way (deleting a gone subscription failed), it logs `code=push_send_partial` and still records the day and `last_push`, so the morning push is not sent again every minute.

The timer is `scheduler.Run(ctx, ticks)`: it calls `Tick` once at start and then on every value from `ticks`, until `ctx` is done. `serve` passes `time.NewTicker(time.Minute).C`, cancels the timer on shutdown and waits for `Run` to return. A failed `Tick` returns its error, and `Run` logs `code=scheduler_tick_failed` and tries again on the next tick. Tests call `Tick` directly with the fake clock, never sleeping. The push sender is the one interface (`scheduler.Sender`), so the tests use a fake.

Nothing stores a "completed" status: `IsDone` computes it on every read (see "Reminder rules" below), so no stored value can disagree with it.

There is no queue, no `SKIP LOCKED` and no retry state.

### Code layout and module rules

Monorepo. The Go module is the repo root. The web app lives in `web/`.

```text
cmd/open-customer/        main + subcommands
api/openapi.yaml          the API contract (OpenAPI 3.0.3), source of truth
internal/
  platform/               config, logging, clock, ids (UUIDv7), errors (RFC 9457), backupstatus (reads status.json), color, files, text (trim and length checks, the notes rule), position (the 0..Max position rule), archive (the archived_at rule), nullval (nullable fields to and from pointers)
  db/                     pgx pool, migrations (goose, embedded), sqlc output
  httpapi/                generated server interface, handlers that call domain modules, middleware (incl. the Origin check)
  auth/                   users, sessions, setup tokens, rate limiting
  settings/               business settings, branding, manifest, icons
  attributes/             definitions, options, value validator
  clients/                clients, phone normalization
  services/               service catalog
  appointments/           appointments, status rules, overlap check
  media/                  photo storage
  templates/              message templates + renderer (own Mustache-like syntax)
  reminders/              rules, due-list computation, reminder log
  push/                   VAPID keys, subscriptions, sending
  scheduler/              daily timer
  exporter/               ZIP export
  status/                 health, readiness, status screen data
  locale/                 date/number formatting, Spanish day and month names
web/                      SvelteKit app (adapter-static, Tailwind CSS v4, own components), embedded with //go:embed all:build
deploy/                   compose.yaml, .env.example, backup image, scripts
design/                   soft-glass.pen (Pencil: the tokens and reusable components), design-map.json (names that break the default mapping)
docs/                     design docs, ADRs
```

Dependency rules:

*   `httpapi` calls domain modules. Domain modules never import `httpapi`.
*   Domain modules use `db` (sqlc queries), `platform` and `locale`; any package may import those three. They import other domain modules only where listed here:
    *   `clients` and `appointments` → `attributes` (`Validate`, called with the same `q`);
    *   `appointments` → `services` (`GetMany`, to copy each service's name, price and duration into the appointment);
    *   `templates` → `attributes` (`List`, for the variable list and the save check);
    *   `reminders` → `templates` (`Render` and the sample data), `appointments` (`IsDone`, a plain function on a status and an end time) and `attributes` (`List` and `Format`, to render values);
    *   the orchestrators `scheduler`, `exporter` and `status` → the modules they call. `exporter` calls `attributes.Format` and `appointments.IsDone`; it reads `reminder_log` through `export.sql`, not through `reminders`. `status` calls only read functions: `push.CountSubscriptions`, `scheduler.State` and `db.Version` (the goose version).
*   `auth` imports no domain module. Setup needs one thing from another module, and `cmd/open-customer` passes it to `auth` as a function when it builds it: the settings creator (a closure over `settings.Create`, which also validates the time zone, currency and country; item 40). `auth` defines the type of that function and of its input, so it never names a `settings` type. Setup writes nothing else: no starter content.
*   Modules are concrete Go types; `httpapi` holds them directly. An interface is added only where a test needs a fake (`push` for the scheduler tests). The method list in item 32 is the contract.
*   **Transactions are passed explicitly.** Every module function takes `(ctx, q db.DBTX, …)`, except the two in-memory limit checks `auth.BeginLogin` and `auth.BeginPasswordChange`, which touch no table and run before the handler opens its transaction (item 41). `db.DBTX` is the interface sqlc generates; both the pool and a `pgx.Tx` satisfy it. Who opens the transaction:
    *   **Every non-GET handler that reads or writes the database** runs its module call inside `db.WithTx` (read-write) and passes the `pgx.Tx`. It commits before it writes the response, so a failed commit is never shown to her as saved (Goal 6). A module write with several statements (an appointment and its service lines, a password change and its session revokes) is then all or nothing. Setup is the same: the `POST /setup` handler opens the transaction for `auth.CompleteSetup`.
    *   **The four snapshot readers** use a read-only `REPEATABLE READ` transaction: bootstrap, `GET /reminders/due` and the daily timer (all three call `DueList`), and export.
    *   **Every other GET handler** passes the pool.
*   **Modules have their own types.** Each module defines plain input and output structs (for example `clients.Client`, `clients.CreateInput`). Handlers convert between them and the generated `gen` types. Modules never import `gen`.
*   **Each module writes only its own tables.** Its `.sql` file (item 29) holds only its own queries. Read-only exceptions (SELECT only): `appointments.sql` reads `clients` (`id`, `name`, `archived_at`) for `client_name` and the archived-client check; `media.sql` reads `appointments` (`id`, `client_id`) and `clients` (`id`, `archived_at`) for the ownership rule; and two larger ones: `reminders.sql` reads `appointments`, `appointment_services`, `clients`, `services`, `message_templates` and `business_settings` with SELECT only, so `reminders` does not import those modules (attribute definitions and options come through `attributes.List`); `export.sql` reads every table.
*   Nothing imports `reminders` except `httpapi`, `scheduler` and `cmd/open-customer`.
*   `cmd/open-customer` wires everything and may import every package.
*   Web (`web/src`): `lib` (components included) never imports `routes`; shipped code never imports `tests/`; the service worker imports only the SvelteKit worker modules and `lib` (guard `layers`). The primitives in `lib/components/ui/` (one per reusable component of `design/soft-glass.pen`, guard `design-parity`) are the only files that may use surface classes (`glass-card`, `glass-panel`, `bg-glass*` fills, `shadow-*`, `backdrop-*`) and radius classes (`rounded-*`); routes and feature components compose them, and padding on a `Card` comes from its `pad` prop. No file uses an arbitrary Tailwind value (`-[…]`). Lint enforces both (`eslint-plugin-better-tailwindcss` `no-restricted-classes`, `no-restricted-syntax`); the uses that existed when the rule landed are pinned in `eslint-suppressions.json`. A primitive imports `lib` modules, `icons/`, other primitives, and the offline-aware wrappers `WriteButton` / `WriteLink` (used by `EmptyState` and `DueCard`).
*   `templates` has no database access in its renderer. It takes a plain data map, so preview, the reminder list and tests share one code path.
*   Router: the standard library `net/http` mux (method and path patterns), with the oapi-codegen `std-http-server` interface. Handlers get `(w, r, params)`, so they set headers and stream bodies directly (item 32).

### Data model

Changes from the SAD are marked **(new)** or **(changed)**. IDs are UUIDv7, generated by the server. The phone never creates IDs in release 1. A `uuid` column already accepts phone-generated IDs, so offline creates can add them later without a schema change.

```text
-- No extensions: client search runs on the phone (see "Clients and search").
-- Shorthand: full types, foreign keys and indexes are listed in item 28.

business_settings (                      -- (changed) one row, created by the setup wizard, no regional defaults in DDL
  id smallint PK CHECK (id = 1),
  name text NOT NULL,
  timezone text NOT NULL,                -- IANA name, validated with time.LoadLocation
  locale text NOT NULL,                  -- 'es' in release 1
  currency char(3) NOT NULL,             -- ISO 4217 code, e.g. USD
  country_code text NOT NULL,            -- ISO 3166-1 alpha-2, e.g. CR; used for phone parsing
  accent_color text NOT NULL,            -- '#rrggbb' (lowercase hex); service colors use the same format
  icon_updated_at timestamptz NULL,      -- NULL = built-in default icon; files in media/branding/, not rows in media
  push_hour smallint NOT NULL DEFAULT 8 CHECK (push_hour BETWEEN 0 AND 19),  -- no push from 20:00
  updated_at timestamptz NOT NULL
)

users (id, username text UNIQUE NOT NULL, password_hash text NOT NULL,   -- (changed) username, not email
       must_change_password bool NOT NULL DEFAULT false,
       created_at)                       -- no role column: release 1 has one owner
sessions (token_hash bytea PK, user_id, created_at, expires_at, last_seen_at)  -- never listed by the API
setup_tokens (token_hash bytea PK, expires_at, used_at)                 -- (new)

attribute_definitions (
  id, key text UNIQUE NOT NULL,          -- generated from label, never shown, never changed
  label text NOT NULL,
  scope text NOT NULL CHECK (scope IN ('client','appointment')),
  type text NOT NULL CHECK (type IN ('text','number','date','select','multiselect')),  -- (changed) no image, no payment_method
  required bool NOT NULL DEFAULT false, position int NOT NULL, archived_at, created_at)
attribute_options (id, attribute_id, parent_id NULL, label, position, archived_at)
  -- options nest to any depth; parent must belong to the same attribute,
  -- parent_id cannot change after creation; enforced by the attributes module

clients (
  id, name text NOT NULL,
  phone_e164 text NULL,                  -- '+50688887777'
  phone_digits text NULL,                -- '50688887777', for the phone's search and wa.me
  notes text, attrs jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(attrs) = 'object'),
  created_at, updated_at, archived_at)

services (id, name, duration_min int NOT NULL, price_cents bigint NULL, color text, position int, archived_at)

appointments (
  id, client_id NOT NULL, starts_at, ends_at,
  status text NOT NULL DEFAULT 'scheduled'
    CHECK (status IN ('scheduled','confirmed','cancelled','no_show')),  -- "done" is computed, never stored
  total_cents bigint NOT NULL DEFAULT 0, -- (new) sum of copied service prices
  paid_cents bigint NULL,                -- (new) amount actually paid
  attrs jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(attrs) = 'object'),
  notes text, created_at, updated_at)
  INDEX (starts_at) WHERE status NOT IN ('cancelled','no_show')
  INDEX (client_id, starts_at DESC)

appointment_services (                   -- (changed) copies price and duration; same service may repeat
  appointment_id ON DELETE CASCADE, position smallint, service_id,
  name text, price_cents bigint NULL, duration_min int,
  PRIMARY KEY (appointment_id, position))

media (id, client_id, appointment_id NULL, storage_key, thumb_key, content_type,   -- client_id always set (see "Photos")
       bytes int, width int, height int, created_at, archived_at)

message_templates (id, name text NOT NULL, body text NOT NULL, archived_at, created_at, updated_at)   -- (new)

reminder_rules (                         -- (changed)
  id,
  kind text NOT NULL CHECK (kind IN ('before_appointment','come_back')),
  service_id uuid NULL,                  -- NULL only for before_appointment (business-wide)
  offset_days int NOT NULL,              -- calendar days: before_appointment >= 0 (0 = the same morning), come_back > 0, e.g. 21
  return_service_id uuid NULL,           -- (new, P4 migration 00002) come_back only: the service to book next; NULL = service_id
  applies_from timestamptz NULL,         -- (new) NULL = include overdue items; set = only items due on or after its local day
  template_id uuid NOT NULL,
  archived_at, created_at,
  CHECK ((kind = 'before_appointment') = (service_id IS NULL)),
  CHECK (kind = 'come_back' OR return_service_id IS NULL))

reminder_log (                           -- (new) replaces the reminders queue
  id, rule_id NOT NULL, appointment_id NOT NULL,
  anchor_at timestamptz NULL,            -- starts_at for before_appointment; NULL for come_back
  action text NOT NULL CHECK (action IN ('sent','skipped')),
  body text NULL, phone_digits text NULL, -- exactly what was sent
  acted_at timestamptz NOT NULL,         -- time of her "Sí" tap, sent by the phone
  created_at,
  UNIQUE NULLS NOT DISTINCT (rule_id, appointment_id, anchor_at))
                                         -- NULLS NOT DISTINCT (PG 15+): one come_back row per (rule, visit)

push_subscriptions (id, user_id, endpoint text UNIQUE, p256dh, auth, created_at)   -- (new)
server_keys (name text PK, value text, created_at)    -- (new) VAPID key pair, generated once
scheduler_state (name text PK, value text, updated_at) -- (new) keys: heartbeat, push_checked_date (the timer's "done for today" mark), last_push (JSON: date, attempted, delivered)
                                         -- server_keys names: vapid_public, vapid_private (base64url)
```

Why the VAPID key pair lives in the database: it is generated on first start and must survive restores. If it lives only in an env file and gets lost, every push subscription stops working without any error. The backup covers the database, so the key pair is backed up with it. Tunnel token and restic password stay in `.env`.

### Attribute values

One validator in `attributes` checks every write (API, CLI, tests). Values are stored in `attrs` by key.

| Type | JSON value | Validation | Template output | CSV export |
| --- | --- | --- | --- | --- |
| text | `"Café negro"` | string, max 500 chars | as is | as is |
| number | `12.5` | JSON number | locale format ("12,5") | locale format |
| date | `"2026-10-24"` | ISO date, no time | "24 de octubre" | ISO date |
| select | `"<option_id>"`: only the picked option, at any depth | ID exists and belongs to the attribute | the path labels from the top, joined by a space, as typed: "Almendra corta" | path labels joined by " / " |
| multiselect | `["<id>","<id>"]`: only the picked options, at any depth | IDs exist and belong to the attribute, no duplicates | each path as for select, then "A, B y C" | paths joined by "; " |

Rules:

*   `key`, `type` and `scope` cannot change after creation. To change them, she creates a new attribute and archives the old one.
*   `key` is generated from the label (lowercase, no accents, `_` for spaces, a number added if taken). Two attributes created at the same moment could pick the same key; the second insert then fails on the unique `key` and is a 500. That is accepted: one owner is effectively one writer. A key uses only `a–z`, `0–9` and `_` and starts with a letter, the same characters the template tokenizer accepts. These names are reserved: `name`, `first_name`, `phone`, `notes`, `date`, `time`, `services`, `id`.
*   **Required** is checked when a record is created, and when a request changes that attribute. Old records without the value can still be edited. An empty text (`""`) counts as no value: it is removed from `attrs`, and it fails a required check.
*   **Empty values:** `""` and an empty multiselect `[]` count as no value (removed, and they fail a required check). `null` removes a key on `PATCH` and is ignored on create.
*   **A value equal to the stored one is not new (D4).** On `PATCH`, a key whose request value equals its stored value is accepted unchanged, even when the attribute or option was archived since, because the phone's edit form re-sends the whole form. In a multiselect, only the IDs added in this request are checked for archived options.
*   **Text is trimmed (D5).** A whitespace-only text counts as empty: it is removed and fails a required check. The 500-character limit counts runes.
*   **Rejected writes (422, with `errors[].field` = `attrs.<key>` and the attribute's label in the message):** an unknown key, a key of the other scope, any write to an archived attribute, and an archived option ID in a new value. Only the keys in the request are checked, so a stored value that points at an option archived later stays valid.
*   **One archive rule for attributes and options:** an archived attribute or option keeps its stored values. They are shown read-only, and they still render in templates and in `datos.json` by their last label (the CSV files leave archived attributes out; see "Export"). A record with no stored value renders the fallback, as for any empty value.
*   **Option trees:** the parent path is not stored in the value. It is read from `parent_id` when the value is shown, rendered or exported. She can pick an option at any level, not only a leaf. Archiving an option hides it and its children from the picker; stored values keep rendering.
*   Archiving an attribute that a template uses shows a warning that lists those templates. The warning is information only; the archive goes ahead. The phone builds the list from the templates it already caches (bootstrap), so there is no endpoint for it. The template keeps rendering stored values by the last label, as in the rule above.

### Message templates

Syntax (internal; she never types it, and the editor keeps what she types and the saved body in agreement):

*   `{{client.name}}`, `{{client.first_name}}`, `{{client.<key>}}`
*   `{{appointment.date}}`, `{{appointment.time}}`, `{{appointment.services}}`, `{{appointment.<key>}}`
*   `{{business.name}}`
*   Fallback: `{{client.<key> | "texto"}}`. The chip editor writes it, so the parser accepts only straight quotes, exactly what the serializer produces. Spaces inside `{{ }}` are allowed. The fallback text cannot contain `"`; the "Si no tiene" field rejects it with a Spanish message, so no escaping is needed.
*   `{{appointment.services}}` joins the service names like a multiselect: "Acrílico, Pedicure y Cejas".
*   **What `appointment.*` means:** in a "before" message, the upcoming appointment. In a come-back message, the past visit that started the reminder (so `{{appointment.services}}` is what she did then).

Behavior:

*   **Saving:** every variable must exist. An archived attribute counts as existing, so a template that already uses it can still be saved; the chip row shows only active attributes, so she cannot add a new use of it. If a variable does not exist at all, there is no label to show, so the plain Spanish error names the variable key only, for example "La variable 'bebida_favorita' no existe". The chip editor never writes such a token: a chip whose variable no longer exists is reported before saving ("Hay un dato en el mensaje que ya no existe"). Any `{{` that does not start a valid variable is also rejected, with "Hay un {{ sin cerrar" and its position, counted in characters (not bytes) from 1. Inside `{{ }}` only ASCII spaces are allowed. Save reports every such error in body order, then unknown variables in body order. The body is stored exactly as sent, with no trimming, so error positions match the editor, and the 2000-character limit counts the body as sent. The renderer is lenient: it copies a bad `{{` as plain text, and a lone `}}`, `{` or `}` is plain text everywhere.
*   **Rendering:** a missing value becomes the fallback, or empty text. The renderer never fails at send time.
*   **Braces.** The syntax has no escape, so a `{{` in her text, or a `{` right before a variable (which reads as `{{`), would not mean what the editor shows. The editor reports these as it goes ("Hay llaves { } sueltas en el mensaje", with no `{{`, key or position) and the save button does not send the body (`bodyProblems` in `lib/chips.ts`). A paste goes through `parseBody`, so a copied message turns its variables into chips. A lone `}}`, `{` or `}` stays plain text, as the renderer reads it.
*   **The editor:** a text box plus a row of chips, one per variable, showing the current label. The chip list comes from the server (`template_vars[]` in `/bootstrap`, built by `templates/vars.go`), so the phone and the save check use one list. Tapping a chip in the row inserts the variable, and the text box shows it as an inline chip. Tapping an inline chip opens "Si no tiene: ___" to set the fallback.
*   **Preview:** `POST /templates/preview` renders on the server, so there is one renderer, written in Go. Above the preview, "Vista previa con:" lets her pick who it is for:
    *   **The sample client** (the default). Her own attributes get placeholder values, so they are visible in the preview: text shows the label in brackets ("[Bebida favorita]"), select and multiselect show the first active option, number shows 1, date shows today.
    *   **A real client** she picks with the client search. The preview uses that client's data and the client's latest appointment that is not cancelled or no-show. If the client has none, the `appointment.*` values come from the sample appointment.
    *   **Lenient, and the same path as the due list.** The preview never rejects a body: a malformed `{{` shows as text and an unknown variable shows its fallback or nothing, so she sees the text while typing. `unknown_vars` lists each unknown variable once as `ns.key`, in body order (the save check reports every occurrence; the preview lists names). Both records go through the same `buildData` as the due list. The latest appointment is the one with the largest `starts_at` among `scheduled` and `confirmed` ones (ties by ID). An archived client can be previewed; an unknown `client_id` is 422 on `client_id`.
    *   Preview lives in `reminders` (item 32), not in `templates`, because it reuses the loader that builds the data map for the due list. That loader reads clients, appointments, services and attributes, which `templates` may not read.

### Reminder rules and the "done" rule

**An appointment is "done"** when:

*   `status` is `scheduled` or `confirmed`, and
*   `ends_at <= now`.

`cancelled` and `no_show` are never done. This rule lives in one function, `appointments.IsDone(status, endsAt, now)`. It takes plain values, so `reminders` calls it on its own query rows. It does not depend on the daily timer having run.

There are two rule kinds:

| Kind | Belongs to | Due when | Shown until | Skipped when |
| --- | --- | --- | --- | --- |
| `before_appointment` | the business (one or more rules, e.g. 1 day, and 0 days for the same morning) | the local day `date(starts_at) - offset_days`, from the start of that day | `starts_at` (then hidden, never sent late) | the appointment is cancelled or no-show |
| `come_back` | a service | the local day `date(ends_at) + offset_days`, from the start of that day | she sends or skips it, or 60 days after its due day | the appointment is not done, or the client has another appointment (not cancelled or no-show) that starts after this one's `ends_at` and **includes the rule's return service** |

*   **No items before the appointment existed.** An item exists only if its due day is on or after the local day of the appointment's `created_at`. Because of that, a same-day booking gets no "1 day before" item (its due day was yesterday), and a past visit typed in from the notebook does not flood the list with old come-backs. A past visit whose come-back day is still ahead works normally.
*   **New or changed rules: "Solo de hoy en adelante" or "Incluir atrasadas".** A new rule, or a change to an existing one, can make many past visits due at once. For example, a new 21-day come-back rule finds every visit from the last 60 days that has not rebooked the return service. So the app asks once:
    *   "Solo de hoy en adelante" (the default): `applies_from` is set to now. An item of this rule exists only if its due day is on or after that local day. Come-back items that fall due later still appear; nothing shows up late. Items of this rule that were waiting as late disappear.
    *   "Incluir atrasadas": `applies_from` is `NULL`. Overdue items from the last 60 days appear, marked late.
    *   The question is asked when the change can make new items appear: creating a rule, changing `offset_days`, changing the return service (a later booking that held an item back may not include the new one), restoring an archived rule, or giving a new template or return service to a rule whose template or return service was archived. Other edits do not ask and keep `applies_from` as it is.
    *   A `before_appointment` rule is never asked: its items hide at the appointment's start, so nothing past can appear, and the only items "Incluir atrasadas" adds are for appointments still ahead whose due day has just passed (a new 2-day rule and tomorrow's visit). The app sends `include_overdue: true` for it, so those visits still get their message; "Solo de hoy en adelante" would drop them.
*   **Archived parts.** A rule cannot be created with, or changed to, an archived template, service or return service (422). A rule whose template, service or return service is archived later produces no items (it does not fall back to the rule's own service: that would silently change which bookings hold the item back). An archived client produces no items: archiving is how a client is removed (see Non-Goals), so she is never asked to message one. Ajustes → Recordatorios shows a rule with an archived template, service or return service with a warning ("La plantilla está archivada") until she picks another template or return service, or archives the rule. Archiving a rule stops new items; handled items stay in the log.
*   **One message per appointment.** A `before_appointment` rule produces one item per appointment, not one per service. `{{appointment.services}}` lists all the services.
*   **Return service.** A `come_back` rule has an optional `return_service_id`: the service the client should book next. Empty means the rule's own service. Example: the rule on "Acrílico" says "21 días después → Relleno". One return service per rule.
*   **Rebooking.** The `come_back` skip rule covers future bookings and also later visits, but only ones that include the return service. A later visit without it does not skip the item: a pedicure after an acrylic set still gets the refill reminder. If that later appointment is cancelled, or the return service is taken off it, the condition is false again, so the reminder returns on the next read. Nothing has to "re-arm" it.
*   **Booking prompt.** On the due day, if the client has a future appointment (starts after now, not cancelled or no-show) that does not include the return service, the item is shown to her as a prompt instead of a client message, for example "Ana tiene cita el 12/11 (Pedicure). ¿Agregar Relleno?" (the real string lives in `es.json`). The earliest such appointment is used. Actions:
    *   "Agregar a la cita": `PATCH /appointments/{id}` on that appointment, with its current `service_ids` plus the return service and no `ends_at`. When `service_ids` is sent without `ends_at`, the server sets `ends_at` = `starts_at` + the sum of all line durations, so `ends_at` grows by the return service's duration ("Appointments" → Editing). The overlap check runs as for any edit that changes the times (a 409 shows "Agendar igual"). After that, the skip rule hides the item.
    *   "Enviar mensaje": the normal client message (`wa.me`) and the normal `sent` log row.
    *   "Omitir": the normal `skipped` log row.

    The log identity stays `(rule_id, appointment_id)`, so a handled prompt never comes back. A later booking without the return service that starts before the due day is in the past when the item falls due, so the item is a normal client message. If the future appointment is cancelled, the item is a normal message again on the next read. The daily push counts prompts with messages (the same `DueList`).
*   **Rescheduling.** For "before" items only: a new `starts_at` changes the `anchor_at` identity, so a reminder already sent for the old time does not block one for the new time. This is intended: the client was told the old time. A `come_back` item is identified by `(rule_id, appointment_id)` only, so editing a past visit (adding a service, fixing its times) never brings back a come-back she already sent or skipped. Text is always rendered at read time from current data, so it is never stale.
*   **Two services with come-back rules** in one appointment give two independent come-back items. Each rule checks its own return service: a later booking with "Relleno" skips the "Acrílico" item, but not the item of a rule whose return service is something else.

### Due-list computation

`reminders.DueList(ctx, q, now)` loads the data in a few queries (steps 1–2), then hands it to a pure function, `compute(data, now)`, for steps 3–6. Unit tests call `compute` with a fake clock; DB tests call `DueList`.

1.  First, load the settings and the active rules, each with its return service (`return_service_id`, or `service_id` when it is empty), and that service's name and archived state.
2.  Then load the candidate appointments, one query after another on the same `q`. The caller passes a read-only `REPEATABLE READ` transaction (bootstrap, `GET /reminders/due` and the timer all do), so every query sees one snapshot:
    *   **Before candidates:** appointments with `starts_at` between `now` and the end of today plus the largest "before" `offset_days`.
    *   **Come-back candidates:** appointments with `ends_at` between today minus (largest come-back `offset_days` + 60 days) and `now`.
    *   For the come-back candidates' clients, the appointments (not cancelled or no-show) that start after the earliest candidate `ends_at`, with their `starts_at` and service lines (`service_id`, `name`). The skip rule and the booking prompt both read these; `compute` keeps, for each visit, only the ones of its client that start after its own `ends_at`.
    *   Log rows for the candidate appointments.
    *   Both candidate loads skip appointments whose client is archived.
3.  For each (rule, appointment) pair, compute `anchor_at` (`starts_at` for "before", none for `come_back`) and the due day, and apply the table above. A `come_back` rule pairs only with appointments that have one of its lines on the rule's service. For a remaining `come_back` item, the earliest later appointment that starts after `now` and lacks the return service makes it a booking prompt ("Booking prompt" above).
4.  Drop pairs that already have a log row: a "before" item matches on `(rule_id, appointment_id, anchor_at)`, a `come_back` item on `(rule_id, appointment_id)`.
5.  Render each remaining item's text with `templates`, and build its `wa.me` link from `phone_digits`. A booking prompt gets the same text and link (for "Enviar mensaje"), plus the future appointment's ID, start and service names and the return service's ID and name.
6.  Finally, return the items as "Para enviar hoy" (due today or earlier), sorted by due day (oldest first), then client name, then the item ID, so the order is fixed. Each item carries a `late` flag when its due day is before today.

The 60-day look-back means a come-back item she never handles drops out of the list 60 days after its due day (it is dropped when the due day is more than 60 days before today). The visit stays in the client's history; the reminder is no longer listed. A log row is only written when she acts on an item.

The same function backs `GET /reminders/due` and the push count. The export does not include pending items; it includes the reminder log. Unit tests cover the table above with a fake clock.

### Sending flow

```mermaid
sequenceDiagram
  participant P as PWA
  participant S as Server
  participant W as WhatsApp
  P->>S: GET /reminders/due
  S-->>P: items (rendered text, wa.me link)
  P->>P: save items in Dexie
  P->>P: tap "Enviar" → store pending confirmation (item id)
  P->>W: open https://wa.me/<digits>?text=<encoded text>
  W-->>P: she comes back
  P->>P: sheet "¿Lo enviaste a Ana?"
  alt Sí
    P->>S: POST /reminders/log {rule, appointment, anchor_at, action: sent, body, acted_at}
    Note over P,S: if the POST fails → error message, item stays in the list
  else Todavía no
    P->>P: item stays in the list
  end
```

*   **If iOS closes the app while she is in WhatsApp**, the question is not lost. The tap on "Enviar" stores the pending confirmation in Dexie `meta` (`{item_id, client_name, started_at}`). On any app open, if one is stored, the app shows "¿Lo enviaste a …?" before anything else. There is no return event to wait for: the sheet is open whenever a confirmation is stored and the cache is loaded, so it is already waiting when she comes back (and on a cold start it is the first thing shown). It cannot be dismissed with Escape, a tap outside or a swipe. "Sí", "Todavía no" and logout clear it. If the item is no longer in the due list (for example it was confirmed from another device), it is cleared without asking.
*   **After the server logs it, the item stays hidden across a reload.** On the answer to the log `POST` ("Sí" or "Omitir"), the item's id is stored in Dexie `meta` (`logged`); for "Sí" the same write clears the pending confirmation, so the question and the hidden item change together. The cached due list may still hold the item until the refresh after the `POST` is stored; if the app is closed or reloaded first (she taps "Enviar" on the next client, iOS closes it, the refresh fails), the stored id keeps the item out of the list, so it never shows again with "Enviar" enabled and is never sent twice.
*   "Omitir" writes `action: skipped`.
*   A client without a phone shows "Agregar teléfono" instead of "Enviar".
*   Nothing is queued while offline. If the `POST` fails, the sheet says "No se guardó. Ya lo enviaste: cuando haya conexión, toca Sí otra vez" and the item stays in the list, so she confirms it later instead of sending it again. A repeated `POST` is safe because of the `UNIQUE` key and `ON CONFLICT DO NOTHING`.
*   While offline, the cached list is read-only: "Enviar" and "Omitir" are disabled. A "before" item whose appointment has started is hidden from the cached list, because the server would no longer return it.
*   The open question on `wa.me` vs `whatsapp://send` is settled by the iPhone test. The link builder is one function, so switching between them changes only that function.

### Daily push

*   On first start, `push` creates a VAPID key pair in `server_keys`. There is no env override: the pair is in the database, so the backup covers it. `VAPID_SUBJECT` (a `mailto:` address) is required.
*   The installed PWA subscribes after she taps "Allow". The subscription is saved with `POST /push/subscriptions`, which inserts it or, for a known endpoint, updates its keys.
*   **Re-subscribe:** each time the installed app opens, it calls `pushManager.getSubscription()`. If there is one, it posts it again, so a subscription the server deleted comes back. If permission is not granted yet, or there is no subscription, `push.ts` reports "needs activation", and "Hoy" and the status screen show an "Activar notificaciones" button. The same button does the first request in CUJ 1 and any later re-subscribe, because iOS asks for permission only after a user tap.
*   The daily timer sends one notification: "Tienes N mensajes para enviar hoy" ("Tienes 1 mensaje para enviar hoy" when N is 1; `locale.PushBody`), with `TTL` 28800 s (8 h) and `Urgency: normal`, so a phone that was off does not get the morning count the next day. The service worker also sets the app badge to N.
*   **The badge follows the list.** After each refresh and each successful log `POST`, the app calls `navigator.setAppBadge(n)` with the count of the due list it shows (`shownDue`, item 67: items the server has already logged are left out), or `clearAppBadge()` at 0, so the Home Screen number never shows the morning count all day.
*   **A 0-day item for an early appointment gets no push.** If the appointment starts before the push hour, the item is hidden at `starts_at` (see "Reminder rules"), so the push does not count it. She sees it only if she opens the app before the appointment.
*   A 404 or 410 answer from the push service deletes that subscription.
*   There is no endpoint to delete a subscription: no screen needs one, and a dead one is deleted on the first 404 or 410.
*   Push is never requested in a normal Safari tab, only in the installed app.

### Phone cache and offline behavior

*   **Storage:** Dexie (IndexedDB) on the phone. The DB has a schema version. A version that changes the shape of cached rows clears only the copied server data in its upgrade (settings, attributes, services, templates, rules, clients, appointments, due) and sets `meta.lastSyncAt` to null, so the next start refreshes from `/bootstrap` while drafts and `meta` (the pending send confirmation, the logged ids, the last `Me`) stay; version 2 did this when due rows gained their position. A cached row that still fails its zod parse on read clears the whole cache, which is then rebuilt from `/bootstrap` (item 67). One exception: a `meta` row stored before `logged` existed reads as no logged items instead of failing, so the pending send confirmation is kept (item 67); a malformed `logged` still fails the parse.
*   **Refresh:** when the app opens, or regains focus more than 30 s after the last refresh, it calls `GET /bootstrap?from=<today-60d>&to=<today+120d>`. While the server does not answer it tries again after 2 s, doubling up to 60 s, and at once when the phone's connection comes back. The response contains settings, attributes and options, services, templates, rules, all active clients, the appointments in the window, and the due list. The client set is expected to stay small (an estimate, not measured: hundreds of KB gzipped at a few thousand clients).
*   **On demand:** a client's full history and photos outside the window are fetched when she opens that client, online only.
*   **Detecting "offline":** "server off" means a network error, or a response without the `X-App-Version` header. The app sets that header on every response, so its absence means the answer came from Cloudflare (530, 502, 524), not from the app. The content type is not used, because 204s, problem errors, the ZIP and JPEGs are normal answers. Details in item 66.
*   **While offline:**
    *   **Time formats, one per audience:** client messages use the 12-hour clock ("a las 3:00 p. m.", Go `locale.Time`, the one formatter for messages); every screen uses the 24-hour `HH:MM` the .pen draws (web `clockText` in `lib/time.ts`, the one formatter for screens).
    *   A banner shows the time of the last successful refresh, with its day when that was not today ("Datos del 4/11 a las 18:42"); the wide sidebar's sync line says it the same way (`syncedWhen` in `lib/time.ts`, shared by both).
    *   Create and edit actions show "Necesitas conexión con el sistema para guardar": every button that writes (`WriteButton`; `SaveButton` and `ActionButton` are built on it) and every link to a create screen (`WriteLink`: "Nueva cita" in the sidebar, "Nueva clienta", "Nueva plantilla") is visibly disabled with that reason, and the floating "+" on Hoy is faded. The offline banner (`OfflineBanner`) carries the same sentence under its sync line, so a faded control always has its reason on screen and no note floats over the content. When the server answers again, the session is checked again (see item 71), so the shell leaves the offline state without a reload.
    *   Open forms are saved as drafts in Dexie, keyed by form, and restored when she returns.
    *   Search works the same as online, because it always runs on the cached clients (see "Clients and search").
*   **Logout and lost session:** on logout ("Cerrar sesión" calls `POST /auth/logout` first, so a failed call leaves her signed in with her data, then clears), and on any `unauthorized` (401) answer, the app deletes the whole Dexie database (cached data, due items, drafts, a pending send confirmation and the logged ids) before it shows the login screen. Cached records then stay on the phone only while a session is valid. If clearing fails (the delete or the reopen), the app logs `code=cache_clear_failed` and still shows the login screen; the cached rows then stay until something replaces or clears them: the next successful refresh replaces every table, and the next clear (a later 401, a bad cached row, or a logout) deletes the database. Drafts and the pending send confirmation are not replaced by a refresh, so they stay until a clear. The logged ids (see "Sending flow") are kept by a refresh only while its due list still holds them. Photos she already viewed may stay in the browser's HTTP cache, but the app no longer has their IDs. A session closed from another device is seen at the next request; until then the cache can still be read on that phone (see "Risks").
*   **No optimistic UI:** every write, including the reminder log, waits for the server's answer before the UI shows it as saved.
*   **Service worker:** precaches the app shell (JS, CSS, static files, `index.html`) and answers every navigation outside `/api/*` from it, so a cold open with the server off shows the app, not a Cloudflare 530 page. `/api/*`, the manifest and the icons always go to the network: Dexie holds the data, and the wizard and settings change the manifest and icons. Details in item 65.
*   **Updates:** see "API" for the version check. The cache name includes the build version, so a new worker installs next to the old one and the old cache is deleted on activate.

### Screen layouts

*   **Two layouts, one code base.** `compact` below 1024 px (the mockup, every phone, an iPad in portrait) and `wide` from 1024 px (a laptop or desktop, an iPad in landscape). The phone layout is never shown stretched, or centered in a narrow column, on a wide screen (owner rule, 2026-11-04). Same routes, same components, same tokens. Structure that differs is chosen by `layout` (`lib/layout.ts`); purely visual changes use Tailwind `lg:`.
*   **Shell:** on wide, a fixed left sidebar of 256 px (`glass-strong`): the business icon and name, a primary "Nueva cita" button, the nav items (Hoy, Para enviar with the due count, Clientas, Ajustes), and at the bottom the sync line and, when needed, "Activar notificaciones". No bottom tab bar and no floating "+" on wide. The content fills the rest of the window up to 1440 px with 32 px padding; the background glows stay.
*   **Hoy:** a header row (the date title, previous/next, a date input, the Día/Semana toggle; Semana is the default on wide). The time grid fills the main column and the week shows 7 day columns, Monday first. A right rail of 360 px holds the "Para enviar hoy" card (the count and the first 5 items, each with Enviar, which opens `/enviar?item=<id>` with that item selected), the backup red line and "Activar notificaciones".
*   **Para enviar:** two panes. Left, 420 px: the list (Atrasados, Hoy, Enviados). Right: the selected item (client, full message, Enviar and Omitir, or the booking prompt and its three actions). On open the item named by `?item=<id>` is selected (the Hoy rail's Enviar), else the first one, also when the id is not in the list; after a send or skip the next one is.
*   **Clientas:** master-detail. Left, 380 px: search and the list. Right: the selected client (`/clientas/[id]`), or "Elige una clienta" at `/clientas`. The list stays on screen while she moves between clients.
*   **Ajustes:** a section list on the left (280 px) and the open section on the right; `/ajustes` on wide shows Negocio.
*   **Forms:** one glass card up to 760 px wide, with paired fields side by side where they fit. Login and setup: a centered 440 px card on the glow background.
*   **Sheets:** `BottomSheet` renders as a centered modal dialog up to 480 px on wide. Escape and a click outside close it; focus is trapped and returns on close, in both layouts.
*   **Pointer and keyboard:** hover states on rows, buttons and chips; a visible `focus-visible` ring; `title` on icon-only buttons; Enter submits forms. Tap targets stay at least 44 px in both layouts.
*   **Photos:** a grid of 3 columns on compact, 5 on wide.

### Auth, sessions and handover

1.  First, on start, if no user exists, `serve` deletes any old setup token, creates a new one (valid 24 h, stored as a hash) and prints `Setup: <PUBLIC_URL>/setup#<token>` to stdout as one plain line, not a log record (the one exception to "never log a token"). Only the hash is stored, so an old link cannot be printed again; a restart or `setup-link` makes a new one and the old link stops working.
2.  Then `/setup` runs the wizard. `POST /setup` creates the settings row, the owner user and her first session in one transaction, and deletes all setup tokens. From then on `/setup` answers `setup-closed`.
3.  The icon is uploaded after that, as the wizard's last branding step, because the upload needs the new session: `POST /settings/icon` with two PNGs. Icons are stored as files under `media/branding/` and recorded only by `icon_updated_at`; they are not rows in `media`, whose rows need a client. This still happens before the install step (see "Branding and the manifest"). After `POST /setup` the wizard keeps its step (icon or install) in the tab's session storage (`finishingStep`, `lib/setup.ts`), so a reload comes back to it: the guard lets her stay on `/setup` while it is set, although she is signed in. "Entrar" clears it, and so does any redirect away from `/setup` and the lost-session (401) path, so a stale step never reopens the wizard.
4.  After that, in the installed PWA (separate cookie storage from Safari), she logs in with her username and password.
5.  If she forgets her password, the author runs `reset-password`. She logs in with the temporary password and must set a new one.

Session and request rules:

*   **Username:** trimmed and lowercased by the server on setup and login, then 3–32 characters of `a–z`, `0–9`, `.`, `_` and `-` (422 otherwise). The inputs use `autocapitalize="off"`, `autocorrect="off"` and `autocomplete` (`username`, `new-password`, `current-password`), so iOS does not capitalize the first letter and iCloud Keychain can fill them.
*   **Tokens:** the setup token and the session token are 32 random bytes, written as unpadded base64url in the link and the cookie, and stored as a SHA-256 hash.
*   **Session cookie:** `oc_session`. `HttpOnly; Secure; SameSite=Lax; Path=/`. `Secure` is left off only when `PUBLIC_URL` is `http://localhost` or `http://127.0.0.1` (development and e2e), because WebKit does not store a `Secure` cookie over plain http. 90-day expiry that slides forward. `last_seen_at` is written at most once per hour per session; the same write extends `expires_at` and re-sends the cookie with a new 90-day `Max-Age`.
*   **Writes:** every request except GET and HEAD must have an `Origin` header equal to `PUBLIC_URL`, or it gets a 403. The check is the same in development: `make dev` sets `PUBLIC_URL` to the Vite address (item 2).
*   **Login rate limit:** 5 attempts per minute per client IP (an IPv6 client per /64), plus a growing delay per username after failures (exact rule in item 41). The client IP is `CF-Connecting-IP` only when the TCP peer is in `TRUSTED_PROXY_CIDRS` (default: the private ranges, which include the Compose network). Otherwise it is the TCP peer address.
*   **No device list:** there is no screen that lists sessions. If she loses a phone, she changes her password from another device (Ajustes → "Cambiar contraseña"), which closes every other session (item 39). If she has no other device, the author runs `reset-password`, which closes all of them.

This replaces the brief's one-time code (Alternative K1).

### Branding and the manifest

*   `GET /manifest.webmanifest` is generated from settings and served with `Cache-Control: no-cache`. Fields: `name` (business name), `short_name` (the business name cut to 12 characters), `start_url: "/"`, `scope: "/"`, `display: "standalone"`, `theme_color` and `background_color` (the accent color), `lang` (the locale), and `icons` (512 px, `purpose: "any"`). The service worker never caches it, so iOS reads the current name at install. Before setup (no settings row) the manifest has `name` and `short_name` "open-customer", `lang: "es"`, no colors, and the default icons.
*   Icon upload: the phone crops the image to a square, resizes it to 512 px and 180 px PNGs, and uploads both. They are served at `/icons/512.png` and `/apple-touch-icon.png` with `Cache-Control: no-cache`, and Cloudflare does not cache them (see "Deployment"). If no icon was uploaded, the built-in icon is served at the same paths: a light disc on a square of the business's accent color, drawn by the server on each request (before setup, the embedded PNG in the default accent). In the app, `BusinessIcon` draws the same shape in CSS while no icon is uploaded, so it follows the color as she picks it; an upload bumps `lib/iconVersion.ts`, which reloads the image on screen. iOS copies the name and icon only when the app is added to the Home Screen, so the wizard sets them before the install step, and a later change shows only after she reinstalls it. The files are `media/branding/icon-512.png` and `icon-180.png`.

### Photos

*   The phone decodes the picked photo with `createImageBitmap(file, {imageOrientation: 'from-image'})` and draws it to a canvas. Then it exports two JPEGs: the long side at 2048 px, quality 0.8, and a thumbnail at 400 px.
*   Canvas export drops EXIF, so GPS data never reaches the server.
*   **Ownership:** a photo is taken from an appointment (CUJ 3) or from a client. With `appointment_id`, the server fills `client_id` from that appointment; a request with a different `client_id` gets 422. The appointment screen shows the client's photos (`GET /clients/{id}/media`) filtered by `appointment_id` on the phone.
*   Upload: `POST /media` with multipart parts `full` and `thumb`. The server checks the JPEG signature of `full`, decodes the JPEG header of both parts, and checks the size limits (`full` 10 MB, `thumb` 1 MB, the whole request 12 MB), then stores the files at `media/<yyyy>/<mm>/<id>.jpg` and `<id>_t.jpg`. The files are written and synced before the row is inserted, which is what makes "never a row without its file" in "Backups" true.
*   **Folders and checks:** `<yyyy>/<mm>` is the UTC year and month of the upload. A part that has the JPEG signature but that `jpeg.DecodeConfig` cannot read (`full` or `thumb`) gets 415. `bytes`, `width` and `height` describe `full`. An unknown or archived client gets 422 on `client_id`.
*   **Failed writes:** if the insert fails, the `media` module removes the two files it just wrote. If the request's commit fails after `Save`, the handler calls `Discard(ctx, md)`, which removes them. A failure of either removal is logged with `code=media_discard_failed`, never with a path.
*   **Archived photos** are hidden from `GET /clients/{id}/media` only; `GET /media/{id}` and `/media/{id}/thumb` still serve them by ID.
*   Serving: `GET /media/{id}` and `/media/{id}/thumb` need a session. They send `Cache-Control: private, max-age=31536000, immutable`, because a media ID never changes content.

### Clients and search

*   Phone numbers are parsed with `nyaruka/phonenumbers`, using the business `country_code` as the default region (the handler reads it from settings and passes it to `clients`). "8888-7777" becomes `+50688887777`. Invalid numbers are rejected with a Spanish message.
*   Creating a client with a phone that another active client already has returns a warning and lets her "Guardar igual". The check compares the normalized E.164 number with the other active clients, so "612 345 678" and "+34 612-345-678" match. On `PATCH` it runs only when the request sets a phone that differs from the stored one, or when it restores an archived client (`archived: false`); `allow_duplicate_phone` skips it in both cases. An empty `phone` string means no phone, the same as `null`.
*   **Search runs only on the phone**, online and offline, over the cached active clients (`/bootstrap` sends all of them). There is no server search: the phone already holds the data, and one search function means one result order.
    *   A client matches when the client's name contains the query, ignoring case and accents (NFD normalization, combining marks removed), or when the query has 3 or more digits and the client's `phone_digits` contains them. Non-digits are removed from the query first, so "8888-7777" matches. A query with no digits never matches by phone.
    *   Order: names where the query starts a word come first ("starts a word" = at the start of the name or right after a space), then the rest, each group by name (the same accent-free key, then the raw name). Top 20.
    *   Archived clients are not cached. "Ver archivadas" in the client list loads them with `GET /clients?archived=true` (online only) and searches that list with the same function.

### Appointments

Status changes:

| From | To | Who | Effect on reminders |
| --- | --- | --- | --- |
| scheduled | confirmed | She taps "Confirmada" | none |
| scheduled, confirmed | cancelled | She taps "Cancelar" | before and come-back items disappear; a come-back item that this booking was holding back (it had the return service) appears again; a booking prompt that pointed at it becomes a normal message, or a prompt for the next future booking without the return service |
| scheduled, confirmed | no_show | She taps "No vino" (any time, also after the appointment is done) | come-back items disappear |
| cancelled, no_show | scheduled | She taps "Reactivar" (the overlap check runs; "Agendar igual" sends `allow_overlap`) | items come back by the same rules |

*   All status changes go through `appointments.Transition(a, to, now)`, which rejects any other change with a 409 and a Spanish message.
*   **Overlap:** create and update check for overlapping active appointments: `a.starts_at < b.ends_at AND a.ends_at > b.starts_at`, so back-to-back appointments (one ends at 15:00, the next starts at 15:00) do not overlap. If there is one, the server answers 409 with problem type `overlap` and the overlapping appointments. The app shows "Agendar igual". Sending again with `allow_overlap=true` saves it.
*   **Concurrent edits:** last write wins. There is one user and no offline edits, so the only possible conflict is her Safari tab against the installed app. Version checks (`If-Match` / 412) come with staff accounts; a `version` column can be added then without changing existing data.
*   **Services:** the price and duration of each service are copied at save time. Changing the service later does not change past appointments.
*   **Editing:** every field can be edited at any time, also on past and done appointments, so she can fix mistakes.
    *   `PATCH` with `service_ids` replaces the whole service list. A service already on the appointment keeps its saved line (name, price, duration), matched by service ID as many times as the appointment has it, so a past visit's lines and `total_cents` never move when a price changes later. Only a service added by the edit (or an extra copy of one already there) is copied from the current service at today's name, price and duration. To take today's price for a kept line, she removes the service and adds it again in a second edit. `total_cents` is recomputed. `paid_cents` is never changed by this.
    *   If the same `PATCH` has no `ends_at`, `ends_at` is recomputed as `starts_at` + the new durations. If it has `ends_at`, that value is kept. No "set by hand" flag is stored.
    *   Changing `starts_at` without `ends_at` moves `ends_at` by the same amount.
    *   The overlap check counts only `scheduled` and `confirmed` appointments, and always leaves out the appointment being saved. It runs on every create, on an update that changes the times of a `scheduled` or `confirmed` appointment (a notes-only edit, or an edit of a cancelled one, skips it), and on "Reactivar". It takes no lock, so two saves at the same moment can both pass it (see "Accepted risks").
    *   An archived client or service cannot be put on an appointment (422 on `client_id` or `service_ids`). Lines already copied from a service archived later stay as they are, by the same keep rule. Only a newly added archived or unknown service is a 422.
*   **Payment:** `paid_cents` is a column, because the export and future reports need a number. There is no built-in payment-method field; she can create a select attribute for it like any other.

### API

*   `api/openapi.yaml` (OpenAPI 3.0.3) is the contract. `oapi-codegen` generates the Go `std-http-server` interface and the models into `internal/httpapi/gen` (no strict mode, so handlers write headers and stream bodies themselves; item 16). `openapi-typescript` generates `web/src/lib/api/schema.d.ts`, and `openapi-fetch` calls it.
*   CI runs both generators and fails if the generated files differ from the committed ones.
*   Errors use `application/problem+json` (RFC 9457). Each error has a `type`, a Spanish `title`, and `errors[]` with `field` for validation problems.
*   Lists are not paged. Every list is small (one business), `/bootstrap` already returns all active clients, and per-client history and photos are short.
*   Every response has an `X-App-Version` header. If it differs from the SPA's build version, the app shows "Hay una versión nueva — toca para actualizar".
    *   **Build version:** the Go binary gets `VERSION` through `-ldflags`, and the web build gets the same value through the `version.name` option of the `sveltekit()` plugin in `web/vite.config.ts` (read from the `VERSION` env var). The SPA and the service worker both read it from `$app/env`.
    *   **Update flow:** the tap activates the new service worker and reloads the page once (steps in item 66).

Main endpoints (all under `/api/v1`):

| Area | Endpoints |
| --- | --- |
| Setup and auth | `GET /setup/status`, `GET /setup/options`, `POST /setup`, `POST /auth/login`, `POST /auth/logout`, `POST /auth/password`, `GET /me` |
| Settings | `PATCH /settings`, `POST /settings/icon` |
| Bootstrap | `GET /bootstrap?from=&to=` |
| Attributes | `POST /attributes`, `PATCH /attributes/{id}` (label, required, position, archived), `POST /attributes/{id}/options`, `PATCH /attributes/{id}/options/{optionId}` |
| Clients | `GET /clients?archived=`, `POST /clients`, `GET /clients/{id}`, `PATCH /clients/{id}`, `GET /clients/{id}/appointments`, `GET /clients/{id}/media` |
| Services | `POST /services`, `PATCH /services/{id}` |
| Appointments | `GET /appointments?from=&to=`, `POST /appointments`, `GET /appointments/{id}`, `PATCH /appointments/{id}`, `POST /appointments/{id}/status` |
| Media | `POST /media`, `GET /media/{id}`, `GET /media/{id}/thumb`, `PATCH /media/{id}` (archive) |
| Templates | `POST /templates`, `PATCH /templates/{id}`, `POST /templates/preview` |
| Reminders | `POST /reminder-rules`, `PATCH /reminder-rules/{id}`, `GET /reminders/due`, `POST /reminders/log`, `GET /reminders/log?client_id=` or `?date=` |
| Push | `GET /push/key`, `POST /push/subscriptions` |
| Export | `GET /export` (streamed ZIP) |
| Status | `GET /status` (session needed), `POST /client-errors` |

Settings, attributes, services, templates and rules have no read endpoint: `/bootstrap` returns all of them, and each write returns the changed object, which the app stores in the cache. `GET /appointments?from=&to=` serves days outside the bootstrap window (the date switcher on "Hoy"). There are no `DELETE` endpoints: archiving is a `PATCH` with `archived: true`.

These endpoints are outside `/api/v1`, with no session. They are not in `api/openapi.yaml`; `httpapi/server.go` registers them on the same mux before the generated routes, and they never overlap with `/api/v1`. Handlers: `status` (item 60) for `/healthz` and `/readyz`, `settings` (item 44) for the manifest and icons, `static.go` (item 35) for everything else:

*   `GET /healthz`: the process is alive.
*   `GET /readyz`: the DB is reachable and migrations are applied.
*   `GET /manifest.webmanifest` and the icons.

### Export

`GET /export` streams a ZIP with `archive/zip`. It never builds the file in memory or on disk first, so it starts sending within Cloudflare's response timeout.

| File | Contents |
| --- | --- |
| `clientas.csv` | One row per client, archived ones included (column `archivada`). Columns: `nombre`, `telefono`, `notas`, `creada`, `archivada`, then one column per active client attribute. |
| `citas.csv` | One row per appointment. Columns: `fecha`, `hora`, `clienta`, `servicios` (names joined by "; "), `total`, `pagado`, `estado` (with "realizada" for done ones, from `IsDone`), `notas`, then one column per active appointment attribute. |
| `servicios.csv` | The service catalog: `nombre`, `duracion_min`, `precio`, `archivado`. |
| `datos.json` | Everything, with IDs: `{"version": 1, "exported_at", "settings", "attributes" (with options and archived ones), "services", "templates", "rules", "clients", "appointments" (with services), "reminder_log"}`. No photos. |

Rules for the CSV files:

*   UTF-8 with a BOM, so Excel opens accents correctly. Go's `encoding/csv` writes and quotes the values, so a decimal comma or a comma in a note is safe.
*   Attribute columns are named by label, in `position` order. If two attributes share a label, each gets ` (<key>)` added.
*   Each array in `datos.json` uses the API schema from item 15 (`reminder_log` uses `LogItem`).
*   An error before the first byte reached the client is a normal problem response (the handler counts the bytes it passes on). After that byte the status is already sent, so the handler logs `code=export_aborted` with the request ID and calls `panic(http.ErrAbortHandler)`, which closes the connection (`recoverPanic` lets that value through, item 34). The phone gets a broken download instead of a file that looks complete. The file name date comes from `Server.businessZone`, the same helper `GET /appointments` and `GET /reminders/log` use.
*   Archived attributes are left out of the CSV; `datos.json` keeps them.
*   Money and numbers use the value table's locale format, with the currency's decimals and no symbol ("12500,00").
*   Fixed values are Spanish words: `estado` is "agendada", "confirmada", "cancelada", "no vino" or "realizada"; `archivada` and `archivado` are "sí" or empty.
*   Dates are `YYYY-MM-DD` and times `HH:MM`, in the business time zone. `telefono` is E.164. The ZIP file name uses the business-local date.
*   The exporter reads in pages of 500 by `id` (`WHERE id > $1 ORDER BY id LIMIT 500`; a table ends at its first empty page), all inside one read-only `REPEATABLE READ` transaction, so the ZIP is one consistent snapshot.
*   The entries are written in this order: `clientas.csv`, `citas.csv`, `servicios.csv`, `datos.json`. Each table is paged twice in the one snapshot, once for its CSV and once for the JSON, so no more than one page is held at a time (attribute definitions are the exception: `attributes.List` loads them all).
*   The CSV file names, column headers, `estado` words and "sí" are in `locale` (`ExportText`), because Spanish in Go lives only in `locale` and `problem`. Money uses `locale.Money` with the currency's decimals from `currency.Standard.Rounding` (`golang.org/x/text/currency`). `datos.json` is not Spanish and is a constant in the exporter.
*   `datos.json` uses the exporter's own types (`Settings`, `Attribute`, `Option`, `Service`, `Template`, `Rule`, `Client`, `Appointment`, `AppointmentService`, `LogItem`), because the exporter may not import `gen`; their JSON keys equal the API schemas of the same name; `TestExportJSONMatchesSchemas` (in `httpapi`, which may import both) compares the `json` tag names and `omitempty` flags of each pair by reflection. `exported_at` is the clock's time in UTC; `appointments[].done` comes from `IsDone`.
*   The exporter returns errors wrapped and logs nothing; the handler logs.

### Status screen

`GET /status` returns what the "Estado del sistema" screen (Ajustes → Estado del sistema) shows:

*   The app version and the database migration version.
*   The timer heartbeat age.
*   The last push result (`last_push`: its date, how many subscriptions were tried and how many accepted it) and the number of push subscriptions. The line is red ("Notificaciones sin entregar") when there are no subscriptions, or when the last push was tried and no subscription accepted it. A quiet day or a day with the PC off does not turn it red, because no push was tried.
*   The last backup.
*   The free space on the media volume.

The last backup is read from `BACKUP_STATUS_FILE` (default `/backup-status/status.json`), which the backup container writes. Its format and the rules for the red line are in "Backups". When the backup line is red, "Hoy" shows it too (`backup_red` in `/bootstrap`, item 72), because she opens "Hoy" every day and this screen rarely.

### Deployment

`deploy/compose.yaml`:

| Service | Image | Notes |
| --- | --- | --- |
| `app` | `ghcr.io/jnaranj09/open-customer:${APP_VERSION}` (pinned) | Runs `serve`. Volumes: `media` at `/data/media`, `backup-status` at `/backup-status` (read-only). Healthcheck: `["CMD", "/usr/local/bin/open-customer", "healthcheck"]` (exec form; a plain string would need a shell, which distroless does not have). Runs as non-root user 65532. |
| `db` | `postgres:18.<minor>-alpine` | Volume `pgdata` at `/var/lib/postgresql`. From PG 18 the image declares `VOLUME /var/lib/postgresql` and sets `PGDATA=/var/lib/postgresql/18/docker` (checked in the image's Dockerfile), so a volume at the old `/var/lib/postgresql/data` path would not hold the data. `pg_isready` healthcheck. |
| `cloudflared` | `cloudflare/cloudflared:<pinned>` | `tunnel run` with `TUNNEL_TOKEN`. No volumes. |
| `backup` | `ghcr.io/jnaranj09/open-customer-backup:${APP_VERSION}` | `postgres:18.<minor>-alpine` + restic + `backup.sh`. Volumes: `media` at `/media` (read-only), `backup-status` at `/backup-status`, the host path `${BACKUP_TARGET}` at `/target`. |

**Volume ownership:** distroless has no shell, so nothing can `chown` at start. Both images contain the mount folders already owned by UID 65532 (`/data/media` and `/backup-status` in `app`, `/media` in `backup`). Docker copies that ownership into an empty named volume on its first mount, whichever container mounts it first, so `app` can always write photos.

A fifth service, `restore`, uses the backup image under `profiles: ["restore"]`, so `docker compose up` never starts it; only `restore.sh` runs it (see "Upgrades and the migration guard"). No service has `ports:`. There is no Caddy service; `deploy/README.md` says in one sentence that a self-hoster without a tunnel can put any reverse proxy in front of `app:8080`.

Cloudflare settings the author sets once (documented in `deploy/README.md`):

*   The public hostname for the subdomain points to `http://app:8080`.
*   A cache rule bypasses the cache for `/api/*`, `/manifest.webmanifest`, `/service-worker.js`, `/icons/*` and `/apple-touch-icon.png`, so a new icon is never hidden by Cloudflare's cache.
*   Rocket Loader is off.
*   Bot Fight Mode is off, because a Cloudflare challenge page has no `X-App-Version` header and the app would show it as "server off".
*   HSTS is on.

### Backups

`backup.sh` has two main modes. With no argument (the container's normal command) it runs the check below once when the container starts, then every hour. With `once [--tag pre-upgrade]` it runs steps 1 and 3–6 a single time, skipping the 24 h check, and exits with a non-zero code on any failure. Every run, and every restore, holds the lock file `/backup-status/backup.lock` (`flock`), so two of them never touch the database dump or the restic repository at the same time.

A failed step never ends the script early: its error goes to `status.json` (step 6) first. In loop mode the script then waits for the next hour instead of exiting, because `restart: unless-stopped` would otherwise restart it against a broken disk again and again. `once` exits non-zero after writing the status.

1.  First, check that `/target/.open-customer-backup-target` exists. If it does not, the target is not there (for a removable disk: not mounted). Write the error to `status.json`, do not write to `/target`, and wait for the next hour.

    Also in step 1, the new-volumes guard: a run that starts without `status.json` is on new volumes (a new PC, or `docker compose down --volumes`), and it creates `/backup-status/restore-first` (before the marker check, so a missing target does not lose it). While that file exists and the repository already has at least one snapshot, the run takes no snapshot and runs no `forget`: a backup of the new, empty database would become `latest`, so `restore.sh latest` would restore nothing, and its `forget` could also drop that day's real snapshot (`forget` keeps only the newest snapshot of each day). It writes the error "Hay respaldos anteriores: restaure primero" to `status.json` and waits for the next hour (`once` exits non-zero). The file, not `status.json`, holds the guard, so the run's own status write does not lift it. A restore also creates the file, before it drops the database, so a restore that fails after that point blocks the next backups the same way until a restore succeeds. It is lifted (the file removed) by a missing repository folder or one with no snapshot files (a first install, which then backs up as before), by a successful restore, or by the operator replacing `status.json` with `{}` (to start empty on purpose, or when only the status file was lost and the database is intact, never after a failed restore, where it would lift the guard over a half-restored database; a file without `checked_at`, which `backup.sh` always writes; command in `deploy/README.md` section 10). It fails closed: while the file exists, a repository whose config or snapshot list cannot be read blocks the run the same way, with "copy: <first line of stderr>". Snapshots are counted as snapshot files (`restic list snapshots`), because `restic snapshots` skips a file it cannot read and answers an empty list with exit 0. The status screen shows only the red line ("Respaldo con problema"); the backup log has an English line starting `guard:`, and the Spanish reason is only in `status.json` (`last_error`).
2.  Then, if the last successful backup is less than 24 h old, stop here.
3.  After that, run `pg_dump -Fc` to `/backup-status/work/db.dump`. The dump is taken before the media copy in step 4, and photos are never hard-deleted, so a restore can have extra photo files without a row, but never a row without its file.
4.  Then run `restic backup` on `db.dump` and `/media` into the repository at `/target/restic`, tagged `app=<APP_VERSION>` (and `pre-upgrade` when given), with the fixed `--host open-customer-backup`. The repository is encrypted with `RESTIC_PASSWORD`. The host is fixed because restic groups snapshots by host name, and every `docker compose run` container gets a new one; without it, old groups would never be pruned.
5.  Then run `restic forget --group-by paths --keep-daily 14 --keep-tag pre-upgrade --prune`. `--group-by paths` also prunes snapshots made before the host name was fixed. A second snapshot on the same day replaces the first one in the daily set, so the snapshot taken before an upgrade is kept by its tag instead. Pre-upgrade snapshots are kept until the author removes them with `restic forget <id>`.
6.  Finally, write `status.json` (format below).

**`status.json`** is written atomically (a temp file, then `mv`) at the end of every run, including runs that stop early. Every time is UTC in exactly the form `YYYY-MM-DDTHH:MM:SSZ` (`date -u +%Y-%m-%dT%H:%M:%SZ`), because that is the only form jq's `fromdateiso8601` reads; Go reads it as RFC 3339.

```json
{"checked_at": "<time>", "last_dump_at": "<time>|null", "last_copy_at": "<time>|null",
 "last_snapshot_id": "<id>|null", "last_error": "<text>|null", "last_error_at": "<time>|null"}
```

| Run outcome | What changes |
| --- | --- |
| Marker missing (step 1) | `checked_at`; `last_error` = "Disco de respaldo no visible" and `last_error_at` |
| New volumes and the repository has snapshots (step 1, the guard) | `checked_at`; `last_error` = "Hay respaldos anteriores: restaure primero" and `last_error_at` |
| New volumes and the repository's config or snapshot list cannot be read (step 1, the guard fails closed) | `checked_at`; `last_error` = "copy: <first line of stderr>" and `last_error_at` |
| Last backup less than 24 h old (step 2) | `checked_at`; `last_error` and `last_error_at` cleared |
| `pg_dump` fails | `checked_at`; `last_error` = "dump: <first line of stderr>" and `last_error_at` |
| `restic backup` or `forget` fails | `checked_at`, `last_dump_at`; `last_error` = "copy: <first line of stderr>" and `last_error_at` |
| TERM or INT during a run (after the running step finishes) | `checked_at`; `last_error` = "interrupted" and `last_error_at` |
| The loop cannot take the lock for a reason other than another run holding it | `checked_at`; `last_error` = "lock: <first line of stderr>" and `last_error_at` |
| Success | every time field, `last_snapshot_id`; `last_error` and `last_error_at` cleared |

The time fields not listed keep their old values. The status screen shows the backup line red when `last_copy_at` is missing or more than 48 h old, when `last_error` is set, or when `checked_at` is more than 2 h old (the backup container is not running). Because the check runs every hour, an unplugged disk turns the line red at the next hourly check (at most 60 min). A missing or unreadable file shows "desconocido" in red.

Notes:

*   The restic password is in `.env`. The author keeps a second copy outside the PC, for example in his password manager. Without it, every backup is unreadable.
*   `scripts/backup-now.sh [--tag pre-upgrade]` runs `docker compose run --rm backup once` with the same arguments and returns its exit code. It waits for the backup to finish; there is no polling and no timeout.

### Upgrades and the migration guard

*   `scripts/upgrade.sh <version>`:
    1.  First, checks that `APP_VERSION` in `.env` matches the running app (`docker compose exec app open-customer version`). If they differ (for example, a retried upgrade that already changed `.env`), it stops and tells the author to fix `.env` first. Otherwise the pre-upgrade snapshot would carry the wrong version tag.
    2.  Pulls the new images with `APP_VERSION=<version> docker compose pull`. `.env` is not changed yet.
    3.  Runs `scripts/backup-now.sh --tag pre-upgrade`. If it fails, the script stops. `.env` and the running containers stay on the old version.
    4.  Sets `APP_VERSION` in `.env`.
    5.  Runs `docker compose up -d`. The new `app` runs the migrations; the guard passes because step 3 just wrote a fresh backup.
*   **The guard:** at start, `serve` checks for pending migrations. It refuses to migrate when all three are true:
    1.  at least one migration is pending;
    2.  the database's goose version is above 0, so at least one migration was applied before (a fresh database has no tables yet, so the first start always migrates; reading `users` there would fail);
    3.  `last_copy_at` in `BACKUP_STATUS_FILE` is missing or older than 60 minutes.

    Then `serve` exits with "Pending migrations and no recent backup. Run scripts/upgrade.sh." This catches an upgrade done without `upgrade.sh`, for example editing `.env` by hand and running `docker compose up -d`. If the backup target is missing on upgrade day (a removable disk unplugged), the backup fails and the upgrade stops before the guard is reached. `ALLOW_MIGRATE_WITHOUT_BACKUP=true` overrides it, for development or an emergency.
*   Migrations only move forward. Rollback is a restore of the snapshot taken before the upgrade. A rollback drops everything she entered after the upgrade, so the author upgrades outside her working hours.
*   Every snapshot is tagged with the app version that was running (`restic backup --tag app=<APP_VERSION>`). The `backup` service gets `APP_VERSION` from `.env`.
*   **Restore** (`scripts/restore.sh <snapshot-id|latest>`):
    1.  First, runs `docker compose --profile restore run --rm restore snapshot-info <snapshot-id|latest>`, which prints the snapshot's ID, time and `app=` tag as one JSON line. The `restore` service mounts `/target` read-only, so `snapshot-info` and `restore` run restic with the global `--no-lock` flag (restic cannot write a lock file there). It asks for confirmation, showing the time and that version.
    2.  Then stops `app` and `backup` (`docker compose stop app backup`), so nothing writes to the database and no hourly backup starts.
    3.  After that, runs the `restore` service (`docker compose --profile restore run --rm restore restore <snapshot-id>`). It is the backup image with `media` mounted read-write. It takes the backup lock, restores the snapshot to a temporary folder, creates `/backup-status/restore-first` (the new-volumes guard of step 1 in "Backups", so a restore that fails from here on leaves no backup of a half-restored database), drops and recreates the database (`dropdb --force --if-exists`, then `createdb`), loads `db.dump` with `pg_restore`, replaces the media volume content, and runs `chown -R 65532:65532 /media`, so `app` can still write new photos. Last, it removes `/backup-status/restore-first` and writes `status.json` (`checked_at`, `last_snapshot_id` = the restored snapshot, `last_error` and `last_error_at` cleared; the dump and copy times keep their values, null on new volumes, because no copy was made), so the hourly backups resume. On new volumes `last_copy_at` is null, so the next loop check backs up; after a rollback on a working install it is still the recent pre-upgrade copy, so the loop skips for up to 24 h: run `scripts/backup-now.sh` after the restore to back up the restored data now (`deploy/README.md` section 10). Dropping the whole database also removes tables that newer migrations added, which `pg_restore --clean` would leave behind.
    4.  Then sets `APP_VERSION` in `.env` to the snapshot's tag, so the restored schema and the app version match.
    5.  Finally, runs `docker compose up -d`. This starts `app` on the restored version, and recreates `backup` with that `APP_VERSION`, so later snapshots carry the right tag. No migration is pending, so the guard does not block it.
*   **A removable backup disk at boot (optional):** `BACKUP_TARGET` is any host folder the owner chooses; for now it is a folder on the PC itself. If she later uses a removable disk, the PC must boot without it, and the backup must never write to the PC's own disk instead. If the disk is missing, the mount point is an empty folder; the marker check (step 1 in "Backups") stops any write there, and the status line turns red with "Disco de respaldo no visible". Plugging the disk in later is picked up by the next hourly check without a container restart. The mount options are in items 81 and 83. A folder on the PC's own disk does not survive that disk dying; `deploy/README.md` says so and suggests a second disk.
*   API changes inside `/api/v1` are additive only, so an older cached SPA keeps working until it reloads.

### Configuration (`.env`)

The "Read by" column says who reads each variable. The app reads and checks only its own variables (item 20). Compose and the backup container read the rest.

| Variable | Read by | Required | Purpose |
| --- | --- | --- | --- |
| `APP_VERSION` | Compose | yes | Pinned image tag for `app` and `backup`. Passed to `backup` for snapshot tags. |
| `PUBLIC_URL` | app | yes | `https://<subdomain>` in production. `http://localhost:<port>` or `http://127.0.0.1:<port>` is also accepted, for development and e2e. Used for links, the Origin check and the manifest. |
| `DB_PASSWORD` | app, Compose | yes | Used by `db`, and to build the app's and the backup's connection settings. |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_NAME`, `DB_SSLMODE` | app, Compose | no | Defaults `db`, `5432`, `open_customer`, `open_customer`, `disable` (traffic stays on the Compose network). Compose passes them to `app` as is, to `db` as `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, and to `backup` and `restore` as `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`. |
| `TUNNEL_TOKEN` | Compose (`cloudflared`) | yes | cloudflared token. |
| `VAPID_SUBJECT` | app | yes | `mailto:` contact for Web Push. |
| `BACKUP_TARGET` | Compose (`backup` bind) | yes | Host path of any folder the owner chooses (a USB or second disk is optional). The marker file `.open-customer-backup-target` must exist in it. |
| `RESTIC_PASSWORD` | `backup` | yes | Backup encryption key. |
| `TRUSTED_PROXY_CIDRS` | app | no | Default: the private ranges `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`, which include any Docker network. Only these peers' `CF-Connecting-IP` is trusted. This is safe because `app` publishes no port: only containers on the Compose network can reach it. A self-hoster who publishes a port sets this to their proxy's address. |
| `MEDIA_DIR` | app | no | Default `/data/media` (the `media` volume mount). |
| `BACKUP_STATUS_FILE` | app | no | Default `/backup-status/status.json`. The file read by the migration guard and the status screen. |
| `ALLOW_MIGRATE_WITHOUT_BACKUP` | app | no | Default `false`. `true` skips the migration guard. `make dev` sets it; in production it is only for an emergency. |
| `LOG_LEVEL` | app | no | Default `info`. |
| `E2E_DATABASE_URL` | e2e tests (`seed.ts`) | e2e only | Not in `.env`. Set by `make e2e` and by CI. |

### Observability

*   Logs are JSON from `log/slog` to stdout. Each request log line has a request ID, `CF-Ray`, the route, the status and the duration.
*   `POST /client-errors` takes errors from the PWA (message, stack, app version, route) and logs them at `warn` with `code=client_error`, the route, the app version, the message and the stack. It opens no transaction and touches no table. Limits, in characters: `message` 1 to 2000, `app_version` 1 to 64, `route` 1 to 200 (outside them: 422 on that field); a `message` over 2000 is rejected, never cut; only a `stack` over 8000 is cut to 8000 (a 422 would drop the report), so the logged `message` and `stack` never exceed their limits. It is rate limited by a token bucket per session, keyed by the session's token hash: a burst of 10, then one more every 6 seconds (429 with `Retry-After` when no token is left). The message and stack are the one raw outside text the app logs: they come from our own PWA and are the only way to debug it (see the exception in `.claude/rules/maintainability.md`).
*   The status screen is the main health signal for her and for the author. There is no Prometheus endpoint.

### Time and locale

*   The binary imports `time/tzdata`, so time zones work inside a distroless image.
*   "Today", the due days, the push hour and all views use `business_settings.timezone`.
*   Every offset is a whole number of calendar days in that zone. There are no hour offsets: the push is daily and she sends by hand, so an exact hour could not be delivered.
*   Spanish day and month names come from a small table in `internal/locale`. Release 1 has only `es`, but `locale` is the one place to add `en`.
*   UI strings live in `web/src/lib/i18n/es.json`. An English UI is out of scope, but keeping strings and formats in one place makes a second language a translation task later, not a code change. This follows priority 5 (generic and self-hostable by others).

### Security summary

*   Personal data (client phones, notes, photos) is only served with a session. Nothing is public except `/setup` (while no user exists), `/healthz`, `/readyz`, the manifest, the icons and the app shell.
*   Passwords are hashed with Argon2id (`golang.org/x/crypto/argon2`; parameters in item 38).
*   Images are distroless (`gcr.io/distroless/static-debian12:nonroot`) for `app`. CI runs `govulncheck`.
*   Cloudflare sees decrypted traffic at its edge: see "Accepted risks".

## Risks

### Accepted risks

*   **One person and one PC.** If the author is away or the PC breaks, nobody can upgrade, restore or reset her password. Her data survives in the backups, but reading them needs the restic password, which the author holds; the handover checklist (item 83) gives her a sealed copy to keep. The export helps only if she downloaded one recently: it needs the server running.
*   **The PC is off at times.** No push, no writes, no backups while it is off. She agreed to this. Two effects follow:
    *   A "before" reminder can be missed. If the PC stays off from the item's due day until the appointment starts, the item is never shown, because it is hidden after `starts_at`. Today's appointments are still in the cache, so she can message the client by hand.
    *   A quiet day and a day with the PC off look the same: no push on either. When she opens the app, the "server off" banner tells them apart.
*   **Password-only login on the public internet.** No second factor. Mitigated only by Argon2id, the login rate limit and the per-username delay. Passkeys and a private overlay network were weighed and rejected (Alternatives D3 and K3).
*   **Anyone who knows her username can slow her logins.** The per-username delay (up to 30 s between attempts, for 15 min after the last failure) applies to everyone trying that name, her included. An attacker can keep her waiting but cannot get in. Her existing sessions are not affected, so she only notices it if she has to log in again during an attack.
*   **Cached data stays readable on a phone with a closed session** until that phone makes its next request, which clears the cache (see "Phone cache and offline behavior"). A lost phone is protected by its own screen lock until then.
*   **The tunnel reaches the home network.** A bug in `app` could expose the Compose network on the author's PC. `app` runs as non-root in a distroless image, and no container publishes a port.
*   **Backups stay in the same house.** Fire or theft can take the PC and its backup disk together. Off-site backup is a future improvement.
*   **Cloudflare sees decrypted traffic** at its edge, because TLS ends there. Accepted for the reference install and written in `deploy/README.md`.
*   **A dead push subscription shows only on "Estado del sistema".** The re-subscribe check fixes most cases on the next open. Otherwise she notices it as the missing morning notification.
*   **The overlap check is not race-safe.** It takes no lock and has no exclusion constraint, and an edit reads then writes. One owner is effectively one writer, so the cost is a rare double booking when two of her devices save at the same moment; she sees both appointments in the day view and can move one.
*   **Ley 8968 (Costa Rica personal data law) is not checked yet.** Before her real data goes in, the author checks her obligations (consent, archive-only deletion, data held on a home PC). This is a step in the pre-handover checklist in `deploy/README.md`, next to the restore drill.

### Handled risks

Each of these is handled in the section named; the mechanism is not repeated here.

| Risk | Where it is handled |
| --- | --- |
| The VAPID key pair is lost, so every push subscription stops working | "Data model" (keys in the database, covered by the backup) |
| A push subscription expires or is deleted | "Daily push" (re-subscribe on open) and "Status screen" (red line) |
| The backup target is missing (a removable disk unplugged or not mounted), or a backup fails | "Backups" (marker check, red line at the next hourly check) and "Status screen" (the same red state also shows on "Hoy", where she looks every day) |
| An upgrade breaks the schema | "Upgrades and the migration guard" (pre-upgrade snapshot, guard, restore) |
| Cloudflare upload size and response timeout limits | "Photos" (phone-side resize) and "Export" (streamed ZIP); the current limits are an Open Question |
| iOS clears the PWA's storage | "Phone cache and offline behavior" (the cache is rebuilt from `/bootstrap` on the next open) |
| `/bootstrap` grows too large | "Phone cache and offline behavior" (size estimate; per-client history is fetched on demand) |

## Alternatives Considered

Grouped by area. Each alternative was rejected or narrowed during the brainstorm (see [concepts_brief.md](history/concepts_brief.md)) or during this draft. Choices kept unchanged from the SAD (Go, pgx + sqlc, one binary, goose, sessions in Postgres, the own template renderer, Web Push) keep the alternatives and reasons written in the [SAD's ADRs](history/sad-original.md); the status of each ADR is in item 13.

### Product: build vs use an existing tool

#### Alternative A1: SaaS (AgendaPro, Fresha, Booksy)

*   **Pros**: Ready today. Hosted. Booking pages and payments included.
*   **Cons**: Custom fields are limited and mostly cannot be used inside messages. Data lives on their platform. Monthly fee, and paid SMS.
*   **Reason for Rejection**: Does not solve the core gap (her attributes inside her messages). It also keeps her data off her own system.

#### Alternative A2: Free self-hosted booking tools (Easy!Appointments, Cal.com, Odoo Appointments)

*   **Pros**: Open source and free. Mature calendars.
*   **Cons**: Built around a public booking page. Heavy to run (Cal.com and Odoo need much more than one small container). Weak support for custom fields inside messages. Not designed for one-hand iPhone use.
*   **Reason for Rejection**: The work to adapt them is larger than building the narrow app she needs. The original PRD claimed that "no free self-hosted option exists", which was wrong; the PRD now says these tools exist and do not fit.

#### Alternative A3: General CRMs (Twenty, EspoCRM)

*   **Pros**: Custom fields, contact management.
*   **Cons**: No appointment model. Heavy (Twenty needs Redis and workers). Desktop-first.
*   **Reason for Rejection**: Wrong shape for an appointment business run from a phone.

#### Alternative A4: NocoDB or Baserow with formulas that build `wa.me` links

*   **Pros**: No backend code. Custom columns come free.
*   **Cons**: She would edit formulas. No come-back logic without an extra automation tool. No offline cache.
*   **Reason for Rejection**: Too technical for her, and the reminder rules do not fit a spreadsheet.

#### Alternative A5: Google Sheet + Apps Script, or iOS Contacts + Calendar + Shortcuts

*   **Pros**: Usable this week. Zero hosting.
*   **Cons**: No come-back rules, no attribute-driven messages, data spread over several apps.
*   **Outcome**: Accepted only as a stopgap while release 1 is built. Not the product.

### Client platform

#### Alternative B1: Native iOS app (SwiftUI)

*   **Pros**: Widgets, background work, push without the Home Screen install step.
*   **Cons**: Needs a Mac and an Apple Developer account. App review. A second client to maintain next to the web app.
*   **Reason for Rejection**: The PWA covers release 1 (push works in installed PWAs since iOS 16.4). The API does not change if a native app is ever needed (SAD ADR 015).

### Priorities

#### Alternative C1: Keep the SAD driver order (performance, then stranger self-hosting)

*   **Pros**: Pushes toward a polished open-source product.
*   **Cons**: Steers effort to S3, multi-arch images and latency gates before her onboarding exists.
*   **Reason for Rejection**: Her daily use comes first. Self-hosting for others moves to the Phase 3 gate.

### Ingress and hosting

#### Alternative D1: Caddy with Let's Encrypt (SAD ADR 011)

*   **Pros**: Automatic TLS on any VPS. No dependency on Cloudflare.
*   **Cons**: Needs inbound ports 80/443. Behind a Cloudflare Tunnel there are none, so the certificate is never issued. Publishing ports on a home network adds attack surface.
*   **Reason for Rejection**: Does not work for the reference install. An untested optional profile would ship with no one using it, so it waits for the Phase 3 gate.

#### Alternative D2: Cloudflare Access in front of the app

*   **Pros**: An extra login wall run by Cloudflare.
*   **Cons**: Its interaction with an installed PWA's cookies and with push notifications is not verified. It adds a second login for her.
*   **Reason for Rejection**: Risk to the core flow, with little gain over the app's own sessions.

#### Alternative D3: A private overlay network (Tailscale or WireGuard) instead of a public hostname

*   **Pros**: The app is not reachable from the public internet at all, so password guessing and the login-delay attack go away.
*   **Cons**: She must install and keep a VPN app running on her iPhone, and it must be connected for the app and for push links to work. A non-technical user who finds the app "broken" because the VPN dropped is the main failure mode. Push still works (it goes through Apple), but tapping it opens an app that cannot load.
*   **Reason for Rejection**: Adds a second app she must manage for every use. The public hostname with Argon2id, rate limiting and the per-username delay is the accepted trade (see "Risks").

### Reminders

#### Alternative E1: Stored reminders queue + background worker with `SKIP LOCKED` (SAD)

*   **Pros**: One model for manual and automatic (Cloud API) sending. Explicit state per reminder.
*   **Cons**: Stored due times go stale on every reschedule, rule edit or service change. After downtime, the worker releases a flood of out-of-date messages. The review found most reminder bugs in this design: every 24 h reminder skipped, duplicates, stale text, foreign-key failures on rule edits.
*   **Reason for Rejection**: In manual mode the server sends nothing, so the queue protects nothing. The due list is computed when read instead. A queue comes back with the Cloud API.

#### Alternative E2: Start come-back reminders from a manual "Completar" tap (SAD)

*   **Pros**: Explicit. No assumptions about what happened.
*   **Cons**: If she forgets the tap, the reminder is never created and nothing warns her. Batch-closing days later shifts the 21 days.
*   **Reason for Rejection**: Fails without any warning, which goes against Goal 3.

#### Alternative E3: An evening "close the day" prompt

*   **Pros**: She confirms each appointment, so the data is accurate.
*   **Cons**: A skipped prompt on a busy day loses those come-back reminders without any warning.
*   **Reason for Rejection**: The same silent failure as E2. The automatic "done" rule plus "No vino" gives her 21 days to fix mistakes.

#### Alternative E4: Per-service "before" rules (SAD)

*   **Pros**: Different text per service.
*   **Cons**: An appointment with two services sends two messages to the same client.
*   **Reason for Rejection**: Duplicate messages. The business-wide rule lists all services in one message.

#### Alternative E5: Skip come-back only for a future booking "of that service" (PRD FR-R7)

*   **Pros**: Matches the original PRD's wording. An unrelated later visit (a pedicure after an acrylic set) no longer drops the come-back, which the earlier "any later active appointment" rule did wrongly.
*   **Cons**: Taken literally, the rule is on "Acrílico" but the next booking is "Relleno", so a client who already rebooked would still get the come-back reminder.
*   **Outcome** (chosen in a modified form): the rule names a return service (empty = its own service), and only a later booking that includes it skips the item. The return service fixes the Acrílico → Relleno problem; the same-service check fixes the pedicure problem. A future booking without the return service turns the item into a booking prompt for her, so she can add the service to that appointment instead of messaging a client who is already coming.
*   **Rejected with it**: a list of return services per rule. One nullable column covers acrylic → refill; a list can be added later (a join table, filled from the column) without rewriting stored data.

#### Alternative E6: Template text stored inline on each rule (SAD)

*   **Pros**: One less table.
*   **Cons**: She would retype the same message per service and keep the copies in sync.
*   **Reason for Rejection**: A template library is used instead.

#### Alternative E7: A rule language or expression engine

*   **Pros**: Any rule can be expressed.
*   **Cons**: She cannot use it. It adds a parser to maintain.
*   **Reason for Rejection**: Two fixed rule kinds (`before_appointment` and `come_back`) cover release 1.

#### Alternative E8: Rendering templates in the browser

*   **Pros**: Previews and sending would work while the server is off.
*   **Cons**: Two renderers (Go and TypeScript) would drift.
*   **Reason for Rejection**: The server renders. The phone caches rendered text from the last refresh.

### Notifications

#### Alternative F1: No push

*   **Pros**: Less work. No VAPID keys or subscriptions.
*   **Cons**: On days without appointments she may not open the app, and come-back reminders wait.
*   **Reason for Rejection**: Come-back reminders often land on quiet days. One daily push protects Goal 3.

#### Alternative F2: One push per reminder as it becomes due (SAD)

*   **Pros**: Each message arrives at its exact time.
*   **Cons**: Many notifications. After downtime they arrive in a burst.
*   **Reason for Rejection**: Noise. One daily summary is enough for a list she works through in one sitting.

### Offline and sync

#### Alternative G1: Create appointments and clients on the phone while the server is off

*   **Pros**: She can book while the PC is off.
*   **Cons**: Pending states in the UI. The overlap check runs only against the cache. A server rejection must be handled after the fact. Data is lost if iOS clears storage before the upload.
*   **Reason for Rejection**: Too many failure cases for release 1. The `uuid` columns accept phone-generated IDs later without schema changes.

#### Alternative G2: Queue "Sí, lo envié" taps on the phone while the server is off

*   **Pros**: She can work through "Para enviar hoy" while the PC is off.
*   **Cons**: The cached list can be stale, so she could send a "before" reminder after the appointment started. A Dexie schema change can wipe unsent taps. A rejected replay needs its own handling. It breaks Goal 6, because a tap shows as saved before the server has it.
*   **Reason for Rejection**: The edge cases cost more than they save. She marks the items once the server is back.

#### Alternative G3: Full offline editing with conflict merge

*   **Pros**: The app works the same with or without the server.
*   **Cons**: Conflict rules for every entity. Hard to test.
*   **Reason for Rejection**: Far more work than one user needs.

#### Alternative G4: Delta sync (`GET /sync?since=`, SAD)

*   **Pros**: Small refresh payloads.
*   **Cons**: The schema cannot support it: most tables have no `updated_at`, nothing records deletions, and a UUIDv7 cursor orders by creation, not by update.
*   **Reason for Rejection**: At this data size a date-window refresh is small enough (size estimate in "Phone cache and offline behavior").

#### Alternative G5: Server-side client search (`pg_trgm` + `unaccent`), with the phone search only for offline

*   **Pros**: Works even if the phone does not hold every client. Postgres ranks by trigram similarity.
*   **Cons**: Two search implementations that cannot rank the same way (JavaScript has no `word_similarity`). Two extensions, an `f_unaccent` wrapper for PG 17+ `search_path` rules, two GIN indexes, escaping rules and DB tests, all for a list the phone already holds.
*   **Reason for Rejection**: `/bootstrap` already sends every active client, and the offline search has to exist anyway. If a later release stops sending all clients (Phase 3, large businesses), server search comes back then.

### Data and backups

#### Alternative H1: SQLite + Litestream

*   **Pros**: Continuous off-site copy within seconds. One less container. No DB password.
*   **Cons**: Less familiar to the author. A harder move if staff accounts or public booking come later. The SAD data layer would need a rewrite.
*   **Reason for Rejection**: PostgreSQL is kept for familiarity and as a long-term base.

#### Alternative H2: PostgreSQL 17 or 19

*   **Pros**: 17 has the longest production history. 19 has the newest features.
*   **Cons**: 17 reaches end of support first. 19's general release is planned for November 19, 2026, so it would be weeks old at project start.
*   **Reason for Rejection**: 18 was released in September 2025, and PostgreSQL supports each major version for 5 years (18 until late 2030).

#### Alternative H3: Hourly dumps, or continuous WAL archiving

*   **Pros**: Lose at most one hour (hourly dumps), or minutes (WAL archiving).
*   **Cons**: More runs, or another tool (wal-g, pgBackRest) and a harder restore.
*   **Reason for Rejection**: A daily dump is accepted. The risk is losing up to one day, which she can re-enter from WhatsApp.

#### Alternative H4: Cloud storage for the off-site copy (Cloudflare R2, Backblaze B2, Google Drive)

*   **Pros**: Survives fire, theft or a power surge at home.
*   **Cons**: Needs a cloud account and maybe a cost.
*   **Author preference**: a USB disk or second machine, with no cloud account. Accepted risk: one event at home can destroy both copies.

#### Alternative H5: Backups from a host systemd timer

*   **Pros**: `Persistent=true` runs missed jobs at boot. The host can mount the USB disk on demand.
*   **Cons**: Outside the Compose install. Needs systemd. Self-hosters must set it up by hand.
*   **Reason for Rejection**: One backup container inside Compose keeps the install in one file. The check at container start plus the hourly check covers missed days.

#### Alternative H6: Two backup containers (postgres-backup-local + a restic/rclone copier)

*   **Pros**: A maintained image for the dump step.
*   **Cons**: Two schedules that can get out of step (the copy runs before the dump finishes). Two places to check when something fails.
*   **Reason for Rejection**: One script controls the whole sequence and writes one status file.

#### Alternative H7: Full export including photos

*   **Pros**: A complete copy of her data.
*   **Cons**: The ZIP can be several GB and must stream through the tunnel.
*   **Reason for Rejection**: Release 1 exports data without photos. Backups cover photos.

#### Alternative H8: No export

*   **Pros**: No work.
*   **Cons**: Only the author could get her data out.
*   **Reason for Rejection**: She needs to be able to leave without him.

### Attributes

#### Alternative I1: The SAD attribute model (7 types, option trees of any depth, service-limited attributes)

*   **Pros**: Covers many future cases.
*   **Cons**: Designed before anyone listed her real attributes. The value format and the change rules were undefined. Seven types and service-limited attributes add screens and rules that no listed need asks for yet.
*   **Outcome** (narrowed, not fully rejected): release 1 keeps the option trees of any depth, because she asked for nested choices (for example shape → length) and a value stores just the picked option ID, so depth does not change the stored shape. It keeps 5 types and a written value table, with no service-limited attributes. A real need from her daily use can widen it later.

#### Alternative I2: EAV tables or `pg_jsonschema`

*   **Pros**: Database-level typing of attribute values.
*   **Cons**: More tables and joins (EAV), or a Postgres extension to install (`pg_jsonschema`).
*   **Reason for Rejection**: JSONB plus one Go validator is enough for this volume.

### API and client stack

#### Alternative J1: Code-first API with hand-written TypeScript types

*   **Pros**: Faster at the start.
*   **Cons**: Go and TypeScript types drift, and nothing catches it.
*   **Reason for Rejection**: Spec-first gives typed code on both sides from one file.

#### Alternative J2: OpenAPI 3.1 with `ogen`

*   **Pros**: Keeps OpenAPI 3.1.
*   **Cons**: A less common generator.
*   **Reason for Rejection**: 3.0.3 with `oapi-codegen` avoids the unverified 3.1 support question.

#### Alternative J3: Server-side WebP thumbnails (SAD)

*   **Pros**: Smaller files.
*   **Cons**: WebP encoding in Go usually needs cgo, which breaks the static distroless build.
*   **Reason for Rejection**: The phone resizes to JPEG before upload.

#### Alternative J4: Unofficial WhatsApp libraries (whatsmeow, Baileys)

*   **Pros**: Automatic sending without Meta's approval process.
*   **Cons**: Against WhatsApp's terms. Risk of a ban on her business number.
*   **Reason for Rejection**: Her business number is too important to risk.

### Auth and setup

#### Alternative K1: One-time code typed inside the installed PWA (brief)

*   **Pros**: Shorter to type than a password. No password typing on the phone at handover.
*   **Cons**: A second login mechanism next to username and password: a code screen, code storage, and expiry.
*   **Reason for Rejection**: The wizard already creates her username and password, so logging in with them in the installed app gives the same result with one fewer mechanism.

#### Alternative K2: Setup link printed only in Docker logs, with no wizard (SAD)

*   **Pros**: Simple.
*   **Cons**: No business settings, no branding, and no guidance to install. Only the author can act on it.
*   **Outcome**: Kept for the author's first step only. The wizard does the rest.

#### Alternative K3: Passkeys (WebAuthn) instead of a password

*   **Pros**: No password to guess or phish. iOS Safari supports passkeys.
*   **Cons**: A passkey is bound to the `PUBLIC_URL` domain, so a domain change locks her out. Recovery needs a second path anyway (the `reset-password` command would have to issue a new registration link). It adds a WebAuthn library, a credentials table and registration screens. Whether passkey prompts work the same in an installed iOS PWA as in Safari is not checked.
*   **Reason for Rejection**: More code and an unverified iOS path for release 1. A candidate for a later release, next to the password.

### Domain

#### Alternative L1: Her own domain from day one

*   **Pros**: Her brand and her property. Ready for Phase 3 public booking.
*   **Cons**: Some cost and setup.
*   **Reason for Rejection**: A subdomain of the author's domain is free. Moving later costs a reinstall and a new login. Revisit before Phase 3.

### Web client

#### Alternative M1: React + Vite as a static SPA

*   **Pros**: A much larger ecosystem (calendars, forms, Dexie hooks). More contributors know it. More examples for iOS PWA edge cases. Like SvelteKit, it builds to static files embedded in the Go binary, with no Node server.
*   **Cons**: None measured for this app.
*   **Author preference**: SvelteKit is kept. The SAD's old reason ("no separate Node server") applied only to Next.js with SSR, not to React.

#### Alternative M2: Plain CSS with design tokens

*   **Pros**: No styling dependency. The runtime accent color is one CSS variable.
*   **Cons**: Slower to style every screen by hand.
*   **Reason for Rejection**: Tailwind v4 is faster to style with, and it can still read the accent color from a CSS variable.

#### Alternative M3: shadcn-svelte (Tailwind + bits-ui)

*   **Pros**: Ready-made, accessible dialogs, sheets and comboboxes.
*   **Cons**: Copied component code must be maintained. Svelte 5 support level is unverified. The default look reads as generic.
*   **Reason for Rejection**: Own components on Tailwind keep the dependency surface small. The app needs only a few interactive components (bottom sheet, chip editor, calendar views).

--------------------------------------------------------------------------------

## Non-Functional Requirements

Each line points to the section that holds the detail.

*   **Security & Privacy**: personal data only with a session, Argon2id, distroless image ("Security summary"). EXIF removed on the phone ("Photos"). Secrets only in `.env`; the VAPID key pair in the database ("Data model"). Deletion is archive-only; Ley 8968 is a go-live check ("Accepted risks").
*   **Performance & Latency**: "Hoy" opens in under 1 s on her iPhone over LTE through the tunnel (Goal 6; method in the Verification Plan). There is no server latency SLO.
*   **Availability**: the server is off at times, and this is accepted. Behavior while off: CUJ 6 and "Phone cache and offline behavior". Catch-up after start: "Process model" (push until 20:00) and "Backups" (check at container start).
*   **Rollout & Rollback**: pinned image tags; backup before every migration; rollback by restore ("Upgrades and the migration guard"). API changes inside `/api/v1` are additive, so a cached older SPA keeps working until it reloads.

--------------------------------------------------------------------------------

## Additional Resources

*   [concepts_brief.md](history/concepts_brief.md) — decisions from the brainstorm.
*   [goldfish_brainstorm_findings.md](history/goldfish_brainstorm_findings.md) — the full review of the PRD and SAD (9 Blocker, 31 Major, 14 Nit).
*   [PRD](../prd.md) (updated for release 1) and the [original SAD](history/sad-original.md); the SAD's ADR index is [docs/sad.md](../sad.md).
*   [PostgreSQL 19 RC 1 announcement](https://www.postgresql.org/about/news/postgresql-19-rc-1-3409/) — source for the version decision.

## Future Improvements

*   **WhatsApp Cloud API**: automatic sending. Brings back a stored queue and a worker, and message templates that Meta must approve before use.
*   **Offline creates**: an outbox for new appointments and clients, using phone-generated IDs.
*   **Off-site cloud backup**: a restic repository on cloud storage, to protect against fire or theft at home.
*   **Read-only ICS calendar feed** for iOS Calendar while the server is off.
*   **Hard delete with a full cascade**, including photo files and the backup retention disclosure.
*   **Her own domain** before public booking (Phase 3).
*   **On-device AI** (FR-I1 to FR-I5), each re-justified against what iOS already does.
*   **"Group" attribute type**: each picked option carries its own typed value (for example "Alergias" with Acrílico = sí and Gel = a date). It is added as a new type, so existing stored values are never rewritten. Added only when a real need shows up.
*   **Owner tasks** (PRD FR-R2): a rule that reminds her to do something herself, for example "call Ana 30 days after a treatment". Added back only when a real need shows up. It needs one new `kind` value, a nullable `task_text` column and a `done` log action; no stored data changes.
*   **Presets per business type**: her exported configuration becomes the first "nail studio" preset, stored as data.

--------------------------------------------------------------------------------

## Detailed Implementation Plan

Conventions used below:

*   **Repository and names (assumed):** `github.com/jnaranj09/open-customer`; images at `ghcr.io/jnaranj09/open-customer` and `ghcr.io/jnaranj09/open-customer-backup`. If the repo lives elsewhere: run `go mod edit -module <new path>`, replace the old path in every `.go` file and in `.golangci.yml`, and change the image names in `Makefile`, `deploy/.env.example`, `deploy/compose.yaml` and `.github/workflows/release.yml`.
*   **Toolchain:** the current stable Go and Node LTS at project start, pinned in `go.mod` (`go` and `toolchain` lines; the `go` line names the `.0` patch, for example `go 1.N.0` with `toolchain go1.N.P`, because `go mod tidy` drops a `toolchain` line equal to the `go` line) and `web/package.json` (`engines`, `packageManager`). Code generators are pinned as in "Pins" below. The web package manager is pnpm.
*   **Tests:** Go unit tests use a fake clock. Database tests use `testcontainers-go` with the pinned `postgres:18.<minor>-alpine`. Web unit tests use Vitest. End-to-end tests use Playwright with the WebKit engine (closest to iOS Safari).
*   **File paths** are relative to the repo root.
*   **Pins:** every tool version is fixed at P0, each in the one place the table names. The exceptions are pins that several files must share:
    *   **Postgres:** the source is `PG_IMAGE` in the `Makefile`. The tests read it from the `PG_IMAGE` env var, which `make` exports. `deploy/compose.yaml`, `deploy/backup/Dockerfile` and the CI service container cannot read the `Makefile`, so they repeat the tag, and `make check-pins` (a CI step from P0) fails if any `postgres:` tag in the repo differs from `PG_IMAGE`.
    *   **Node and Go images:** `node:` and `golang:` in the `Dockerfile` must match `engines.node` and the `toolchain` line. `make check-pins` compares them too.
    *   **golangci-lint:** the `Makefile` variable `GOLANGCI_LINT_VERSION` is the source; `make check-pins` fails if the CI action's `version:` differs.
    *   Dependabot ignores `postgres`, `node` and `golang` image updates (item 10), so each group is bumped by hand, all copies in one commit.

    | Tool | Pinned in | How it is installed |
    | --- | --- | --- |
    | Go | `go.mod` (`go`, `toolchain`) | `actions/setup-go` with `go-version-file: go.mod` |
    | Node | `web/package.json` `engines.node` (one exact version) | `actions/setup-node` with `node-version-file: web/package.json` |
    | pnpm | `web/package.json` `packageManager` (one exact version) | `pnpm/action-setup` in CI with `package_json_file: web/package.json` (the repo has no root `package.json`); `npm install -g pnpm@<that version>` in the Dockerfile's Node stage. Not Corepack, which Node 25 and later no longer ship. |
    | oapi-codegen, govulncheck, deadcode | `go.mod` `tool` lines | `go tool <name>` |
    | sqlc | `Makefile` variable `SQLC_IMAGE` (`sqlc/sqlc:<version>`) | `docker run` |
    | golangci-lint | `Makefile` variable and the CI action's `version:` (one exact v2.x version; `.golangci.yml` uses `version: "2"`) | `golangci/golangci-lint-action` in CI, the same version locally |
    | Playwright | exact `@playwright/test` version in `web/package.json` | `pnpm exec playwright install --with-deps webkit`, which matches the package version |
    | Postgres | `PG_IMAGE` in the `Makefile` (`postgres:18.<minor>-alpine`); copies in `deploy/compose.yaml`, `deploy/backup/Dockerfile` and the CI service, checked by `make check-pins` | image tag |
    | restic, jq | the Alpine packages of the pinned Postgres image's Alpine release | `apk add`; the backup log prints `restic version` |
    | cloudflared | exact tag in `compose.yaml` | image tag |
    | distroless | `gcr.io/distroless/static-debian12:nonroot@sha256:<digest>` in the `Dockerfile` | image digest; Dependabot updates it |

### Build order

The items below are grouped by area, not by order. Build them in these phases. A phase is done only when its exit check passes on a clean clone, and the next phase starts from a green CI run.

*   **The spec grows phase by phase.** `api/openapi.yaml` (item 15) gets only the endpoints of the current phase, so the generated server interface never has methods without a handler. `POST /templates/preview` is added in P4 with `reminders`, not in P3 with `templates`.
*   **The web toolchain exists from P0.** P0 creates items 62–63, a minimal `web/src/app.html` and a placeholder `web/src/routes/+page.svelte`, so `make lint test vuln build` works from the first commit. `make generate` and `check-generated` start in P1, the first phase with a spec and queries. P5 replaces the placeholders with items 64 and 72.
*   **Code that needs a later phase is added in that phase.** An item listed in an earlier phase gets only the parts its phase can compile: `serve.go` (item 18) and `status` (item 60) say which part comes when. Setup (item 40) takes only the settings creator and seeds nothing, so P2 does not depend on the P3–P4 modules.
*   **CI steps start in a given phase** (item 8), so CI never runs a check whose inputs do not exist yet.
*   **The agent harness exists before P0** (item 88). Its `TASKS.md` T1 is P0; the later tasks add lint rules, guards and sensors, and each one updates the item it changes here.

| Phase | Items | What exists at the end | Exit check |
| --- | --- | --- | --- |
| P0 Repo and tooling | 1–5, 7–10, 17 (`version`), 20–24, 36, 62–63 (plus the placeholder page) | Module, `Makefile`, `web-stub`, the web toolchain, lint config, CI, the platform packages | CI green: `make lint test vuln build check-pins` |
| P1 Database and HTTP skeleton | 6 (`sqlc.yaml`; it needs the migrations, so it moved here from P0), 15–16 (`GET /setup/status` only), 17 (`serve`), 18 (P1 steps), 25–32, 33 (`handlers_auth.go`: `GET /setup/status` only), 34 (steps 1–5), 35, 37 (middleware steps 1–5, problem JSON, static), 40 (`SetupNeeded` only), 60 (`/healthz`, `/readyz`), 61, 66 (`schema.d.ts` only, generated by `make generate`) | `serve` migrates an empty DB and serves `/healthz`, `/readyz`, `GET /api/v1/setup/status` and the placeholder page | `make check-generated test test-db`; `go run ./cmd/open-customer serve` against Docker Postgres returns 200 on `/readyz` and `{"needs_setup":true}` on `/api/v1/setup/status` |
| P2 Auth, setup and settings | 17 (`reset-password`, `setup-link`, `healthcheck`), 18 (P2 step), 19, 33 (auth, settings), 34 (steps 6–8), 37 (session and `must_change_password` steps), 38–42, 44 | Setup link, owner creation (setup creates nothing else), login, sessions, settings, manifest, icons | `make test test-db cover` (gate: `auth`); with `curl -H "Origin: $PUBLIC_URL"`: setup → login → `GET /me` → logout |
| P3 Catalog | 33 (catalog), 37 (409 overlap responses), 45–52, 53 (without `sample.go`), 54 | Attributes, clients, services, appointments, media, templates | `make test test-db cover` (gates added: `attributes`, `appointments`, `templates`) |
| P4 Reminders, push and export | 18 (P4 steps), 28 (migration `00002`), 33 (reminders and `POST /templates/preview`, push, export, status, bootstrap), 53 (`sample.go`), 55–59, 60 (`Status()`, client errors) | Due list (messages and booking prompts), reminder log, daily push, export, the status screen data | `make test test-db cover` (gate added: `reminders`); a `db`-tagged Go test in `internal/reminders` (`cuj_db_test.go`) inserts the CUJ 4 and 5 rows (CUJ 5 including the booking prompt), and `GET /reminders/due` returns those items |
| P5 Web app | 62–63 (finished), 64–78 | The PWA on top of the finished API | `make lint test build`; the binary serves the real app; manual pass of CUJ 1–6 in Safari's iPhone view, and of each main screen at 1440 × 900 (desktop layout) |
| P6 End-to-end tests | 79–80 | Playwright specs | `make e2e` green in CI |
| P7 Deploy, backups and docs | 9 (backup-image step), 11–14, 81–87 | Compose stack, backup image and scripts, docs moved, root files deleted | The Verification Plan's manual steps on the PC, including the restore drill |

### Target Files to Create/Modify

#### Repository root

##### 1. `go.mod`, `go.sum` (Create)

*   **Type / Scope**: Go module.
*   **Proposed Changes**:
    *   Module `github.com/jnaranj09/open-customer`.
    *   Dependencies:
        *   `github.com/jackc/pgx/v5` (including `pgx/v5/pgxpool` and `pgx/v5/stdlib`), `github.com/pressly/goose/v3`, `github.com/oapi-codegen/runtime`
        *   `github.com/google/uuid` v1.6.0 or later (UUIDv7; `NewV7` returns an error, which is handled)
        *   `github.com/nyaruka/phonenumbers`
        *   `golang.org/x/crypto` (argon2), `golang.org/x/time` (rate), `golang.org/x/text` (currency), `golang.org/x/sys` (`unix.Statfs`)
        *   `github.com/oapi-codegen/nullable` (the `nullable.Nullable[T]` type from `nullable-type`)
        *   `github.com/SherClockHolmes/webpush-go` v1.4.0 (it pulls in `github.com/golang-jwt/jwt/v5`)
        *   `github.com/testcontainers/testcontainers-go` and its `modules/postgres` (test only)
    *   `tool` lines for `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`, `golang.org/x/vuln/cmd/govulncheck` and `golang.org/x/tools/cmd/deadcode` (the dead-code check in `make lint`, `TASKS.md` T7). There is no goose `tool` line: the goose CLI pulls in every database driver, and the app runs migrations itself. A new migration is a hand-made file named `0000N_<name>.sql` with the markers from item 28.
    *   `sqlc` is **not** a `tool` line: its PostgreSQL parser needs cgo, which breaks the `CGO_ENABLED=0` setup. `make generate` runs it from the pinned `sqlc/sqlc` Docker image instead.

##### 2. `Makefile` (Create)

*   **Type / Scope**: Developer entry point.
*   **Proposed Changes**: Targets:

    | Target | What it runs |
    | --- | --- |
    | `generate` | oapi-codegen, sqlc, openapi-typescript (from P1; P0 has no spec or queries) |
    | `check-generated` | `generate`, then fails if `git status --porcelain` shows a change under `internal/httpapi/gen`, `internal/db/sqlc` or `web/src/lib/api/schema.d.ts`, so modified and new untracked generated files are caught, but unrelated uncommitted work is not |
    | `check-pins` | Runs `scripts/check-pins.sh`. Scans only `deploy/compose.yaml`, `deploy/backup/Dockerfile`, `.github/workflows/ci.yml` and the root `Dockerfile` (each once it exists; never `docs/` or `*.md`). Fails if a `postgres:` tag differs from `PG_IMAGE`, if the `Dockerfile`'s `node:`, `golang:` and `pnpm@` versions differ from `engines.node`, the `toolchain` line and `packageManager`, or if the CI golangci-lint `version:` differs from `GOLANGCI_LINT_VERSION` (see "Pins") |
    | `web-stub` | Creates `web/build/index.html` with the text "Run make web" if the file is missing. Never overwrites a real build. |
    | `node-check` | Fails unless `node --version` equals `engines.node` (with fnm: `fnm use` in `web/`). `web`, `test` and `lint` depend on it |
    | `web` | pnpm build into `web/build`, with `VERSION` exported |
    | `build` | `web`, then the Go binary at `bin/open-customer` |
    | `test` | `web-stub`, then Go + web unit tests (`vitest run --passWithNoTests`, kept from P0, when there were no web tests) |
    | `test-db` | `web-stub`, then Go tests tagged `db` |
    | `vuln` | `web-stub`, then `govulncheck` on `$(GO_PKGS)` |
    | `e2e` | `build`; then drop and recreate the e2e database; then `pnpm e2e` (Playwright, item 79). If `E2E_DATABASE_URL` is already set (CI sets it to its service container), no container is started: `psql` against the same server's `postgres` database runs `DROP DATABASE IF EXISTS … WITH (FORCE)` and `CREATE DATABASE …` for the database the URL names. The target refuses unless that name ends in `_e2e`, so a mistyped URL cannot drop a real database. Otherwise the target starts or reuses the `dev` Postgres container (the same recipe as `dev`), recreates `open_customer_e2e` on it with `dropdb --if-exists --force` and `createdb` through `docker exec`, and sets `E2E_DATABASE_URL=postgres://open_customer:devpassword@localhost:$(DEV_PG_PORT)/open_customer_e2e`. |
    | `lint` | `web-stub`, then `go mod tidy -diff` (fails on `go.mod` or `go.sum` drift), golangci-lint on `$(GO_DIRS)` (installed by the Makefile into `bin/tools/golangci-lint-<version>/`; CI passes `GOLANGCI_LINT=golangci-lint` from its action), `go build` of `$(GO_BUILD_PKGS)` (the packages with non-test files) for `GOOS=darwin` and `GOOS=windows` (`GOARCH=amd64`, `CGO_ENABLED=0`; the app runs on Linux, but `make dev` runs `serve` on the host, so an OS-specific call must not break other systems), `go tool deadcode -test -tags db` (so the db-tagged tests are roots and the `dbtest` helper is not dead) against an exact `DEADCODE_BUDGET`, shellcheck on every `*.sh`, `pnpm check` (`svelte-kit sync` + svelte-check + the worker's `tsc`), and `pnpm lint` (eslint, prettier check, knip, jscpd) |
    | `cover` | `web-stub`, then `go test -tags db -coverprofile` for each package in `COVER_PKGS`; writes `coverage-<name>.out` and fails, naming the package, if any package's total (`go tool cover -func`) is below `COVER_FLOOR` (90.0). `COVER_PKGS` grows by phase: `auth` (P2); `attributes`, `appointments`, `templates` (P3); `reminders` (P4) |
    | `dev` | `web-stub`, then Docker Postgres (`PG_IMAGE`) in the container `open-customer-dev` on host port `DEV_PG_PORT` (default `5433`; started or reused, password `devpassword`, dev only), the binary built to `bin/dev/open-customer` running `serve` on `:8080` in the background, and the Vite dev server on `:5173` in the foreground with the proxy from item 63. A `trap` stops `serve` when Vite exits; it runs the built binary, not `go run`, because `go run` does not pass SIGTERM on to the server. Env and the Origin rule are below the table. |
    | `image` | `docker build --build-arg VERSION=$(VERSION)` of the app image |
    | `test-deploy` | Runs `scripts/test-deploy.sh`: builds `deploy/backup` as `open-customer-backup:test`, starts `$(PG_IMAGE)` as `oc-deploy-pg` on a new network `oc-deploy-net` (no published port), runs `deploy/tests/backup_test.sh` inside the backup image (entrypoint overridden, `deploy/` mounted read-only) against it, and removes the containers (with their anonymous volumes, `docker rm -f -v`) and the network in a `trap`, pass or fail. Then it runs `deploy/tests/scripts_test.sh` on the host: each operator script runs from a temp copy of `deploy/` with its own `.env` and a fake `docker` first on `PATH` that logs each call and answers from `FAKE_APP_VERSION`, `FAKE_SNAPSHOT_JSON` and `FAKE_FAIL_ON`; only its `compose config is valid` test uses the real `docker compose config`, which starts nothing. The backup tests run for real, not against stubs, because `backup.sh` owns the outcome table, the marker rule and the restore, and a stub would not catch a wrong restic flag |

    *   **`dev` env:** the Go server gets `PUBLIC_URL=http://localhost:5173`, `DB_HOST=localhost`, `DB_PORT=$(DEV_PG_PORT)` and `DB_PASSWORD=devpassword` for the container, `VAPID_SUBJECT=mailto:dev@localhost` and `ALLOW_MIGRATE_WITHOUT_BACKUP=true` (there is no backup status file in dev). The browser's `Origin` is `http://localhost:5173`, and the Vite proxy forwards it unchanged, so the Origin check passes.
    *   `VERSION ?= $(or $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//'),dev)`. The same value goes to `-ldflags`, the web build and `make image`; the release workflow sets it from the tag.
    *   `GO_PKGS := $(shell go list -e ./... | grep -v /node_modules/)`. Every Go target uses it instead of a bare `./...`, so Go never walks into `web/node_modules` (some npm packages ship `.go` files). golangci-lint takes directories, not import paths, so `lint` uses `GO_DIRS` (the same list as `./`-relative folders). `go build` refuses a package with only test files (`internal/guards`), so the cross-OS builds in `lint` use `GO_BUILD_PKGS` (the same list, kept to packages with non-test files).
    *   **Targets by phase:** P0 has `check-pins`, `node-check`, `web-stub`, `web`, `build`, `test`, `vuln`, `lint` and `image`. `generate` and `check-generated` come in P1, `test-db` in P1, `cover` in P2, `dev` in P1 (it needs `serve`), `e2e` in P6, `test-deploy` in P7.
    *   Why `web-stub`: `web/embed.go` has `//go:embed all:build`, and `web/build` is gitignored. On a clean checkout the embed has no files, so every Go command fails to compile. The stub gives the embed one file without running Node. A plain `go build` without `make` on a clean checkout still fails, with the embed error; `README.md` says to use `make`.

    Why: one command set for the author and for CI.

##### 3. `README.md` (Create)

*   **Type / Scope**: Documentation.
*   **Proposed Changes**: What the app is, who it is for, a link to `deploy/README.md` for install, the `make` targets for development (and that Go commands must go through `make` on a clean checkout, because of `web-stub`), and links to `docs/designs/release-1.md`, `docs/sad.md` and `docs/prd.md`. It states that the license is not chosen yet.

##### 4. `.gitignore` (Create)

*   **Type / Scope**: Repo config.
*   **Proposed Changes**: Ignore `web/build`, `web/node_modules`, `web/.svelte-kit`, `web/.eslintcache` (written by the edit hook, item 88), `deploy/.env`, `bin/`, `coverage*`, `.claude/settings.local.json`, and StrykerJS's work folders `web/.stryker-tmp` and `web/reports` (`TASKS.md` T9). Generated code is **not** ignored, because CI checks it for drift. Written before P0 with the harness (item 88).

##### 5. `.golangci.yml` (Create)

*   **Type / Scope**: Lint config.
*   **Proposed Changes**: Enable `govet`, `staticcheck`, `errcheck`, `gosec`, `revive`, `depguard`, plus `exhaustive` (a `default` does not count as covering the cases), `wrapcheck`, `nilerr`, `nolintlint` (a reason and a linter name are required), `thelper`, `tparallel`, `unused` and `dupl` (threshold 80: a pasted 20-line function trips it), from `TASKS.md` T4, T5 and T7; `gofmt` as formatter. Tests are excluded from `dupl` and `wrapcheck`. `wrapcheck` also ignores `problem.Field(` (`extra-ignore-sigs`): it builds a final error value, like `errors.New`, so there is nothing to wrap. `internal/platform/problem/messages.go` is excluded from gosec G101 only: it reads a Spanish message constant named `MsgPasswordTooShort` as a credential. `internal/auth/sessions.go` is excluded from gosec G124 only: the session cookie's `Secure` follows `PUBLIC_URL` and is false only for plain-http localhost (development and e2e), where WebKit drops a `Secure` cookie ("Session and request rules"). `run.build-tags: [db]` makes lint also check the `db`-tagged tests. Lint runs on `$(GO_DIRS)` through `make lint`, also in CI, so `web/node_modules` is never loaded. `depguard` enforces only the coarse module rules: no domain package imports `internal/httpapi` or `internal/httpapi/gen`; `internal/auth` imports no domain package; and nothing outside `httpapi`, `scheduler` and `cmd/open-customer` imports `internal/reminders`. The per-module list of allowed domain imports ("Code layout and module rules") is checked in review, not by lint, so the config stays small.

##### 6. `sqlc.yaml` (Create)

*   **Type / Scope**: Code generation config.
*   **Proposed Changes**: Engine `postgresql`. Schema from `internal/db/migrations`, queries from `internal/db/queries`, output to `internal/db/sqlc` with `sql_package: pgx/v5` and `emit_pointers_for_null_types: true`. Overrides:

    | `db_type` | `nullable` | Go type |
    | --- | --- | --- |
    | `uuid` | false | `github.com/google/uuid.UUID` |
    | `uuid` | true | `github.com/google/uuid.NullUUID` |
    | `jsonb` | false | `encoding/json.RawMessage` |
    | `timestamptz` | false | `time.Time` |
    | `timestamptz` | true | `time.Time` with `pointer: true` (the object form of `go_type`: `go_type: {import: time, type: Time, pointer: true}`; verified against sqlc 1.31.1, which generates `*time.Time`) |
    | `date` | false | `time.Time` |

    Nullable `text`, `int` and `bigint` become pointers through `emit_pointers_for_null_types`.

##### 7. `Dockerfile` (Create)

*   **Type / Scope**: App image.
*   **Proposed Changes**: Three stages:
    1.  Node (`node:<exact version>-alpine`, the same version as `engines.node`): `npm install -g pnpm@<packageManager version>`, then `pnpm install --frozen-lockfile && VERSION=$VERSION pnpm build` in `web/`, so the SPA and the binary carry the same version.
    2.  Go (`golang:<exact version>-alpine`, the same version as the `toolchain` line): copies `web/build`, then `CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$VERSION"`. This stage also creates empty `/data/media` and `/backup-status` folders.
    3.  `gcr.io/distroless/static-debian12:nonroot@sha256:<digest>` (the pin in "Pins") with the binary, and `COPY --chown=65532:65532` of the two empty folders.

    `ARG VERSION=dev` is declared before the first stage and again in stages 1 and 2. It is set with `docker build --build-arg VERSION=<version>` by `make image` and by the release workflow.

    The binary is copied to `/usr/local/bin/open-customer`, which is on the distroless `PATH`, so the Compose healthcheck (`open-customer healthcheck`) and `docker compose exec app open-customer …` find it by name. `ENTRYPOINT ["/usr/local/bin/open-customer"]`, `CMD ["serve"]`.

    A root `.dockerignore` keeps `.git`, `bin`, `web/node_modules`, `web/build`, `web/.svelte-kit`, `deploy/.env` and coverage files out of the build context. The Node stage also copies `web/pnpm-workspace.yaml` (item 62) before `pnpm install`.

##### 8. `.github/workflows/ci.yml` (Create)

*   **Type / Scope**: CI.
*   **Proposed Changes**: On push and PR, with `permissions: contents: read`. Each step is added in the phase shown:

    | Step | From phase |
    | --- | --- |
    | Setup: `actions/checkout` with `fetch-depth: 0`, `actions/setup-go`, `pnpm/action-setup`, `actions/setup-node`, `pnpm install --frozen-lockfile` in `web/`, `golangci/golangci-lint-action` with `install-only: true` and the pinned `version:` | P0 |
    | Secret scan: `gitleaks/gitleaks-action` over the whole history (no license key is needed for a personal account). Reviewed false positives are listed by fingerprint, each with its reason, in `.gitleaksignore` (today the item 19 temp-password alphabet and one made-up hash fixture in a test) | P0 |
    | `pnpm audit --audit-level high` in `web/` | P0 |
    | `make check-pins` | P0 |
    | `make lint GOLANGCI_LINT=golangci-lint` | P0 |
    | `make test` | P0 |
    | `make vuln` | P0 |
    | `make check-generated` | P1 (the first phase with a spec) |
    | `make test-db` (Docker is available on GitHub runners) | P1 |
    | `make cover` | P2 (the first gated package) |
    | `make test-deploy` (after `make test-db`) | P7 |
    | `make build` | P0 (runs after every check above, so they run with only the stub; see "Clean checkout" in the Verification Plan) |
    | `pnpm exec playwright install --with-deps webkit`, then `make e2e` against the built binary + a `postgres:18.<minor>-alpine` service container (the pinned tag, user, password and database `open_customer`/`e2e`/`open_customer`, `pg_isready` health check), with `E2E_DATABASE_URL=postgres://open_customer:e2e@localhost:5432/open_customer_e2e`. `make e2e` uses the runner's preinstalled `psql` client (seen in run 42097441087). On failure `web/test-results/` is uploaded as an artifact | P6 |

##### 9. `.github/workflows/release.yml` (Create)

*   **Type / Scope**: Release.
*   **Proposed Changes**: On tag `v*`, build and push the `app` image (root `Dockerfile`) and the `backup` image (`deploy/backup/Dockerfile`, a second `docker/build-push-action` step with `context: deploy/backup`, pushed as `ghcr.io/jnaranj09/open-customer-backup:<version>`) to GHCR, both tagged with the version. `amd64` only (multi-arch is a non-goal).
    *   **Version:** the tag without its `v` (`v1.2.3` → `1.2.3`, from `${GITHUB_REF_NAME#v}`). That value is the image tag, the `APP_VERSION` in `.env`, and the `--build-arg VERSION` of the app image.
    *   **Login:** `docker/login-action` to `ghcr.io` with `GITHUB_TOKEN`. The workflow sets `permissions: {contents: read, packages: write}`.

##### 10. `.github/dependabot.yml` (Create)

*   **Type / Scope**: Dependency updates.
*   **Proposed Changes**: Weekly updates for `gomod`, `npm` (in `web/`), `docker` (directory `/` from P0; `/deploy/backup` added in P7, when that Dockerfile exists), `github-actions`, and, from P7, the image tags in `deploy/compose.yaml` (the `docker-compose` ecosystem, if Dependabot supports it; otherwise cloudflared is bumped by hand). The `docker` and `docker-compose` entries `ignore` the `postgres`, `node` and `golang` images, which are bumped by hand (see "Pins"). Checked at P0: Dependabot supports the `docker-compose` ecosystem (GitHub's "Dependabot options reference" lists Docker Compose v2 and v3 as `docker-compose`), so P7 adds a `docker-compose` entry for `/deploy` and cloudflared is not bumped by hand.

#### Documentation

##### 11. `docs/designs/release-1.md` (Modify)

*   **Type / Scope**: Design doc.
*   **Proposed Changes**: This document. It stays the only architecture source. `docs/sad.md` (item 13) is an index of ADRs that links here, and it changes only when an ADR is added or changes status. Its links to the repo root are rewritten in P7 by item 14, in the same commit that moves the files (item 87).

##### 12. `docs/prd.md` (Create, moved from `PRD — Sistema de citas para emprendimientos.md`; the old file is deleted)

*   **Type / Scope**: Product requirements (Spanish).
*   **Proposed Changes**:
    *   **Priorities:** replace the "Performance y self-hosting mandan" line with the five drivers in order.
    *   **Problem text:** fix the claim that no free self-hosted option exists. Name Easy!Appointments, Cal.com and Odoo, and state the real gap.
    *   **Goals:** add Goal "she configures the empty app herself" with its check. Replace the reminder metric with this design's Goal 3 checks (the due-list test cases and the 4-week "no missed reminder" log). Replace the p95 metric with the iPhone "Hoy < 1 s" check.
    *   **FR changes:**
        *   FR-A2 (attribute types; was 7 including payment method and image): five types.
        *   FR-A3 (sub-options, e.g. Forma → Almendra → Corta): option trees of any depth; a value stores only the picked option ID.
        *   FR-A6 (attributes limited to some services): deferred.
        *   FR-R1 (rules per service, before or after an event): two rule kinds, a business-wide `before_appointment` and a per-service `come_back`.
        *   FR-R2 (reminders addressed to the owner, e.g. "call Ana"): deferred.
        *   FR-R7 (skip the come-back if she already has a future appointment of that service): skip only on a later active appointment that includes the rule's return service (empty = the rule's own service); a future booking without it gives her a booking prompt (Alternative E5).
        *   FR-R8 (Web Push when reminders are waiting): one daily summary push.
        *   FR-C4 (archive instead of delete; hard delete only on request): hard delete deferred.
        *   FR-P3 (statuses include a stored `completed`): `scheduled`, `confirmed`, `cancelled`, `no_show`; "done" is computed, never stored.
        *   FR-P5 (rescheduling and cancelling recalculate pending reminders): nothing is stored to recalculate; the due list is computed on every read.
        *   FR-I1 to FR-I5 (on-device AI: notebook OCR, SINPE receipt reading, dictation, photo background removal, photo tags): deferred.
    *   **New FR-O group** (onboarding and settings): setup link, wizard, branding, CLI reset, empty states that lead her to the first service, template and rule (no starter content).
    *   **New release-1 table:** every FR and NFR ID marked In or Out.
    *   **NFRs:**
        *   NFR-1 (API read p95 < 50 ms): replaced by the iPhone "Hoy < 1 s" check.
        *   NFR-4 (install from zero in < 10 min), NFR-5 (1 vCPU, 1 GB RAM) and NFR-9 (repo ready for contributors): moved to the product Phase 3 gate (not build phase P3).
        *   NFR-6 (daily automatic backups): daily backup + copy to a folder chosen at install (`BACKUP_TARGET`; a second disk is optional).
        *   NFR-7 (full export as JSON + CSV): CSV + JSON without photos.
        *   NFR-8 (mandatory HTTPS with automatic TLS): "HTTPS through the chosen ingress".
        *   NFR-3 (client search < 100 ms per keystroke): kept; met by the phone-only search over the cache.
        *   NFR-11 (default time zone `America/Costa_Rica`): no default; the wizard sets the time zone, starting from the browser's zone (Goal 7).
    *   **Open questions:** close the answered ones (license stays open). Add the iPhone test. Record the owner decision that replaced the interview: the app ships empty.
    *   **Glossary:** relleno / come-back, SINPE Móvil, bandeja / "Para enviar hoy", manual asistido, owner, template (message) vs template (Meta).

##### 13. `docs/sad.md` (Create, a short index) and `docs/designs/history/sad-original.md` (Create, moved from `SAD — Sistema de citas para emprendimientos.md`; the root file is deleted)

*   **Type / Scope**: Architecture index (Spanish).
*   **Proposed Changes**: The old SAD is moved unchanged to `docs/designs/history/sad-original.md`, so its full ADR text stays in the repo next to the index. Its body is not copied into `docs/sad.md`: `docs/designs/release-1.md` is the only source of architecture detail, so there is no second copy to keep in sync. `docs/sad.md` keeps:
    *   **Intro:** three or four Spanish sentences: what the system is, where it runs (home PC + Cloudflare Tunnel, four containers), and a link to `docs/designs/release-1.md`.
    *   **ADR index:** the table below, plus a fourth column in the written file that links each ADR that is not superseded (including the new 018–025, which have no separate file) to the section of `docs/designs/release-1.md` that holds its detail.

        | ADR | Title | Status in release 1 |
        | --- | --- | --- |
        | 001 | Go backend | Kept |
        | 002 | PostgreSQL | Kept; version set by 025 |
        | 003 | `pgx` v5 + `sqlc` | Kept |
        | 004 | Spec-first OpenAPI with `oapi-codegen` | Changed: OpenAPI 3.0.3 instead of 3.1 |
        | 005 | Modular monolith | Changed: one `serve` process, no separate `worker` mode |
        | 006 | Postgres job queue with `SKIP LOCKED` | Superseded by 019 |
        | 007 | Embedded `goose` migrations at start | Kept; with the backup guard from 022 |
        | 008 | SvelteKit `adapter-static` embedded with `go:embed`, as a PWA | Kept; styling by 023 |
        | 009 / 015 | Installed PWA instead of a native iOS app | Kept |
        | 010 | Image storage: local volume, optional S3 | Changed: local only, S3 deferred |
        | 011 | Caddy reverse proxy with Let's Encrypt | Superseded by 018 |
        | 012 | Opaque sessions in Postgres + Argon2id, in a cookie | Kept |
        | 013 | Own Mustache-like template renderer | Kept; template storage by 020 |
        | 014 | License (MIT or AGPL-3.0) | Pending |
        | 016 | Web Push with VAPID | Changed: one daily summary push |
        | 017 | Optional on-device AI | Deferred |
        | 018 | Cloudflare Tunnel ingress | New |
        | 019 | Reminder due list computed at read time + reminder log | New |
        | 020 | Message template library | New |
        | 021 | Read-only offline + date-window refresh | New |
        | 022 | Backups: one container, restic, USB target, migration guard | New |
        | 023 | Tailwind CSS v4 + own components | New |
        | 024 | Password login in the installed PWA | New |
        | 025 | PostgreSQL 18 | New |

    *   **Rule:** a design change edits `docs/sad.md` only when it adds an ADR or changes an ADR's status.

##### 14. `docs/designs/history/concepts_brief.md`, `docs/designs/history/goldfish_brainstorm_findings.md`, `docs/designs/history/review-release-1/round-<N>/` (Create where missing; the brief and findings are moved from the repo root, and the root copies are deleted)

*   **Type / Scope**: Design history.
*   **Proposed Changes**: No content change. Kept as the record behind the decisions.
    *   `review-release-1/round-<N>/` holds one folder per review round, each with the six reports (`zoom_out_report.md`, `comprehension_report.md`, `critic_report.md`, `intermediate_zoom_out_report.md`, `implementation_readiness_report.md`, `consolidated_review_report.md`). Round folders that already exist stay as they are. Any review reports still in the repo root at commit time go to the next free round number.
    *   In the same commit, every link in `docs/designs/release-1.md` that points at the root is rewritten:

        | Where | Link | New target |
        | --- | --- | --- |
        | Predecessors | brief | `history/concepts_brief.md` |
        | Predecessors | PRD | `../prd.md` |
        | Predecessors | SAD | `history/sad-original.md` |
        | Terms ("The review") | findings | `history/goldfish_brainstorm_findings.md` |
        | Alternatives intro | brief, SAD ADRs | `history/concepts_brief.md`, `history/sad-original.md` |
        | Additional Resources | brief, findings | `history/concepts_brief.md`, `history/goldfish_brainstorm_findings.md` |
        | Additional Resources | PRD, "original" SAD | `../prd.md`, `history/sad-original.md` |

        A check before the commit: `grep -n '\.\./\.\./' docs/designs/release-1.md` returns nothing.

#### API contract

##### 15. `api/openapi.yaml` (Create)

*   **Type / Scope**: OpenAPI 3.0.3 contract.
*   **Proposed Changes**:
    *   Every endpoint in the "API" table of the Technical Plan, with the request and response schemas below.
    *   A shared `Problem` schema (RFC 9457 with `errors[].field`).
    *   Cookie security scheme `oc_session`.
    *   **Response headers are set by the code that has the `ResponseWriter`**, not generated: handlers set `Set-Cookie` (setup, login, logout), `Content-Disposition` (export) and `Cache-Control` (media); the middleware sets `X-App-Version` (item 34); `problem.Write` sets `Retry-After` (item 24). The spec may document them, but generates no code for them.
    *   **`datos.json` keys.** The export's JSON types (`internal/exporter/json.go`) copy the keys of the `Settings`, `Attribute`, `Option`, `Service`, `Template`, `Rule`, `Client`, `Appointment`, `AppointmentService` and `LogItem` schemas; `TestExportJSONMatchesSchemas` keeps them equal.
    *   **`attrs`** is `type: object, additionalProperties: true` with `x-go-type: json.RawMessage` and `x-go-type-import: {path: encoding/json}`. The handlers pass it on unchanged, and `attributes/validate.go` decodes each value against its attribute type.
    *   **Absent vs `null` in `PATCH`:** every `PATCH` body field is optional. A field that may be cleared is also `nullable: true`. With `output-options.nullable-type: true` (item 16), oapi-codegen generates `nullable.Nullable[T]` for those fields, which tells "absent", "null" and "a value" apart. Optional fields that cannot be cleared become pointers (absent = `nil`). The option name is checked against the pinned oapi-codegen version at P1.
    *   **Multipart and streaming.** `POST /media` and `POST /settings/icon` are `multipart/form-data`; the handler reads them with `r.MultipartReader()`. `GET /export` and `GET /media/{id}` are `format: binary`; the handler writes to `w` (the export straight from `archive/zip`). Every `/api/v1` route is generated, so no hand-registered route can clash with one.

    **Conventions**

    | Topic | Rule |
    | --- | --- |
    | IDs | UUID strings, generated by the server. |
    | Times | `starts_at`, `created_at` and similar: RFC 3339 in UTC. Local days (`due_date`, `from`, `to`): `YYYY-MM-DD` in the business time zone. `from` and `to` are both inclusive and match the local day of `starts_at`. |
    | `position` | On create, the server puts the row last (the current maximum + 1). To reorder, the phone sends a `PATCH` with the new `position` of each row that moved. |
    | Money | Integer minor units (`*_cents`), plus `currency` from settings. The number of decimals comes from the currency's ISO 4217 exponent, read from `golang.org/x/text/currency` (`currency.Standard.Rounding(unit)` gives the scale; for example 2 for most currencies, 0 for JPY); inputs and display use it, so for a 0-decimal currency `*_cents` holds whole units. |
    | `PATCH` | `application/json`. An absent field is unchanged; `null` clears a nullable field. `attrs` is merged by key, and a key set to `null` is removed. `archived: true/false` archives or restores; archiving twice keeps the first time. An unknown id is 404 before any field is validated. |
    | Lists | `{items: [...]}`, with every matching row. No paging (see "API"). |
    | Errors | `type` is `urn:open-customer:problem:<name>`. Every name, status, Spanish `title` and extension field is in the error table below. |
    | Error responses | Each operation lists the problem statuses its own handler answers, plus a `default` response (`$ref: "#/components/responses/Problem"`) for the ones the middleware (item 34) can answer on any route: 403 `forbidden-origin` (every method but GET and HEAD), 403 `must-change-password` (step 8, public routes included) and 500 `internal`. One `default` per operation, not a 403 and a 500 entry per operation, because those answers do not depend on the route, and the web `schema.d.ts` then types every non-2xx answer as a `Problem`. `TestEveryOperationHasADefaultProblemResponse` fails an operation without it. |

    **Errors** (`application/problem+json`; `title` is shown to her as is)

    | Name | Status | `title` | Extension fields |
    | --- | --- | --- | --- |
    | `validation` | 422 | "Revisa los datos" | `errors[]` (`field`, `message` in Spanish) |
    | `not-found` | 404 | "No encontrado" | — |
    | `unauthorized` | 401 | "Inicia sesión de nuevo" | — |
    | `invalid-credentials` | 401 | "Usuario o contraseña incorrectos" | — |
    | `must-change-password` | 403 | "Debes cambiar tu contraseña" | — |
    | `forbidden-origin` | 403 | "Solicitud no permitida" | — |
    | `setup-closed` | 409 | "La configuración ya se completó" | — |
    | `setup-token-invalid` | 410 | "El enlace de configuración ya no sirve" | — |
    | `overlap` | 409 | "Ya hay una cita a esa hora" | `appointments[]` (`Appointment`) |
    | `duplicate-phone` | 409 | "Otra clienta tiene ese teléfono" | `clients[]` (`Client`) |
    | `invalid-transition` | 409 | "Ese cambio de estado no es posible" | — |
    | `payload-too-large` | 413 | "El archivo es muy grande" | — |
    | `unsupported-media-type` | 415 | "Tipo de archivo no permitido" | — |
    | `rate-limited` | 429 | "Demasiados intentos. Espera un momento" | `retry_after_s`, plus the `Retry-After` header |
    | `internal` | 500 | "Algo salió mal" | — (details only in the log, by request ID) |

    **Main schemas** (`?` = nullable)

    | Schema | Fields |
    | --- | --- |
    | `Me` | `id`, `username`, `must_change_password` |
    | `Settings` | `name`, `timezone`, `locale`, `currency`, `country_code`, `accent_color`, `push_hour`, `has_custom_icon` |
    | `Attribute` | `id`, `key`, `label`, `scope`, `type`, `required`, `position`, `archived`, `options[]` (`Option`) |
    | `Client` | `id`, `name`, `phone_e164?`, `phone_digits?`, `notes?`, `attrs`, `archived`, `created_at`, `updated_at` |
    | `Service` | `id`, `name`, `duration_min`, `price_cents?`, `color`, `position`, `archived` |
    | `Appointment` | `id`, `client_id`, `client_name` (joined from `clients`, so an archived client's name still shows), `starts_at`, `ends_at`, `status`, `done` (from `IsDone`), `total_cents`, `paid_cents?`, `attrs`, `notes?`, `services[]` (`service_id`, `name`, `price_cents?`, `duration_min`), `created_at`, `updated_at` |
    | `Template` | `id`, `name`, `body`, `archived` |
    | `Media` | `id`, `client_id`, `appointment_id?`, `width`, `height`, `bytes`, `created_at`, `archived` |
    | `Option` | `id`, `attribute_id`, `parent_id?`, `label`, `position`, `archived` |
    | `Rule` | `id`, `kind`, `service_id?`, `return_service_id?` (`come_back` only; null = the rule's own service), `offset_days`, `template_id`, `applies_from?`, `archived` |
    | `DueItem` | `id` (stable: `<rule_id>:<appointment_id>:<anchor_at as Unix seconds>` for "before", `<rule_id>:<appointment_id>` for `come_back`), `rule_id`, `appointment_id`, `anchor_at` (nullable; null for `come_back`), `kind`, `client_id`, `client_name`, `due_date`, `late`, `text`, `phone_digits?`, `wa_link?` (null when the client has no phone), `mode` (`message` or `booking_prompt`; always `message` for "before"), `prompt?` (set only for `booking_prompt`: `appointment_id`, `starts_at` and `services[]` names of the future appointment, `return_service_id`, `return_service_name`) |
    | `LogEntry` | `rule_id`, `appointment_id`, `anchor_at` (nullable; null for `come_back`), `action`, `body?`, `phone_digits?`, `acted_at` |
    | `LogItem` | `LogEntry` plus `id`, `kind`, `client_id`, `client_name`. Returned by `GET /reminders/log`, so "Enviados" and the client history show the name even when the appointment is outside the cached window. |
    | `Bootstrap` | `server_time`, `settings`, `attributes[]` (with options), `services[]`, `templates[]`, `rules[]` (these four include archived rows, with `archived: true`, because the phone renders old values by their last label and warns about archived templates and services), `clients[]` (active only), `appointments[]` (in the window), `due[]`, `template_vars[]` (`ns`, `key`, `label`, for the chip row), `backup_red` (the same value as `Status.backup.red`, for the line on "Hoy") |
    | `Status` | `version`, `migration_version`, `heartbeat_age_s?` (null until the timer's first tick), `backup` (`checked_at?`, `last_dump_at?`, `last_copy_at?`, `last_error?`, `red`; a missing or unreadable file gives all times null and `red: true`, which the UI shows as "desconocido"), `media_free_bytes?` (null when the volume cannot be read), `push` (`subscriptions`, `last_push?` (`date`, `attempted`, `delivered`), `red`) |

    **Request bodies**

    | Endpoint | Body | Success |
    | --- | --- | --- |
    | `GET /setup/status` | — | 200 `{needs_setup, accent_color}` (`accent_color` once setup is done: public like the manifest's `theme_color`, so the login screen on a new device shows her color) |
    | `POST /setup` | `token`, `username`, `password`, `settings` (`Settings` without `has_custom_icon`). The username rules are in "Session and request rules". | 201 `Me` + `Set-Cookie` |
    | `POST /auth/login` | `username`, `password` | 200 `Me` + `Set-Cookie` |
    | `POST /auth/logout` | — | 204 + expired `Set-Cookie` |
    | `POST /auth/password` | `current_password`, `new_password`. A wrong `current_password` is 422 `validation` on that field, not 401, which would log her out. | 204 |
    | `POST /attributes` | `label`, `scope`, `type`, `required` | 201 `Attribute` |
    | `POST /attributes/{id}/options` | `label`, `parent_id?` | 201 `Option` |
    | `POST /clients` | `name`, `phone?`, `notes?`, `attrs`, `allow_duplicate_phone` | 201 `Client` |
    | `POST /services` | `name`, `duration_min`, `price_cents?`, `color` | 201 `Service` |
    | `POST /appointments` | `client_id`, `starts_at`, `ends_at?` (default: `starts_at` + the sum of the service durations), `service_ids[]` (order kept, repeats allowed), `attrs`, `notes?`, `allow_overlap`. At least one service, or an explicit `ends_at`, is required; an archived client or service is rejected with 422. | 201 `Appointment` |
    | `POST /appointments/{id}/status` | `status`, `allow_overlap` (default `false`; used when the change is to `scheduled`) | 200 `Appointment` |
    | `POST /templates` | `name`, `body` | 201 `Template` |
    | `POST /templates/preview` | `body`, `client_id?` (none: the sample client; set: that client and the client's latest appointment that is not cancelled or no-show, see "Message templates") | 200 `{text, unknown_vars[]}`; `unknown_vars[]` holds `ns.key` strings, each once, in body order; an unknown `client_id` is 422 |
    | `POST /reminder-rules` | `kind`, `service_id?`, `return_service_id?` (`come_back` only; absent or null = the rule's own service), `offset_days`, `template_id`, `include_overdue` (bool, optional, default `false` = "Solo de hoy en adelante"). An archived template, service or return service is 422. | 201 `Rule` |
    | `POST /reminders/log` | `LogEntry`. An unknown `rule_id` or `appointment_id` is 422, not a foreign-key 500. | 201 `LogItem`, or 200 `LogItem` (the stored row) when the row already existed |
    | `POST /push/subscriptions` | the browser's `PushSubscription` JSON (`endpoint`, `expirationTime?` (number or null; accepted and ignored: the browser always sends it and unknown fields are rejected), `keys.p256dh`, `keys.auth`) | 201 `{id}` for a new endpoint, 200 `{id}` for a known one (keys updated) |
    | `POST /media` | multipart: `full` (JPEG, ≤ 10 MB), `thumb` (JPEG, ≤ 1 MB), `client_id?`, `appointment_id?` (at least one; the request is ≤ 12 MB). The server always stores `client_id`: with `appointment_id`, it is filled from the appointment; a different one is 422 (see "Photos"). | 201 `Media` |
    | `POST /settings/icon` | multipart: `icon512`, `icon180` (PNG, each ≤ 1 MB; the request is ≤ 3 MB; the PNG signature is checked, otherwise 415) | 204 |
    | `POST /client-errors` | `message`, `stack?`, `app_version`, `route` (limits in "Observability": 422 on a field outside them; no transaction) | 204, or 429 past the session's token bucket (a burst of 10, then one every 6 seconds) |

    **`PATCH` bodies** (every field optional; `?` = may be `null` to clear)

    | Endpoint | Fields that may change | Success |
    | --- | --- | --- |
    | `PATCH /settings` | `name`, `timezone`, `accent_color`, `push_hour`. `currency`, `country_code` and `locale` are fixed at setup (422 if sent), because stored amounts use the currency's decimals. | 200 `Settings` |
    | `PATCH /attributes/{id}` | `label`, `required`, `position`, `archived` | 200 `Attribute` |
    | `PATCH /attributes/{id}/options/{optionId}` | `label`, `position`, `archived`. `parent_id` cannot change. | 200 `Option` |
    | `PATCH /clients/{id}` | `name`, `phone?`, `notes?`, `attrs` (merged by key), `archived`, `allow_duplicate_phone` | 200 `Client` |
    | `PATCH /services/{id}` | `name`, `duration_min`, `price_cents?`, `color`, `position`, `archived` | 200 `Service` |
    | `PATCH /appointments/{id}` | `starts_at`, `ends_at`, `service_ids[]` (replaces the list), `attrs` (merged by key), `notes?`, `paid_cents?`, `allow_overlap`. `client_id` cannot change; `status` changes only through `POST /appointments/{id}/status`. | 200 `Appointment` |
    | `PATCH /media/{id}` | `archived` | 200 `Media` |
    | `PATCH /templates/{id}` | `name`, `body`, `archived` | 200 `Template` |
    | `PATCH /reminder-rules/{id}` | `offset_days`, `template_id`, `return_service_id?` (`come_back` only; null = back to the rule's own service), `archived`, `include_overdue`. `kind` and `service_id` cannot change; she creates a new rule instead. `include_overdue` (default `false`) is used only on a change that asks (see "Reminder rules") and ignored otherwise. An archived `template_id` or `return_service_id` is 422. | 200 `Rule` |

    **`GET` responses and query parameters**

    | Endpoint | Query | Success |
    | --- | --- | --- |
    | `GET /setup/options` | — (no session) | 200 `{countries[]}` from `settings.Countries()`: the ISO codes from `phonenumbers.GetSupportedRegions()`, sorted. The phone shows them with `Intl.DisplayNames(['es'], {type: 'region'})`. |
    | `GET /me` | — | 200 `Me` |
    | `GET /bootstrap` | `from`, `to` (`YYYY-MM-DD`, both required; `to` is not before `from` and at most 200 days after it, else 422 on `to`: "Elige un rango válido de hasta 200 días"; the check runs before the transaction) | 200 `Bootstrap` |
    | `GET /clients` | `archived?` (default `false`) | 200 `{items: Client[]}`, by name. There is no `q`: search runs on the phone. |
    | `GET /clients/{id}` | — | 200 `Client` |
    | `GET /clients/{id}/appointments` | — | 200 `{items: Appointment[]}`, newest first |
    | `GET /clients/{id}/media` | — | 200 `{items: Media[]}`, newest first, archived ones left out |
    | `GET /appointments` | `from`, `to` (`YYYY-MM-DD` local days in the business time zone, both required, `to` not before `from` and at most 31 days after it, else 422 on `to`) | 200 `{items: Appointment[]}`, by `starts_at` |
    | `GET /appointments/{id}` | — | 200 `Appointment` |
    | `GET /media/{id}`, `GET /media/{id}/thumb` | — | 200 `image/jpeg` (binary) |
    | `GET /reminders/due` | — | 200 `{items: DueItem[]}` |
    | `GET /reminders/log` | exactly one of `client_id` or `date` (`YYYY-MM-DD`), or 422. `date` filters on `acted_at`, as a local day in the business time zone. | 200 `{items: LogItem[]}`, newest first by `acted_at`; an unknown `client_id` is 404 |
    | `GET /push/key` | — (no session) | 200 `{public_key}` (base64url) |
    | `GET /export` | — | 200 `application/zip` (binary), `Content-Disposition: attachment; filename="open-customer-<YYYY-MM-DD>.zip"` |
    | `GET /status` | — | 200 `Status` |

    **Push payload** (JSON, encrypted by Web Push): `{"title": <business name>, "body": "Tienes N mensajes para enviar hoy", "count": N, "url": "/enviar"}` (the body is singular for N = 1: "Tienes 1 mensaje para enviar hoy").

##### 16. `api/oapi-codegen.yaml` (Create)

*   **Type / Scope**: Generator config.
*   **Proposed Changes**: Package `gen`, output `internal/httpapi/gen/api.gen.go`, `generate: {std-http-server: true, models: true, embedded-spec: true}`, `output-options: {nullable-type: true}` (see item 15 for `x-go-type` and nullable fields). The one `compatibility` flag is `always-prefix-enum-values: true`, so enum constants carry their type name (`AttributeScopeClient`) and a later schema named like a bare value (`Appointment`) cannot clash with it. No other `compatibility` flags: middleware steps 7 and 8 read `security` and the `x-oc-` extension from the embedded spec (item 34), not from generated context values (the scopes-on-context flag is deprecated and fails open for a route it does not mark). No `strict-server`: the plain interface gives each handler `(w, r, params)`, so headers, multipart and streaming need no workaround. The cost is that Go does not check at compile time which status and body a handler writes; the handler tests (item 37) and e2e cover that.

#### Command

##### 17. `cmd/open-customer/main.go` (Create)

*   **Type / Scope**: Entry point.
*   **Proposed Changes**: Parses the subcommand (`serve`, `reset-password`, `setup-link`, `healthcheck`, `version`) and calls the matching function. It holds the `version` variable set by `-ldflags` (default `"dev"`). Each phase adds the dispatch for its own subcommands (see "Build order").

##### 18. `cmd/open-customer/serve.go` (Create)

*   **Type / Scope**: Server startup.
*   **Proposed Changes**: The steps, and the phase that adds each one:
    1.  Loads config and opens the pool. (P1)
    2.  Runs `db.Migrate` with the backup guard. (P1)
    3.  Ensures the VAPID keys exist, with `push.EnsureKeys` in its own `db.WithTx` (the two keys are two inserts, so a half pair never persists). (P4, built)
    4.  Prints the setup link if there are no users: `Setup: <PUBLIC_URL>/setup#<token>`, one plain stdout line, not slog (see "Auth, sessions and handover"). `NewSetupToken` runs in its own `db.WithTx`. (P2)
    5.  Builds the modules and `httpapi.Server`. (P1, with each phase adding its modules)
    6.  Starts `scheduler.Run` in a goroutine on a one-minute ticker; on shutdown it cancels it and waits for it to return. (P4, built)
    7.  Serves on `:8080`. (P1)
    8.  Shuts down gracefully on SIGTERM. (P1)

##### 19. `cmd/open-customer/admin.go` (Create)

*   **Type / Scope**: Admin subcommands.
*   **Proposed Changes**:
    *   `reset-password <username>`: prints a random 12-character temporary password from `crypto/rand` over `a–z`, `A–Z` and `2–9` without look-alike characters (no `0`, `O`, `1`, `l`, `I`), and sets `must_change_password`. The username is normalized like login; an unknown one exits 1 with "no such user".
    *   `setup-link`: only when no user exists (otherwise exit 1, "setup is already complete"); deletes old tokens and prints a new 24 h link, `<PUBLIC_URL>/setup#<token>`.
    *   `healthcheck`: GET `http://127.0.0.1:8080/healthz` with a 3 s timeout; exit 0 on 200, otherwise 1. It reads no config, so it runs in the distroless image with no environment.
    *   `reset-password` and `setup-link` do not migrate: only `serve` does, behind the backup guard. If migrations are pending they exit 1 with "migrations pending; start serve first". Their stdout carries only the password or the link; every error goes to stderr.
    *   A wrong argument count prints `usage: open-customer serve|reset-password <username>|setup-link|healthcheck|version` and exits 1.

#### Platform

##### 20. `internal/platform/config/config.go` (Create)

*   **Type / Scope**: Config.
*   **Proposed Changes**: Reads the `.env` variables listed in the Technical Plan into a struct. It reads only the variables marked "app" in the Configuration table. It validates them: `PUBLIC_URL` must be `https`, unless its host is `localhost` or `127.0.0.1`; and `PUBLIC_URL`, `DB_PASSWORD` and `VAPID_SUBJECT` must be present. There is no separate "insecure" flag, so nothing can turn off `https` on a real subdomain. It fails at start with one message that lists every missing variable. `DATABASE_URL` is built from `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` and `DB_SSLMODE`.

##### 21. `internal/platform/logging/logging.go` (Create)

*   **Type / Scope**: Logging.
*   **Proposed Changes**: A JSON `slog` handler, plus HTTP middleware that adds a request ID, `cf_ray`, route, status and duration to each request log.

##### 22. `internal/platform/clock/clock.go` (Create)

*   **Type / Scope**: Time source.
*   **Proposed Changes**: A `Clock` interface (`Now()`), a real implementation, and a fake one for tests. Every module that reads the time takes a `Clock`.

##### 23. `internal/platform/ids/ids.go` (Create)

*   **Type / Scope**: IDs.
*   **Proposed Changes**: `New()` returns a UUIDv7. `Parse()` validates IDs that come from the phone.

##### 24. `internal/platform/problem/problem.go` (Create)

*   **Type / Scope**: Errors.
*   **Proposed Changes**:
    *   One Go value per row of the error table in item 15: `Validation{Errors []FieldError}`, `NotFound`, `Unauthorized`, `InvalidCredentials`, `MustChangePassword`, `ForbiddenOrigin`, `SetupClosed`, `SetupTokenInvalid`, `Conflict{Name string, Extra map[string]any}` (for `overlap`, `duplicate-phone` and `invalid-transition`; `httpapi/conflicts.go` fills `Extra` with `appointments` or `clients` as `gen` types from the typed module errors `appointments.OverlapError` and `clients.DuplicatePhoneError`, which unwrap to a `Conflict` with no `Extra`, because modules cannot import `gen`), `PayloadTooLarge`, `UnsupportedMediaType`, `RateLimited{RetryAfter}`. Any other error is `internal`.
    *   `Write(w, r, err)`: picks the row, writes `application/problem+json` with the status, the Spanish `title` and the extension fields, and logs `internal` errors with the request ID.
    *   Handlers write these errors with `problem.Write`. A JSON body that cannot be decoded is `validation` (the shared `decodeJSON` helper, item 32). Path and query binding errors (a bad UUID in `{id}`, a bad `from` date) are raised by the generated wrapper, and `StdHTTPServerOptions.ErrorHandlerFunc` (item 32) maps them to `validation`. Middleware (item 34) calls `problem.Write` for 401, 403 and the panic 500. `auth.BeginLogin`, `auth.BeginPasswordChange` and `status.LogClientError` return `RateLimited`, and their handlers write the 429.
    *   For `RateLimited`, `problem.Write` also sets the `Retry-After` header.
    *   `messages.go` holds the Spanish `errors[].message` texts as named constants (OP14), for example `MsgUsernameInvalid` and `MsgPasswordTooShort`. Modules build `Validation` errors with these constants; no field message is written inline.

#### Database

##### 25. `internal/db/db.go` (Create)

*   **Type / Scope**: Connection.
*   **Proposed Changes**: A `pgxpool` setup, `type DBTX = sqlc.DBTX` (sqlc generates the interface in its own package; this alias is the `db.DBTX` every module uses), and `WithTx(ctx, pool, opts, fn func(pgx.Tx) error)`. `pool` is a `db.Beginner`: the `*pgxpool.Pool`, or in a test a fake whose `Commit` fails (item 37), or `dbtest.TxModeRecorder`, which records the isolation level and read-only flag Postgres gives each transaction. `WithTx` begins the transaction with `opts` (read-write for every non-GET handler that reads or writes the database; read-only `REPEATABLE READ` for bootstrap, `GET /reminders/due`, the timer and export, which `TestSnapshotReadersUseReadOnlyRepeatableRead` (httpapi) and `TestTickCountsInASnapshot` (scheduler) check through that recorder; see "Code layout and module rules"), commits on nil and rolls back on error. Module functions build `sqlc.New(q)` from the `db.DBTX` they get, so the same code runs on the pool or inside a transaction. `DATABASE_URL` comes from `config` (item 20).

##### 26. `internal/db/migrate.go` (Create)

*   **Type / Scope**: Migrations.
*   **Proposed Changes**: Embeds `migrations/*.sql` with `//go:embed migrations/*.sql` and passes `fs.Sub(embedFS, "migrations")` to goose, because the Provider looks for files at the root of the FS it gets. Opens a `*sql.DB` from the pool with `pgx/v5/stdlib` (`stdlib.OpenDBFromPool`), then runs these steps:
    1.  Build a goose v3 `Provider` (`goose.NewProvider(goose.DialectPostgres, db, fs)`). The pinned goose version must include `HasPending` (check it when pinning).
    2.  Unless `ALLOW_MIGRATE_WITHOUT_BACKUP=true`, return an error when all three conditions from "Upgrades and the migration guard" hold: `HasPending` is true, AND the goose version (the Provider's `GetDBVersion`) is above 0, AND `last_copy_at` in `BACKUP_STATUS_FILE` (default `/backup-status/status.json`, read with `backupstatus.Read`) is missing or more than 60 minutes old.
    3.  Call `Up`.

    It also exports `Version(ctx, pool)` (the goose version) and `HasPending(ctx, pool)`, for `status` and `/readyz`.

##### 27. `internal/db/migrate_test.go` (Create)

*   **Type / Scope**: DB test.
*   **Proposed Changes**: The guard blocks with an old or missing `last_copy_at`, allows with one from the last 60 minutes, allows on a fresh database (goose version 0), and allows when no migration is pending. A `last_copy_at` in the `YYYY-MM-DDTHH:MM:SSZ` form that `backup.sh` writes parses correctly. A test also checks that the embedded FS yields at least one migration, so a wrong `fs.Sub` path fails in CI.

##### 28. `internal/db/migrations/00001_init.sql` (Create)

*   **Type / Scope**: Schema.
*   **Proposed Changes**:
    *   Starts with `-- +goose Up`. There is no `-- +goose Down` section, because migrations only move forward (see "Upgrades and the migration guard"). Functions with `$$` bodies are wrapped in `-- +goose StatementBegin` / `-- +goose StatementEnd`. Every later migration follows the same rules.
    *   Every table, CHECK constraint and index in "Data model".
    *   **Shorthand rules for "Data model":**
        *   `id` is `uuid PRIMARY KEY` (no default; the server sets UUIDv7).
        *   A `*_id` column is `uuid NOT NULL REFERENCES <table>(id)` unless it is marked `NULL`. Targets that are not obvious: `template_id` → `message_templates`, `rule_id` → `reminder_rules`, `attribute_id` → `attribute_definitions`.
        *   `created_at` and `updated_at` are `timestamptz NOT NULL DEFAULT now()`. `archived_at`, `used_at` and `last_seen_at` are `timestamptz NULL`. `expires_at` is `timestamptz NOT NULL`.
        *   `name`, `label`, `storage_key`, `thumb_key`, `content_type`, `color`, `endpoint`, `p256dh` and `auth` are `text NOT NULL`. `notes` and `body` are `text NULL` where "Data model" gives no type (`message_templates.body` is `NOT NULL`). `server_keys.value` and `scheduler_state.value` are `text NOT NULL`. `position` is `int NOT NULL`, except `appointment_services.position`, which is `smallint`. `starts_at`, `ends_at` and `acted_at` are `timestamptz NOT NULL`; `appointment_services.duration_min` is `int NOT NULL`; `appointment_services.service_id` is `uuid NOT NULL REFERENCES services(id)`; `media.bytes`, `media.width` and `media.height` are `int NOT NULL`.
    *   **Foreign keys:** every reference uses the default `NO ACTION`, because release 1 never hard-deletes, with three exceptions that use `ON DELETE CASCADE`: `appointment_services.appointment_id`, `sessions.user_id` and `push_subscriptions.user_id`. `attribute_options.parent_id` references `attribute_options(id)`.
    *   **Extra CHECKs:** `ends_at > starts_at`; `duration_min > 0`; `price_cents`, `total_cents` and `paid_cents` `>= 0`; `offset_days >= 0`; `kind <> 'come_back' OR offset_days > 0`.
    *   **Extra indexes** the queries need: `appointments(ends_at) WHERE status NOT IN ('cancelled','no_show')`, `reminder_log(appointment_id)`, `media(client_id)`, `media(appointment_id)`, `sessions(user_id)`, `attribute_options(attribute_id)`.
    *   No regional defaults in the DDL.
    *   **`00002_reminder_return_service.sql` (P4):** `00001` is already applied, and migrations only move forward, so the return service is a second migration: `ALTER TABLE reminder_rules ADD COLUMN return_service_id uuid NULL REFERENCES services(id)` plus `CHECK (kind = 'come_back' OR return_service_id IS NULL)`. Existing rows keep NULL (= the rule's own service).

##### 29. `internal/db/queries/` — `settings.sql`, `auth.sql`, `attributes.sql`, `clients.sql`, `services.sql`, `appointments.sql`, `media.sql`, `templates.sql`, `reminders.sql`, `push.sql`, `scheduler.sql`, `export.sql` (Create)

*   **Type / Scope**: sqlc queries.
*   **Proposed Changes**: One file per module, holding only the queries that module runs:
    *   `appointments.sql`: the overlap check and the window load.
    *   `reminders.sql`: the candidate loads for `DueList`, the log insert with `ON CONFLICT DO NOTHING`, the log reads (`GetLogItem`, `ListLogForClient`, `ListLogInRange`) and the existence checks `Record` needs.
    *   `push.sql`: the `server_keys` read and insert-if-missing, the subscription upsert (`ON CONFLICT (endpoint)`, returning whether the row is new), list, delete and count.
    *   `scheduler.sql`: `UpsertState` (name, value, `updated_at` from the clock) and `ListState`, on `scheduler_state`.
    *   `export.sql`: one keyset-paged query per table (`WHERE id > @after ORDER BY id LIMIT 500`), as in "Export": `ExportClients`, `ExportAppointments` (with the client name), `ExportServices`, `ExportTemplates`, `ExportRules` and `ExportLog` (with the rule kind, client id and client name), plus `ExportAppointmentLines` (`appointment_id = ANY(@ids)`, the lines of one page of appointments). sqlc `:many` loads all rows of a query into a slice, so paging is what keeps memory bounded.

##### 30. `internal/db/sqlc/` (Create, generated)

*   **Type / Scope**: Generated code.
*   **Proposed Changes**: Output of `sqlc generate`. Committed, and checked for drift in CI.

#### HTTP layer

##### 31. `internal/httpapi/gen/api.gen.go` (Create, generated)

*   **Type / Scope**: Generated code.
*   **Proposed Changes**: Output of oapi-codegen. Committed.

##### 32. `internal/httpapi/server.go` (Create)

*   **Type / Scope**: Server implementation.
*   **Proposed Changes**: The `Server` struct holds the pool and the module values (concrete types), and implements `gen.ServerInterface`.
    *   Mounting: `gen.HandlerWithOptions(server, gen.StdHTTPServerOptions{BaseURL: "/api/v1", BaseRouter: mux, ErrorHandlerFunc: <writes a validation problem>})`.
    *   `New` refuses an `Options` without any of these 13 values: the twelve module values (`Auth`, `Settings`, `Attributes`, `Services`, `Clients`, `Appointments`, `Media`, `Templates`, `Reminders`, `Push`, `Status`, `Exporter`) and the `Clock`, so a dependency `serve` forgot to pass fails at startup, not on the first request that reaches it. The server keeps the clock for the handlers that need "now" (`GET /reminders/due`), and `Server.snapshot` is the read-only `REPEATABLE READ` `db.WithTx` the snapshot readers share, and `Server.businessZone(ctx, q)` reads the settings and loads the business time zone for the handlers that need it (`GET /appointments`, `GET /reminders/log`, `GET /export`; `GET /bootstrap` reads the time zone and the locale from the `settings.Get` result it already has).
    *   A catch-all `/api/` handler answers every unknown path and every wrong method under `/api/` with 404 `not-found` problem JSON. Without it, the `/` fallback in item 35 would answer them with `index.html` and a 200.
    *   Handler shape: generated handlers take `(w, r, <path params>, params)`, for example `(w, r, id, params)`.
    *   Shared helpers: `decodeJSON(w, r, &v)` (decodes into the generated request type with `DisallowUnknownFields`, because the web client is ours and an unknown field is a bug; there is no `Validate()` on generated types: the module that owns a field validates it and answers the field errors; a body over the limit, `*http.MaxBytesError`, gives `payload-too-large`; any other decode failure gives `validation`; the multipart readers check `MaxBytesError` the same way) ; `decodePatch(w, r, &v, nullable...)` for every `PATCH` body: `decodeJSON`, plus a top-level field sent as `null` is `validation` on that field unless the endpoint lists it as nullable (the `?` fields of the `PATCH` table; the generated patch fields are pointers, so a `null` would otherwise read as "not sent"), and a bare `null` body is `validation`; `readParts(r, limits)` (in `multipart.go`, with `bodyProblem`, `uploadRejectCode` and `withoutFileText`; the one multipart reader of `POST /media` and `POST /settings/icon`: form-data only, each part at most once, only the named parts, each read up to its limit; the handler checks which parts are required); and `writeJSON(w, status, v)`. Response values are the generated `gen` types, so field names come from the spec.
    *   Body limits are applied by the code that reads the body, not by path in `server.go`: `decodeJSON` wraps `r.Body` in `http.MaxBytesReader` at 1 MB and also rejects trailing data after the value; each multipart handler wraps it with its own limit, `POST /media` (12 MB) and `POST /settings/icon` (3 MB). A body over the limit answers `payload-too-large`.
    *   The SPA handler (item 35) is mounted on `/` for GET and HEAD only; any other method on a path outside `/api/` gets 405 with `Allow: GET, HEAD`. (`GET /` cannot be registered next to the method-less `/api/` catch-all: the standard mux rejects the pair as conflicting.)

    **Module methods.** Every method also takes `ctx` and `q db.DBTX` first; they are left out below. Types are the module's own structs, named after the API schemas (item 15). `scheduler` is not called by `httpapi`.

    | Module | Methods |
    | --- | --- |
    | `auth` | `SetupNeeded() bool`; `NewSetupToken() string`; `CompleteSetup(SetupInput) (Me, token)` (the handler opens the transaction); `BeginLogin(LoginInput, ip) LoginAttempt` (no `ctx` or `q`: the rate limits of item 41, in memory, before the handler opens the transaction); `Login(LoginAttempt, password) (Me, token)` (only an attempt `BeginLogin` returned; the attempt holds no password, so logging it cannot leak one; an unknown username is still verified, against a fixed dummy hash, so the response time does not reveal which usernames exist; OP17); `Logout(session)` (the session middleware step 6 loaded, so the cookie is parsed only there); `Authenticate(token) (Me, Session, refreshed bool)` (`refreshed` tells the middleware to re-send the cookie); `BeginPasswordChange(me) PasswordChangeAttempt` (no `ctx` or `q`: the per-username delay, before the transaction); `ChangePassword(session, PasswordChangeAttempt, current, new)` (keeps that session, revokes the others, clears `must_change_password`; a wrong `current` counts toward the per-username delay, item 41); `ResetPassword(username) tempPassword` |
    | `settings` | `Get() Settings`; `Create(SettingsInput) Settings` (called only through `auth` setup; `cmd` wraps it in a closure that drops the returned `Settings`, to match the creator type in item 40); `Update(SettingsPatch) Settings`; `SetIcons(icon512, icon180 []byte)`; `Manifest() []byte`; `Icon(size) ([]byte, updatedAt)`. `Countries() []string` is a package-level function with no `q`, not a method. |
    | `attributes` | `List() []Attribute`; `Create(AttributeInput) Attribute`; `Update(id, AttributePatch) Attribute`; `AddOption(attrID, OptionInput) Option`; `UpdateOption(attrID, optionID, OptionPatch) Option`; `Validate(scope, stored, request, isCreate) (attrs, error)` |
    | `clients` | `List(archived bool) []Client` (`List(false)` is the active list); `Get(id) Client`; `Create(ClientInput, region) Client`; `Update(id, ClientPatch, region) Client` (`region` is `settings.country_code`, read by the handler) |
    | `services` | `List() []Service`; `GetMany(ids) map[id]Service`; `Create(ServiceInput) Service`; `Update(id, ServicePatch) Service` |
    | `appointments` | `ListRange(from, to, loc *time.Location) []Appointment` (the handler passes the business time zone; every status is listed; `to` before `from` is a 422 on `to`, and there is no length limit here: `GET /appointments` checks its 31-day limit and bootstrap reads a longer window); `ListForClient(clientID) []Appointment`; `Get(id) Appointment`; `Create(AppointmentInput) Appointment`; `Update(id, AppointmentPatch) Appointment`; `SetStatus(id, status, allowOverlap) Appointment` (returns `overlap` 409 when the change is to `scheduled` and an overlap exists) |
    | `media` | `Save(MediaInput) Media`; `Open(id, thumb bool) (io.ReadSeekCloser, Media)`; `Discard(ctx, Media)`; `ListForClient(clientID) []Media`; `SetArchived(id, archived *bool) Media` (an absent `archived` leaves the photo as it is; archiving twice keeps the first time) |
    | `templates` | `List() []Template`; `Vars(loc) []TemplateVar` (for `template_vars[]`); `Create(TemplateInput) Template`; `Update(id, TemplatePatch) Template`. `Render(tmpl, data)`, `Sample(loc, attrs, now, tz)` and `UnknownVars(parsed, attrs)` are plain functions with no `q`. |
    | `reminders` | `Preview(body, clientID?) Preview{Text, UnknownVars}` (builds the data map with the same loader as `DueList`, so preview and the due list share one code path; with a client, it picks the client's latest appointment that is not cancelled or no-show); `ListRules() []Rule`; `CreateRule(RuleInput) Rule`; `UpdateRule(id, RulePatch) Rule`; `DueList(now) []DueItem` (messages and booking prompts, `DueItem.mode`); `Record(LogEntry) (LogItem, created bool)`; `LogForClient(clientID) []LogItem`; `LogForDate(day, loc) []LogItem` |
    | `push` | `PublicKey() string`; `Subscribe(userID, Subscription) (id, created bool)`; `CountSubscriptions() int`; `EnsureKeys()`; `SendAll(Payload) SendResult` (`attempted`, `delivered`) |
    | `exporter` | `Write(w io.Writer)` (the handler opens the read-only transaction). `FileName(day) string` (`open-customer-YYYY-MM-DD.zip`, from the business-local day) is a plain function with no `q`. |
    | `status` | `Health()`; `Ready() bool`; `Status(ctx, pool) (Report, error)`; `Backup() Backup` and `BackupRed() bool` (read `BACKUP_STATUS_FILE` with `backupstatus.Read`, no `q`); `LogClientError(ctx, sessionKey, ClientError) error` (`sessionKey` is the hex of the session's token hash) |

    Every method returns `error` last. Errors are `problem` types (item 24).

##### 33. `internal/httpapi/handlers_auth.go`, `handlers_settings.go`, `handlers_attributes.go`, `handlers_clients.go`, `handlers_services.go`, `handlers_appointments.go`, `handlers_media.go`, `handlers_templates.go`, `handlers_reminders.go`, `handlers_push.go`, `handlers_export.go`, `handlers_status.go`, `handlers_bootstrap.go` (Create)

*   **Type / Scope**: Handlers.
*   **Proposed Changes**: One file per area. Each handler decodes the request, calls one module function and encodes the result. No business logic lives here.
    *   `handlers_bootstrap.go` opens one read-only `REPEATABLE READ` transaction with `db.WithTx` and passes the `pgx.Tx` to each module's read function in turn (`settings.Get`, `attributes.List`, `services.List`, `templates.List`, `reminders.ListRules`, `clients.List(false)`, `appointments.ListRange`, `reminders.DueList`, `templates.Vars`), so all parts come from the same snapshot. The range check (`maxBootstrapRangeDays` = 200) runs before the transaction. `templates.Vars` gets the locale of the settings read. Outside the transaction it adds `server_time` from the `Clock` (the same `now` passed to `DueList`) and `backup_red` from `status.BackupRed()`. There is no `pgx.Batch`: the sqlc `DBTX` has no `SendBatch`, and one snapshot already makes the reads consistent. `GET /reminders/due` and the timer also call `DueList` inside a read-only `REPEATABLE READ` transaction.
    *   Multipart: `handlers_media.go` (`POST /media`) and `handlers_settings.go` (`POST /settings/icon`) read it with `r.MultipartReader()`. Binary bodies: `handlers_media.go` (`GET /media/{id}`, `/thumb`) and `handlers_export.go` write to `w`, with the headers from item 15.
    *   `handlers_auth.go` also serves `GET /setup/options` (no session).
    *   `handlers_settings.go` also holds the handlers, outside `/api/v1` and with no session, of `GET /manifest.webmanifest` (`settings.Manifest`), `GET /icons/512.png` and `GET /apple-touch-icon.png` (`settings.Icon`), with `Cache-Control: no-cache`; `handlers_status.go` holds those of `/healthz` and `/readyz`. `server.go` registers all five by literal path, before `/`. An icon file error's text can name a path, so the icon handlers answer it as `internal` with a fixed text and never log it.
    *   Every non-GET handler that reads or writes the database wraps its module call in `db.WithTx` ("Code layout and module rules").

##### 34. `internal/httpapi/middleware.go` (Create)

*   **Type / Scope**: Middleware.
*   **Proposed Changes**: Applied in this order. Steps 1–5 are built in P1. Steps 6–8 are built in P2, because they need `auth.Authenticate`.
    1.  Request ID + logging. The layer just inside step 5 (around the mux) records the matched route pattern with `logging.SetRoute`, because the `r.WithContext` copies in steps 2–5 keep the mux's `r.Pattern` from reaching the log line.
    2.  `X-App-Version` header. It is set before any handler or the recovery code runs, so every response from the app has it, including errors and panics; the PWA uses its absence to detect "server off".
    3.  Panic recovery: logs the stack with the request ID and answers `internal` through `problem.Write`. It re-panics `http.ErrAbortHandler` untouched: the export handler uses it to close a connection after the stream started, and a problem body there would be glued onto the half-written file.
    4.  Client IP (`CF-Connecting-IP` only from `TRUSTED_PROXY_CIDRS`).
    5.  Origin check on every method except GET and HEAD (the mux routes HEAD to GET handlers).
    6.  Session loading, plus the hourly sliding write from "Session and request rules" (`last_seen_at`, `expires_at` and a new `Set-Cookie`). `auth.Authenticate` runs in a short `db.WithTx`, because it may write the slide. A malformed, unknown or expired cookie leaves the request without a session; a database error is a 500.
    7.  Auth required for `/api/v1/*` except `GET /setup/status`, `GET /setup/options`, `POST /setup`, `POST /auth/login` and `GET /push/key` (401 `unauthorized`). The public set is read once at `httpapi.New` from the embedded spec (`gen.GetSpec`): the operations whose own `security` is present and empty (`security: []`), as `"METHOD /api/v1<path>"` patterns. Every request needs a session unless its `r.Pattern` is in that set, so an empty or unknown pattern needs one too (fail closed). `New` returns an error when a `security` requirement, global or on an operation, names any scheme but `sessionCookie` or is the empty `{}`.
    8.  The `must_change_password` gate. It allows exactly `POST /auth/password`, `GET /me` and `POST /auth/logout`, and answers every other `/api/v1` route with `must-change-password`. That includes the public routes of step 7: a request to `GET /setup/status`, `GET /setup/options`, `POST /setup` or `POST /auth/login` that carries a must-change session cookie is 403 too, because step 6 loaded the session and step 8 checks every session it finds (a request without the cookie is not affected). The allowed set is the operations marked `x-oc-allowed-while-must-change-password: true` in the spec, read once at `httpapi.New` from the embedded spec (`gen.GetSpec`) as `"METHOD /api/v1<path>"` patterns and compared with `r.Pattern`; a non-boolean value stops `New` with an error.

    Steps 6–8 run as `gen.StdHTTPServerOptions.Middlewares`, after routing and only on the generated `/api/v1` routes (OP5); there is no route list in Go. The generated wrapper applies that list so the last entry runs first, so it is ordered 8, 7, 6. The generated wrapper binds path and query parameters before these steps run, so a malformed or missing parameter gives 422 before the session check (401); that 422 path reads and writes no data.

##### 35. `internal/httpapi/static.go` (Create)

*   **Type / Scope**: SPA serving.
*   **Proposed Changes**:
    *   Serves `web.Build` (item 36) through `fs.Sub(web.Build, "build")`. It has no `go:embed` of its own.
    *   `/index.html` and the fallback write the embedded `index.html` with `http.ServeContent`, status 200. Not `http.FileServer` or `http.ServeFileFS`: both answer `/index.html` with a 301 to `/`, and the service worker would precache that redirect, which breaks the cold open with the server off.
    *   Other built files (JS, CSS, images) go through `http.FileServerFS`.
    *   `/` is routed to the same index handler. Unknown files and directories (no listings) fall back to `index.html`. Unknown `/api/` paths never reach this file (item 32).
    *   `_app/version.json` is `no-cache`.
    *   Cache headers:
        *   `_app/immutable/*`: `public, max-age=31536000, immutable`.
        *   `service-worker.js`, `index.html`, the manifest and icons: `no-cache`.

##### 36. `web/embed.go` (Create)

*   **Type / Scope**: Go embed package.
*   **Proposed Changes**: `package web` with `//go:embed all:build` and `var Build embed.FS`. It sits in `web/` because `go:embed` cannot reach parent directories.

##### 37. `internal/httpapi/*_test.go` (Create)

*   **Type / Scope**: Handler tests.
*   **Proposed Changes**: `httptest`-based tests for:
    *   middleware order;
    *   the Origin check;
    *   client IP trust;
    *   the SPA fallback and cache headers, and `GET /index.html` answering 200 with the HTML, not a redirect;
    *   for each endpoint of the phase, the success status and the body fields match the tables in item 15 (the check strict mode would have made at compile time);
    *   problem JSON shape, including a bad UUID path parameter and a bad date query parameter (422 problem JSON, not `text/plain`). P1 has no route with parameters, so P1 tests the `ErrorHandlerFunc` directly; the per-route tests come with the first such routes in P3;
    *   `X-App-Version` present and non-empty on success, error and generated responses;
    *   409 overlap responses (P3): the body lists the overlapping appointments and has no `errors` (`TestCreateAppointmentOverlapIs409WithAppointments`).
    *   An unknown `/api/v1` path and a wrong method both return 404 problem JSON, not HTML.
    *   An oversized JSON body returns 413, not 422.

    These tests use real modules and a database, so they carry the `db` build tag.

#### Auth

##### 38. `internal/auth/password.go` (Create)

*   **Type / Scope**: Password hashing.
*   **Proposed Changes**: Argon2id (m=64 MiB, t=3, p=1, 16-byte salt, 32-byte key) in PHC string format: `$argon2id$v=19$m=65536,t=3,p=1$<salt>$<hash>`, with salt and hash in unpadded standard base64 (`base64.RawStdEncoding`). Verification parses the stored parameters, recomputes, and compares with `crypto/subtle.ConstantTimeCompare`. At most two hashes or verifies run at once (`maxConcurrentHashes`, a buffered channel; item 41); the rest wait for a slot, and one whose `ctx` ends first returns the `ctx` error without hashing. Minimum password length is 8.

##### 39. `internal/auth/sessions.go` (Create)

*   **Type / Scope**: Sessions.
*   **Proposed Changes**:
    *   Create, load and revoke sessions. Sessions are never listed (no device screen).
    *   Token, cookie and the sliding expiry: as in "Session and request rules". This file has the functions; the middleware (item 34 step 6) calls them.
    *   A password change (`POST /auth/password`, which needs the current password) revokes every other session of the user and keeps the current one. `reset-password` revokes all sessions.

##### 40. `internal/auth/setup.go` (Create)

*   **Type / Scope**: First-run setup.
*   **Proposed Changes**:
    *   `NewSetupToken`: deletes old tokens, creates one valid 24 h. Used by `serve` at start and by `setup-link`. Once a user exists it returns 409 `setup-closed`, even though both callers check first (fail closed). Both `NewSetupToken` and `CompleteSetup` take a transaction from the caller and take one advisory lock (`pg_advisory_xact_lock`) as their first statement, so two token calls never leave two live tokens and two setups never both create a user.
    *   `CompleteSetup` runs in the handler's transaction: it checks first that no user exists (409 `setup-closed`), then the token (unused, not expired, else 410), normalizes and checks the username ("Session and request rules"), calls the settings creator (which validates and inserts the settings row), creates the owner user, deletes all setup tokens, and creates a session. It creates nothing else: the app ships empty.
    *   `auth` defines the types: `SetupSettings{Name, Timezone, Locale, Currency, CountryCode, AccentColor string; PushHour *int}` (a pointer, so an absent push hour reaches `settings`, which fills in 8), `SettingsCreator func(ctx, q db.DBTX, s SetupSettings) error`. The settings creator is a closure in `cmd` that converts `SetupSettings` to `settings.SettingsInput` and calls `settings.Create`. This keeps `auth` from importing `settings` and the other domain modules. `auth.New` returns an error when the function is nil (fail closed).

##### 41. `internal/auth/ratelimit.go` (Create)

*   **Type / Scope**: Login protection.
*   **Proposed Changes**:
    *   An `x/time/rate` limiter per client IP on `POST /auth/login` only (5/min, burst 5), with idle entries removed after 10 min. An IPv4 address (an IPv4-mapped IPv6 address is unmapped first) has its own bucket; an IPv6 address counts against its /64, because one client usually holds a whole /64 and could otherwise change address on every attempt. A peer address that did not parse gets one bucket shared by all such requests.
    *   A per-username delay: after the first failure the next attempt must wait 1 s, doubling after each failure up to 30 s. An attempt that comes too early gets 429 `rate-limited` with `Retry-After`; the server never sleeps. It resets on success. Entries expire 15 min after the last failure, so the map holds only names that failed in the last 15 min; there is no size cap.
    *   `POST /auth/password` shares the per-username delay: `ChangePassword` counts the attempt for the session user's username before it checks `current_password` and clears the delay when it is right, so a stolen session cannot guess the current password without limit. A wrong one is still 422 on `current_password`; a try that comes too early is 429 `rate-limited`, and so is a login for that username until the delay has passed. The per-IP limiter does not apply to it.
    *   **The limits run before the transaction.** The handlers call `auth.BeginLogin` (username format, IP bucket, per-username delay) and `auth.BeginPasswordChange` (the delay, for the session's username) before `db.WithTx`, so a 429 opens no transaction of its own (`POST /auth/password` still has the session step's, item 34 step 6). They count the attempt as before; `Login` and `ChangePassword` take only the attempt value a `Begin` call returned, and refuse the zero value or an attempt for another user as a 500 (`errAttemptNotBegun`), so no caller can skip the limits.
    *   **Hashing is capped.** Each argon2id hash or verify takes 64 MiB, so at most two run at once (item 38); a burst of logins for different usernames, each under its own delay, waits for a slot instead of holding that much memory per request. A verify still runs inside the handler's transaction, so a login waiting for a slot holds its transaction (and a pool connection) while it waits.

##### 42. `internal/auth/*_test.go` (Create)

*   **Type / Scope**: Tests.
*   **Proposed Changes**:
    *   Hash round trip.
    *   Session expiry and sliding, including the re-sent cookie after an hour.
    *   A setup token cannot be reused.
    *   Setup is refused once a user exists.
    *   Rate limit by trusted IP vs untrusted peer.
    *   `reset-password` sets `must_change_password`.
    *   Username: " Ana " logs in as "ana"; a name with a space inside, or under 3 characters, is rejected.
    *   A wrong current password on `ChangePassword` is a `validation` error, not `unauthorized`.
    *   Login hardening (item 41): `TestHashConcurrencyIsCapped`, `TestHashWaitsReturnsOnCtxDone`, `TestIPv6SameSlash64SharesBucket`, `TestIPv6OtherSlash64HasOwnBucket`, `TestIPv4MappedIsIPv4`, `TestLoginWithoutBeginLoginFailsClosed`, `TestChangePasswordAttemptForAnotherUserFailsClosed`; in httpapi, `TestLoginRateLimitedOpensNoTransaction` and `TestChangePasswordRateLimitedOpensOnlyTheSessionTransaction`.

#### Domain modules

##### 43. (Removed)

*   **Type / Scope**: None.
*   **Proposed Changes**: Was the starter content (`internal/starter`). The owner decided the app ships empty: setup creates only the settings row, her user and her first session. The number is kept so the other item numbers stay stable.

##### 44. `internal/settings/settings.go`, `manifest.go`, `icons.go`, `defaulticon.go`, `default-icon-512.png`, `default-icon-180.png` (Create)

*   **Type / Scope**: Settings and branding.
*   **Proposed Changes**:
    *   Get and update settings. Validate `name` (trimmed, 1–80 characters), `locale` (`es` only), `accent_color` (`color.Normalize`: lowercased, `^#[0-9a-f]{6}$`), `push_hour` (0–19, default 8 when absent), the time zone with `time.LoadLocation`, the currency with `currency.ParseISO` from `golang.org/x/text/currency` (no hand-written list), and the country code against `phonenumbers.GetSupportedRegions()`.
    *   Build the manifest JSON from settings.
    *   Serve the uploaded icons, or the built-in icon (`defaulticon.go`): a light disc (`--bg-100`) on a square of the accent color, drawn at 512 or 180 px. A stored accent that does not parse logs `code=accent_color_invalid` and serves the embedded PNG. The embedded PNGs (the same shape in the default accent) serve before setup.
    *   `SetIcons` checks the PNG signature (415) and the 1 MB limit (413) of each part, writes `media/branding/icon-512.png` and `icon-180.png` (temp file, fsync, rename) and only then sets `icon_updated_at`. `Icon` serves the file when `icon_updated_at` is set; a missing file logs `code=icon_file_missing` and serves the built-in icon. Any size other than 512 or 180 is `not-found`.
    *   Before setup there is no settings row: the manifest is minimal (`name` and `short_name` "open-customer", `lang: "es"`, no colors, the default icons) and the icons are the defaults.

##### 45. `internal/attributes/attributes.go`, `keys.go`, `validate.go` (Create)

*   **Type / Scope**: Attributes.
*   **Proposed Changes**:
    *   CRUD for attributes and options. `type`, `key` and `scope` cannot change. Options nest to any depth; the module rejects a parent from another attribute and a change of `parent_id`. A cycle cannot happen: a parent must exist before its child, and `parent_id` never changes.
    *   `GenerateKey(label, taken)` (D7): lowercase, remove accents, `_` for every character outside `a–z0–9`, runs of `_` collapse and leading and trailing `_` are dropped, an empty result is `a`, `a_` is prefixed if the result starts with a digit, the base is cut to 40 characters before any suffix, and a taken or reserved name gets a suffix starting at `_2` ("name" → "name_2"). "Taken" means every key in `attribute_definitions`, archived ones and both scopes included.
    *   `Validate(ctx, q, scope, stored, request, isCreate) (attrs, error)` loads the definitions, applies the value table, the "required on create or touch" rule and the rejected writes from "Attribute values", and does the merge itself: the touched keys are the keys of `request`, `null` removes a key on `PATCH` and is ignored on create, so no caller computes "touched" differently. Errors are a `validation` problem with one `attrs.<key>` field each.
    *   Options (D8): only `select` and `multiselect` attributes take options (422 on `type` otherwise). A parent must be an active option of the same attribute (422 on `parent_id`), and an archived attribute takes no new option (422 on `attribute`). Positions count among siblings: the same attribute and the same `parent_id`. Labels are 1–80 runes, trimmed.
    *   `Format(def, raw, loc, target) string` (the options travel in `def.Options`): the one function that turns a stored value into text, following the value table. `target` is `ForTemplate` (path labels joined by a space, "A, B y C") or `ForCSV` (" / " and "; "). Archived attributes and options format by their last label. `reminders` (due list and preview) and `exporter` both call it, so the two outputs never drift.

##### 46. `internal/attributes/*_test.go` (Create)

*   **Type / Scope**: Tests.
*   **Proposed Changes**: A table test per type in the value table, each rejected write (unknown key, other scope, archived attribute, archived option in a new value) and `""` on a required attribute, the option tree rules (foreign parent and `parent_id` change rejected; a pick at any depth accepted), key generation (accents, duplicates, reserved names), required-on-create vs untouched old records, archived options still rendering, and `Format` for each type and both targets (a 3-level path, an archived parent, "A, B y C" vs "; ").

##### 47. `internal/clients/clients.go`, `phone.go` (Create)

*   **Type / Scope**: Clients.
*   **Proposed Changes**:
    *   CRUD and archive.
    *   A duplicate-phone warning, overridden with `allow_duplicate_phone=true`.
    *   `NormalizePhone(raw, region)` returns E.164 + digits, or a validation error.

##### 48. `internal/clients/*_test.go` (Create)

*   **Type / Scope**: Tests.
*   **Proposed Changes**:
    *   "8888-7777" with region `CR` gives `+50688887777`.
    *   Invalid numbers are rejected.
    *   `List(archived)` returns only archived or only active clients, by name.
    *   The duplicate-phone warning works.

##### 49. `internal/services/services.go`, `services_test.go` (Create)

*   **Type / Scope**: Service catalog.
*   **Proposed Changes**: CRUD, ordering, archive. `GetMany` returns archived services too, and an unknown id is absent from the map. The test that archived services stay readable on past appointments lives in `appointments` (it owns `appointment_services`).

##### 50. `internal/appointments/appointments.go`, `status.go`, `overlap.go` (Create)

*   **Type / Scope**: Appointments.
*   **Proposed Changes**:
    *   Create and update. The services' name, price and duration are copied into `appointment_services`. `ends_at` = start + the sum of the durations, unless set by hand. `total_cents` = the sum of the prices.
    *   `Transition(a, to, now)` applies the status table from "Appointments" and returns the updated appointment, or an `invalid-transition` error naming the two statuses.
    *   `IsDone(status, endsAt, now)` returns true when the status is `scheduled` or `confirmed` and `endsAt <= now`.
    *   The list and detail queries join `clients` for `client_name`.
    *   `Overlaps(start, end, excludeID)` finds overlapping appointments whose status is `scheduled` or `confirmed`.
    *   `service_ids: []` without `ends_at` is a `validation` error (422), not a CHECK failure.

##### 51. `internal/appointments/*_test.go` (Create)

*   **Type / Scope**: Tests.
*   **Proposed Changes**:
    *   Every allowed and rejected transition.
    *   `IsDone` at the `ends_at` boundary, and for cancelled and no-show.
    *   Price snapshot unchanged after a service edit.
    *   Overlap detection and `allow_overlap`, also on "Reactivar" through `SetStatus`.
    *   A create whose service-line insert fails leaves no `appointments` row (the handler's transaction rolls back). The test installs a `BEFORE INSERT` trigger on `appointment_services` that raises, and runs `Create` inside `db.WithTx`.
    *   An archived client or service in a create or `service_ids` is rejected; a line of a service archived later is kept on a `service_ids` edit; a kept active service keeps its saved price, and only an added one is copied at today's price.
    *   `ListRange` uses the business's local days, also across a DST change (`Europe/Madrid`).
    *   The same service twice in one appointment.
    *   The editing rules: `service_ids` keeps saved lines, copies added ones, and recomputes `total_cents` but not `paid_cents`; `ends_at` recomputed without `ends_at` in the request and kept with it; moving `starts_at` moves `ends_at`; a past done appointment can be edited.

##### 52. `internal/media/media.go`, `media_test.go` (Create)

*   **Type / Scope**: Photos.
*   **Proposed Changes**:
    *   Accepts multipart `full` + `thumb`. Checks the JPEG magic bytes of `full`, decodes the JPEG header of both parts, and checks the limits (`full` ≤ 10 MB, `thumb` ≤ 1 MB).
    *   Reads width and height of `full` with `image/jpeg.DecodeConfig`; `thumb` must decode too, and either failing is 415.
    *   Writes and syncs both files to `<MEDIA_DIR>/<yyyy>/<mm>/` (UTC), then inserts the row. A failed insert removes the files; `Discard(ctx, md)` does the same for a failed commit.
    *   Serves with a session and the cache header.

    *   With `appointment_id`, sets `client_id` from the appointment (ownership rule in "Photos").

    Tests: rejects non-JPEG, rejects oversize, and rejects an upload with neither `client_id` nor `appointment_id`. With only `appointment_id`, the stored `client_id` is the appointment's client; a mismatching `client_id` is rejected.

##### 53. `internal/templates/parse.go`, `render.go`, `vars.go`, `templates.go` (Create), `sample.go` (P4)

*   **Type / Scope**: Templates.
*   **Proposed Changes**:
    *   `parse.go`: a tokenizer for `{{ ns.key | "fallback" }}` that accepts only what the chip serializer writes (straight quotes). The parse result lists the variables used.
    *   `render.go`: `Render(tpl, Data) string`. It never returns an error; missing values become the fallback or empty text. `Data` holds values that are already formatted:

        ```go
        type Data struct {
            Client      map[string]string // "name", "first_name", and one entry per client attribute key
            Appointment map[string]string // "date", "time", "services", and one entry per appointment attribute key
            Business    map[string]string // "name"
        }
        ```

        A missing or empty entry renders the fallback. `templates` never formats values itself; the caller fills `Data` with `attributes.Format` (item 45) and `internal/locale`.
    *   `vars.go`: one table of the fixed variables with two views. `Vars(loc)` lists the fixed variables and the active attributes with their current labels, and is sent to the phone as `template_vars[]` in `/bootstrap`, for the chip row. The save check uses the other view, `known`, which counts every attribute of the matching scope, archived ones included, so a template that uses an archived attribute still saves. The labels of the fixed variables come from `locale.VarLabel` (item 61). `UnknownVars(parsed, attrs)` returns every occurrence that `known` rejects, in body order; the save check and the preview both call it.
    *   `sample.go` (built in P4, because its only caller is `reminders.Preview`, which sets its inputs): `Sample(loc, attrs, now, tz) SampleRecord` builds the built-in sample client and appointment for previews, with the placeholder value per attribute type (see "Message templates"). `SampleRecord` holds `ClientName`, raw `ClientAttrs` and `AppointmentAttrs` JSON, `StartsAt` and `ServiceNames`. Values are raw: `reminders` formats them with `attributes.Format`, as for a real client. Only active attributes get a value. Text is its label in brackets, number is 1, date is today's local day (`YYYY-MM-DD`), select is the ID of the first active root option (by position, then ID), multiselect is a list of that ID; a select with no such option gets no key. `StartsAt` is tomorrow at 15:00 in the business zone. The sample names and texts come from `internal/locale` (`SampleClientName`, `SampleServiceNames`), so no language is written in this package. The sample has no phone: the preview never shows one.
    *   `templates.go`: CRUD. The preview endpoint is served by `reminders.Preview` (item 32), which loads the data, fills `Data` and calls `Render`.

##### 54. `internal/templates/*_test.go` (Create)

*   **Type / Scope**: Tests.
*   **Proposed Changes**:
    *   Fallback used / not used.
    *   An unknown variable at save gives an error naming the token as written.
    *   `Render` with pre-formatted `Data`: a value, an empty value with and without a fallback.
    *   Saving a template that uses an archived attribute succeeds.
    *   Any `{{` that does not start a valid variable is a validation error naming its position.

    Formatting is tested where it lives: nested option paths, an archived parent and "A, B y C" in `attributes` (item 46); Spanish dates and times in `locale` (item 61); `first_name`, archived attributes rendering by their last label, and archived options in `reminders` (item 56), which builds `Data`.

##### 55. `internal/reminders/rules.go`, `due.go`, `log.go`, `links.go` (Create)

*   **Type / Scope**: Reminders.
*   **Proposed Changes**:
    *   `rules.go`: CRUD for the two kinds. It sets `applies_from` from `include_overdue` (absent or `false`: now; `true`: `NULL`) on create and on the changes listed in "Reminder rules". It rejects an archived template, service or return service (422). The CHECK rules are mirrored in Go: `service_id` and `return_service_id` are set only for `come_back`; `offset_days` is `>= 0` for `before_appointment` and `> 0` for `come_back`; `kind` and `service_id` cannot change after creation; `return_service_id` can. Range: `offset_days <= 365` for both kinds. A `before_appointment` rule sent a `service_id` or `return_service_id` is 422 `MsgServiceNotAllowed` on that field; a `come_back` rule without `service_id` is 422 `MsgServiceRequired`. The template, service and return service must exist and be active (an unknown id is 422 like an archived one); every bad field is collected. `UpdateRule` looks the rule up first (unknown id: 404, before any field check) and has no `kind` or `service_id`. The "asks" changes (they set `applies_from` to now, or to `NULL` with `include_overdue`; any other change keeps it and ignores `include_overdue`) are: a new `offset_days`; a changed *effective* return service (`return_service_id`, or `service_id` when empty, so setting it to the rule's own service, or back to null, does not ask); restoring an archived rule; a new template while the stored one is archived; a new return service while the stored effective one is archived. Archiving uses `archive.At`. `created_at` and `applies_from` come from the injected clock.
    *   `due.go`: `DueList(ctx, q, now)` and the pure `compute(data, now)`, exactly as in "Due-list computation".
    *   `log.go`: `ParseAction` (422 on `action`) and `Record(LogEntry) (LogItem, created bool)` with `ON CONFLICT DO NOTHING`. Validation (D11) reports every bad field as a 422: an unknown `rule_id` or `appointment_id`; `anchor_at` unset for a "before" rule, or set for a `come_back` rule; `phone_digits` that are not 1 to 15 digits. An archived rule still records, because she acted on what she saw. When the item (rule, appointment, anchor) is already logged, nothing is written and `Record` returns the stored `LogItem` with `created` false (O4); the handler answers 200 with it, not 409.
    *   `log.go` also has `LogForClient(clientID)` (newest `acted_at` first; an unknown client is 404) and `LogForDate(day, loc)`. Both join the rule, appointment and client, so each row carries `kind`, `client_id` and `client_name` (`LogItem`). `LogForDate` returns the rows whose `acted_at` falls on the local day of `day` in `loc`, the business time zone.
    *   `links.go`: `WhatsAppLink(digits, text)`. This is the only place that builds the link: `https://wa.me/<digits>?text=<encoded>`. It switches to `whatsapp://send?phone=<digits>&text=<encoded>` only if the Open Questions test on her iPhone fails with `wa.me`. The text is percent-encoded with spaces as `%20`: Go's `url.QueryEscape` writes `+`, which WhatsApp may show as a literal plus (not checked), so the encoder replaces it. The replace runs after `QueryEscape`, so a literal plus in the text stays `%2B`.

##### 56. `internal/reminders/due_test.go`, `due_comeback_test.go`, `due_db_test.go`, `cuj_db_test.go` (Create)

*   **Type / Scope**: Tests (fake clock, DB-backed).
*   **Proposed Changes**: One case per row and bullet of the rules table. The unit tests of `compute` (`due_test.go`, `due_comeback_test.go`) cover the cases it owns; the cases that need the database (a cancelled later booking or a return service taken off it, an archived client, the `include_overdue` default, the push count) are `DueList` tests (`due_db_test.go`, `db`-tagged). `cuj_db_test.go` is the P4 exit test ("Build order"): it makes the CUJ 4 and 5 rows with the real modules and checks the three items `DueList` returns (a "before" message, a come-back message and a booking prompt):
    *   A 1-day "before" rule is not skipped by its own appointment (the SAD bug: its skip rule counted the appointment itself as a later one).
    *   It is hidden after `starts_at`.
    *   One item for a two-service appointment.
    *   Come-back appears on day N with no "complete" tap.
    *   A no-show removes it.
    *   A later booking that includes the return service hides it; cancelling that booking, or taking the return service off it, brings it back.
    *   A later visit without the return service does not hide it (a pedicure after an acrylic set).
    *   An empty return service means the rule's own service: a later booking of that service hides it.
    *   A future booking without the return service turns the item into a booking prompt with that appointment's ID, start and services and the return service's name; with two such bookings, the earliest is used; after the return service is added to it, the item is gone.
    *   A later booking without the return service that started before the due day gives a normal message, not a prompt.
    *   A sent or skipped booking prompt stays hidden (the same `(rule_id, appointment_id)` log row).
    *   Two come-back rules on one appointment each check their own return service.
    *   A rule whose return service is archived produces no items; creating or changing a rule to an archived return service is 422.
    *   A booking prompt counts in the push count.
    *   A reschedule gives a "before" item a new identity and re-rendered text: a "before" item that was sent shows again after `starts_at` changes.
    *   A rule whose template or service is archived produces no items.
    *   `applies_from`: a new come-back rule with "Solo de hoy en adelante" gives no late items for old visits, but does give an item for a visit whose due day is ahead; with "Incluir atrasadas" it gives the late items from the 60-day look-back. Lowering `offset_days` with "Solo de hoy en adelante" removes items that were already late.
    *   An archived client produces no items, of either kind.
    *   A come-back that was sent or skipped stays hidden after the visit's `ends_at` changes (a service added, the times fixed).
    *   Items handled in the log are excluded.
    *   The 60-day look-back drops old items.
    *   A downtime catch-up gives late come-backs and no stale "before" items.
    *   The time zone boundary at local midnight.
    *   A same-day booking gets no 1-day "before" item, but does get a 0-day one.
    *   A visit created today with `ends_at` 40 days ago and a 21-day rule gives no item; with `ends_at` 10 days ago it gives one 21 days after `ends_at` (11 days from now).
    *   A come-back rule created today with "Solo de hoy en adelante" gives the item whose due day is today.
    *   `first_name` (the name up to the first space, after trimming), and archived attributes and options rendering by their last label, in the built `Data`.
    *   The 60-day boundary: an item 60 days past its due day is listed; at 61 it is gone.
    *   A come-back item counts at 00:01 local time on its due day, so the push at `push_hour` includes it.
    *   A 0-day "before" item whose appointment starts before `push_hour` is not in the list at `push_hour`.
    *   A rule created without `include_overdue` behaves as "Solo de hoy en adelante".
    *   Items are sorted by due day, then client name, then item ID.

##### 57. `internal/push/push.go`, `push_db_test.go`, `send_db_test.go` (Create)

*   **Type / Scope**: Web Push.
*   **Proposed Changes**:
    *   `EnsureKeys`: generates the pair with `webpush.GenerateVAPIDKeys` and stores it in `server_keys` (`vapid_public`, `vapid_private`, base64url) if the public key is missing. It runs in a transaction, so both keys are stored together. The private key is never logged or written anywhere else.
    *   `Subscribe`: checks the subscription (the endpoint is an absolute `https` URL; `keys.p256dh` decodes from base64url to 65 bytes and `keys.auth` to 16; each bad field is a 422 on `endpoint`, `keys.p256dh` or `keys.auth` with `MsgPushEndpointInvalid`), then inserts it, or gives a known endpoint the new keys (its row and ID stay; `created` is false).
    *   `SendAll(payload)` to every subscription, one after another, with `TTL: 28800` and `Urgency: normal`. A 2xx answer counts as delivered. A 404 or 410 deletes the subscription. Any other answer or a network error counts as not delivered and logs `code=push_send_failed` with the status only (never the endpoint or the error, which quotes it). Answer bodies are closed. Returns how many were tried and how many the push service accepted. A database error (loading the list or the keys, deleting a gone subscription) is returned.
    *   `Options.HTTPClient` sends the requests; nil means a client with a 15 s timeout.
    *   `VAPID_SUBJECT` starts with `mailto:`. webpush-go v1.4.0 adds `mailto:` to every subscriber that does not start with `https:` (checked in its `vapid.go`), so `push.New` strips the prefix before passing it on, and the JWT `sub` has exactly one `mailto:` (`TestVAPIDSubjectHasOneMailto` decodes the JWT and checks `sub`). v1.4.0 also signs the JWT expiry with its own clock and ignores `VapidExpiration`; the module still sets it from the injected clock.

    Tests (`db`-tagged) use a fake push service from `httptest.NewTLSServer`, with its `Client()` as `Options.HTTPClient`, and generate every key at test time.

##### 58. `internal/scheduler/scheduler.go`, `scheduler_db_test.go` (Create)

*   **Type / Scope**: Daily timer.
*   **Proposed Changes**:
    *   `Run(ctx, ticks)`: `Tick` at start, then per tick, as in "Process model".
    *   `Tick(ctx)`, in this order:
        1.  Writes `heartbeat` = now (RFC 3339) on the pool; an error is returned.
        2.  `settings.Get`: `NotFound` (before setup) returns nil; another error is returned.
        3.  In the business time zone: before `push_hour`, from 20:00, or when `push_checked_date` is already today (`YYYY-MM-DD`), it returns nil.
        4.  Counts `DueList` in a read-only `REPEATABLE READ` transaction; an error is returned and nothing is recorded.
        5.  When the count is above 0, `SendAll` with `{title: business name, body: locale.PushBody(n), count: n, url: "/enviar"}`. An error with no attempt is returned; an error after some attempts logs `code=push_send_partial` and goes on.
        6.  In one write transaction: `push_checked_date` = today and, when it sent, `last_push` = `{date, attempted, delivered}`.
    *   `State(ctx, q)`: reads the `scheduler_state` keys (heartbeat, `push_checked_date`, `last_push`) for `status`. A stored heartbeat or `last_push` it cannot parse is nil and logs `code=heartbeat_invalid` or `code=last_push_invalid`.

    Tests (fake clock):
    *   Push sent once.
    *   A late start before 20:00 sends it; after 20:00 it does not.
    *   No push when the count is 0.
    *   The day boundary in the business time zone.
    *   `last_push` records attempted and delivered counts; no settings row means no push and no error.
    *   A count failure records nothing; a send that fails with no attempt records nothing; a partial send still records the day.
    *   A corrupt `last_push` or heartbeat row logs its code; `Run` ticks at start and per tick until cancelled.

##### 59. `internal/exporter/export.go`, `json.go`, `export_test.go`, `export_db_test.go`, `export_json_db_test.go` (Create)

*   **Type / Scope**: Export.
*   **Proposed Changes**: `New(Options{Clock, Settings, Attributes})`; `Write(ctx, q, w)` streams a ZIP with the four files and the rules from "Export" (`export.go`: the ZIP and the CSV files; `json.go`: the `datos.json` types and writer); `FileName(day)`. Tests (db, settings `USD` and `Europe/Madrid`; they read the ZIP back with `archive/zip` and `encoding/csv`): `TestExportHasFourFilesInOrder`; `TestExportCSVHeadersAndBOM` (`EF BB BF`, then `nombre,telefono,notas,creada,archivada,<labels>`); `TestExportAccentedNameAndCommaNote` ("Lucía Rojas", "Uñas, cortas"); `TestExportMultiselectCell` ("A; B; C"); `TestExportSameLabelGetsKey` (`Color (color)`, `Color (color_2)`); `TestExportArchivedClientMarked`; `TestExportArchivedAttributeOnlyInJSON`; `TestExportAppointmentRow` (2026-10-24T13:00Z gives `2026-10-24`, `15:00`, "Acrílico; Pedicure", "12500,00", an empty `pagado`, "realizada" once past `ends_at`); `TestExportNoShowAppointment` ("no vino"); `TestExportZeroDecimalCurrency` (`JPY`, "12500"); `TestExportPagesPast500` (501 clients in the CSV and the JSON); `TestExportJSONRoundTripsIDs` (client, appointment, rule and log IDs, `version` 1, `reminder_log[0].client_name`); `TestExportDBErrorIsReturned` (`FailingDB` at the second call). Unit: `TestFileNameUsesTheDay`.

##### 60. `internal/status/status.go`, `mediafree_statfs.go`, `mediafree_other.go`, `status_test.go` (Create)

*   **Type / Scope**: Health and status.
*   **Proposed Changes**:
    *   `/healthz` (P1): `200` with the plain-text body `ok`.
    *   `/readyz` (DB ping + no pending migrations, through `db.HasPending`) (P1): `200 ok`, or `503` with the plain-text body `not ready`.
    *   `New(Options{Version, Clock, MediaDir, BackupStatusFile, Push, Scheduler})` builds the module. `Status(ctx, pool)` (P4) returns a `Report` (`Version`, `MigrationVersion`, `HeartbeatAgeS *int64`, `Backup`, `MediaFreeBytes *int64`, `Push`). It owns no table and calls only read functions: the version string passed in by `main` when `status` is built, `db.Version` (migration version), `scheduler.State` (heartbeat age, last push result), `push.CountSubscriptions`, the `BACKUP_STATUS_FILE` content (`backupstatus.Read`), and media volume free space via `golang.org/x/sys/unix.Statfs` (`Bavail * Bsize`, each field widened to `uint64` because its type differs per OS; nil with `code=status_statfs_failed` when it fails). `Statfs` is built only for Linux, macOS, FreeBSD and DragonFly; on any other system, Windows included, a stub file returns nil, so `go build` works on every OS (`make lint` builds darwin and windows). `heartbeat_age_s` is whole seconds since the heartbeat, nil when there is none. The backup line is red when the file is missing or unreadable (all times nil), `last_copy_at` is nil or older than 48 h, `last_error` is set, or `checked_at` is nil or older than 2 h (`Backup()`; `BackupRed()` is its `Red`). The push line is red with 0 subscriptions, or when `last_push` has `attempted > 0` and `delivered == 0`.
    *   `LogClientError` (P4) is the client-error log endpoint: limits and logged fields in "Observability", rate limited by a token bucket per session, a burst of 10 then one more every 6 seconds (`golang.org/x/time/rate` on the injected clock).

    Tests: missing or corrupt `status.json` gives `backup.red: true` with null times, not a crash; each red rule from "Backups" (old `last_copy_at`, `last_error` set, old or missing `checked_at`); a file written in the exact `backup.sh` time form parses.

##### 61. `internal/locale/locale.go`, `es.go`, `locale_test.go` (Create)

*   **Type / Scope**: Formatting.
*   **Proposed Changes**: Day and month names, date and time formats, number format and list joining per locale; `LocalDay` and `AddDays` for calendar-day arithmetic in a zone. Currency formats are added in the phase of their first caller: `Money(minor, scale)` (P4, the export) formats an amount in minor units with the decimal mark and no symbol (`1250000, 2` → "12500,00"; `12500, 0` → "12500"). `ExportText()` (P4) gives the export's CSV file names, column headers, `estado` words and "sí". `import _ "time/tzdata"` (also imported in `cmd/open-customer/main.go`). `Locale.VarLabel(ns, key)` gives the label of a fixed template variable (`""` for an unknown pair), and `SampleClientName()` ("Ana Pérez") and `SampleServiceNames()` (["Servicio de ejemplo"]) give the sample texts that template previews use, because Spanish text in Go lives only in `locale` and `problem`. Tests cover Spanish formats (`appointment.date` as "sábado 24 de octubre", `appointment.time` as "3:00 p. m."), a DST zone (`Europe/Madrid`) for day offsets, `TestMoney`, `TestExportText` and `TestExportTextIsACopy`.

#### Web app (`web/`)

##### 62. `web/package.json`, `web/pnpm-lock.yaml` (Create)

*   **Type / Scope**: Web package.
*   **Proposed Changes**:
    *   Dependencies (shipped in the bundle): `dexie`, `openapi-fetch`, `zod` (runtime parsing of API responses and cache rows, items 66–67), `@fontsource-variable/nunito` (the one font, as in the mockup; bundled so the app never calls an outside font server and the service worker precaches them; item 64).
    *   Dev dependencies (build and check tools; the app is static, so nothing else ships): `@sveltejs/kit` (v3), `@sveltejs/adapter-static`, `@sveltejs/vite-plugin-svelte`, `vite`, `svelte` 5, `tailwindcss` v4 + `@tailwindcss/vite`, `typescript` (6.0.x: typescript-eslint and SvelteKit do not accept 7 yet), `openapi-typescript`, `vitest`, `@playwright/test` (exact version), `eslint`, `@eslint/js`, `globals`, `typescript-eslint`, `eslint-plugin-svelte`, `@vitest/eslint-plugin`, `eslint-plugin-playwright`, `prettier`, `prettier-plugin-svelte`, `svelte-check`, `knip`, `jscpd`, `@types/node`, `fake-indexeddb`, `pg` and `@types/pg` (for `seed.ts`, item 80), `postcss` (the `design-tokens` guard reads `app.css` with it), `eslint-plugin-better-tailwindcss` (UI pipeline stage 4).
    *   Scripts: `build`, `dev`, `check` (`svelte-kit sync && svelte-check --fail-on-warnings && tsc -p tsconfig.service-worker.json`), `lint` (`eslint . && prettier --check . && knip && jscpd`), `format`, `test` (`vitest run --passWithNoTests`), `e2e` (`playwright test`, since P6). `gen:api` (`openapi-typescript ../api/openapi.yaml -o src/lib/api/schema.d.ts`) exists since P1.
    *   `web/pnpm-workspace.yaml` is written by pnpm (its release-age settings) and committed. `web/eslint-suppressions.json` is the ESLint bulk-suppressions file (`{}` at P0), pinned by the web `suppression-budget` guard.
    *   Created in P0 with a minimal `web/src/app.html` and a placeholder `web/src/routes/+page.svelte` (one line of text), so `make lint test build` work from the first commit. P5 replaces both (items 64 and 72).

##### 63. `web/vite.config.ts`, `web/tsconfig.json`, `web/eslint.config.js`, `web/.prettierrc`, `web/.prettierignore`, `web/knip.json`, `web/.jscpd.json` (Create)

*   **Type / Scope**: Build config.
*   **Proposed Changes**:
    *   SvelteKit 3 has no `svelte.config.js`: the kit options below are passed to `sveltekit({...})` in `vite.config.ts`.
    *   `adapter-static` with `fallback: 'index.html'`.
    *   `serviceWorker.register: true`.
    *   `version.name: process.env.VERSION ?? 'dev'`, the same value as the Go `-ldflags` version. The `Dockerfile` and the `Makefile` pass `VERSION` to both builds.
    *   The Tailwind Vite plugin, listed before `sveltekit()` in the Vite plugins.
    *   `tsconfig.json` extends `$app/tsconfig` (SvelteKit 3), includes `src`, `tests` and the root config files, and excludes `src/service-worker` and `src/service-worker.ts` (SvelteKit asks for the first; the second is the file itself, because TypeScript's `exclude` does not add the extension). `tsconfig.service-worker.json` extends `$app/tsconfig/service-worker` (the `webworker` lib) with the same strict flags and covers the worker and `lib/sw-routes.ts`, so `pnpm check` type-checks the worker too.
    *   A dev proxy to `http://localhost:8080` for `/api`, `/manifest.webmanifest`, `/icons` and `/apple-touch-icon.png` (the Go server generates the manifest and serves the icons).
    *   Strict TypeScript: `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `noFallthroughCasesInSwitch`, `noImplicitReturns`.
    *   Vitest (the `test` block of `vite.config.ts`): `include` `src/**/*.test.ts` and `tests/guards/**/*.test.ts`, `allowOnly: false`, `restoreMocks: true`.
    *   `eslint.config.js`: flat config (`defineConfig`) with `typescript-eslint` `strictTypeChecked` (type-aware; `no-explicit-any`, `no-floating-promises`, `consistent-type-imports`, `switch-exhaustiveness-check` as errors) and `eslint-plugin-svelte`; `@vitest/eslint-plugin` on test files (`valid-title`, `expect-expect`, `no-conditional-in-test`, `no-conditional-expect`, `no-focused-tests`, `no-disabled-tests`, `no-restricted-matchers` for snapshots); `eslint-plugin-playwright` on `tests/e2e` (`no-skipped-test`, `no-wait-for-timeout`, `no-focused-test`, `no-raw-locators`, `prefer-native-locators`); `eslint-plugin-better-tailwindcss` on `src` (its `correctness` rules with `entryPoint: src/app.css`, and `no-restricted-classes`: arbitrary values everywhere, surface and radius classes outside `lib/components/ui/`) plus `no-restricted-syntax` for padding or an expression in a `<Card class>`, a spread on a Card and a `<style>` block (see "Code layout and module rules"); outside `ui/` the restricted classes are also looked for in every script variable, object value and `$derived` / `$derived.by` argument, with or without the `!` modifier. `.prettierrc`: `prettier-plugin-svelte`. All plugins are dev dependencies (item 62).
    *   `knip.json`: listed the dependencies a later phase starts using; each phase removed the ones it used, and P6 removed the last (`pg`, `@types/pg`), so it lists none now. Knip's Playwright plugin entries are listed by hand (`"playwright": {"config": [], "entry": ["playwright.config.ts", "tests/e2e/**/*.spec.ts"]}`): loading the config runs `appEnv`, which throws without `E2E_DATABASE_URL`. `.jscpd.json`: `src`, `tests`, `eslint.config.js` and `vite.config.ts`, TypeScript, JavaScript and Svelte, `**/*.test.ts` and `**/schema.d.ts` ignored, threshold 0, `failOnEmpty` (a scan that analyzes no files fails; `src` alone is too small early on to pass it). `failOnEmpty` fails only on a fully empty scan, and no `.svelte` file reaches `minTokens` yet, so Svelte coverage is not yet shown.

##### 64. `web/src/app.html`, `web/src/app.css` (Create)

*   **Type / Scope**: Shell and styles.
*   **Proposed Changes**:
    *   `app.html`: `lang="es"`, the iOS meta tags (`apple-mobile-web-app-capable`, `apple-mobile-web-app-status-bar-style` `default`, `viewport-fit=cover`), `theme-color`, and links to `/manifest.webmanifest` and `/apple-touch-icon.png`.
    *   `app.css`: `@import "tailwindcss"`, the font package, and an `@theme inline` block (`inline` makes the utilities use the runtime variables instead of values fixed at build time). The tokens are the mockup's (`design/soft-glass.pen`, `variables`): colors `bg-100`, `bg-200`, `glow-blush`, `glow-peach`, `glow-lilac`, `glass`, `glass-strong`, `glass-edge`, `glass-tint`, `ink`, `ink-muted`, `sage-200`, `success`, `warning`, `error`; radii `sm` 10, `md` 16, `lg` 24, `xl` 32 and `pill`; one font, `sans` (Nunito), for everything: screen titles 30 px extrabold (`text-3xl font-extrabold`), card and section titles 20 px bold (`text-xl font-bold`), as in the mockup; spacing is Tailwind's 4 px scale. The accent is not fixed: `--accent` is set at runtime from settings and starts as the mockup's rose-500 (`#c76b85`, also `DEFAULT_ACCENT` in `lib/color.ts`). `accent-100`, `accent-300`, `accent-700`, `glass-tint`, `glow-blush` and the shadows' tint (the mockup's rose) are derived from it with `color-mix(in oklab, …)`, so her color recolors the whole app with no pink left over. A filled accent control (Button `strong`, the floating "+", the selected Día/Semana toggle, a count badge) uses `--accent-fill` with `--on-accent` text, both set with `--accent` by `applyAccent` (`accentFill` in `lib/color.ts`): the fill is the accent's 700 shade and the text is white or `ink`, whichever has more contrast; when neither reaches WCAG AA (4.5:1) the fill darkens until white does, so the labels stay readable for any accent she picks (`.pen` variable `accent-fill`, the default accent's value). Its hover is `--accent-fill-hover`, moved away from the label (darker under white, lighter under ink), so the label keeps AA on it. The accent as text on a light surface (ghost Button, TextButton `accent` and `back`, Chip `selected`, the variable chips, the status chips, the active NavItem, the time grid's times and today's head) is `--accent-text` (`accentText`): the 700 shade, darkened until it reaches AA on every surface it sits on: the page at its darkest (`darkestPage`: `--bg-100` or the center of a body glow, peach, lilac or the accent's blush), `--accent-100`, `--accent-300` (the hover of a ghost Button and an `accent` IconButton) and `glass-tint` (`glassTint`: `--accent-300` at 35% alpha painted over that darkest page, as the CSS composes it; a glass card between them only lightens it). `accent100` and `accent300` compute the same OKLab mixes as the CSS; for a near-black accent, where even black text would miss AA on the 45% mix, `accent300` mixes less accent in, and `applyAccent` sets that `--accent-300`. `.pen` variables `accent-fill-hover` and `accent-text` hold the default accent's values. Utility classes `glass-card` and `glass-panel` (fill, 1 px edge, `backdrop-filter: blur(24px)`, the mockup's card shadow; used only by the primitives in `components/ui/`, `Card` first) and `solid-panel` (the opaque `bg-100` panel of `Card` `solid`), safe-area padding helpers, `bottom-fab` (the floating "+" above the tab bar and the safe area), `min-h-tap` (44 px), and named values in place of arbitrary ones: `max-h-sheet` (90dvh), `max-h-grid` (75dvh), `content-placeholder` (`attr(data-placeholder)`).
    *   `lib/layout.ts`: `layout.current` (`compact` or `wide`), following `matchMedia('(min-width: 1024px)')`. See "Screen layouts".

##### 65. `web/src/service-worker.ts` (Create)

*   **Type / Scope**: Service worker.
*   **Proposed Changes**:
    *   On `install`: precaches `immutable` + `assets` from `$app/manifest` (SvelteKit 3 removed `$service-worker`; its paths have no leading slash, so the worker makes them absolute), plus `/index.html`, into the cache `oc-${version}`. `index.html` is listed by hand, because the SPA fallback page is not in SvelteKit's `build` or `prerendered` lists. The manifest and the icons are not precached. It does not call `skipWaiting` on its own.
    *   On `activate`: deletes every cache except `oc-${version}`.
    *   On `fetch`: navigation requests outside `/api/*` get the cached `/index.html`. Navigation requests to `/api/*` are not handled, so they go to the network. Precached files are cache-first. `/api/*`, the manifest and the icons are never cached; the worker does not handle them, so they always go to the network.
    *   On `message` `{type: "SKIP_WAITING"}`: calls `self.skipWaiting()`.
    *   On `push`: parses the payload once (`pushShown` in `lib/sw-routes.ts`), shows the notification, and calls `self.navigator.setAppBadge(n)` when it exists. Every push shows a notification, because the subscription is `userVisibleOnly` and iOS can revoke one whose push shows none: a push with no payload or a bad one (for example from a newer server than this worker) shows the fixed `push_fallback_title` and `push_fallback_body` texts, sets no badge and logs `code=push_payload_invalid`.
    *   Types come from `self` in `$app/service-worker` (typed `ServiceWorkerGlobalScope`) under `tsconfig.service-worker.json`. The worker imports only `$app/env`, `$app/manifest`, `$app/service-worker` and `lib/` modules (guard `layers`). The routing choice is the pure `swAction(url, mode, precached)` in `lib/sw-routes.ts`.
    *   On `notificationclick`: closes the notification, then focuses an open window and navigates it to `/enviar`, or opens one (`openFromTap` in `lib/sw-routes.ts`). `navigate()` rejects for a window the worker does not control (`matchAll` with `includeUncontrolled` returns those), so on any rejection it logs `code=notification_navigate_failed` and opens a new window instead.

##### 66. `web/src/lib/api/schema.d.ts` (Create, generated), `web/src/lib/api/client.ts` (Create)

*   **Type / Scope**: API client.
*   **Proposed Changes**:
    *   `schema.d.ts` is generated by openapi-typescript and committed.
    *   `client.ts` wraps `openapi-fetch`:
        *   Treats a network error, or a response without the `X-App-Version` header (Cloudflare's 502, 524 and 530 included), as `Offline`. It checks the header before openapi-fetch parses the body, using openapi-fetch's response middleware. A 5xx from the app carries the header and is shown as an error, not as offline. 204 responses and `problem+json` errors are handled normally.
        *   Reads `X-App-Version`, compares it with `version` from `$app/env`, and raises the update prompt; the tap runs the update flow:
            *   First, the tap calls `registration.update()`. Then, once the new worker is installed and waiting, the page posts `{type: "SKIP_WAITING"}`. After that the worker calls `self.skipWaiting()`. Finally, on `controllerchange` the page reloads once. If no new worker shows up within 10 s, the page reloads anyway.
        *   Maps problem JSON to typed errors.
        *   Parses every response body once with its zod schema before returning it. A body that fails the parse becomes a typed `response_invalid` error (shown as an error, not as Offline) and is reported through the client-error log with the path and code only, never the body.
        *   On an `unauthorized` (401) error, `client.ts` calls its `onUnauthorized` hook and returns the error, except for a 401 on `POST /client-errors` (best effort: a page with no session must not be sent away by its own error report). The wired instance in `web/src/lib/api/app.ts` sets that hook to `clearCache()` from `db.ts`, then sends her to the login screen; `client.ts` itself never imports the cache. `invalid-credentials` on the login form does not clear anything.
        *   The Ajustes screens use `updateSettings`, `createService` and `updateService`, `createAttribute` and `updateAttribute`, `createOption` and `updateOption`, `uploadIcon` and `logout`.
        *   `listAppointments({from, to})` is `GET /appointments` for Hoy's days outside the cache window; it returns the `items` array, each parsed with the appointment schema.
        *   The client screens use `listClients({archived})`, `getClient`, `updateClient` (a 409 `duplicate-phone` carries the clients that share the phone), `listClientAppointments`, `listReminderLog({client_id})` (or `{date}`, for Hoy's sent list) and `uploadMedia` with `client_id` or `appointment_id`.
        *   `app.ts` also wires the other hooks: `version` from `$app/env`, `onNewVersion` raises a flag the update prompt reads, and `reportInvalid` posts `{message: "response_invalid", route, app_version}` through `logClientError` (`postErrorReport`, shared with the global error handlers of item 71). A `response_invalid` on `/client-errors` itself is not reported again, so a broken log endpoint cannot loop.

##### 67. `web/src/lib/cache/db.ts`, `refresh.ts`, `retry.ts`, `drafts.ts`, `store.svelte.ts` (Create)

*   **Type / Scope**: Phone cache.
*   **Proposed Changes**:
    *   `db.ts`: a Dexie schema (settings, attributes, services, templates, rules, clients, appointments, due items, drafts, meta) with a version number (2; its upgrade clears the server-data tables only, see "Storage"), and `clearCache()`, which deletes the database. Logout (`logOut` in `lib/logout.ts`, called by the Ajustes list) and the 401 handler call it. `meta` holds `lastSyncAt`, the last `Me` (for the offline layout, item 71), the pending send confirmation (see "Sending flow"), `templateVars` and `backupRed` from the last `/bootstrap`, and `logged`, the ids of due items the server has logged that the cached `due` table may still hold (a meta row stored before the field existed reads as none). Each `due` row is `{id, position, item}`: the table is keyed by `id` and Dexie reads rows back in key order, so `position` keeps the server's order (due date, then name) and `readAll` sorts by it. `writeSnapshot` keeps in `logged` only the ids the new due list still holds, in the same transaction that replaces the tables. Every row read from Dexie (drafts included) is parsed once with its zod schema, so a bad cached row never reaches a screen. A snapshot or meta row that fails the parse (an old schema, a broken write) calls `clearCache()` and then the store's bad-row listener (`onBadRow`) runs a refresh from `/bootstrap` while a session is live. A draft that fails its parse (one from an older form shape) is deleted alone and reads as no draft (`code=draft_invalid`): it is the app's own local value, so the rest of the cache, the pending send confirmation included, stays.
    *   `retry.ts`: `retryWhileOffline`, the refresh while the server does not answer (`cache.offline`): after 2 s, doubling up to 60 s (`retryDelay`), and at once on the window's `online` event, until a refresh reaches the server.
    *   `refresh.ts`: calls `/bootstrap` on open and focus, replaces the cached tables in one transaction, stores `lastSyncAt`, and updates the app badge to the shown count (see "Daily push").
    *   `store.svelte.ts`: the reactive read model screens use (`cache.snapshot`, `cache.meta`, `cache.offline`, `cache.lastError`, `cache.shownDue`, `load()`, `rememberMe()`, `setSyncing()`, `refresh()`). `load()` reads Dexie only; `rememberMe(me)` stores the user the server named in `meta.me`, which the shell reads for an offline open. Clearing the cache (401, bad row, logout) also empties the store's in-memory snapshot and meta (`onCacheCleared` in `db.ts`), so the dead session's user and data are never reused. Refreshes never overlap (`lib/serial.ts` `serialLatest`): a call made while one runs waits for it and then refreshes once more, shared by every call made meanwhile, so an older answer never overwrites a newer one. The shell tells the store whether a session is live (`setSyncing`); a bad snapshot or meta row found then runs a refresh, except during a refresh, which reads the cache again when it ends. `cache.shownDue` is the one due list every screen shows and counts: "Para enviar", Hoy's `DueSummary` (compact and the wide rail), the sidebar count and the badge. It is `shownDue` in `lib/due.ts`: the cached list without the items the server has logged (`logged`: set by `markLogged` after a log `POST`, which stores the id in `meta.logged` and, for "Sí", clears the pending confirmation in the same write; `load()` reads the stored ids back, so a reload keeps them hidden) and, offline, without a cached "before" item whose `anchor_at` has passed (item 74). A successful refresh drops from `logged` only the ids its list no longer holds: a logged item never comes back on the server, so an id still there is from a refresh asked before its log `POST`. `offline` is true only when the last refresh got no answer from the server; `lastError` holds any failure of the last refresh (the server's error, or `cache_write_failed` when the phone could not store the answer, in which case the old data stays) and is null after a success.
    *   `refresh.ts` takes the window from the business day: `todayIn(settings.timezone, now)` from the cached settings, or the browser zone before the first refresh.
    *   `drafts.ts`: only the new-appointment and new-client forms. The key is `<form>:<record id or "new">`. A draft is saved 500 ms after the last input and cleared after a successful save. `formDraft` (`formDraft.svelte.ts`) is the lifecycle both forms use: load on open, save on change, "Empezar de nuevo", clear.

##### 68. `web/src/lib/i18n/es.json`, `web/src/lib/i18n/index.ts` (Create)

*   **Type / Scope**: UI strings.
*   **Proposed Changes**: Every UI string is in `es.json`, and `t(key, params)` looks it up. No inline Spanish text in components. This is a review check; there is no lint rule in release 1.

##### 69. `web/src/lib/search.ts`, `web/src/lib/photos.ts`, `web/src/lib/push.ts`, `web/src/lib/push-browser.ts`, `web/src/lib/install.ts` (Create)

*   **Type / Scope**: Utilities.
*   **Proposed Changes**:
    *   `search.ts`: the only client search, with the rules from "Clients and search": NFD normalization, accent removal, name substring, phone substring for 3 or more digits, word-start first, top 20.
    *   `photos.ts`: `createImageBitmap` with `imageOrientation: 'from-image'` (with an `<img>` + canvas fallback if the call or the option is not supported), a canvas resize so the long edge is 2048 px and 400 px, JPEG 0.8 output, and a square icon crop to 512 and 180 PNG.
    *   `push-browser.ts` builds the real `PushDeps` from the page (`navigator`, `Notification`, the display mode); tests pass a fake environment. `push.ts`: permission request (in the installed app only, from the "Activar notificaciones" tap), subscription, and the check on each open that sets "needs activation" when permission is not granted or no subscription exists (see "Daily push"). `checkPush` returns `unsupported`, `not_installed` (checked first: iOS Safari tabs have no push), `needs_activation` or `active`; a failed re-post (offline) stays `active` and the next open retries. `pushControl.svelte.ts` is the one owner of what the screens show about push (the last check, the busy tap, the error text): the shell calls `check()` on open, and every "Activar notificaciones" button (`PushActivate.svelte`, in the sidebar and on Hoy) reads it and calls `activate()`. `activatePush` asks for permission before any request, so it stays inside the tap, then reads the key (`GET /push/key`), subscribes, and posts the subscription; any browser-side failure is the `push_unavailable` error.
    *   `install.ts`: detects iOS + not standalone, for the install guide.

##### 70. `web/src/lib/components/` — primitives in `ui/`: `Card.svelte`, `Callout.svelte`, `Button.svelte`, `buttonStyle.ts`, `TextButton.svelte`, `IconButton.svelte`, `Chip.svelte`, `TextField.svelte`, `SelectField.svelte`, `fieldStyle.ts`, `chipStyle.ts` (the variable chip that `lib/chipsDom.ts` builds by hand), `ListRow.svelte`, `EmptyState.svelte`, `OfflineBanner.svelte`, `BottomSheet.svelte`, `BottomNav.svelte`, `NavItem.svelte`, `DueCard.svelte`; feature components: `UpdatePrompt.svelte`, `ColorPicker.svelte`, `OptionTree.svelte`, `SettingsList.svelte`, `ChipEditor.svelte`, `TemplatePreview.svelte`, `TemplateForm.svelte`, `TemplatePane.svelte`, `AttributeField.svelte`, `ClientPicker.svelte`, `ServicePicker.svelte`, `DayView.svelte`, `WeekView.svelte`, `ConfirmSentSheet.svelte`, `DueSummary.svelte`, `NoRulesState.svelte`, `DueItemRow.svelte`, `OverlapSheet.svelte`, `PhotoGrid.svelte`, `PhotoSection.svelte`, `AppointmentForm.svelte`, `ClientForm.svelte`, `ClientList.svelte`, `FormCard.svelte`, `DuplicatePhone.svelte`, `ActionButton.svelte`, `WriteButton.svelte`, `WriteLink.svelte`, `SaveButton.svelte`, `LoadFailure.svelte`, `TimeGrid.svelte`, `PushActivate.svelte`, `InstallGuide.svelte`, `StatusRow.svelte`, `RuleSentence.svelte`, `RuleSheet.svelte`, `AuthCard.svelte`, `BusinessIcon.svelte`, `Toast.svelte`, `Sidebar.svelte`, `navItems.ts`, `icons/Icon.svelte`, `icons/paths.ts` (Create)

*   **Type / Scope**: Own UI components (Tailwind).
*   **Proposed Changes**:
    *   `ui/` holds the primitives: one per reusable component of `design/soft-glass.pen` (`.claude/rules/ui.md` "UI pipeline"; a primitive with variants exports `type <Name>Variant`, equal to the .pen's variant names, guard `design-parity`), and the only files that may use surface and radius classes. `Card` is the one glass surface (the .pen `GlassCard`): `variant` `default`, `strong`, `tint` or `solid` (opaque, `solid-panel`: the toast, so the text under it does not show through); `pad` `none`, `list`, `note`, `field`, `trail`, `message`, `md`, `lg` or `section` (`p-5 lg:p-8`); `radius` `none`, `sm`, `lg` (default), `xl` or `pill`; `as` picks the element; `selected` draws the accent outline of a picked row; `class` is a plain string of layout classes. `Callout` is the tinted box with no glass edge (a note, a restored draft, the booking prompt, an inline form; `pad` `md` or `trail`). `IconButton` is the icon-only control (`add` is the floating "+"; `plain`, `muted`, `glass`, `accent` (inside a selected chip), `square` (a row tool); other attributes such as `disabled` and pointer handlers pass through to the button; `inert` is the faded, inactive look), `TextButton` a text-only pill that sizes to its label (`accent`, `danger`, `back` with a chevron), `Chip` a toggle chip (`default`, `selected`; an optional `trailing` control in the same pill, the service picker's minus), `NavItem` one of the four places (`tab` in `BottomNav`, `row` in `Sidebar`). Feature components and routes compose them.
    *   `BottomSheet`: a focus trap, swipe down to close, and the safe area. Sheets can stack (a discard question over an archive sheet): `lib/sheetStack.ts` keeps the open ones, so the page scroll locks when the first opens and unlocks when the last closes, whatever the closing order, and only the top sheet answers Escape and Tab. It moves itself to its own layer of `<body>` when it opens (`toBody`, `lib/inert.ts`): a glass card (`backdrop-filter`) is its own stacking context, so a sheet written inside one would sit under the tab bar. Every other layer of `<body>` is `inert` while it is the top sheet (`inertBehind`, `lib/inert.ts`; the stack names each new top), so no tap, Tab or screen reader reaches the page behind; a layer marked `data-live-layer` (the toast) stays live, and its controls join the sheet's Tab cycle after the sheet's own, so "Deshacer" is reachable by keyboard; the ui-probe's inert check skips that layer and nothing else.
    *   `Button` (variants named as in the .pen: `strong` (the accent fill), `glass`, `ghost`, `danger`; the .pen's pale `Button/Primary` was removed and its uses in the mockup screens point at `Button/Strong`, which is what the code draws; a busy state; `disabled` is typed to come with a `disabledReason`, shown under the button, or with a `reasonId`, the id of one line elsewhere on the screen that says why for many buttons (the due cards point at the offline banner's line, `ui/offlineReason.ts`, through `WriteButton` `sharedOfflineReason`, and at "Para enviar"'s one `due_skip_wait` line while an "Omitir" can be undone), so a disabled button always says why and a list does not repeat it under every card; `busy` needs none, its spinner is the reason) and `TextField` (label, error, hint; text, password, tel, email, search, date and time inputs) with `SelectField` (the same look, over a list of `{id, label}`; `ui/fieldStyle.ts` `fieldClasses` is that field surface, shared with the attribute picker and the template editor); `AuthCard` is the frame of the stand-alone screens (setup, login, new password): flat on the glow background on a phone, a centered 440 px glass card on a wide screen; `ui/buttonStyle.ts` holds the button classes that links styled as buttons share. `Toast`: a short message with an optional action ("Deshacer"), gone after `ms`; the app has one, drawn by the root layout from `lib/toaster.svelte.ts` `toaster` (every screen calls `toaster.say`, a new message replaces the one on screen, `dismiss` takes away a toast whose action no longer applies, as "Para enviar" does when an "Omitir" is sent or undone), so two toasts never overlap (default `UNDO_MS`, 5000, the one constant that `lib/undo.ts` `delayed` also uses, so the toast and the action share one delay). Stacking: `BottomSheet` is `z-50`; the toast is `z-60` at the top of the screen (a sheet rises from the bottom or is centered), so it never covers a sheet's buttons; it is its own live layer of `<body>`, so its "Deshacer" stays tappable while a sheet makes the page inert. `icons/Icon.svelte`: the Lucide icons the mockup uses, drawn inline (no icon package); their paths and the `IconName` type are in `icons/paths.ts`, so typed lint can read the type. `ColorPicker` offers `swatches()` from `lib/color.ts` (each with a Spanish color name for its accessible name, "Color rosa", never the hex code) plus a native color input. `FilePicker` is the one file field: an app `Button` with Spanish text that opens the chooser of a visually hidden `<input type="file">` (out of the tab order and the accessibility tree), so no browser-language "Choose File" shows or becomes the accessible name; `PhotoSection` ("Agregar foto") and `IconFileInput` ("Elegir imagen" and the picked file's name) use it.
    *   The focus trap's wrap rule is `nextFocus` in `lib/focus.ts`, tested; components and routes import `lib/` with relative paths: SvelteKit 3.1.1 replaced the `$lib` alias with `#lib`, and the app uses neither.
    *   `Sidebar` (wide) and `BottomNav` (compact) are the two shells of "Screen layouts": the same four places (`navItems.ts`, with `isActivePath` and `syncLine` in `lib/nav.ts`), the sidebar with the business icon and name, "Nueva cita", the due count, the sync line and "Activar notificaciones"; the tab bar the mockup's floating pill (Hoy, Enviar, Clientas, Ajustes).
    *   `ChipEditor`:
        *   A `contenteditable` field that shows tokens as chips.
        *   A row of chips to insert variables.
        *   Tapping a chip opens "Si no tiene: ___".
        *   It serializes to template syntax and gets the preview from `/templates/preview` (debounced at 300 ms).
        *   "Vista previa con:" above the preview: "Clienta de ejemplo" (default) or a client picked with `ClientPicker` (`allowAdd` off: no "Agregar clienta nueva" here), sent as `client_id`. The preview is `TemplatePreview`: the first answer is not delayed, a later one waits 300 ms after the last change, and an answer that arrives after a newer change is dropped.
        *   Chips are non-editable atoms (`contenteditable="false"`); a chip with a fallback shows "Etiqueta · texto". Enter inserts a plain newline (`beforeinput`), a paste is plain text, formatting commands and drops are refused, and the DOM is not rewritten while an IME composes. A chip or a trailing newline at the very end gets a skipped `<br>` after it so the caret has somewhere to sit. The "Si no tiene" field rejects a straight quote (`fallbackError`); an empty fallback means none.
        *   The mapping is split so the rules are testable without a DOM: `lib/chips.ts` (`parseBody`, `serializeBody`, `templateUses`, `piecesFromDom` over a `DomNode` list, `chipLabel`, `chipText`, `fallbackError`) is pure and matches `internal/templates/parse.go`; a `{{` that starts no valid variable, and a variable of a namespace the editor never writes, stays plain text in the editor, and `bodyProblems` reports it before saving (see "Braces" in "Message templates"), so the server's save check is a backstop. `lib/templateEdit.ts` (`templateDirty`) is the unsaved-changes rule. `lib/chipsDom.ts` is the thin browser glue (chip elements, reading the field into `DomNode`s, the caret).
    *   `TemplateForm`: the name, the `ChipEditor`, "Guardar plantilla" (`FormCard`), and for a saved template "Archivar plantilla" (a confirm sheet) or "Restaurar plantilla"; the server's 422 on `name` or `body` shows under its field.
    *   `AttributeField`: renders the right input per type, using native `<input type=date>` and `<select>` (a flat select offers an empty choice unless the attribute is required), and a drill-down sheet for nested options and for every multiselect: one list per level, a chevron beside an option that has children, and she can stop at any level. `optionPath` (`lib/booking.ts`) is the chain from the top option to the picked one; a value is shown as its labels joined by a space, as templates do. Archived options are not offered but still show in a stored value.
    *   `DayView` and `WeekView`: time grids (both are `TimeGrid` with one column or seven; `lib/agenda.ts` `layoutDay` places each appointment in the day's grid and gives overlapping ones side-by-side columns, back-to-back ones the full width; it lays out the drawn span, at least `TAP_MINUTES` (the 44 px tap height), so short back-to-back ones also go side by side and no box covers another). The grid holds the whole day (00:00 to 24:00) in its own scroll area, because there is no business-hours setting; it opens at the hour of the earliest appointment that starts on the shown days, or 08:00, the assumed opening hour (`scrollHour`; the after-midnight tail of an appointment from the day before does not count), the week's day heads stay pinned while it scrolls down and the hour column while it scrolls sideways. The scroll area is a focusable region ("Horario"), so a keyboard can scroll an empty day. Every appointment box is at least 44 px tall; one of 30 minutes or less shows the time and the client on one line. The week starts on Monday and its column heads open that day. Cancelled appointments are hidden; "No vino" ones are shown greyed out. An appointment that crosses midnight is clipped to each day it touches.
    *   `DueSummary`: the "Para enviar hoy" count. On compact a tinted card that links to `/enviar`; on wide a card with the heading, the count link and the first 5 items, each with an "Enviar" link to `/enviar?item=<id>`, which opens that item. With no rules it is an `EmptyState` that leads to Ajustes.
    *   `ServicePicker`: tapping a picked service again adds a second line, shown as "×2" (`tapService`); a minus beside a picked chip takes one copy away (`removeService`). With no services it is an `EmptyState` that leads to Ajustes. A service archived since the booking shows only while it is on the appointment.
    *   `ClientPicker`: type to search the cached clients (`lib/search.ts`, the one search); "Agregar clienta nueva" opens a sheet with name and phone (`POST /clients`; a `duplicate-phone` 409 shows who has the number through `DuplicatePhone` and offers "Guardar igual", which resends with `allow_duplicate_phone`), selects the new client and refreshes the cache.
    *   `AppointmentForm`: the form of "Nueva cita" and of the detail's edit mode, with the pure rules in `lib/booking.ts` (`formErrors`, `createBody`, `patchBody`, `endShown`, `restoreForm`). On compact the fields stack in the order of the mockup; on wide the card (up to 760 px) puts services and date/time left and the attributes and notes right. The end time is a time field that follows the services until she sets it by hand (`end` is null until then; "Calcular de nuevo" hands it back); the total is the sum of the prices. New: it keeps a draft (`autosave`, key `cita:new`) from the first change, restores it on the next open (with "Empezar de nuevo") and clears it after a save. Edit: only what changed is sent (`patchBody`): `service_ids` only when the list changed, `ends_at` only when she set it, so the server's recompute rules in "Appointments" apply. A saved line keeps its price and duration in the form's quote (`withSavedLines`). A 409 `overlap` opens `OverlapSheet`; "Agendar igual" sends the same body with `allow_overlap`. Offline the save button is disabled with the reason. The card, the recovered-draft notice and the save button are `FormCard`, shared with `ClientForm`; the draft lifecycle is `lib/cache/formDraft.svelte.ts`.
    *   `PhotoGrid`: the thumbnails (3 columns on compact, 5 on wide). "Archivar foto" asks first and comes from a long-press (500 ms) on compact (the image has `-webkit-touch-callout: none`, utility `touch-callout-none`, so iOS does not open its own image menu over it) and from a button on the tile on wide; the caller does the `PATCH /media/{id}`. `PhotoSection` is the whole photo card of a screen: the list from `GET /clients/{id}/media` (filtered by `appointment_id` on an appointment, the whole gallery on a client), "Agregar foto" (`toJpegs`, then `POST /media` with `appointment_id` or `client_id`; disabled offline), the grid and its toast. It reads its list once, so the screen keys it on the owner.
    *   `ListRow` (a link line with title, subtitle and an optional trailing part) and `EmptyState` take an `href` that the caller already resolved with `resolve`, so a dynamic route (`/clientas/[id]`) can pass its values. `EmptyState` with `creates` shows its action as a `WriteLink`. `WriteButton` is a button that writes to the server and `ActionButton` is one in a detail card's action row (built on it); both are off, with the reason, while the server is off; `WriteLink` does the same for a link to a create screen; `LoadFailure` shows why a record could not be read, with "Reintentar". `lib/cachedLoad.ts` `showThenFetch` is the load every detail screen shares: the cached copy at once, the server's after, and a server that is off keeps the copy.
    *   `ConfirmSentSheet`: the "¿Lo enviaste a …?" sheet. It reads the pending confirmation from the cache store (Dexie `meta`) and clears it ("Sí" and "Todavía no", or when its item is gone from the due list), so `+layout.svelte` can open it on any app start. The rules are `lib/pending.ts` (`startSend`, `pendingToAsk`, `confirmSent`, `skipItem`, `notYet`), tested. The sheet has no dismiss: Escape, a tap outside and a swipe do nothing (`BottomSheet` `dismissable={false}`), so the question is only cleared by an explicit answer. A failed `POST` keeps the question and shows "No se guardó…".
    *   `DueItemRow` (a line of the list: a due item that selects, or a sent one with its time) and `DueCard` (the .pen `DueCard`: the client, the full message and the actions, or the booking prompt with its three actions; "Agregar teléfono" when she has no phone). `OverlapSheet` is the 409 sheet ("Se cruza con otra cita", "Agendar igual"). `NoRulesState` is the empty state of "no reminder rules yet", shared by `DueSummary` and "Para enviar".

##### 71. `web/src/routes/+layout.ts`, `+layout.svelte`, `web/src/lib/guard.ts`, `session.ts`, `nav.ts`, `titles.ts` (Create)

*   **Type / Scope**: App shell.
*   **Proposed Changes**:
    *   `+layout.ts`: `ssr = false`, `prerender = false`.
    *   `+layout.svelte`:
        *   Loads the cache and starts the refresh on open and again on focus (`shouldRefreshOnFocus`), and while offline keeps trying (`retryWhileOffline`, item 67), so the banner clears with the app left open; the store orders the refreshes so they never overlap. A failed refresh shows `cache.lastError` through `errorText` as a line at the top (the offline case has its banner instead).
        *   Sets `--accent`.
        *   Sets the page `<title>`: `pageTitle(page.route.id)` (`lib/titles.ts`) maps each route id to its Spanish title in `es.json`; the map is typed by the route ids, so a new route without a title does not compile.
        *   Shows the update prompt and, below it, the offline banner, both in the page's flow (the prompt never covers the banner). On a page with no shell (setup, login, new password) the prompt floats at the top instead, so it does not push the centered card down; those pages have no banner.
        *   Sends any uncaught `error` and `unhandledrejection` of the page, while there is a session (`canSync`), to `POST /client-errors` through `lib/report.ts`: `errorReport` gives the error's name and code as the message (never its text, which can hold a name or a phone), the stack from its first frame on (the header, which repeats the error text, is dropped; an empty name becomes `Error`), the route pattern (`page.route.id`) and the app version, cut to the endpoint's limits. `createReporter` sends one report per distinct message and first stack frame per page load and at most 10 a minute, so a burst of errors cannot spam or loop. It posts through the same `postErrorReport` as `reportInvalid` (`lib/api/app.ts`).
        *   Runs `checkPush` (with `browserPushDeps`) on open; the sidebar shows "Activar notificaciones" when it returns `needs_activation` and calls `activatePush` on the tap (the compact layout gets the button on Hoy and the status screen; both read `pushControl`).
        *   Opens `ConfirmSentSheet` first when a pending send confirmation is stored.
        *   Bottom navigation (`BottomNav`) on compact: Hoy, Enviar, Clientas, Ajustes; `Sidebar` on wide.
        *   Redirects to `/login` when the server says there is no session, and to `/setup` when setup is pending. While offline it never redirects: with a cached `Me` it shows the cached app; without one it shows "Sin conexión con el sistema".
        *   The redirect rules are `guard(path, session)` in `lib/guard.ts` (pure, tested); `lib/session.ts` `resolveSession` builds the `Session` from `GET /setup/status` and `GET /me` (an online session also carries the status's `accent_color`, which `+layout.svelte` applies while there is no cached settings row) (a `must-change-password` 403 on the status call means she is signed in with a forced change, so it goes on to `/me`), and any other answer it cannot read counts as offline, so a hiccup never redirects. `/login`, `/setup` and `/cambiar-clave` show without the navigation. `verdictFor` answers app pages at once for a settled session, so moving between them keeps the shell mounted. The layout asks the server again after a page change when the answer can have changed (no session, setup open, a forced password change, or a stand-alone page), and after a refresh that works while the session is still `offline` (`recoversSession`), so a phone that opened offline recovers when the server returns.
        *   `/setup` opened without a token in the link shows "Usa el enlace de configuración".

##### 72. `web/src/routes/+page.svelte` (Create)

*   **Type / Scope**: "Hoy".
*   **Proposed Changes**:
    *   **Compact**, top to bottom: the red backup line, the title ("Hoy" and the long date), the `DueSummary` (the count, linking to `/enviar`), "Activar notificaciones" when `pushControl` says "needs activation", the date switcher (previous, next, the short date as a native date input, and the Día/Semana toggle; Día by default), then the grid. A floating "+" opens `/citas/nueva?dia=YYYY-MM-DD` with the chosen day. Offline the "+" is shown faded; the offline banner says "Necesitas conexión con el sistema para guardar". The "+" sits above the tab bar and the home-indicator safe area (`bottom-fab` in `app.css`).
    *   **Wide**: the header row (the title, previous/next around a date input, the toggle; Semana by default), the grid in the main column, and the 360 px rail with the `DueSummary`, the backup line and "Activar notificaciones". No "+" (the sidebar has "Nueva cita").
    *   While Hoy is open it publishes the day it shows (`lib/shownDay.svelte.ts`), so the sidebar's "Nueva cita" opens `/citas/nueva?dia=` on that day (today on any other page). The appointment blocks of the grid link to `/citas/[id]`. A day she picks (arrows, date input, a week head) is remembered as she chose it, except today: picking today stores nothing (`pickedValue`), so the screen follows the business day and moves to the new one after midnight. The chosen day and view are local state; the day starts as the business day (`lib/clock.svelte.ts`, which reads the clock again on focus, when the app becomes visible and after each refresh, so a PWA resumed after midnight moves "Hoy" to the new day). `lib/agenda.ts` `viewRange` gives the days of the view (the day, or Monday to Sunday). When both ends are inside the `/bootstrap` window around the day of the last refresh (`inWindow`, `bootstrapWindow`), the appointments come from the cache; otherwise `GET /appointments?from=&to=` (online only) and, with the server off, the text "Necesitas conexión con el sistema para ver otros días"; any other failure shows its problem text with "Reintentar".
    *   With no rules yet, the count area is an empty state that leads to Ajustes → Recordatorios. When `backup_red` from `/bootstrap` is true, a red line says "Respaldo con problema" and links to `ajustes/estado`.

##### 73. `web/src/routes/setup/+page.svelte`, `web/src/routes/login/+page.svelte`, `web/src/routes/cambiar-clave/+page.svelte` (Create)

*   **Type / Scope**: Setup and login.
*   **Proposed Changes**:
    *   Setup: a wizard with one question per screen (business name, time zone defaulting to the browser's zone, currency, country, color, username, password). Then it calls `POST /setup` with `locale: "es"` (the only locale in release 1) and no `push_hour` (default 8; she changes it in Ajustes → Negocio). After that, with the new session, it asks for the icon (optional) and uploads it through `POST /settings/icon`. Finally it shows the install guide (on iOS outside the installed app; elsewhere a short "ready" text), and "Entrar a la app" opens the app. A step counter ("Paso 3 de 9") sits under the title. A server validation error on `POST /setup` takes her back to the step that holds the field (`stepForField` in `lib/setup.ts`). The country list loads from `GET /setup/options` when the wizard opens. `lib/setup.ts` holds the step order and the `Intl` lists.
        *   **Where the lists come from:** time zones from `Intl.supportedValuesOf('timeZone')` and currencies from `Intl.supportedValuesOf('currency')`, shown by IANA ID with a label from `Intl.DateTimeFormat('es', {timeZone, timeZoneName: 'longGeneric'})`; currency names from `new Intl.DisplayNames(['es'], {type: 'currency'})` (`DisplayNames` has no time-zone type); countries from `GET /setup/options`. Go has no list of time zones or currencies to send, so the server only validates the picked value (item 44). `Intl.supportedValuesOf` needs iOS 15.4 or later (not checked on her phone). If it is missing, the field is a text box with the browser's zone or a typed ISO code.
    *   Login: username + password. Inputs as in "Session and request rules" (`autocapitalize="off"`, `autocomplete`).
    *   `cambiar-clave`: current password + new password. Forced when `must_change_password` is set, and opened at any time from Ajustes → "Cambiar contraseña".

##### 74. `web/src/routes/enviar/+page.svelte` (Create)

*   **Type / Scope**: "Para enviar hoy".
*   **Proposed Changes**:
    *   Items grouped by late and today.
    *   Each item has Enviar / Omitir. A booking prompt (`mode: booking_prompt`) shows the prompt text ("Ana tiene cita el 12/11 (Pedicure). ¿Agregar Relleno?") with "Agregar a la cita" (`PATCH /appointments/{id}` with the appointment's `service_ids` plus the return service; the overlap sheet on a 409; then a refresh), "Enviar mensaje" (the normal send flow) and "Omitir". "Omitir" has no confirmation sheet: a 5-second "Deshacer" toast shows before the `POST` is sent (unless she leaves the page first, which sends it at once; see "Layouts" below). After the server logs it, `skipItem` refreshes the cache; if the phone cannot store the hidden item, the skip is still logged, so the refresh still runs and she sees "No se pudieron guardar los datos en el teléfono" (`cache_write_failed`), not a generic error.
    *   An "Enviados" section below: today's reminder-log rows with `action: sent` (`GET /reminders/log?date=<today>`; a skipped item is not listed), each with the client, the time and the exact text that was sent. Online only: the log is not cached, so offline the section shows "Necesitas conexión para ver los enviados".
    *   The send flow uses `ConfirmSentSheet` and then moves to the next item. After each successful log `POST` it updates the app badge.
    *   While offline, it shows the cached list read-only, with the actions disabled, and hides "before" items whose `anchor_at` (the appointment's start) is in the past.
    *   **Layouts:** compact shows each item as a card with its full message and actions; wide shows the 420 px list and the selected item on the right (`DueItemRow`, `DueCard`). After a send or skip the item that followed the chosen one takes the pane. "Enviar" opens the link in a new tab on wide and in the same window on compact; either way the confirmation is stored first. One "Omitir" waits for its "Deshacer" at a time (the others say why they are off). "Agregar a la cita" reads the appointment from the server (`GET /appointments/{id}`) and sends its current `service_ids` plus the return service, because the PATCH replaces the whole list. After a log `POST` the item is hidden locally (the store's `logged` ids, stored in Dexie `meta.logged` so they survive a reload, in `cache.shownDue`) until a successful refresh returns a list without it, so neither a failed refresh, nor one asked before the `POST`, nor a reload or closed app before the refresh after it is stored shows a logged item with "Enviar" enabled. On compact, "Enviar" first sends a waiting "Omitir" (`delayed.flush`), because the link unloads the page. A waiting "Omitir" is also sent at once when she leaves the page (`beforeNavigate`), on `pagehide`, and when the app goes to the background (`visibilitychange` to hidden; `onPageLeave` in `lib/undo.ts`), so an app sent to the background (which iOS may kill) or another page does not lose a skip the toast already called "Omitido". A reload or a closed tab can still cancel that request in flight (seen in WebKit).

##### 75. `web/src/routes/citas/nueva/+page.svelte`, `web/src/routes/citas/[id]/+page.svelte` (Create)

*   **Type / Scope**: Appointments.
*   **Proposed Changes**:
    *   New appointment: client picker (with quick add), services, date and time, the end time (computed from the services, and editable), appointment attributes, notes; the overlap warning sheet. With no services yet, the services step shows an empty state that leads to Ajustes → Servicios (an appointment can still be saved with an end time and no service). After saving, it goes back to the screen she came from (`?volver=<app path>`, checked by `returnTo`; Hoy by default). `?dia=YYYY-MM-DD` is the day it starts on (today when missing or not a real day). Hoy's "+" and the sidebar's "Nueva cita" pass `dia`.
    *   Detail: edit, status buttons (Confirmada, Cancelar, No vino, Reactivar; "Reactivar" shows the overlap sheet on a 409), amount paid (its own field and "Guardar pago", which sends `paid_cents`; empty clears it, and the amount is read with the currency's own decimals, `lib/money.ts`), and the appointment's photos with "Agregar foto" (CUJ 3 step 5). The photo input is `<input type="file" accept="image/*">` behind the "Agregar foto" button (`FilePicker`), which on iOS offers the camera and the photo library. The photos come from `GET /clients/{id}/media` filtered by `appointment_id`; "Agregar foto" decodes with `toJpegs` and posts to `/media` with `appointment_id`, and offline it is disabled. A long-press on a photo (compact) or the tile's button (wide) offers "Archivar foto" (`PATCH /media/{id}`), after a confirmation.
    *   Detail: the page shows the cached copy at once and replaces it with `GET /appointments/{id}`; offline it keeps the cached copy read-only (every write button is disabled with the reason). The status buttons come from `statusActions` (the table in "Appointments"); a status change waits for the server's answer.

##### 76. `web/src/routes/clientas/+layout.svelte`, `+page.svelte`, `clientas/nueva/+page.svelte`, `clientas/[id]/+page.svelte` (Create)

*   **Type / Scope**: Clients.
*   **Proposed Changes**:
    *   **Layout:** `+layout.svelte` holds `ClientList` for the whole `/clientas` tree. On wide the list is a 380 px column on the left, always there, and the right side is the page: the selected client, the form, or at `/clientas` the empty state "Elige una clienta". On compact the list is the screen at `/clientas` (the mockup's Clientas frame), and a client or a form replaces it; the list stays mounted (only hidden), so her search and "Ver archivadas" survive a visit to a client.
    *   **List:** `ClientList` shows every cached client by name while the box is empty, and `search.ts` over the cache when she types (`visibleClients` in `lib/clients.ts`), online and offline. "Ver archivadas" loads `GET /clients?archived=true` (again on each cache refresh while it shows, so a restore or an archive updates it; online only; offline the button is off with the reason) and searches that list with the same function; "Ver activas" goes back. An archived row carries an "Archivada" badge. "Nueva clienta" opens the form.
    *   **Form:** `ClientForm` (new and edit, `FormCard`): name | phone side by side on wide, then the client attributes (`AttributeField`) and notes. New keeps a draft (`autosave`, key `clienta:new`). Edit sends only what changed (`clientPatchBody`; a cleared phone or notes goes as `null`, a cleared attribute as `null` under its key). A phone another client has (409 `duplicate-phone`) shows who has it and "Guardar igual" (`allow_duplicate_phone`). After saving, it goes back to the screen she came from (`?volver=<app path>`, checked by `returnTo`; the list when there is none); on wide the screen she came from is the list pane, so the new client is selected in it (`/clientas/[id]`, `savedDestination`). An edit closes the form on her page. Days in the history and the reminder rows show the year when it is not the current business year (`formatDayWithYear`). While the duplicate-phone warning shows, the form offers only "Guardar igual" until she changes the phone.
    *   **Phone display:** the list and the detail show a phone through `formatPhone` (`lib/phone.ts`): the calling code, then the national number in its country's groups ("+506 8888 7777") for the countries it knows; any other number shows as stored. Only the display changes: the stored E.164, the search digits and the `wa.me` link stay as they are.
    *   **Detail:** her phone, attributes, notes, "Editar" and "Archivar" (a sheet asks first; `PATCH /clients/{id}` with `archived: true`). An archived client (opened from "Ver archivadas") shows a note and "Restaurar clienta" instead; a restore that meets the duplicate-phone check offers "Restaurar igual". Appointment history: the appointments of the cached window (`/bootstrap`'s) come from the cache, newest first; "Ver todas las citas" (online only) loads `GET /clients/{id}/appointments` and lists the ones outside the window under "Otras citas" (`historySplit`). The photo gallery is `PhotoSection` with `client_id`. Reminder history: `GET /reminders/log?client_id=` (online only; offline the section says so), each row with sent or skipped, the day and time, and the text that was sent. The page shows the cached copy at once and replaces it with `GET /clients/{id}`; offline an active client stays readable and every write button is off with the reason.

##### 77. `web/src/routes/ajustes/+page.svelte` and sub-routes `negocio/`, `servicios/`, `atributos/`, `plantillas/`, `recordatorios/`, `estado/`, `exportar/` (each `+page.svelte`) (Create)

*   **Type / Scope**: Settings.
*   **Proposed Changes**:
    *   The Ajustes root: one list with Negocio, Servicios, Atributos, Plantillas, Recordatorios, Estado del sistema, Descargar mis datos, "Cambiar contraseña" (opens `cambiar-clave`, item 73) and "Cerrar sesión" (`POST /auth/logout`, then `clearCache()`).
    *   **Layout:** `ajustes/+layout.svelte` frames every section. Wide: the section list (`SettingsList`, 280 px) stays on the left and the open section fills the right; `/ajustes` shows Negocio there, and the open row is marked. Compact: `/ajustes` is the list (the mockup's two groups, with "Cerrar sesión" last), and a section replaces it under a "Volver a Ajustes" bar (`showSettingsBack`; a template's editor has its own back to the list, so it gets no bar). Saves and archives say so in a toast. "Cerrar sesión" is `logOut` (`lib/logout.ts`): `POST /auth/logout`, then `clearCache()`, then `/login`; offline the call fails and she stays signed in with the error shown.
    *   `negocio`: name, time zone (the same list as the wizard), color (the app recolors as she picks it, and goes back to the saved color if she leaves without saving), push hour (00:00 to 19:00) and the icon, which is its own upload card. The save sends only the fields that changed (`businessPatch`). The icon card says that a new icon shows on the Home Screen only after she removes the app and adds it again. Currency, country and language are not shown: they are fixed at setup.
    *   `servicios`: the list (active, then "Archivadas"), each row with its minutes, price and color. A sheet adds or edits one: name, minutes (1 to 1440), price in the business currency (optional), color. "Archivar servicio" is one tap and "Restaurar servicio" undoes it. New services go last; reordering services and attributes is out of scope for release 1 (the spec asks to reorder only options); the phone lists services and attributes by `position`, then name (`byPosition`, applied when the cache is read).
    *   `atributos`: the list grouped by "De la clienta" and "De cada cita", then the archived ones. A sheet creates one with label + a plain question for scope ("¿Es igual siempre para la clienta, o cambia en cada cita?") + type; once it exists, the same sheet edits it (label and required; type and scope are fixed) and, for a select or multiselect, shows `OptionTree`: options in one list per level, with "+ Agregar opción" at each level, drag to reorder, and "Archivar" in the row menu (it archives at once and the toast offers "Deshacer", which sends `archived: false`). The menu also has "Subir", "Bajar", "Cambiar nombre" and "Agregar opciones dentro"; the handle takes the up and down keys, so a reorder needs no pointer drag. A move sends one `PATCH` with the new `position` for each row that moved (`reorder` in `lib/attributes.ts`, which reuses the positions the siblings hold). The cache is refreshed after a move whether or not every `PATCH` worked, so the tree never shows an order the server does not have. Archive asks first, with the template warning (`archiveWarning`), built from the cached active templates (a template uses the attribute if its body has the variable `client.<key>` or `appointment.<key>`, with or without a fallback; the check uses the same tokenizer as `ChipEditor`).
    *   `plantillas`: the list of templates (active first, then "Archivadas") and `TemplateForm` with the `ChipEditor` (item 70). Wide enough: the list (320 px) on the left and the editor card, with the preview under the text, on the right; with nothing chosen the right side says "Elige una plantilla". Narrow: the list is one screen, the editor the next, with a back button. The open template is the route `/ajustes/plantillas/[id]` (or `/nueva`), so Back, reload and deep links work. The two panes follow the width the page is given, not the window (a container query, `@3xl` = 48 rem), so inside the Ajustes layout they fall back to one screen at a time when the section is narrow. Leaving the form with unsaved changes (another template, "Nueva plantilla", back, any link) asks "¿Descartar los cambios?" first (`beforeNavigate`; `templateDirty` in `lib/templateEdit.ts`). The page has no shell of its own: the Ajustes layout frames it.
    *   `recordatorios`: rules built as sentences. Come-back: "Enviar [plantilla] [21] días después de [servicio], para agendar [servicio de regreso]" (the return service; it starts as the same service). Before: "Enviar [plantilla] [2] días antes de cada cita" (1 shows as "1 día antes", 0 as "el mismo día"). When she saves a new come-back rule or a come-back change that can add items, a `BottomSheet` first asks "¿Incluir recordatorios atrasados?" with "Solo de hoy en adelante" and "Incluir atrasadas", sent as `include_overdue`. Changing the return service asks the same question. Archived templates, services or return services show the warning from "Reminder rules" under the sentence. A rule is edited in a sheet (template, days, return service; its kind and service are fixed), archived or restored there, and archived rules are listed apart. The question is decided by `overdueChoice` (`lib/rules.ts`), which also asks when a restore or a new template follows an archived template; for a "before" rule it answers `include` (no question, `include_overdue: true`). With no active template yet, the screen shows an empty state that leads to `plantillas`. The empty state on "Hoy" and on "Para enviar" leads here.
    *   `estado`: the status screen: one `StatusRow` per line of `statusLines` (`lib/status.ts`: version, migration, timer heartbeat, last backup, free space, push, last daily push), the red ones in the error color (the backup line reads "Desconocido" with no status file, and otherwise the last copy time, with "Respaldo con problema" beside it while it is red), and the "Activar notificaciones" button when `push.ts` reports "needs activation". It reads `GET /status` when it opens and is not cached; offline it says why it cannot load. "Hoy"'s backup line leads here.
    *   `exportar`: the download button, an `<a href="/api/v1/export" download>` link (see CUJ 8). The service worker lets this navigation go to the network.

##### 78. `web/src/lib/**/*.test.ts` (Create)

*   **Type / Scope**: Vitest unit tests.
*   **Proposed Changes**:
    *   `client.ts`: 530 HTML without the header → Offline; a network error → Offline; a 500 `problem+json` with the header → error, not Offline; a 204 with the header → success; a 409 overlap → typed error; a different version header → prompt; a 200 with a body that fails its zod schema → `response_invalid` error, not Offline.
    *   `db.ts`: a cached snapshot row that fails its zod schema → `clearCache()` and a refresh; a draft that fails its schema → only that draft is deleted, and the pending confirmation stays.
    *   `shownDue` (the list of "Para enviar", Hoy, the sidebar and the badge): a cached "before" item is hidden offline once its `anchor_at` has passed; a logged item stays hidden after a refresh asked before its log `POST` answers with the old list, and after the store is reopened (a reload); a refresh that no longer lists a logged item drops its stored id.
    *   `search.ts`: "8888-7777" matches `50688887777`.
    *   Badge: after a refresh with 3 items the badge is 3; after the last log `POST` it is cleared.
    *   Pending confirmation: stored on "Enviar", shown again after a reload, cleared on "Sí", on "Todavía no", and when the item is gone from the due list.
    *   `search.ts`: "maria" finds "María"; "ana" finds "Ána López" and ranks it before "Mariana"; a letters-only query matches no phone; 3+ digits match a phone substring, 1–2 digits do not; `%` and `_` are plain characters; the top-20 cap.
    *   `chips.ts`: the `ChipEditor` serialization round trip (text, a variable, a fallback, accents, emoji, line breaks), the grammar edge cases of `parse.go` (curly quotes, a non-ASCII space, an unclosed fallback, a key starting with a digit, a variable inside the fallback of a token kept as text), `templateUses` (with and without a fallback, another namespace), `piecesFromDom`, `chipLabel`, `bodyProblems` and `chipKnown`; `templateEdit.ts`: `templateDirty`.
    *   `photos.ts` sizing math, and `toJpegs` (the full photo at 2048 px and the thumbnail at 400 px, JPEG at 0.8, and the decode timeout).
    *   `booking.ts`: a second tap adds a copy and `removeService` takes one; the end time is the sum of the durations (and wraps past midnight); `optionPath`; `returnTo` refuses a path that leaves the app; `formErrors`; `createBody` and `patchBody` (only what changed; `ends_at` only when she set it, also when the start moved; `service_ids` only when the list changed; a cleared attribute or note is `null`); `endShown`, `withSavedLines`, `restoreForm`, `statusActions`, `attrDisplay`. `money.ts`: the currency's decimals, parsing an amount with a comma or a point.
    *   `clients.ts`: `visibleClients` (the whole list while the box is empty, the search otherwise), `historySplit` (appointments outside the `/bootstrap` window go to "load more"), `sharingNames`, and the client form's checks and request bodies (`clientFormErrors`, `clientCreateBody`, `clientPatchBody`, `clientFormFrom`, `restoreClientForm`). `cachedLoad.ts`: `showThenFetch` shows the cached copy, then the server's, and keeps the copy when the server is off.

##### 79. `web/playwright.config.ts`, `web/tests/e2e/setup.spec.ts`, `booking.spec.ts`, `reminders.spec.ts`, `offline.spec.ts`, `export.spec.ts`, `ui-probe.spec.ts` (Create)

*   **Type / Scope**: End-to-end tests (WebKit, iPhone viewport).
*   **Proposed Changes**: Each spec automates one or more CUJs against the release binary + a test Postgres. There is no test clock and no test-only endpoint. Time-based journeys get their past from the data: the spec inserts rows with past dates through `seed.ts` (item 80), and the real clock then finds them due. Time-of-day rules (push after `push_hour`, nothing after 20:00) are covered only by the Go unit tests, which use a fake clock.
    *   **Workers:** `workers: 1` and `fullyParallel: false`, because every spec shares one database and `seed.ts` truncates tables.
    *   **Server:** `playwright.config.ts` `webServer` starts the built binary (`../bin/open-customer serve`) on `:8080` and waits for `/readyz`, with `PUBLIC_URL=http://localhost:8080`, `DB_*` split out of `E2E_DATABASE_URL` with `new URL()` (`DB_SSLMODE=disable`), `VAPID_SUBJECT=mailto:e2e@localhost` and a temporary `MEDIA_DIR` (each Playwright process removes the one it made on exit). One helper, `tests/e2e/env.ts` (`appEnv`), builds that env; it throws `E2E_DATABASE_URL is not set` when the variable is missing, and the setup spec's `setup-link` call uses it too, so both processes see the same database. The binary applies the migrations itself on start. The database is fresh, so the migration guard does not apply.
    *   **Device:** WebKit with Playwright's `iPhone 15` profile, `locale: 'es-CR'` and `timezoneId: 'America/Costa_Rica'`. The setup spec picks the same business time zone, so "today" is one day for the test, the browser and the server; the zone has no DST, so seeded times are `<date>T<hh:mm>:00-06:00`.
    *   **Login:** a `setup` project runs first. It gets the setup token by running `open-customer setup-link` against the same database and reading the printed link. It completes CUJ 1 (the login on a new device included) and saves the logged-in state (`storageState`, `tests/e2e/.auth/owner.json`, git-ignored). Every other spec is in the `cujs` project (or, for `ui-probe.spec.ts`, the two probe projects), depends on `setup` and reuses that state, so no spec needs a password hash outside Go.
    *   `setup.spec.ts`: CUJ 1 and 2 (without the real install and push). It is the `setup` project. It creates, in the UI, the data every later spec builds on (made up): the business "Estudio Prueba" (`USD`, country `CR`, the default color), the owner `duena`; the services Acrílico (90 min, 45.00), Relleno (60 min, 30.00) and Pedicure (60 min, 25.00); the client attribute "Bebida favorita" (text) and the appointment attribute "Forma" (select: Almendra, Cuadrada); the templates "Recordatorio de cita" and "Relleno", each with the "+ Bebida favorita" chip, checked against the live preview; a "before" rule (1 day, "Recordatorio de cita"); and the come-back rule on Acrílico (21 days, "Relleno", return service Relleno). Its last test opens the used setup link again and expects the login screen, not the wizard.
    *   `booking.spec.ts`: CUJ 3, including overlap and the photo step. The photo is `tests/e2e/fixtures/photo-gps.jpg`, a made-up 64 × 64 JPEG with an Exif GPS block (tag `0x8825`, latitude and longitude 0), sent through `setInputFiles`; the spec then fetches `GET /media/{id}` with the page's session and checks the stored file has no `Exif\0\0`. This is the only automated check of "no GPS on the server"; manual check 8 stays.
    *   `reminders.spec.ts`: CUJ 4 (the `wa.me` navigation is intercepted) and CUJ 5. For CUJ 5 it seeds Ana's visit ending 21 days ago, with `created_at` also in the past, so the item is due today and not late. It uses the 21-day come-back rule that `setup.spec.ts` creates in CUJ 2 (rules are not truncated, so a second copy would give two items); the rule exists before the seeded visit, and the visit's due day is today, so `include_overdue` does not matter. It checks that a later booking with "Relleno" hides the item until that booking is cancelled, that a future "Pedicure" booking turns the item into the booking prompt and "Agregar a la cita" adds "Relleno" to it, and that a visit marked "No vino" makes no item. CUJ 4 uses the "before" rule and template that `setup.spec.ts` creates in CUJ 2; `reminders.spec.ts` creates no rule of its own.
    *   **Shared steps:** `jscpd` scans `tests/` with threshold 0, so a helper two specs need (for example `fill`, which fills an `es.json` text's placeholders) lives in `tests/e2e/steps.ts`, not in both.
    *   `offline.spec.ts`: CUJ 6, run with `serviceWorkers: 'block'` so Playwright routing sees every request, using a route that returns a 530 HTML page. Whether Playwright's WebKit routing works while a service worker is active is not checked, so the cold-open-with-server-off case stays the manual check in the Verification Plan.
    *   `ui-probe.spec.ts`: the runtime sensors of the UI pipeline (`.claude/rules/ui.md` stage 7), in two projects that depend on `setup`: `probe-compact` (the `iPhone 15` profile with the whole 393 × 852 screen, as the installed app gets it) and `probe-wide` (Playwright's `Desktop Safari` at 1440 × 900). Its table has one entry per `+page.svelte` under `src/routes` (a route on disk with no entry, or an entry with no route, fails): the address, a text that shows once the screen has its data, the sheets it can open and its data feeds (API paths). Before each test it seeds a client with a 60-letter one-word name, a 30-minute appointment today and one tomorrow (due today through the "before" rule), a sent message of today's appointment that carries the name (`insertSentReminder`; Enviados and her Recordatorios draw it) and a photo of today's appointment, uploaded through `POST /media` with the booking fixture, so every list the empty tests empty has a row. Each screen, and each opened sheet, runs `@axe-core/playwright` with the tags `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa` and `wcag22aa` (the run throws when `target-size` is not in the results) and the layout probe in `probe.ts`: no sideways scroll (`scrollWidth > clientWidth` on the document), no control under 44 × 44 px (a field inside its label is measured by the label), no text field, select or text area under 16 px, no box with `overflow: hidden` or `clip` whose content is cut (an ellipsis on a `nowrap` line is allowed), and no text wider than the block box it sits in (each text node's rectangle, so text in an inline element counts); with a sheet open, every other child of `<body>` must be inert or `aria-hidden`. Each feed is answered, through `page.route`, with its lists emptied (the screen shows its empty text), with a 500 `internal` problem carrying the server's `X-App-Version` (an alert with the `error_internal` text), and not at all (a `role="status"` line with one of the `*_loading` texts that is still there once every other API call has answered); axe and the layout probe also run on the empty and error states. Known findings are in `ui-probe.allow.ts`, each with an id (`UIP-…`) and a one-line `why`, scoped to a route, a state (`base`, a sheet, or `<feed> error|busy|empty`) and optionally a width, an exact target (the element's tag and whole text, or axe's selector) or a detail substring; an entry that matches nothing on its screen fails, so the list only shrinks, and an entry whose route or state the table does not probe fails too. An entry for every route (`route: '*'`, no `state`: the missing `<title>` and the accent contrast) names a `witness` route where it must still match. Service workers are blocked, as in `offline.spec.ts`, so routing sees every request.
    *   `export.spec.ts`: CUJ 8. It reads the downloaded ZIP with `tests/e2e/zip.ts` (`readZip`, no zip dependency): it walks the local file headers and inflates each entry with Node's `zlib.inflateRawSync`, and throws on a compression method other than stored or deflate, or on a stored entry with a data descriptor (it cannot be sized). It checks the four files of "Export", the "Bebida favorita" column of `clientas.csv`, and the attributes, templates and rules in `datos.json`.

##### 80. `web/tests/e2e/seed.ts` (Create)

*   **Type / Scope**: E2E test data.
*   **Proposed Changes**:
    *   Connects to the test Postgres with the `pg` package and `E2E_DATABASE_URL`, outside the app.
    *   Before each spec except `setup.spec.ts`, truncates `clients`, `appointments`, `appointment_services`, `media` and `reminder_log` (`TRUNCATE … CASCADE`). The user, sessions, settings, attributes, services, templates, rules, server keys and `goose_db_version` stay, so the saved login keeps working. A spec that needs more rules or services creates them through the API.
    *   Inserts clients (`insertClient`: name, an E.164 phone stored with its digits, attributes by label) and appointments (`insertAppointment`: client, service names, `starts_at`, optional `created_at`, status and length in minutes) and sent messages (`insertSentReminder`: an appointment, the text and the time, logged under the active "before" rule) with SQL, so a spec can place a visit in the past; the API always sets `created_at` to now. Like the API, it copies each service's name, price and duration into `appointment_services`, ends the appointment after the sum of the durations (or the given length) and sets `total_cents` to the sum of the prices. `businessDate(n)` and `at(n, "hh:mm")` give a business day and a time `n` days from today in the business zone. One `pg` pool per Playwright worker; each spec that seeds closes it in `afterAll`.
    *   Used only by the specs; nothing in the release binary changes.

#### Deployment (`deploy/`)

##### 81. `deploy/compose.yaml` (Create)

*   **Type / Scope**: Compose stack.
*   **Proposed Changes**:
    *   The four services in "Deployment", with no `ports:`.
    *   Named volumes `pgdata`, `media` and `backup-status`.
    *   `${BACKUP_TARGET}:/target` on `backup` in the long bind syntax with `bind.propagation: rslave`, so a removable disk mounted or unmounted on the host after the container started is seen inside it (`BACKUP_TARGET` may be any host folder). `APP_VERSION` is passed to it for snapshot tags.
    *   A `restore` service under `profiles: ["restore"]`: the backup image, `media` read-write, `backup-status` read-write (the restore takes the backup lock and unpacks into `/backup-status/work`), `/target` read-only. It never starts with `up`.
    *   Environment as in "Configuration (`.env`)": the optional variables with `${VAR:-default}`, the required ones with `${VAR:?<VAR> is required in .env}`. The app's own optional variables (`TRUSTED_PROXY_CIDRS`, `MEDIA_DIR`, `BACKUP_STATUS_FILE`, `ALLOW_MIGRATE_WITHOUT_BACKUP`, `LOG_LEVEL`) are passed as `${VAR:-}`: empty means the app's default, so each default has one owner.
    *   `depends_on` with healthchecks (`app` and `backup` on a healthy `db`, `cloudflared` on a healthy `app`).
    *   `restart: unless-stopped` on the four long-running services; `restore` has none (it only runs with `run --rm`).
    *   The `app` healthcheck in exec form: `test: ["CMD", "/usr/local/bin/open-customer", "healthcheck"]`.
    *   `stop_signal: SIGTERM` on `backup` and `restore` (the `postgres` base image sets `SIGINT`).
    *   `pgdata` mounted at `/var/lib/postgresql` on `db` (see "Deployment").

##### 82. `deploy/.env.example` (Create)

*   **Type / Scope**: Config template.
*   **Proposed Changes**: Every variable from the Configuration table except `E2E_DATABASE_URL`, with comments. The `RESTIC_PASSWORD` comment says: "store a copy outside this PC".

##### 83. `deploy/README.md` (Create)

*   **Type / Scope**: Install and operations guide.
*   **Proposed Changes**: Sections:
    1.  Requirements.
    2.  Create the tunnel and the public hostname.
    3.  Cloudflare settings (the list in "Deployment").
    4.  Prepare the backup target: pick any host folder for `BACKUP_TARGET` and `touch .open-customer-backup-target` in it once. Only for a removable disk, also add an `/etc/fstab` entry with `nofail` at that path (so the PC boots without the disk). Say plainly that a folder on the same disk does not survive a dead disk, and suggest a second disk later.
    5.  `.env`.
    6.  First start + the setup link.
    7.  The in-person handover checklist (wizard → install → login → allow notifications → give her a sealed paper copy of `RESTIC_PASSWORD` and say what it is for).
    8.  The restore drill and the Ley 8968 check (both required before real data).
    9.  Upgrade (outside her working hours; see "Upgrades and the migration guard").
    10. Restore.
    11. Password reset.
    12. The security note about Cloudflare seeing decrypted traffic.
    13. One sentence: without a tunnel, put any reverse proxy in front of `app:8080`.

##### 84. `deploy/backup/Dockerfile` (Create)

*   **Type / Scope**: Backup image.
*   **Proposed Changes**: `FROM postgres:18.<minor>-alpine` (the pin in "Pins"), `apk add bash restic jq` (checked at P7: the image's busybox already has `flock` with `-n`, so no other package), copies `backup.sh`, creates `/media` empty and owned by `65532:65532` (see "Volume ownership"; Alpine ships `/media/cdrom`, `floppy` and `usb`, which Docker would copy into an empty volume, so the folder is removed and made again), `ENTRYPOINT ["/backup.sh"]` and `CMD []` (the base image's `CMD ["postgres"]` would otherwise become the mode).

##### 85. `deploy/backup/backup.sh` (Create)

*   **Type / Scope**: Backup loop.
*   **Proposed Changes**: Implements "Backups" (modes, steps, restic flags, `status.json` and its outcome table) and the `restore` steps in "Upgrades and the migration guard". Only the details that those sections do not give are listed here:
    *   Arguments: none (the hourly loop), `once [--tag pre-upgrade]`, `snapshot-info <snapshot-id|latest>` and `restore <snapshot-id>`.
    *   The lock: `flock /backup-status/backup.lock`. `once` and `restore` wait for it.
    *   `snapshot-info` prints `{"id", "time", "app"}` from `restic --no-lock snapshots --json`.
    *   `restore` unpacks the snapshot into `/backup-status/work/restore` (on the `backup-status` volume, not the container layer) and uses `pg_restore` without `--clean`, because the database was just recreated empty. Without `--no-owner` either, so it relies on `PGUSER` owning the objects in the dump: changing `DB_USER` between a backup and its restore needs care.
    *   The new-volumes guard of step 1 in "Backups": the state is the file `/backup-status/restore-first`; the snapshot count comes from `restic --no-lock list snapshots` (snapshot files, so a corrupt one still counts). Tests: `fresh volumes with an existing repository take no snapshot` (also: it holds on later runs, across a marker-missing run, and in the loop), `restore lets backups resume`, `a failed restore blocks the next backup` (a failing `pg_restore`), `an empty status file allows a fresh start`, `the guard fails closed when the snapshots cannot be counted` (a corrupt snapshot file; a failing listing).
    *   Runs `restic init` on the first run if the repository is missing. "Missing" means `restic cat config` fails and `/target/restic` does not exist; if the folder exists but `cat config` fails, it reports an error instead of creating a new repository.
    *   `#!/usr/bin/env bash` (the image adds `bash`). A `trap` on `TERM` and `INT` lets the running step finish or writes "interrupted" to `status.json`, then exits; bash as PID 1 ignores signals it does not trap.
    *   The hourly loop takes the lock with `flock -n`: if a `once` run or a restore holds it, that hour is skipped. Busybox `flock` exits 1 both on contention and on a real error; only a real error prints to stderr, so the loop records that one in `status.json` ("lock: …") instead of skipping silently. A lock file that cannot be opened, or a failed blocking `flock` in `once` or `restore`, exits 1 with one line naming the lock file; nothing runs without the lock.
    *   Time math (the 24 h check) uses `jq` (`now`, `fromdateiso8601`) on the `status.json` times, not busybox `date -d`.
    *   `set -uo pipefail`, without `-e`: each step's exit code is checked by hand, so a failure reaches `status.json` (see "Backups"). Each step logs one line to stdout.
    *   The loop and `once` share one function (`run_backup`), which always writes `status.json` before it returns; the step 2 skip happens only in the loop. A wrong mode or argument exits 2 with one line; a missing variable exits 2 naming it.
    *   `status.json` is written mode `0644`, because `app` reads it as user 65532 and the backup container runs as root. A failed step also sets `last_error_at` (see the outcome table in "Backups").
    *   One test-only knob, default off: `BACKUP_INTERVAL_SECONDS` (default 3600), the wait between two loop checks, so `make test-deploy` does not wait an hour. It is documented at the top of the script.

##### 86. `deploy/scripts/backup-now.sh`, `deploy/scripts/upgrade.sh`, `deploy/scripts/restore.sh`, `deploy/scripts/lib.sh` (Create)

*   **Type / Scope**: Operator scripts.
*   **Proposed Changes**: As in "Backups" and "Upgrades and the migration guard".
    *   `backup-now.sh [--tag pre-upgrade]`: see the notes in "Backups".
    *   `upgrade.sh <version>`: `set -euo pipefail`; the five steps in "Upgrades and the migration guard". The version must match `^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$`.
    *   `restore.sh <snapshot|latest>`: the five steps in "Upgrades and the migration guard". The argument is `latest` or a hex snapshot ID. It asks `Restore snapshot <id> from <time> (app <version>)? [y/N]` and reads one line; anything but `y` exits 1 with nothing stopped, and so does a snapshot without an `app=` tag. After the stop it stops on any failure and leaves `app` and `backup` stopped, with a message naming them.
    *   Each script does `cd "$(dirname "$0")/.."`, so it works from any folder and `.env` is `deploy/.env`. `upgrade.sh` and `restore.sh` rewrite only the `APP_VERSION` line of `.env`, in place.
    *   `lib.sh` (sourced by `upgrade.sh` and `restore.sh`, never run): the `.env` checks (it exists, exactly one `APP_VERSION=` line) and `set_app_version`, which writes a temp copy `.env.XXXXXX` (mode 0600) and copies it over `.env`, and returns non-zero when `.env` was not changed (a failed write included), so the caller stops before `up -d`. A temp copy without the new `APP_VERSION=` line (an empty one included) never replaces `.env` (test `an empty temp copy never replaces .env`). An `EXIT` trap removes the temp copy, also on INT or TERM; `.gitignore` covers `deploy/.env.*` (except `.env.example`) for a copy a SIGKILL leaves.
    *   Tests: `deploy/tests/scripts_test.sh` (item 2, `make test-deploy`).

#### Files deleted from the repo root

##### 87. `PRD — Sistema de citas para emprendimientos.md`, `SAD — Sistema de citas para emprendimientos.md`, `concepts_brief.md`, `goldfish_brainstorm_findings.md`, `design_draft.md`, and any review reports still in the root (Delete)

*   **Type / Scope**: Moved documents.
*   **Proposed Changes**: Replaced by items 12–14. Review reports still in the root are moved to the next free `review-release-1/round-<N>/` by item 14. `design_draft.md` is an older copy of this document and is not kept. Removing the root copies avoids two versions of the same doc. After this item, the repo root holds no `.md` file except `README.md` and the harness files from item 88 (`CLAUDE.md`, `TASKS.md`, `technical-taste.md`).

#### Agent harness

##### 88. `CLAUDE.md`, `TASKS.md`, `technical-taste.md`, `.claude/rules/` (`testing.md`, `maintainability.md`, `troubleshooting.md`, `boundaries.md`, `api.md`, `ui.md`), `.claude/agents/code-reviewer.md`, `.claude/hooks/check-edited.sh`, `.claude/hooks/run-tests-on-stop.sh` (Create, before P0)

*   **Type / Scope**: Rules, checks and setup tasks for coding agents.
*   **Proposed Changes**: Written by the `dev-project-bootstrap` skill before any code.
    *   `CLAUDE.md`: commands, the gate, what never to commit, and the load-bearing constraints (contract first, module rules, explicit transactions, injected time, Spanish UI text in `es.json`). No overview: this document is the architecture source.
    *   `.claude/rules/*.md`: path-scoped rules, each naming the check that enforces it or saying "Prose only". `boundaries.md` lists every outside input with its parse site.
    *   `.claude/agents/code-reviewer.md`: a fresh reviewer that re-runs the gate and breaks each guard once to see it go red.
    *   Hooks: `check-edited.sh` (gofmt + `go vet`, eslint + `pnpm check`, or shellcheck, on the edited file) and `run-tests-on-stop.sh` (`make test`). Both switch to the pinned Node with fnm when fnm is installed. They are registered in `.claude/settings.json` (committed) by `TASKS.md` T2, after the P0 gate is green.
    *   Guard tests (`TASKS.md` T6), run by `make test`: Go in `internal/guards` (a test-only package: `agent-context` with doc-paths, `suppression-budget` for `//nolint`, `catch-blocks`, `test-hygiene`, `forbid-only`; the security pipeline adds `threat-model` and `security-allow-list`, see `docs/designs/devsecops.md`), web in `web/tests/guards` (`layers`, `catch-blocks`, `test-hygiene`, `forbid-only`, `raw-colors` (P5: no hex, `rgb(`, `hsl(` or `oklch(` literal in `web/src` outside `app.css`, except the `DEFAULT_ACCENT` line in `lib/color.ts` and the `theme-color` meta tag in `app.html`; no Tailwind arbitrary color or pixel value in a `.svelte` file), `suppression-budget` for `eslint-suppressions.json` and inline `eslint-disable` / `@ts-*` / `svelte-ignore` comments, `design-tokens` (every `design/soft-glass.pen` variable is a CSS token in `app.css` with the same value, read with postcss; name exceptions in `design/design-map.json`; runtime-derived tokens on an allow-list with a reason) `design-parity` (every reusable `.pen` component maps to one primitive in `lib/components/ui/` and back; `X/Y` is primitive `X`, variant `y`, exceptions in the same map) and `ui-lint` (runs the real ESLint config on in-memory probes: each way around "only primitives style" is an error, legitimate layout is not)). Each was seen red once by breaking what it protects.
    *   `README.md` "Release checklist" (`TASKS.md` T9): duplicates, dead code, slow tests, mutation testing (gremlins, StrykerJS), run before each release tag.
    *   `TASKS.md`: T1 is P0. T2–T10 add hooks, CI secret scan and audit, stricter lint (`exhaustive`, `wrapcheck`, `nilerr`, `nolintlint`, type-aware typescript-eslint), test-lint plugins, guard tests (Go in `internal/guards`, web in `web/tests/guards`), dead-code and duplication checks, and the mutation-testing release checklist (gremlins, StrykerJS).
    *   `technical-taste.md`: the engineering principles and the sensor per row for this stack. The owner sections start empty.
    *   The 90% `make cover` floor stays (owner decision); mutation testing is added in the clean-up pass, not as a gate.

### Verification Plan

*   **Compilation/Build**: `make build` (web build + `CGO_ENABLED=0` Go build). `make image` builds the app image. The backup image is built by `make test-deploy` and in the release workflow.
*   **Unit Tests**: `make test` (`go test $(GO_PKGS)` + `pnpm test`).
*   **Database Tests**: `make test-db` (`go test -tags db $(GO_PKGS)` with testcontainers Postgres 18).
*   **Deploy Tests**: `make test-deploy` (`deploy/tests/backup_test.sh` in the backup image against a throwaway Postgres, then `deploy/tests/scripts_test.sh` on the host; item 2).
*   **Static Analysis & Linting**: `make lint` (golangci-lint with the `depguard` module rules, the darwin and windows `go build`, `deadcode`, shellcheck, `pnpm check` (svelte-check), `pnpm lint` (eslint, prettier, knip, jscpd)) and `make vuln` (govulncheck). CI adds gitleaks and `pnpm audit --audit-level high`.
*   **Clean checkout:** CI runs on a fresh clone, and `make build` is its last step before e2e (item 8), so `make lint`, `make test`, `make vuln` and `make test-db` passing first shows the targets compile with only the stub present.
*   **Generated Code**: `make check-generated` must show no diff.
*   **Test Coverage Requirement**: 90%+ statement coverage (the `go tool cover -func` total) on `internal/reminders`, `internal/templates`, `internal/attributes`, `internal/appointments` and `internal/auth`, enforced in CI by `make cover`. No target for handlers or UI. Their behavior is covered by e2e.
*   **E2E/Integration**: `make e2e`. Playwright WebKit runs CUJ 1–6 and 8 against the built binary, and the UI probe (axe and the layout, empty, error and busy checks on every route at 393 × 852 and 1440 × 900, item 79).
*   **Manual checks before handover** (recorded in `deploy/README.md` as a checklist):
    1.  On her iPhone through the tunnel: the `wa.me` / `whatsapp://` test from Open Questions.
    2.  On her iPhone through the tunnel: install from Safari, log in, allow notifications, receive a test push.
    3.  "Hoy" opens in under 1 s over LTE (Goal 6). Method: close the app from the app switcher, start a screen recording, tap the Home Screen icon, and count the time from the tap to the first appointment row painted, using the recording's frames. Pass: 3 of 3 runs under 1 s.
    4.  With the server stopped: the app opens from the cache with the banner (CUJ 6).
    5.  The restore drill: back up, wipe the volumes, restore, confirm data + photos (CUJ 7), then upload one new photo to confirm `app` can still write to the restored media volume.
    6.  Remove the marker file from the target (or unplug the target, if it is a removable disk): the status screen shows the red backup line by the next hourly check (at most 60 min), and nothing was written to the target folder.
    7.  Goal 2: time CUJ 3 steps 1–4 with a stopwatch on her iPhone, three times with an existing client. Pass: 3 of 3 under 30 s.
    8.  CUJ 3 step 5 on her iPhone: attach a photo from the camera and one from the library. Both upload, show on the appointment and in the client's gallery, and the stored file has no EXIF GPS data.
    9.  "¿Lo enviaste?" after iOS closes the app: tap "Enviar", close the PWA from the app switcher while in WhatsApp, reopen it. The sheet shows for that client.
    10. Goal 7: `grep -rnE 'Costa|\bCRC\b|America/Costa_Rica|\+506|[Uu]ña|\b[Nn]ails?\b' --include='*.go' --include='*.sql' internal` (this covers the migrations in `internal/db/migrations`) returns nothing outside `internal/locale/es.go` and `_test.go` files. Currencies come from `golang.org/x/text/currency` and phone rules from `nyaruka/phonenumbers`, both outside `internal`, so no country list is written by hand. SQL comments in migrations use no country examples.
*   **Goal checks after handover**:
    *   Goal 1: she does the setup herself while the author watches and times it.
    *   Goal 3: the "missed reminder" bug log during the 4-week gate (see Goal 3).
    *   Goal 4: the 4-week daily-use gate (days with the PC off do not count; see Goal 4).

--------------------------------------------------------------------------------

*Drafted with EGM*
