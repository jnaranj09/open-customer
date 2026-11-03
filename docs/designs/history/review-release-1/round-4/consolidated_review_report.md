# Consolidated Review Report: release-1.md (round 4)

Target: `docs/designs/release-1.md` (2308 lines), after the round-3 fixes (R1–R20).
Mode: Consolidated report, all five stages, no extra context. Every reviewer ran in a fresh context.

## Stage reports

| Stage | Report | Result |
| --- | --- | --- |
| 5 Initial Zoom Out | `zoom_out_report.md` | 0 / 5 / 5 |
| 6 Comprehension | `comprehension_report.md` | 0 / 2 / 6 |
| 7 Critic (design, structural, style) | `critic_report.md` | 0 / 9 / 26 |
| 8 Intermediate Zoom Out | `intermediate_zoom_out_report.md` | 0 / 8 / 13 |
| 9 Implementation Readiness (IR, CD, style) | `implementation_readiness_report.md` | 0 / 21 / 48 |

Counts are Blocker / Major / Nit, before merging duplicates.

## Verdict

**No Blockers in any stage, for the first time. Implementation can start with P0–P2.**
- The readiness reviewer says an engineer can build P0–P2 from the plan without making design calls. Every later Major must be fixed before the phase that builds it.
- The comprehension reviewer could paraphrase the whole doc. The technical-accuracy reviewer confirmed:
  - every item number and cross-reference;
  - the phase lists;
  - the link-rewrite table;
  - the disk layout.
- The round-3 edits introduced no new Blockers. One technical gap came from them, and one older gap was found:
  - **jq's date parsing** needs one exact time format (W8). Round 3 introduced the jq time math.
  - **`/index.html` and the redirect:** with `http.FileServer` it is served as a redirect, which breaks the offline cold open (W5). This gap is older than round 3: the precache of `/index.html` dates from round 1.
- Reviewers again proposed reopening owner decisions. Stage 8 says to turn all of them down:
  - read-only offline (design-4-1, design-4-5);
  - option depth (Stage 5 #6);
  - the overdue prompt (Stage 5 #2).

## Needs the owner

- **W1: Drop the oapi-codegen `strict-server` mode?** Stages 5 and 8 recommend it.
  - Strict mode causes most of the HTTP workarounds:
    - headers declared in the spec;
    - the `X-App-Version` caveat;
    - the P1 multipart/binary spike, with a hand-written fallback;
    - two error hooks.
  - With the plain `std-http-server`, handlers get `(w, r, params)` and all of those go away.
  - Cost: Go response bodies are not type-checked at compile time. Tests cover that.
  - The TypeScript client is unchanged.
- **W2: Photos.** No goal or CUJ checks them.
  - (a) Add one CUJ 3 step ("attach a photo of the finished set") plus a manual iPhone check.
  - (b) Defer photos until the interview with her shows a need. This removes the media table, the media module, three endpoints, `photos.ts` and the media part of backup and restore.

## Action items (the agent's recommendation, to apply)

### 🟡 Majors

**Phone flow (Goal 3)**
- **W3: The "¿Lo enviaste?" confirmation survives iOS closing the app.**
  - Store the pending item in Dexie `meta`, show the sheet on any app open, and clear it on Sí, Todavía no or logout.
  - Add a manual check, not an e2e test.
  - Sources: design-4-2, IR-M7, Stage 8 #2.
- **W4: Keep the badge in step.** Call `setAppBadge(due.length)` or `clearAppBadge()` after each refresh and each log POST.
  - Sources: design-4-3, IR-N13, Stage 8 #3.
- **W5: Serve `/index.html` with a 200, not a redirect** (no `http.FileServer` for that path), plus a test.
  - Sources: IR-M3, CD-M5.
- **W6: Downtime wording, not features.**
  - Goals 3 and 4: days with the PC off don't count as misses and don't break the 4-week run.
  - Accepted risks:
    - a "before" reminder can be missed if the PC stays off until the appointment starts;
    - a silent quiet day can also mean the PC is off.
  - No zero-count push.
  - Sources: design-4-1, 4-4, 4-5 (their "or" options), Stage 8 #4.

**Backups and restore (data safety)**
- **W7: restic.**
  - Global `--no-lock` for `snapshot-info` and `restore`, because `/target` is read-only.
  - A fixed `hostname:` on `backup` and `restore`, plus `forget --group-by paths`, so retention spans container restarts.
  - Sources: IR-M1, CD-M1, CD-M2.
- **W8: `status.json` and the backup script.**
  - Every time is UTC `%Y-%m-%dT%H:%M:%SZ`, the one format jq `fromdateiso8601` accepts.
  - Step failures are caught and written to `status.json`; loop mode never exits.
  - After a restore, `chown -R 65532:65532 /media`, and "upload a photo after the restore" joins the drill.
  - The migration guard is written as a numbered rule (all three conditions must hold).
  - Sources: CD-M3, CD-M4, IR-M2, ST-M2.

**Plan gaps (each before its phase)**
- **W9: Data rules.**
  - `Appointment.client_name`.
  - Photo ownership: `client_id` comes from the appointment; a mismatch → 422.
  - `Validate` rules for unknown keys, wrong-scope keys, archived attributes, archived options and `""`.
  - Username: lowercase and trim, `[a-z0-9._-]` with 3–32 characters, plus `autocapitalize="off"`.
  - Sources: IR-M4, M5, M6, M9, design-n1.
- **W10: Screens.**
  - The Ajustes root gets "Cambiar contraseña" (at any time) and "Cerrar sesión".
  - The wizard's time zone, currency and country lists come from `GET /setup/options`, built from the same Go sources, with labels from `Intl.DisplayNames('es')`.
  - Sources: IR-M11, IR-M8.
- **W11: Tooling.**
  - Playwright `workers: 1`.
  - Dependabot ignores `postgres`, `node` and `golang` (bumped by hand together), and the `/deploy` entries are added in P7.
  - `generate` and `check-generated` start in P1.
  - The Postgres pin: all five places listed, plus a CI grep step.
  - The "clean checkout" order fixed.
  - Sources: IR-M10, CD-M6, CD-M7, IR-N1, ST-M1, ST-M3, CD-N7, CD-N8.

**Doc**
- **W12: Owner vs author.** Delete the "round-3 review" sentence and "the author approved"; line 1080 says "she asked for".
  - Sources: Stage 6 #1–2, style-4-2, Stage 8 #7.
- **W13: Remove the drift-prone repeats.** One owner per topic, with a pointer from the other place (six topics).
  - Sources: Stage 5 #3, Stage 8 #8.
- **W14: Labels for alternatives.**
  - M1 and H4 get "Author preference:".
  - A5, I1 and K2 use "Outcome:".
  - Sources: structural-4-2, design-n4, style-n4, style-n5.
- **W15: The naming rule** (five word edits).
  - Source: style-4-1.

### 🟢 Nits (apply the cheap ones)

- G4: drop "avisa a <owner>" and `backup_contact_name`.
- G5: lint only the coarse depguard rules.
- G6: `include_overdue` is optional, defaulting to "Solo de hoy en adelante".
- G7: Bot Fight Mode off.
- G8: rollback data-loss sentence.
- G10: the 0-day item before the push hour.
- G2: merge items 11, 14 and 87.
- G12: the mechanical list.
- IR-N2 to N17 and CD-N1 to N16 where they are one line each:
  - `git status --porcelain`;
  - the dev proxy paths;
  - 422 for a wrong current password;
  - `%20` in `wa.me`;
  - `IsDone(status, endsAt, now)`;
  - token encodings;
  - CSV values;
  - `position`;
  - `GenerateKey`;
  - inclusive `to`;
  - archived-reference checks;
  - offline layout redirects;
  - `svelte-kit sync`;
  - signature mismatches;
  - `country_code` passed to `clients`;
  - the CUJ 5 seed;
  - the `DB_*` split;
  - the `\bnails?\b` grep;
  - the goose tool line;
  - the restic/jq pin row.
- Style Nits (ST-N, style-n) where they delete text.

### Do not do (Stage 8)

- Drop the overdue prompt (it reopens R17).
- Cap option depth (it reopens D5.1).
- Offline "Enviar", a cached tomorrow list, or offline "+" with a draft list (they reopen D1).
- An always-on daily push.
- Deferring export paging or `client-errors`.
- A VPS alternative.
- "Mitigation:" leads.
- Section reordering.

## Contested

- **structural-4-1** (links break after P7): item 14 already rewrites every listed link, with a grep check. The technical-accuracy reviewer confirmed the table is complete. Merging items 11, 14 and 87 (G2) makes this visible.

## Traceability matrix

| Raw finding | → Item |
| --- | --- |
| Stage 5 #1, Stage 8 #5 | W1 |
| Stage 5 #4, Stage 8 #6 | W2 |
| design-4-2, IR-M7, Stage 8 #2 | W3 |
| design-4-3, IR-N13, Stage 8 #3 | W4 |
| IR-M3, CD-M5 | W5 |
| design-4-1, 4-4, 4-5, Stage 8 #1, #4 | W6 (and "Do not do") |
| IR-M1, CD-M1, CD-M2 | W7 |
| CD-M3, CD-M4, IR-M2, ST-M2 | W8 |
| IR-M4, M5, M6, M9, design-n1 | W9 |
| IR-M8, IR-M11 | W10 |
| IR-M10, CD-M6, CD-M7, IR-N1, ST-M1, ST-M3, CD-N7, CD-N8 | W11 |
| Stage 6 #1–3, style-4-2, style-n11, Stage 8 #7 | W12 |
| Stage 5 #3, Stage 8 #8 | W13 |
| structural-4-2, design-n4, style-n4, style-n5 | W14 |
| style-4-1, Stage 8 G1 | W15 |
| Stage 5 #2, #6, #7, #10, design-n5 | Do not do |
| structural-4-1 | Contested |
| All other -n / N findings | Nits |

---

## Resolution (2026-11-03)

The owner decided W1 (drop `strict-server`) and W2 (a: keep photos, add a CUJ 3 step and a manual check). The agent applied every other item with its recommendations, and most Nits. The edits are in `docs/designs/release-1.md`. The findings above refer to the document **before** these edits; their line numbers no longer match.

| ID | Outcome |
| --- | --- |
| W1 | `generate` is `std-http-server` + models, no `strict-server`. Removed: the P1 spike and its fallback, the headers declared in the spec, the `X-App-Version` caveat, the strict error hooks. Handlers set headers, read `r.MultipartReader()` and write binary bodies to `w`; `decodeJSON` / `writeJSON` helpers; `gen.ServerInterface`. Handler tests now check each endpoint's status and body against item 15. |
| W2 | CUJ 3 step 5 (photo of the finished set after the appointment); the 30 s check covers steps 1–4. "Agregar foto" on the appointment screen. e2e uploads a fixture JPEG. Manual iPhone check (camera, library, no GPS). |
| W3 | Pending confirmation in Dexie `meta`; the sheet shows on any app open; cleared on Sí, Todavía no, logout, or when the item is gone. Vitest case and a manual check. |
| W4 | `setAppBadge` / `clearAppBadge` after each refresh and each log `POST`; Vitest case. |
| W5 | `/index.html` and the fallback served with `http.ServeContent` (200); handler test. |
| W6 | Goal 3 and Goal 4 wording; two accepted-risk lines (a missed "before" item, quiet day vs PC off). No zero-count push. |
| W7 | `--no-lock` on `snapshot-info` and `restore`; `restic backup --host open-customer-backup` (an explicit flag instead of a Compose `hostname:`); `forget --group-by paths`. |
| W8 | `status.json` times are UTC `YYYY-MM-DDTHH:MM:SSZ`; a failed step writes `status.json` and loop mode never exits (`set -uo pipefail`, no `-e`); `chown -R 65532:65532 /media` after a restore, and the drill uploads a photo; the guard is a numbered list; Go parse tests. |
| W9 | `Appointment.client_name`; photo ownership (422 on a mismatch); `Validate` rejections and `""`; username rules and input attributes. |
| W10 | Ajustes root list with "Cambiar contraseña" and "Cerrar sesión". Changed from the proposal: Go has no list of time zones or currencies to send, so the wizard takes those from `Intl.supportedValuesOf` (iOS 15.4+, not checked on her phone, with a text-box fallback); `GET /setup/options` returns only the countries (`phonenumbers.GetSupportedRegions()`). |
| W11 | Playwright `workers: 1`; Dependabot ignores `postgres`, `node`, `golang`, and `/deploy` entries start in P7; `generate` / `check-generated` start in P1; `PG_IMAGE` as the source, plus `make check-pins` from P0; CI order puts `make build` last before e2e. |
| W12 | "Round-3 review" and "the author approved" sentences deleted; I1 says "she asked for". |
| W13 | Applied to the timer (item 58), the cookie (items 34, 39), backups (item 85) and `backup-now.sh` (item 86). Not changed: PGDATA and volume ownership (items 81 and 84 already point to "Deployment") and offline detection (the section already points to item 66). |
| W14 | M1 and H4: "Author preference". A5, I1, K2: "Outcome". |
| W15 | "before the appointment", "refill", "Refills" and "the nudge" replaced; Terms define "come-back" with relleno only as the nail example. |
| Nits applied | G4 ("avisa a <owner>" and `backup_contact_name` removed); G5 (coarse depguard only); G6 (`include_overdue` optional, default false; no conditional 422); G7 (Bot Fight Mode off); G8 (rollback sentence); G10 (0-day item note and test); G12 items: FR/NFR Terms line, "PRD FR-R7", "largest come-back `offset_days`", Pins row, `text` fence, blank lines before lists, Cloudflare stated once, icon timing once, the K1 pointer, calendar analogy and filler sentence deleted, G4 points to the size estimate, `docker compose logs app`, the PG 18 release date. IR and CD one-liners: `git status --porcelain`, dev proxy paths, 422 for a wrong current password, `%20`, `IsDone(status, endsAt, now)`, token encodings, CSV values, `position`, `GenerateKey` characters, inclusive `to`, archived-reference checks, offline layout and `/setup` without a token, `svelte-kit sync`, signature fixes, `region` passed to `clients`, the 21-day CUJ 5 seed, `DB_*` split, `\b[Nn]ails?\b`, no goose `tool` line, the restic/jq Pins row, server key and branding file names, `SampleData(locale, countryCode)`, preview in the P4 spec, due-list order. Style: ST-N1, N2, N3, N5, N6, N9, N10, N11, N13. |
| Nits not applied | G2 (merging items 11, 14 and 87 would renumber the plan; item 11 now points to items 14 and 87 instead); the J3/J4 move; CD-N1, CD-N3, CD-N9, CD-N16 (not confirmed; still checked at P0/P7); ST-N4 partly (the `dev` env moved below the table, the cell kept short); ST-N7 ("she" in plan items); ST-N12 (the Dependabot outcome is now written into item 10); ST-N14, ST-N15; structural-n1, n4, n6, n8, n9 and the other G13 items Stage 8 said to skip. |
| Do not do | Not applied, as Stage 8 advised: dropping the overdue prompt, a depth cap, offline "Enviar" / cached tomorrow list / offline "+", an always-on push, a VPS alternative, "Mitigation:" leads, section reordering. |
| Contested | structural-4-1: item 11 now points to item 14, which rewrites the links. |
