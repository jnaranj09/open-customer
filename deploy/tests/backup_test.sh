#!/usr/bin/env bash
# Tests for deploy/backup/backup.sh (design doc item 85, "Backups"). Run by make test-deploy
# (scripts/test-deploy.sh) inside the backup image, against a throwaway Postgres: PGHOST, PGPORT,
# PGUSER, PGPASSWORD and PGDATABASE come from that script. /target, /media and /backup-status are
# the container's own folders, emptied before every test, and every test gets a fresh database.
# All data here is made up.
set -uo pipefail

# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. "$(dirname "$0")/lib.sh"

export RESTIC_PASSWORD=test-only-restic-password
export APP_VERSION=0.0.0-test
export RESTIC_REPOSITORY=/target/restic
BACKUP=/backup.sh
STATUS=/backup-status/status.json
MARKER=/target/.open-customer-backup-target
LOGS=$(mktemp -d)
trap 'rm -rf "$LOGS"' EXIT

# fresh: empty folders, no marker, no status file and an empty database.
fresh() {
  mkdir -p /target /media /backup-status
  find /target /media /backup-status -mindepth 1 -delete
  dropdb --force --if-exists "$PGDATABASE"
  createdb "$PGDATABASE"
}

# field <name>: one status.json field as text ("null" when null).
field() {
  jq -r --arg k "$1" '.[$k]' "$STATUS"
}

# seed_status <json>: an earlier run's status.json.
seed_status() {
  printf '%s\n' "$1" >"$STATUS"
}

old_status='{"checked_at":"2000-01-01T00:00:00Z","last_dump_at":"2000-01-01T00:00:00Z",
  "last_copy_at":"2000-01-01T00:00:00Z","last_snapshot_id":"old","last_error":null,"last_error_at":null}'

snapshots() {
  restic --no-lock snapshots --json
}

# wait_until <seconds> <command...>: polls the command every 0.1 s; fails on timeout.
wait_until() {
  local limit=$(($1 * 10)) i=0
  shift
  until "$@"; do
    i=$((i + 1))
    [ "$i" -lt "$limit" ] || fail "timed out waiting for: $*"
    sleep 0.1
  done
}

# exited <pid>: true once the process is gone.
exited() {
  ! kill -0 "$1" 2>/dev/null
}

test_marker_missing() {
  fresh
  seed_status "$old_status"
  local rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "exit code" 0 "$rc"
  assert_eq "files under /target" "" "$(find /target -mindepth 1)"
  assert_eq "last_error" "Disco de respaldo no visible" "$(field last_error)"
  assert_ne "checked_at" "2000-01-01T00:00:00Z" "$(field checked_at)"
  assert_ne "last_error_at" "null" "$(field last_error_at)"
  assert_eq "last_copy_at" "2000-01-01T00:00:00Z" "$(field last_copy_at)"
  assert_eq "last_dump_at" "2000-01-01T00:00:00Z" "$(field last_dump_at)"
}

test_first_run() {
  fresh
  touch "$MARKER"
  local rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_eq "exit code" 0 "$rc"
  [ -f /target/restic/config ] || fail "no /target/restic/config"
  local snaps
  snaps=$(snapshots)
  assert_eq "snapshot count" 1 "$(jq length <<<"$snaps")"
  assert_eq "snapshot host" open-customer-backup "$(jq -r '.[0].hostname' <<<"$snaps")"
  assert_eq "snapshot tags" "app=$APP_VERSION" "$(jq -r '.[0].tags | join(",")' <<<"$snaps")"
  assert_eq "last_snapshot_id" "$(jq -r '.[0].id' <<<"$snaps")" "$(field last_snapshot_id)"
  local f
  for f in checked_at last_dump_at last_copy_at; do
    [[ $(field "$f") =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] ||
      fail "$f is '$(field "$f")'"
  done
  assert_eq "last_error" null "$(field last_error)"
  assert_eq "last_error_at" null "$(field last_error_at)"
  assert_eq "status.json mode" 644 "$(stat -c %a "$STATUS")"
}

test_pre_upgrade_kept() {
  fresh
  touch "$MARKER"
  "$BACKUP" once --tag pre-upgrade >"$LOGS/out" 2>&1
  "$BACKUP" once >>"$LOGS/out" 2>&1
  "$BACKUP" once >>"$LOGS/out" 2>&1
  local snaps
  snaps=$(snapshots)
  assert_eq "pre-upgrade snapshots" 1 \
    "$(jq '[.[] | select(.tags | index("pre-upgrade"))] | length' <<<"$snaps")"
  assert_eq "snapshots (the pre-upgrade one + the day's latest)" 2 "$(jq length <<<"$snaps")"
}

test_unreadable_repo() {
  fresh
  touch "$MARKER"
  "$BACKUP" once >"$LOGS/out" 2>&1
  local before
  before=$(cd /target && find . -type f -exec md5sum {} + | sort)
  jq '.last_dump_at = "2000-01-01T00:00:00Z"' "$STATUS" >"$LOGS/s" && cp "$LOGS/s" "$STATUS"
  local rc=0
  RESTIC_PASSWORD=a-different-password "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "exit code" 0 "$rc"
  assert_prefix "last_error" "copy: " "$(field last_error)"
  assert_eq "files under /target" "$before" "$(cd /target && find . -type f -exec md5sum {} + | sort)"
  assert_ne "last_dump_at" "2000-01-01T00:00:00Z" "$(field last_dump_at)"
}

# restic init itself refuses a folder that has a config, so the check is seen only on a broken
# repository whose config is gone. An earlier status.json, so the fresh-volumes guard lets the run
# through to the copy step (the new last_dump_at shows it got there).
test_broken_repo() {
  fresh
  touch "$MARKER"
  seed_status "$old_status"
  mkdir -p /target/restic/data
  local rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "exit code" 0 "$rc"
  assert_prefix "last_error" "copy: " "$(field last_error)"
  assert_ne "last_dump_at" "2000-01-01T00:00:00Z" "$(field last_dump_at)"
  assert_eq "files under /target/restic" "/target/restic/data" "$(find /target/restic -mindepth 1)"
}

test_dump_failure() {
  fresh
  touch "$MARKER"
  seed_status "$old_status"
  local rc=0
  PGHOST=oc-no-such-host.invalid "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "exit code" 0 "$rc"
  assert_prefix "last_error" "dump: " "$(field last_error)"
  assert_eq "last_dump_at" "2000-01-01T00:00:00Z" "$(field last_dump_at)"
  assert_eq "files under /target" "$MARKER" "$(find /target -mindepth 1)"
}

checked_at_changed() {
  [ "$(field checked_at)" != "2000-01-01T00:00:00Z" ]
}

test_loop_skips_recent() {
  fresh
  touch "$MARKER"
  local hour_ago
  hour_ago=$(jq -nr 'now - 3600 | todate')
  seed_status "{\"checked_at\":\"2000-01-01T00:00:00Z\",\"last_dump_at\":\"$hour_ago\",
    \"last_copy_at\":\"$hour_ago\",\"last_snapshot_id\":\"old\",\"last_error\":\"copy: old\",
    \"last_error_at\":\"2000-01-01T00:00:00Z\"}"
  BACKUP_INTERVAL_SECONDS=1 "$BACKUP" >"$LOGS/out" 2>&1 &
  local pid=$!
  wait_until 30 checked_at_changed
  kill -TERM "$pid"
  wait_until 10 exited "$pid"
  [ ! -e /target/restic ] || fail "a repository was made"
  assert_eq "last_error" null "$(field last_error)"
  assert_eq "last_error_at" null "$(field last_error_at)"
  assert_eq "last_copy_at" "$hour_ago" "$(field last_copy_at)"
}

lock_is_held() {
  ! flock -n /backup-status/backup.lock true
}

test_lock() {
  fresh
  touch "$MARKER"
  seed_status "$old_status"
  cp "$STATUS" "$LOGS/seed"
  mkfifo "$LOGS/release"
  # Holds the lock until a line is written to the fifo. One process, so killing it frees the lock.
  (
    exec 8>/backup-status/backup.lock
    flock 8
    read -r _ <"$LOGS/release"
  ) &
  local holder=$!
  wait_until 10 lock_is_held

  BACKUP_INTERVAL_SECONDS=1 "$BACKUP" >"$LOGS/loop" 2>&1 &
  local loop=$!
  wait_until 10 grep -q 'lock held' "$LOGS/loop"
  kill -TERM "$loop"
  wait_until 10 exited "$loop"
  cmp -s "$LOGS/seed" "$STATUS" || fail "the skipped hour changed status.json"
  [ ! -e /target/restic ] || fail "the skipped hour made a repository"

  "$BACKUP" once >"$LOGS/once" 2>&1 &
  local once=$!
  wait_until 10 grep -q 'waiting' "$LOGS/once"
  sleep 1
  kill -0 "$once" 2>/dev/null || fail "once did not wait for the lock"
  [ ! -e /target/restic ] || fail "once ran while the lock was held"
  echo release >"$LOGS/release"
  wait "$holder"
  local rc=0
  wait "$once" || rc=$?
  assert_eq "once exit code" 0 "$rc"
  assert_eq "snapshots" 1 "$(snapshots | jq length)"
}

# A lock file that cannot be opened (here a folder in its place) stops every mode that needs it:
# without the lock, two runs could touch the dump and the repository at once.
test_lock_unusable() {
  fresh
  touch "$MARKER"
  mkdir /backup-status/backup.lock
  local mode rc out
  for mode in once loop; do
    rc=0
    if [ "$mode" = once ]; then
      out=$("$BACKUP" once 2>&1) || rc=$?
    else
      out=$(BACKUP_INTERVAL_SECONDS=1 timeout 20 "$BACKUP" 2>&1) || rc=$?
    fi
    assert_eq "$mode exit code" 1 "$rc"
    assert_eq "$mode output" "backup.sh: cannot open the lock file /backup-status/backup.lock" "$out"
  done
  [ ! -e /target/restic ] || fail "a run went ahead without the lock"
  [ ! -e "$STATUS" ] || fail "a run went ahead without the lock"
}

# The slow step: the restic copy of a 64 MB media file. TERM goes in when once logs its dump
# line, so restic init, backup and forget (seconds) are still ahead; the trap must let the running
# step finish and then write "interrupted".
test_term_interrupted() {
  fresh
  touch "$MARKER"
  head -c 67108864 /dev/urandom >/media/big.jpg
  "$BACKUP" once >"$LOGS/out" 2>&1 &
  local pid=$!
  wait_until 30 grep -q '^dump: ok' "$LOGS/out"
  kill -TERM "$pid"
  wait_until 60 exited "$pid"
  local rc=0
  wait "$pid" || rc=$?
  assert_ne "exit code" 0 "$rc"
  jq -e . "$STATUS" >/dev/null || fail "status.json does not parse"
  assert_eq "last_error" interrupted "$(field last_error)"
  assert_eq "temp files left in /backup-status" "" "$(find /backup-status -maxdepth 1 -name '.status.json.*')"
}

test_snapshot_info() {
  fresh
  touch "$MARKER"
  "$BACKUP" once >"$LOGS/out" 2>&1
  local id out
  id=$(field last_snapshot_id)
  for arg in latest "$id"; do
    out=$("$BACKUP" snapshot-info "$arg")
    assert_eq "lines for $arg" 1 "$(printf '%s\n' "$out" | wc -l)"
    assert_eq "keys for $arg" '["app","id","time"]' "$(jq -c 'keys' <<<"$out")"
    assert_eq "id for $arg" "$id" "$(jq -r .id <<<"$out")"
    assert_eq "app for $arg" "$APP_VERSION" "$(jq -r .app <<<"$out")"
    assert_ne "time for $arg" null "$(jq -r .time <<<"$out")"
  done
}

rows() {
  psql -XAtq -c "$1"
}

test_restore() {
  fresh
  touch "$MARKER"
  rows "CREATE TABLE clients (name text); INSERT INTO clients VALUES ('Cliente Uno'), ('Cliente Dos')"
  mkdir -p /media/2026/11
  echo 'made-up photo' >/media/2026/11/a.jpg
  "$BACKUP" once >"$LOGS/out" 2>&1
  local id
  id=$(field last_snapshot_id)

  rows "INSERT INTO clients VALUES ('Cliente Tres'); CREATE TABLE after_snapshot (x int)"
  rm /media/2026/11/a.jpg
  echo 'newer photo' >/media/newer.jpg

  local rc=0
  "$BACKUP" restore "$id" >"$LOGS/out" 2>&1 || rc=$?
  assert_eq "exit code" 0 "$rc"
  assert_eq "rows" "Cliente Dos,Cliente Uno" "$(rows "SELECT string_agg(name, ',' ORDER BY name) FROM clients")"
  assert_eq "after_snapshot" "" "$(rows "SELECT to_regclass('after_snapshot')")"
  assert_eq "restored photo" "made-up photo" "$(cat /media/2026/11/a.jpg)"
  [ ! -e /media/newer.jpg ] || fail "a photo added after the snapshot is still there"
  assert_eq "owners other than 65532:65532" "" \
    "$(find /media -exec stat -c '%u:%g %n' {} + | grep -v '^65532:65532 ' || true)"
  [ ! -e /backup-status/work/restore ] || fail "/backup-status/work/restore was not removed"
}

GUARD_ERROR="Hay respaldos anteriores: restaure primero"
GUARD_FILE=/backup-status/restore-first

# fresh_volumes_with_repo: a repository with one snapshot, then the backup-status volume emptied
# (a new PC, or `docker compose down --volumes`), with the database emptied too.
fresh_volumes_with_repo() {
  fresh
  touch "$MARKER"
  "$BACKUP" once >"$LOGS/out" 2>&1 || fail "the first once failed"
  find /backup-status -mindepth 1 -delete
  dropdb --force "$PGDATABASE"
  createdb "$PGDATABASE"
}

test_fresh_volumes_guard() {
  fresh_volumes_with_repo
  local before rc=0
  before=$(snapshots | jq -c '[.[].id]')
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "once exit code" 0 "$rc"
  assert_eq "last_error" "$GUARD_ERROR" "$(field last_error)"
  assert_ne "last_error_at" null "$(field last_error_at)"
  assert_ne "checked_at" null "$(field checked_at)"
  assert_eq "snapshots" "$before" "$(snapshots | jq -c '[.[].id]')"
  grep -q '^guard: ' "$LOGS/out" || fail "no guard line in the output"

  # It holds on later runs too: the status file it wrote does not lift it, nor does a run with
  # the target missing in between.
  rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "second once exit code" 0 "$rc"
  rm "$MARKER"
  "$BACKUP" once >"$LOGS/out" 2>&1 || true
  assert_eq "last_error without the marker" "Disco de respaldo no visible" "$(field last_error)"
  touch "$MARKER"
  rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "once after the marker is back" 0 "$rc"
  assert_eq "last_error after the marker is back" "$GUARD_ERROR" "$(field last_error)"

  # The loop waits for the next hour instead of exiting, and takes no snapshot either.
  seed_status "$(jq '.checked_at = "2000-01-01T00:00:00Z"' "$STATUS")"
  BACKUP_INTERVAL_SECONDS=1 "$BACKUP" >"$LOGS/loop" 2>&1 &
  local pid=$!
  wait_until 30 checked_at_changed
  kill -0 "$pid" 2>/dev/null || fail "the loop exited"
  kill -TERM "$pid"
  wait_until 10 exited "$pid"
  assert_eq "loop last_error" "$GUARD_ERROR" "$(field last_error)"
  assert_eq "snapshots after the loop" "$before" "$(snapshots | jq -c '[.[].id]')"
}

test_restore_resumes() {
  fresh_volumes_with_repo
  local id
  id=$(snapshots | jq -r '.[0].id')
  "$BACKUP" once >"$LOGS/out" 2>&1 || true
  assert_eq "last_error before the restore" "$GUARD_ERROR" "$(field last_error)"
  local rc=0
  "$BACKUP" restore "$id" >"$LOGS/out" 2>&1 || rc=$?
  assert_eq "restore exit code" 0 "$rc"
  assert_eq "last_snapshot_id after the restore" "$id" "$(field last_snapshot_id)"
  assert_eq "last_error after the restore" null "$(field last_error)"
  assert_eq "last_error_at after the restore" null "$(field last_error_at)"
  assert_ne "checked_at after the restore" null "$(field checked_at)"
  rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_eq "once after the restore" 0 "$rc"
  assert_eq "snapshots" 2 "$(snapshots | jq length)"
}

# A restore that fails after the database was dropped leaves it empty; the next backup must not
# save that as `latest`. Here pg_restore fails (a fake first on PATH).
test_failed_restore_blocks_backup() {
  fresh
  touch "$MARKER"
  rows "CREATE TABLE clients (name text); INSERT INTO clients VALUES ('Cliente Uno')"
  "$BACKUP" once >"$LOGS/out" 2>&1 || fail "the first once failed"
  local id before rc=0
  id=$(field last_snapshot_id)
  before=$(snapshots | jq -c '[.[].id]')
  mkdir -p "$LOGS/bin"
  printf '#!/bin/sh\necho "made-up pg_restore failure" >&2\nexit 1\n' >"$LOGS/bin/pg_restore"
  chmod +x "$LOGS/bin/pg_restore"
  PATH="$LOGS/bin:$PATH" "$BACKUP" restore "$id" >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "restore exit code" 0 "$rc"
  grep -q 'made-up pg_restore failure' "$LOGS/out" || fail "the restore error is not shown"
  rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "once after the failed restore" 0 "$rc"
  assert_eq "last_error" "$GUARD_ERROR" "$(field last_error)"
  assert_eq "snapshots" "$before" "$(snapshots | jq -c '[.[].id]')"
  # A restore that succeeds lifts it.
  "$BACKUP" restore "$id" >"$LOGS/out" 2>&1 || fail "the second restore failed"
  "$BACKUP" once >"$LOGS/out" 2>&1 || fail "once after a good restore failed"
}

test_empty_status_fresh_start() {
  fresh_volumes_with_repo
  "$BACKUP" once >"$LOGS/out" 2>&1 || true
  assert_eq "last_error before the reset" "$GUARD_ERROR" "$(field last_error)"
  echo '{}' >"$STATUS"
  local rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_eq "once after the reset" 0 "$rc"
  assert_eq "last_error" null "$(field last_error)"
  assert_eq "snapshots" 2 "$(snapshots | jq length)"
  [ ! -e "$GUARD_FILE" ] || fail "$GUARD_FILE is still there"
  # A later failed run does not bring the guard back.
  rc=0
  PGHOST=oc-no-such-host.invalid "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_prefix "last_error after a failed run" "dump: " "$(field last_error)"
  rc=0
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_eq "once after the failed run" 0 "$rc"
}

# snapshot_files: the repository's snapshot files and their checksums.
snapshot_files() {
  (cd /target/restic/snapshots && find . -type f -exec md5sum {} + | sort)
}

test_guard_fails_closed() {
  fresh_volumes_with_repo
  local before rc=0
  # A snapshot file restic cannot read: `restic snapshots` skips it and answers [] with exit 0.
  # The cache would still answer from the first run, so it goes too.
  rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/restic"
  local f
  f=$(find /target/restic/snapshots -type f | head -n1)
  echo 'not a snapshot' >"$f"
  before=$(snapshot_files)
  "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "once exit code with a corrupt snapshot file" 0 "$rc"
  assert_eq "last_error with a corrupt snapshot file" "$GUARD_ERROR" "$(field last_error)"
  assert_eq "snapshot files" "$before" "$(snapshot_files)"

  # The snapshots cannot be listed at all (an I/O error): blocked too, with the copy error.
  mkdir -p "$LOGS/bin"
  cat >"$LOGS/bin/restic" <<'FAKE'
#!/usr/bin/env bash
for a in "$@"; do
  case "$a" in
    snapshots | list) echo "made-up read error" >&2; exit 1 ;;
  esac
done
exec /usr/bin/restic "$@"
FAKE
  chmod +x "$LOGS/bin/restic"
  rc=0
  PATH="$LOGS/bin:$PATH" "$BACKUP" once >"$LOGS/out" 2>&1 || rc=$?
  assert_ne "once exit code when snapshots cannot be listed" 0 "$rc"
  assert_eq "last_error when snapshots cannot be listed" "copy: made-up read error" "$(field last_error)"
  assert_eq "snapshot files after the failed listing" "$before" "$(snapshot_files)"
  [ -e "$GUARD_FILE" ] || fail "$GUARD_FILE was removed"
}

# exits_2 <args...>: the mode exits 2 with one output line.
exits_2() {
  local rc=0 out
  out=$("$BACKUP" "$@" 2>&1) || rc=$?
  assert_eq "exit code of '$*'" 2 "$rc"
  assert_eq "output lines of '$*'" 1 "$(printf '%s\n' "$out" | wc -l)"
}

test_bad_args() {
  fresh
  exits_2 restore
  exits_2 snapshot-info
  exits_2 no-such-mode
  exits_2 once --tag something-else
  exits_2 restore a b
}

t "marker missing writes nothing to the target" test_marker_missing
t "first run initialises the repository and backs up" test_first_run
t "pre-upgrade tag is kept by forget" test_pre_upgrade_kept
t "an unreadable repository is never re-initialised" test_unreadable_repo
t "a repository folder without a config is never re-initialised" test_broken_repo
t "dump failure is recorded" test_dump_failure
t "the loop skips a recent backup and clears the error" test_loop_skips_recent
t "the loop skips while the lock is held; once waits" test_lock
t "an unusable lock file stops once and the loop" test_lock_unusable
t "TERM during a run writes interrupted" test_term_interrupted
t "snapshot-info prints id, time and app" test_snapshot_info
t "restore recreates the database and hands media to 65532" test_restore
t "fresh volumes with an existing repository take no snapshot" test_fresh_volumes_guard
t "restore lets backups resume" test_restore_resumes
t "a failed restore blocks the next backup" test_failed_restore_blocks_backup
t "an empty status file allows a fresh start" test_empty_status_fresh_start
t "the guard fails closed when the snapshots cannot be counted" test_guard_fails_closed
t "bad args exit 2" test_bad_args
finish
