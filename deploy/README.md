# Install and operate open-customer

This guide takes one person from an empty PC to a running, backed-up install, and covers the jobs
you do afterwards: upgrade, restore and password reset. The architecture is in
[docs/designs/release-1.md](../docs/designs/release-1.md). Every command below is run from this
folder (`deploy/`) unless it says otherwise; the scripts in `scripts/` also work from any folder.

The stack is five services in `compose.yaml`: `db` (PostgreSQL), `app`, `cloudflared`, `backup`,
and `restore`, which only `scripts/restore.sh` starts. No service publishes a port. Every service
drops all Linux capabilities and adds back only what it needs (`backup`: `DAC_OVERRIDE`;
`restore`: `CHOWN`, `DAC_OVERRIDE`, `FOWNER`), and runs with a read-only root file system and a
memory limit (`db`, `backup` and `restore` 1 GB, `app` 512 MB, `cloudflared` 256 MB; these are
ceilings, not what each one uses). A service that goes over its limit is killed, and Compose
restarts it (`restore` then fails, and `scripts/restore.sh` says so).

## 1. Requirements

- A PC that stays on and has a stable connection, with Docker Engine and the Compose plugin
  (`docker compose version` must work).
- A Cloudflare account with a domain you control, and Cloudflare Zero Trust enabled.
- A folder for the backups (section 4).
- A way to keep a copy of `RESTIC_PASSWORD` outside this PC, for example a password manager.

## 2. Create the tunnel and the public hostname

1. In the Cloudflare dashboard open Zero Trust, Networks, Tunnels, and create a tunnel of type
   cloudflared. Name it anything.
2. Copy the tunnel token. It goes in `.env` as `TUNNEL_TOKEN` (section 5). Do not run the install
   command the dashboard shows: the `cloudflared` service in `compose.yaml` does that job.
3. Add a public hostname to the tunnel: your subdomain (for example `citas.example.com`), service
   type HTTP, URL `http://app:8080`.

## 3. Cloudflare settings

Set these once. They are not in the repo, so a new Cloudflare account needs them again.

1. The public hostname points to `http://app:8080` (step 3 of section 2).
2. A cache rule bypasses the cache for `/api/*`, `/manifest.webmanifest`, `/service-worker.js`,
   `/icons/*` and `/apple-touch-icon.png`, so a new icon is never hidden by Cloudflare's cache.
3. Rocket Loader is off.
4. Bot Fight Mode is off. A Cloudflare challenge page has no `X-App-Version` header, so the app
   would show it as "server off".
5. HSTS is on.

## 4. Prepare the backup target

`BACKUP_TARGET` is any folder on the PC that you choose. The backup service writes there only while
the marker file `.open-customer-backup-target` exists in it, so a folder that went missing is never
filled by mistake.

1. Create the folder and the marker, once:

   ```sh
   mkdir -p /srv/backups/open-customer
   touch /srv/backups/open-customer/.open-customer-backup-target
   ```

2. Use that folder as `BACKUP_TARGET` in `.env` (section 5).

**A folder on the PC's own disk does not survive that disk dying.** It protects against a bad
upgrade or a deleted photo, not against a failed disk, theft or fire. Add a second disk or a USB
disk later: only the `BACKUP_TARGET` value in `.env` changes, then run
`docker compose up -d` so the `backup` service picks it up, and `scripts/backup-now.sh` so the new
folder gets a backup now (the hourly check skips for up to 24 h after the last one).

Only for a removable disk (optional): mount it at the target folder by an `/etc/fstab` line with
`nofail`, so the PC still boots when the disk is missing. Use your own UUID and path:

```
UUID=1b2c3d4e-0000-4000-8000-123456789abc  /srv/backups/open-customer  ext4  defaults,nofail  0  2
```

Run `touch` for the marker with the disk mounted. When the disk is not mounted, the mount point is
an empty folder without the marker, so nothing is written and the status screen shows
"Disco de respaldo no visible". Plugging the disk back in is picked up by the next hourly check
without a restart.

## 5. `.env`

1. Copy the example and open it:

   ```sh
   cp .env.example .env
   chmod 600 .env
   ```

2. Fill in every required value. `.env.example` explains each one; the design doc's
   "Configuration (`.env`)" table is the reference.

   | Variable | What to put |
   | --- | --- |
   | `APP_VERSION` | The release to run, for example `1.0.0` (the tag without its `v`). |
   | `PUBLIC_URL` | `https://citas.example.com` |
   | `DB_PASSWORD` | A long random password. |
   | `TUNNEL_TOKEN` | The token from section 2. |
   | `VAPID_SUBJECT` | `mailto:` and a contact address. |
   | `BACKUP_TARGET` | The folder from section 4. |
   | `RESTIC_PASSWORD` | A long random password. Keep a copy outside this PC: without it every backup is unreadable. |

3. Check that Compose accepts it. A missing value stops with a message naming it:

   ```sh
   docker compose config --quiet
   ```

Never commit `.env`. After first setup, `scripts/upgrade.sh` and `scripts/restore.sh` edit only its
`APP_VERSION` line.

## 6. First start and the setup link

Restoring onto a new PC, or after the volumes were lost? Do sections 1 to 5, then section 10. Do
not run this section.

1. Pull the app and backup images of `APP_VERSION` through the signature check, then start the
   stack. On a first start the `backup` service sees no earlier backup and runs one:

   ```sh
   scripts/pull.sh
   docker compose up -d
   docker compose ps
   ```

   `scripts/pull.sh` stops, and nothing is pulled, if an image is not signed by this repository's
   release workflow (see "Signatures" in section 9). Do not run `docker compose up -d` before it
   succeeds: compose would pull the missing images without the check.

   Wait until `db` and `app` show `healthy`.

2. While no user exists, `app` prints the setup link as one line in its log:

   ```sh
   docker compose logs app | grep Setup:
   ```

   The link looks like `https://citas.example.com/setup#<token>` and is valid for 24 hours. If it
   expired, or you lost it, make a new one (the old link stops working):

   ```sh
   docker compose exec app open-customer setup-link
   ```

   This works only while no user exists.

3. Open the link on her phone (section 7).

The `backup` service also has an empty anonymous volume under `/var/lib/postgresql`, because the
PostgreSQL base image declares that volume. It holds nothing and is harmless.

## 7. Handover, in person

Do these with her, in this order, on her iPhone:

1. Open the setup link in Safari and complete the wizard.
2. Install the app: Share, Add to Home Screen.
3. Open it from the Home Screen icon and log in with the username and password she chose. The
   installed app does not share Safari's cookies, so she logs in again.
4. Allow notifications when asked.
5. Give her a sealed paper copy of `RESTIC_PASSWORD`. Tell her what it is for: with it, and a
   backup, her data can be read again if this PC dies. Without it, every backup is unreadable.

### Before handover

Do not hand over until every box is ticked.

- [ ] On her iPhone, through the tunnel: the `wa.me` / `whatsapp://` test from the design doc's
      "Open Questions" works.
- [ ] On her iPhone, through the tunnel: install from Safari, log in, allow notifications and
      receive a test push.
- [ ] "Hoy" opens in under 1 s over LTE. Close the app from the app switcher, start a screen
      recording, tap the Home Screen icon, and count the time from the tap to the first
      appointment row painted, using the recording's frames. Pass: 3 of 3 runs under 1 s.
- [ ] With the server stopped (`docker compose stop app`), the app opens from the cache with the
      banner. Start it again with `docker compose start app`.
- [ ] The restore drill (section 8) passed.
- [ ] Remove the marker file from the backup folder (or unplug the target, if it is a removable
      disk). The status screen shows the red backup line by the next hourly check (at most
      60 min), and nothing was written to the folder. Put the marker back (or plug the disk in)
      afterwards.
- [ ] Time creating an appointment (CUJ 3 steps 1 to 4) with a stopwatch on her iPhone, three
      times, with an existing client. Pass: 3 of 3 under 30 s.
- [ ] Attach a photo from the camera and one from the library to an appointment. Both upload,
      show on the appointment and in the client's gallery, and the stored file has no EXIF GPS
      data.
- [ ] Tap "Enviar" on "¿Lo enviaste?", close the app from the app switcher while in WhatsApp, and
      reopen it. The sheet shows for that client.
- [ ] No place or currency is written into the code. From the repo root this prints nothing
      outside `internal/locale/es.go` and `_test.go` files:

  ```sh
  grep -rnE 'Costa|\bCRC\b|America/Costa_Rica|\+506|[Uu]ña|\b[Nn]ails?\b' --include='*.go' --include='*.sql' internal
  ```

## 8. The restore drill and the Ley 8968 check

Both are required before her real data goes in.

### Restore drill

Use made-up data. This wipes the volumes of this install, so do it before real use.

1. Create a test client with one appointment and one photo.
2. Take a backup and wait for it to finish:

   ```sh
   scripts/backup-now.sh
   ```

3. Stop the stack and remove its volumes. The backup folder is not touched:

   ```sh
   docker compose down --volumes
   ```

4. Start only the database, so the restore has something to fill:

   ```sh
   docker compose up -d db
   ```

   Starting the whole stack here is also safe: on empty volumes `backup` sees the earlier
   snapshots and takes none until the restore (section 10).

5. Restore the newest snapshot. Read the snapshot line and answer the prompt by typing `y`
   yourself; piped input is not supported:

   ```sh
   scripts/restore.sh latest
   ```

   At the end it runs `docker compose up -d`, which starts the whole stack, `cloudflared`
   included.

6. Open the app. Confirm the client, the appointment and the photo are back.
7. Upload one new photo, to confirm `app` can still write to the restored media volume.

- [ ] The restore drill passed (steps 1 to 7).

### Ley 8968 check

Before her real data goes in, check her obligations under Costa Rica's personal data law
(Ley 8968): consent, archive-only deletion (the app never hard-deletes), and client data held on a
home PC.

- [ ] Ley 8968 checked; the result is written down.

## 9. Upgrade

Do it outside her working hours. A rollback drops everything she entered after the upgrade.

1. Pick the version, for example `1.2.3`. Check that `APP_VERSION` in `.env` is the version now
   running; the script stops if they differ.
2. Run:

   ```sh
   scripts/upgrade.sh 1.2.3
   ```

   It first checks the signature of both new images (the app and the backup image), then pulls
   them and checks that each pulled image is the exact digest whose signature it checked, takes a
   backup tagged `pre-upgrade`, sets `APP_VERSION` in `.env`, and runs
   `docker compose up -d`. The new `app` runs the migrations. If the backup fails (for example the
   target folder or disk is missing), the script stops and `.env` stays on the old version.
3. Check `docker compose ps` and open the app.

If you upgrade by editing `.env` and running `docker compose up -d` yourself, compose pulls the new
images without the signature check, and `app` refuses to migrate without a backup from the last
60 minutes and exits with "Pending migrations and no recent backup. Run scripts/upgrade.sh."

### Signatures

Each release's images are signed by this repository's release workflow (keyless, with cosign
through GitHub's OIDC; no key is stored anywhere), and each carries a CycloneDX SBOM attestation.
`scripts/upgrade.sh` runs cosign from its pinned image (`COSIGN_IMAGE` in `scripts/lib.sh`; you
install nothing) and accepts only a signature whose identity matches `SIGNER_IDENTITY_REGEXP`
(this repository's `release.yml` on a `v*` tag) from `SIGNER_ISSUER` (GitHub Actions), made in
`SIGNER_REPOSITORY` on a `SIGNER_TRIGGER` (tag push) event. If the check fails, the script stops
before it pulls anything or takes a backup, `.env` stays as it was, and the message names the
image. cosign reports the digest it verified; after the pull, each pulled image must carry exactly
that digest, or the script stops before the backup with `.env` unchanged (the tag changed in the
registry between the check and the pull: run the upgrade again, and look into the registry if it
repeats). `compose.yaml` sets no `pull_policy`, so `docker compose up -d` runs those checked local
images.

**Local images are trusted.** An app or backup image already on this PC got there through a
verified pull (`scripts/pull.sh`, `scripts/upgrade.sh`, `scripts/restore.sh`,
`scripts/backup-now.sh`) or through a local build, so the scripts use it without the network: an
offline restore of local images still works. Each script runs the check before a `docker compose`
command that would pull a missing app or backup image (`restore.sh` for `.env`'s version and for
the snapshot's version, before it stops anything; `backup-now.sh` for `.env`'s version). A
`docker compose` command you type yourself has no check: on a PC that does not have the images of
`APP_VERSION` yet, run `scripts/pull.sh` first. Check, in this order: the PC can reach `ghcr.io` and Sigstore's
public services (`sigstore.dev`); the version exists; for a fork, the lines below. There is no
option to skip the check.

To check one image by hand, from this folder, with the same pin and identity the script uses (it
prints the verified `<image>@<digest>` when the signature is good):

```sh
bash -c '. scripts/lib.sh && verify_image ghcr.io/jnaranj09/open-customer:1.2.3'
```

**A fork** that builds and publishes its own images changes, in `scripts/lib.sh`,
`SIGNER_IDENTITY_REGEXP` to its own repository
(`^https://github\.com/<owner>/<repo>/\.github/workflows/release\.yml@refs/tags/v`) and
`SIGNER_REPOSITORY` to `<owner>/<repo>`, next to the image names in `compose.yaml`.
`SIGNER_ISSUER` and `SIGNER_TRIGGER` stay as they are while the fork releases from GitHub Actions
on a tag push.

**The cosign pin** (`COSIGN_IMAGE` in `scripts/lib.sh`) is bumped by hand: Dependabot cannot read
`lib.sh`. Its version must equal `cosign-release` in `.github/workflows/release.yml`; change both
(and the image digest) in one commit, and `make check-pins` checks that they agree.

## 10. Restore

Use it after data loss, or to roll back an upgrade (restore the `pre-upgrade` snapshot). Migrations
only move forward, so a rollback is always a restore.

After the volumes were lost (a new PC, or `docker compose down --volumes`), restore before her
data goes in again. Until then `backup` takes no snapshot: it finds earlier snapshots in the
backup folder but no `status.json` on the new volumes, so a backup now would hold an empty
database and become `latest`, and `scripts/restore.sh latest` would then restore nothing. Its
clean-up could also drop that day's real snapshot (it keeps only the newest snapshot of each day).
The status screen and "Hoy" show the backup line red ("Respaldo con problema"), and
`scripts/backup-now.sh` exits non-zero. The backup log has an English line starting `guard:`; the
Spanish reason, "Hay respaldos anteriores: restaure primero", is only in the status file
(`last_error`):

```sh
docker compose logs backup | tail -n 5
docker compose exec backup cat /backup-status/status.json
```

A restore lifts it and the hourly backups go on.

Never use this reset after a failed restore: it lifts the guard over a half-restored database,
and the next backup would make that `latest`. Run `scripts/restore.sh` again instead (see the end
of this section).

If the database is intact and only the status file or the `backup-status` volume was lost, do not
restore: a restore of `latest` can lose up to about a day of her work. Replace the status file with
an empty one instead, then take a backup now. Do the same to start empty on purpose and keep the
old snapshots:

```sh
docker compose run --rm --entrypoint sh backup -c 'echo {} > /backup-status/status.json'
scripts/backup-now.sh
```

1. Find the snapshot id. On a new PC, run `scripts/pull.sh` first (the command below runs the
   backup image, and compose would pull it without the check). From this folder:

   ```sh
   docker compose --profile restore run --rm restore snapshot-info latest
   ```

   prints one JSON line with the id, time and app version of the newest snapshot. To list them
   all, with their times and tags:

   ```sh
   docker compose --profile restore run --rm --entrypoint restic restore -r /target/restic --no-lock snapshots
   ```
2. Run the restore with an id, or `latest`:

   ```sh
   scripts/restore.sh <snapshot-id|latest>
   ```

   It shows the snapshot's time and app version and asks `[y/N]`. Answer it yourself at the
   prompt; piped input is not supported. Anything but `y` cancels with nothing stopped.
3. Before it stops anything, it makes sure the app and backup images of the snapshot's version
   are on this PC, pulling them through the signature check when they are not (it does the same
   for `.env`'s version before it reads the snapshot); a failed check stops it with nothing
   changed. Then it stops `app` and `backup`, recreates the database from the snapshot, replaces the media
   folder (owned by `65532:65532`, so `app` can write photos), sets `APP_VERSION` in `.env` to the
   snapshot's version, and runs `docker compose up -d`.

After a rollback on a working install, the hourly check still sees the recent pre-upgrade backup
and skips for up to 24 h. Run `scripts/backup-now.sh` after the restore to back up the restored
data now. On new volumes the next hourly check backs up by itself.

If it fails after the stop, `app` and `backup` stay stopped on purpose, and the message says so.
Fix the cause and run `scripts/restore.sh` again. Until a restore succeeds, `backup` takes no
snapshot (the same guard as above), so a half-restored database never becomes `latest`.

## 11. Password reset

When she forgets her password:

```sh
docker compose exec app open-customer reset-password <username>
```

It prints a temporary password and closes all her sessions. She logs in with it and must choose a
new one. Give her the password in person or by a private message.

## 12. Security note

Cloudflare terminates TLS, so Cloudflare sees the traffic decrypted, including logins and client
data. Between Cloudflare and this PC the tunnel is encrypted. Login is a password only, with no
second factor, on the public internet. Pick a long password for her.

## 13. Without a tunnel

If you do not use Cloudflare, put any reverse proxy in front of `app:8080` and set
`TRUSTED_PROXY_CIDRS` in `.env` to that proxy's address.
