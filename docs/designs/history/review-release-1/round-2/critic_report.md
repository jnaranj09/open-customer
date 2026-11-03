# Critic Report: Stage 7 (round 2)

Target: `docs/designs/release-1.md`, FeatureRange lines 1–1113.

Reviewers (fresh contexts, direct dispatch), all completed:
- `egm-design` (feature-spec-standard)
- `egm-structural` (feature-spec-standard)
- `egm-style` (no standard)

Verdict: no Blockers. 16 Major, 37 Nit after merging duplicates. Finding IDs use the form `<reviewer>-2-<n>`.

---

## 🟡 Major

### Reminder correctness (Goal 3)

**design-2-1: editing a past appointment brings back a handled come-back.**
- Scope: lines 409, 479, 609–612.
- Problem:
  - The `come_back` identity uses `anchor_at = ends_at`.
  - Adding a service, or fixing `starts_at`, on a visit whose come-back was already sent or skipped moves `ends_at`.
  - The log row then no longer matches, so the item shows up again.
  - The same happens to a sent "before" item when `starts_at` changes by a few minutes.
- Fix:
  - key `come_back` on `(rule_id, appointment_id)`;
  - or list which edits re-arm an item, and add test rows for them.

**design-2-2: a "2 h before" rule acts as a whole-day rule.**
- Scope: lines 268–280, 472.
- Problem:
  - Due = the local day of `starts_at - offset_hours`, shown from the start of that day.
  - So a 2 h rule for 17:00 shows at 00:00 and is counted in the morning push.
  - `offset_hours` promises precision that the due logic does not have.
- Fix: show from the exact time `starts_at - offset_hours`, or make the setting days-only and drop the 2 h example.

**design-2-3: contradictory rule for archived attributes in templates.**
- Scope: lines 440, 442, 455.
- Problem:
  - Line 440 says archived values render by their last label.
  - Line 442 says an archived attribute renders the fallback.
  - The save check (455) does not say whether an archived attribute counts as existing.
- Fix:
  - an archived option renders by its last label;
  - an archived attribute renders the fallback;
  - define how save validation treats archived attributes.

**design-2-4: archived clients still produce reminder items.**
- Scope: lines 115, 470–492.
- Problem:
  - Archiving is how a record is deleted.
  - The rules never exclude `clients.archived_at`.
  - Also relevant to Ley 8968 (line 130).
- Fix: archived clients produce no items, plus a test row.

**style-2-1 = comprehension-5 = design-nit-1: "three" rule kinds.**
- Scope: line 899 (E7).
- Problem: there are two kinds.

### PWA, offline and client

**design-2-5: the "server off" check uses the content type.**
- Scope: lines 542, 548.
- Problem: these normal responses would read as an outage:
  - 204 responses with no body (logout, password change, icon, `/client-errors`);
  - `application/problem+json` errors, including the 409 overlap warning that "Agendar igual" needs;
  - the ZIP export and JPEG media.
- Fix: count as "server off" only:
  - a network error;
  - a 5xx;
  - a response with no `X-App-Version` header.

**design-2-6: the service worker may answer the export download with `index.html`.**
- Scope: lines 549, 653, CUJ 8; also item 66 (line 1845).
- Problem: every navigation is served from the cached shell. So a link to `/api/v1/export` or `/media/{id}` gets the app shell, not the file.
- Fix:
  - send `/api/*` navigations to the network;
  - state how the export is triggered (link, or fetch + blob).

**design-2-7: fixed icon URLs can leave a stale or default Home Screen icon.**
- Scope: lines 549, 556, 571–573, 695–699.
- Problem:
  - `/icons/*.png` and `/apple-touch-icon.png` are precached.
  - They are not in the Cloudflare cache-bypass rule.
  - iOS copies the icon only once, at install.
- Fix:
  - version the icon URLs (`?v=<icon_updated_at>`);
  - or send `no-cache`, add them to the bypass rule, and drop them from the precache.

**design-2-8: cached personal data stays on the phone after logout, a 401, or a revoked session.**
- Scope: lines 539–548, 565, 774.
- Problem: this contradicts the security summary ("only served with a session").
- Fix:
  - clear Dexie on logout and on a 401;
  - or list it in Accepted risks.

### Backup and restore (data safety)

**design-2-9: restore can leave newer schema objects behind and mislabel later snapshots.**
- Scope: lines 729–736.
- Problem:
  - `pg_restore --clean --if-exists` does not drop objects that the newer migrations added.
  - The `backup` container keeps the newer `APP_VERSION`, so the next snapshots carry the wrong tag.
  - `backup` is not stopped during the restore.
- Fix:
  - drop and recreate the schema (or the DB) before restoring;
  - stop `backup` during the restore;
  - recreate `backup` afterwards.

**design-2-10: `forget --keep-daily 14` can prune the pre-upgrade snapshot.**
- Scope: lines 705–709, 718–722, 729.
- Problem:
  - A second `once` on the same day replaces the earlier one. This happens on a retried upgrade or two upgrades in one day.
  - On a retry, `.env` already holds the new version.
- Fix:
  - tag pre-upgrade snapshots and keep them with `--keep-tag`, or use `--keep-last N`;
  - `upgrade.sh` refuses to start when `.env` and the running version differ.

### Push

**design-2-11: nothing restores a lost push subscription, and the 3-day red line gives false alarms.**
- Scope: lines 529–535, 674, 913.
- Problem:
  - After a 404 or 410 deletes the subscription, nothing re-subscribes.
  - Quiet days or PC-off days with nothing sent turn the line red while everything works.
- Fix:
  - re-POST `getSubscription()` on each standalone open;
  - base the red line on failed attempts, not on time since the last success.

### Alternatives and risks

**design-2-12: alternatives miss the cheaper options for the largest accepted security risk.**
- Scope: lines 785–786, 843–855.
- Problem:
  - Two options are not weighed: a Tailscale or WireGuard overlay, and passkeys / WebAuthn.
  - The per-username delay lets anyone who knows her username slow her logins (line 564). This denial-of-service is not listed.
- Fix: add both alternatives with rejection reasons, and list the login-delay DoS as a risk.

**structural-2-2: the risk section is buried, says "no mitigation", and is incomplete.**
- Scope: lines 779–789.
- Problem: these failure modes are handled in the text but missing from the list:
  - VAPID key loss (421);
  - push expiry (674);
  - USB unplugged (737);
  - Cloudflare upload size and timeout limits (129, 653);
  - iOS clearing IndexedDB (926);
  - bootstrap payload size (540, 945).
- Fix: an H2 "Risks & Mitigations" section (risk / mitigation / where handled). Accepted risks become one part of it.

### Doc consistency

**structural-2-1 = comprehension-7: links that the plan's own file moves will break.**
- Scope: lines 6, 47, 126, 793, 1094–1096 vs items 12–14 and 87.
- Problem: item 14 updates only the Additional Resources and Terms links.
- Fix: list every link to rewrite, or point the links at the final paths now.

**style-2-2 = zoom-out-2-M1 (scope) = design-nit-2: I1's rationale goes against its own decision.**
- Scope: lines 1001–1003, 432, 441.
- Problem:
  - I1 says shape → length "is really two flat attributes", yet keeps trees of any depth.
  - The value table at 432 then uses that same path.

**style-2-3 = comprehension-8: "Hoy" is not defined, and how it relates to "Para enviar hoy" is unclear.**
- Scope: lines 97–100, 167, 182, 205, 495, 644, 1086; Terms at 39–48.
- Impact: Goal 6's check depends on it.

---

## 🟢 Nit

### Design

| ID | Lines | Issue |
|---|---|---|
| design-n3 | 104, 734 | Goal 8 says "four services". There is a fifth, profile-only `restore` service. |
| design-n4 | 139, 556 | CUJ 1 asks for the icon before the login fields. The upload happens after `POST /setup`. |
| design-n5 | 579–580, 707 | Not stated: the media files are written before the `media` row is committed. |
| design-n6 | 723–726 | Guard condition 2 reads `users`, which does not exist on a fresh install. |
| design-n7 | 603–613 | "Reactivar" skips the overlap check. |
| design-n8 | 663 | CSV has no formula-injection prefix for `= + - @`. |
| design-n9 | 547, 592 | Postgres `unaccent` folds more characters than NFD (ø, æ, ß). So "same set" is not strict. |
| design-n10 | 397–405 | Not stated: `offset_hours` / `offset_days` are set per kind and are positive. |
| design-n11 | 126 | The reviewer says the PG18 data path is a known answer: the volume goes at `/var/lib/postgresql`, with `PGDATA` in a version folder. (Reviewer claim, not checked here.) |

### Structural

| ID | Lines | Issue |
|---|---|---|
| structural-n3 | 67–72 | "Proposed solution, in short" sits inside The Problem. |
| structural-n4 | 462, 646, 662, 671, 695 | No blank line before a list. |
| structural-n5 | 8, 1079, 1090, 1112 | Horizontal rules are used unevenly. |
| structural-n6 | 1081 | NFR section is far from Goals and architecture. |
| structural-n7 | 282, 1086 | Cross-references by name don't match the headings, and have no anchors. |
| structural-n8 | 227 | CUJ 8's "Then" is not a bullet list. |
| structural-n9 | 84 | "the four checks below" is followed by five bullets. |

### Style

| ID | Lines | Issue |
|---|---|---|
| style-n4 | 588, 1009 | "fine at this size", "enough for this volume": the size is not given. |
| style-n5 | 607 | "easy to add": say "one additive migration". |
| style-n6 | 807 | "is wrong": say "is inaccurate". |
| style-n7 | 62, 806, 812 | "heavy" and "weak" are vague; name the concrete requirement. |
| style-n8 | 919, 939, 1035, 1076 | Opinion fragments ("Noise.", "Far more work…") used as rationale. |
| style-n9 | 475, 862 | "flood" is a metaphor. |
| style-n10 | 781 | Filler clause. |
| style-n11 | 486–495, 554–558, 705–710 | Numbered lists also carry "First / Then / Finally". |
| style-n12 | 770 | "keep generic things generic" priority does not exist. The priority is #5, "Generic and self-hostable by others". |
| style-n13 = comprehension-9 | 48 | "EGM" is defined but unused in the range, and has no link. |
| style-n14 | 47 | One Terms entry defines two terms, and uses "main". |
| style-n15 | 793 | "Each alternative was rejected": A5, K2 and I1 were narrowed, not rejected. |
| style-n16 | 875 | "21 days" is an example `offset_days`, not a fixed value. |
| style-n17 | 1043 | "and approved": passive voice, so who approved is hidden. |
| style-n18 | 1065 | M1 rejection rests on an unmeasured first-load claim. |
| style-n19 | 1022–1023 | Not stated whose 3.1 support is unverified. |
| style-n20 | 209 | Missing "and". |
| style-n21 | 14 | "real" adds nothing. |
| style-n22 | 280 | Timer step 2 is one five-clause sentence. |
| style-n23 | 913 | "protects Goal 3" is vague. |
