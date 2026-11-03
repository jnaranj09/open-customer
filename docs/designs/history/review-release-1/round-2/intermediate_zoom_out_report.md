# Intermediate Zoom Out Report: Stage 8 (round 2)

Target: `docs/designs/release-1.md`, full file (lines 1–2088).
Reviewer: `egm-zoom-out`, in a fresh context. It read the Stage 5, 6 and 7 reports.
Standard: `zoom-out-standard.md`.

Verdict: no Blockers, 7 Major, 9 Nit.
- Most of the earlier fixes are small and needed.
- The risk is where a fix offers a choice and one option adds scope.
- Some fixes conflict with each other.
- Some findings are better fixed by deleting text than by adding features.

---

## 🟡 Major

**1. For design-2-2, pick days-only offsets.**
- Exact-time "before" items promise a precision that one daily push cannot deliver.
- A single `offset_days` column for both kinds (`>= 0` for "before"; 0 means the morning of the appointment) removes:
  - `offset_hours` and its CHECK;
  - the "exact durations" line (768);
  - the 2 h test (1762);
  - the "24 h and 2 h" example.
- It also covers half of design-n10.

**2. For design-2-1, change the identity; do not list "re-arm" edits.**
- A `come_back` item is identified by `(rule_id, appointment_id)`.
- A `before` item keeps `starts_at` as its anchor. If `starts_at` changes, the item correctly comes back, because the client was told the wrong time.
- Scope the Rescheduling bullet (479) to "before" items only.

**3. structural-2-2 (full Risks & Mitigations H2) conflicts with zoom-out Major 3 (no duplicated mechanics).**
- Rename the section "Risks".
- Keep the accepted risks.
- For each handled risk, add one line that points to the section that handles it. Do not copy the mitigation text.

**4. For design-2-11, store the last push result. Do not add failure tracking.**
- A 404 or 410 deletes the subscription row, so failure state stored on that row would be lost.
- Instead, the daily timer writes the last push result (date, delivered or failed, count) to `scheduler_state`.
- The line is red when there are no subscriptions, or when the last attempt was delivered to none.
- This drops `push_subscriptions.last_ok_at` and the 3-day rule.
- Keep the re-POST of `getSubscription()` on each standalone open.

**5. For design-2-3, use one archive rule, not two.**
- Archived attributes and archived options both render their stored value by its last label.
- Save validation counts archived attributes as existing.
- The archive warning stays, as information only.

**6. For design-2-9 and design-2-10, reuse what the plan already has.**
- `restore.sh` takes the same `flock` as `backup.sh`.
- `restore.sh` ends with `docker compose up -d`. This recreates `backup` with the restored `APP_VERSION`.
- The restore drops and recreates the DB before `pg_restore`.
- `upgrade.sh` adds `--tag pre-upgrade`, and `forget` adds `--keep-tag pre-upgrade`.
- `upgrade.sh` checks that the `.env` version matches the running version.

**7. Fix comprehension-1, -4 and -6 by deleting text.**
- comprehension-1 (and zoom-out Nit 4): delete "come-back time" from item 77.
- comprehension-6: "Enviados" is online only. Delete the "cached copy" sentence.
- comprehension-4:
  - The app requires only `PUBLIC_URL`, `DB_PASSWORD` and `VAPID_SUBJECT`.
  - The tunnel, restic and backup variables are Compose-level, not app config.
  - Allow `http://localhost` as `PUBLIC_URL`.

---

## 🟢 Nit

1. **Per-username delay** (zoom-out Nit 2 vs design-2-12):
   - Keep the delay, but drop the 10k cap.
   - List the lockout DoS as a one-line accepted risk.
   - Add Tailscale and passkeys only as rejected alternatives.
2. **design-2-7 (icons):** use `no-cache` plus the Cloudflare bypass, and remove the icons from the precache. Versioned URLs do not work with the static `app.html` (1836).
3. **design-2-5:** accept the `X-App-Version` signal. It removes the content-type rule and its special cases.
4. **Not worth adding:** do not add design-n8 (CSV formula prefix) or design-n7 (overlap check on Reactivar). For Reactivar, a one-line note is enough.
5. **design-n6:** make guard condition 2 "goose version > 0", not "a row in `users`".
6. **New:** the cursor-pagination convention (621, 634, 644, 1360) clashes with `/bootstrap` returning everything. Every list is already limited in size, so drop paging if Stage 5 Major 2 is applied.
7. **New:** `COMPOSE_SUBNET` and the `ipam` block (564, 752–753, 1547, 2001) are not needed:
   - No container publishes a port.
   - Default `TRUSTED_PROXY_CIDRS` to the RFC 1918 ranges.
8. **New:** two push features have no user:
   - the `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` env override (the keys are already in the DB and in the backup);
   - `DELETE /push/subscriptions/{id}` (no screen calls it).
9. **Cuts that remove several findings at once:**
   - Flat options (Stage 5 M1) removes style-2-2, the archived-parent case, the cycle and `parent_id` rules, and the drill-down picker.
   - Search from the cache only (Stage 5 M2) removes design-n9, style-n4@588, the wrapper function and extensions, and the duplicate tests.
   - No sessions screen (Stage 5 M4) removes the public `sessions.id` and the `Session` schema, and most of design-2-8. Keep the Dexie clear on logout and on a 401.

---

## Simplest combined change

Five cuts, each of which deletes parts and adds none:
1. Flat options.
2. Search from the cache only.
3. Days-only offsets.
4. No device-sessions screen.
5. `restore.sh` takes the lock and ends with `compose up -d`.

The remaining needed fixes are small:
- design-2-4, 2-5 and 2-6;
- the re-subscribe part of design-2-11;
- comprehension-2, -3, -5 and -7.
