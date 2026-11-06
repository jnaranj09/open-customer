#!/usr/bin/env bash
# The backup container's entry point (design doc item 85): "Backups" and the restore steps in
# "Upgrades and the migration guard".
#
#   backup.sh                              the hourly loop (the container's normal command)
#   backup.sh once [--tag pre-upgrade]     one backup now; exit 0 only when it succeeded
#   backup.sh snapshot-info <id|latest>    one JSON line {"id","time","app"}
#   backup.sh restore <snapshot-id>        recreate the database and the media folder from it
#
# Env: RESTIC_PASSWORD, APP_VERSION, PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE (from .env
# through Compose). Test-only knob, default off: BACKUP_INTERVAL_SECONDS (default 3600), the wait
# between two loop checks, so make test-deploy does not wait an hour.
#
# No `set -e`: each step's exit code is checked by hand, so a failure always reaches status.json.
set -uo pipefail

readonly MARKER=/target/.open-customer-backup-target
readonly REPO=/target/restic
readonly STATUS_DIR=/backup-status
readonly STATUS=$STATUS_DIR/status.json
# Present while these volumes must be restored before any backup (see fresh_volumes_guard).
readonly RESTORE_FIRST=$STATUS_DIR/restore-first
readonly RESTORE_FIRST_ERROR="Hay respaldos anteriores: restaure primero"
readonly LOCK=$STATUS_DIR/backup.lock
readonly WORK=$STATUS_DIR/work
readonly DUMP=$WORK/db.dump
readonly MEDIA=/media
# Fixed, because restic groups snapshots by host and every `docker compose run` gets a new one.
readonly HOST=open-customer-backup
readonly DAY_SECONDS=86400

log() {
  echo "$*"
}

usage() {
  echo "usage: backup.sh [once [--tag pre-upgrade] | snapshot-info <snapshot-id|latest> | restore <snapshot-id>]" >&2
  exit 2
}

# require <name...>: exits 2 naming the first variable that is unset or empty (never its value).
require() {
  local name
  for name in "$@"; do
    if [ -z "${!name:-}" ]; then
      echo "backup.sh: $name is not set" >&2
      exit 2
    fi
  done
}

now() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

# first_line <file>: the first line of a step's stderr, for status.json.
first_line() {
  local line
  line=$(head -n1 "$1" 2>/dev/null)
  echo "${line:-no error output}"
}

# write_status <field>=<value>...: sets those fields, and checked_at, then replaces status.json
# atomically (a temp file on the same volume, then mv). The value `now` is the current time and
# `null` clears the field. A missing or unreadable file counts as all fields null. The app reads
# the file as another user, so it is made world-readable.
write_status() {
  local current tmp pair args=()
  current=$(jq -ce 'if type == "object" then . else error end' "$STATUS" 2>/dev/null) || current='{}'
  for pair in "$@"; do
    args+=(--arg "${pair%%=*}" "${pair#*=}")
  done
  tmp=$(mktemp "$STATUS_DIR/.status.json.XXXXXX") || {
    log "status: cannot create a temp file in $STATUS_DIR"
    return 1
  }
  if jq -n --argjson cur "$current" --arg now "$(now)" --arg checked_at now "${args[@]}" '
      ($ARGS.named | del(.cur, .now)) as $set
      | reduce ($set | to_entries[]) as $f (
          {checked_at: null, last_dump_at: null, last_copy_at: null, last_snapshot_id: null,
           last_error: null, last_error_at: null} + $cur;
          .[$f.key] = (if $f.value == "now" then $now elif $f.value == "null" then null else $f.value end))
      | {checked_at, last_dump_at, last_copy_at, last_snapshot_id, last_error, last_error_at}' \
    >"$tmp" && chmod 0644 "$tmp" && mv -f "$tmp" "$STATUS"; then
    return 0
  fi
  rm -f "$tmp"
  log "status: cannot write $STATUS"
  return 1
}

# fail_status <message> [field=value...]: records a failed run.
fail_status() {
  local message=$1
  shift
  write_status "last_error=$message" last_error_at=now "$@"
}

# --- the trap ----------------------------------------------------------------------------------
# Bash runs a trap only after the foreground command ends, so a running step finishes first.
# PID 1 ignores signals it does not trap, so both are trapped. A backup run in progress records
# "interrupted"; the loop between runs stops quietly (exit 0); any other mode (once waiting for the
# lock, snapshot-info, restore) exits non-zero, so its caller never takes it for a success.

in_run=0
in_loop=0
sleep_pid=

on_signal() {
  if [ -n "$sleep_pid" ]; then
    kill "$sleep_pid" 2>/dev/null
  fi
  if [ "$in_run" -eq 1 ]; then
    rm -f "$STATUS_DIR"/.status.json.*
    fail_status interrupted
    log "interrupted"
    exit 143
  fi
  if [ "$in_loop" -eq 1 ]; then
    exit 0
  fi
  echo "backup.sh: interrupted" >&2
  exit 143
}
trap on_signal TERM INT

# --- one backup run (steps 1-6 of "Backups") -----------------------------------------------------

# backup_recent: true when the last successful copy is less than 24 h old (time math in jq, not
# busybox date).
backup_recent() {
  jq -e --argjson day "$DAY_SECONDS" \
    '(.last_copy_at | type == "string") and (now - (.last_copy_at | fromdateiso8601) < $day)' \
    "$STATUS" >/dev/null 2>&1
}

# fresh_volumes_guard: true when this run must take no snapshot. A run that starts without
# status.json is on new volumes (a new PC, or `docker compose down --volumes`); if the repository
# already has snapshots, backing up the empty database would make it `latest` and its forget
# could drop the day's real snapshot. The state is kept in $RESTORE_FIRST, because the run's own
# status.json write must not lift it. It is lifted by a missing or empty repository (a first
# install), by a successful restore, and by the operator replacing status.json with `{}` (a file
# without checked_at, which backup.sh always writes). An unreadable repository leaves it set and
# lets the run go on to the copy step, which reports that error.
fresh_volumes_guard() {
  if [ ! -e "$RESTORE_FIRST" ]; then
    return 1
  fi
  if [ -f "$STATUS" ] && ! jq -e 'has("checked_at")' "$STATUS" >/dev/null 2>&1; then
    rm -f "$RESTORE_FIRST"
    return 1
  fi
  local count
  if ! restic -r "$REPO" cat config >/dev/null 2>&1; then
    if [ ! -e "$REPO" ]; then
      rm -f "$RESTORE_FIRST"
    fi
    return 1
  fi
  if ! count=$(restic -r "$REPO" --no-lock snapshots --json 2>/dev/null | jq -e length); then
    return 1
  fi
  if [ "$count" -eq 0 ]; then
    rm -f "$RESTORE_FIRST"
    return 1
  fi
  return 0
}

# run_backup <check-age|always> [tag]: one run. The loop passes check-age (step 2 exists only
# there); once passes always. It returns the run's result and has always written status.json.
run_backup() {
  local age=$1 tag=${2:-} dump_at snapshot_id
  in_run=1

  # No status.json: new volumes. Recorded before step 1, so a missing target does not lose it.
  if [ ! -e "$STATUS" ]; then
    : >"$RESTORE_FIRST"
  fi

  # Step 1: the marker proves the target folder is the backup disk, not an empty mount point.
  if [ ! -e "$MARKER" ]; then
    log "marker: missing, nothing written to /target"
    fail_status "Disco de respaldo no visible"
    in_run=0
    return 1
  fi
  log "marker: ok"

  # Step 1b: earlier backups and new volumes; restore first.
  if fresh_volumes_guard; then
    log "guard: the repository has snapshots and these volumes have none; restore first"
    fail_status "$RESTORE_FIRST_ERROR"
    in_run=0
    return 1
  fi

  # Step 2.
  if [ "$age" = check-age ] && backup_recent; then
    log "skip: the last backup is less than 24 h old"
    write_status last_error=null last_error_at=null
    in_run=0
    return 0
  fi

  # Step 3.
  mkdir -p "$WORK"
  rm -f "$DUMP"
  if ! pg_dump -Fc -f "$DUMP" 2>"$WORK/dump.err"; then
    log "dump: failed"
    fail_status "dump: $(first_line "$WORK/dump.err")"
    in_run=0
    return 1
  fi
  dump_at=$(now)
  log "dump: ok"

  # Step 4. A folder whose config cannot be read (wrong password, broken repository) is never
  # initialised again: that would hide the real backups behind a new, empty repository.
  if ! restic -r "$REPO" cat config >/dev/null 2>"$WORK/copy.err"; then
    if [ -e "$REPO" ]; then
      log "copy: the repository cannot be read"
      fail_status "copy: $(first_line "$WORK/copy.err")" "last_dump_at=$dump_at"
      in_run=0
      return 1
    fi
    if ! restic -r "$REPO" init >/dev/null 2>"$WORK/copy.err"; then
      log "copy: init failed"
      fail_status "copy: $(first_line "$WORK/copy.err")" "last_dump_at=$dump_at"
      in_run=0
      return 1
    fi
    log "copy: repository initialised"
  fi

  local tags=(--tag "app=$APP_VERSION")
  if [ -n "$tag" ]; then
    tags+=(--tag "$tag")
  fi
  if ! restic -r "$REPO" backup --host "$HOST" "${tags[@]}" --json "$DUMP" "$MEDIA" \
    >"$WORK/copy.out" 2>"$WORK/copy.err"; then
    log "copy: backup failed"
    fail_status "copy: $(first_line "$WORK/copy.err")" "last_dump_at=$dump_at"
    in_run=0
    return 1
  fi
  snapshot_id=$(jq -r 'select(.message_type == "summary") | .snapshot_id' "$WORK/copy.out" 2>/dev/null)
  if [ -z "$snapshot_id" ]; then
    log "copy: no snapshot id in the restic output"
    fail_status "copy: no snapshot id in the restic output" "last_dump_at=$dump_at"
    in_run=0
    return 1
  fi
  log "copy: snapshot $snapshot_id"

  # Step 5.
  if ! restic -r "$REPO" forget --group-by paths --keep-daily 14 --keep-tag pre-upgrade --prune \
    >/dev/null 2>"$WORK/copy.err"; then
    log "forget: failed"
    fail_status "copy: $(first_line "$WORK/copy.err")" "last_dump_at=$dump_at"
    in_run=0
    return 1
  fi
  log "forget: ok"

  # Step 6.
  write_status "last_dump_at=$dump_at" last_copy_at=now "last_snapshot_id=$snapshot_id" \
    last_error=null last_error_at=null
  local rc=$?
  in_run=0
  return "$rc"
}

# --- modes --------------------------------------------------------------------------------------

# try_lock: takes the lock on fd 9 without waiting. Busybox flock exits 1 both when another run
# holds the lock and on a real error; only a real error prints to stderr, so that tells them
# apart. Returns 0 (taken), 1 (held by another run) or 2 (error, first stderr line in lock_error).
lock_error=
try_lock() {
  if flock -n 9 2>"$STATUS_DIR/lock.err"; then
    return 0
  fi
  if [ -s "$STATUS_DIR/lock.err" ]; then
    lock_error=$(first_line "$STATUS_DIR/lock.err")
    return 2
  fi
  return 1
}

# take_lock: waits for the lock on fd 9, saying so when another run holds it. Exits non-zero when
# the lock cannot be taken, so once and restore never run without it.
take_lock() {
  local rc=0
  try_lock || rc=$?
  if [ "$rc" -eq 1 ]; then
    log "lock: another backup or restore is running, waiting"
    flock 9 2>"$STATUS_DIR/lock.err" || rc=2
    if [ "$rc" -eq 2 ]; then
      lock_error=$(first_line "$STATUS_DIR/lock.err")
    else
      rc=0
    fi
  fi
  if [ "$rc" -ne 0 ]; then
    echo "lock: cannot take $LOCK: $lock_error" >&2
    exit 1
  fi
}

mode_loop() {
  local interval=${BACKUP_INTERVAL_SECONDS:-3600}
  if ! [[ $interval =~ ^[1-9][0-9]*$ ]]; then
    echo "backup.sh: BACKUP_INTERVAL_SECONDS must be a positive number of seconds" >&2
    exit 2
  fi
  log "$(restic version)"
  in_loop=1
  while true; do
    # A once run or a restore holds the lock: this hour is skipped, status.json untouched.
    local rc=0
    try_lock || rc=$?
    case "$rc" in
      0)
        run_backup check-age
        flock -u 9
        ;;
      1)
        log "lock held by another run, skipping this check"
        ;;
      *)
        log "lock: cannot take $LOCK"
        fail_status "lock: $lock_error"
        ;;
    esac
    sleep "$interval" &
    sleep_pid=$!
    wait "$sleep_pid"
    sleep_pid=
  done
}

mode_once() {
  log "$(restic version)"
  take_lock
  run_backup always "$1"
  exit $?
}

mode_snapshot_info() {
  local out
  # The restore service mounts /target read-only, so restic cannot write a lock file there.
  if ! out=$(restic -r "$REPO" --no-lock snapshots --json "$1" 2>"/tmp/snapshot-info.err"); then
    echo "snapshot-info: $(first_line /tmp/snapshot-info.err)" >&2
    exit 1
  fi
  if ! jq -ce 'if length == 1 then .[0] else error("not one snapshot") end
      | {id, time, app: ([.tags[]? | select(startswith("app=")) | ltrimstr("app=")][0])}' \
    <<<"$out" 2>/dev/null; then
    echo "snapshot-info: no single snapshot matches $1" >&2
    exit 1
  fi
}

# restore_fail <message>: removes the unpacked snapshot and exits non-zero.
restore_fail() {
  rm -rf "$WORK/restore"
  echo "restore: $1" >&2
  exit 1
}

mode_restore() {
  local id=$1 out=$WORK/restore
  take_lock
  mkdir -p "$WORK"
  rm -rf "$out"
  # On the backup-status volume, not the container layer.
  if ! restic -r "$REPO" --no-lock restore "$id" --target "$out" >/dev/null 2>"$WORK/restore.err"; then
    restore_fail "restic: $(first_line "$WORK/restore.err")"
  fi
  if [ ! -f "$out$DUMP" ] || [ ! -d "$out$MEDIA" ]; then
    restore_fail "the snapshot has no database dump or media folder"
  fi
  log "restore: snapshot $id unpacked"

  # Dropping the whole database also removes tables newer migrations added, which
  # pg_restore --clean would leave behind; the new database is empty, so no --clean.
  if ! dropdb --force --if-exists "$PGDATABASE" 2>"$WORK/restore.err" ||
    ! createdb "$PGDATABASE" 2>>"$WORK/restore.err"; then
    restore_fail "database: $(first_line "$WORK/restore.err")"
  fi
  if ! pg_restore --single-transaction -d "$PGDATABASE" "$out$DUMP" 2>"$WORK/restore.err"; then
    restore_fail "pg_restore: $(first_line "$WORK/restore.err")"
  fi
  log "restore: database loaded"

  if ! find "$MEDIA" -mindepth 1 -delete 2>"$WORK/restore.err" ||
    ! cp -a "$out$MEDIA/." "$MEDIA/" 2>"$WORK/restore.err" ||
    ! chown -R 65532:65532 "$MEDIA" 2>"$WORK/restore.err"; then
    restore_fail "media: $(first_line "$WORK/restore.err")"
  fi
  log "restore: media replaced, owned by 65532:65532"
  rm -rf "$out"
  # The hourly backups resume: the guard is lifted and status.json names the restored snapshot.
  # The dump and copy times keep their values (null on new volumes): no copy was made here.
  rm -f "$RESTORE_FIRST"
  if ! write_status "last_snapshot_id=$id" last_error=null last_error_at=null; then
    restore_fail "the data is restored, but $STATUS cannot be written"
  fi
  log "restore: done"
}

# --- arguments and env, checked before anything runs -------------------------------------------

mode=${1:-loop}
case "$mode" in
  loop)
    [ $# -eq 0 ] || usage
    ;;
  once)
    case $# in
      1) once_tag= ;;
      3) [ "$2" = --tag ] && [ "$3" = pre-upgrade ] || usage
         once_tag=pre-upgrade ;;
      *) usage ;;
    esac
    ;;
  snapshot-info | restore)
    [ $# -eq 2 ] && [ -n "$2" ] || usage
    ;;
  *)
    usage
    ;;
esac

case "$mode" in
  loop | once) require RESTIC_PASSWORD APP_VERSION PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE ;;
  snapshot-info) require RESTIC_PASSWORD ;;
  restore) require RESTIC_PASSWORD PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE ;;
esac

case "$mode" in
  loop | once | restore)
    if ! mkdir -p "$STATUS_DIR" 2>/dev/null; then
      echo "backup.sh: cannot create $STATUS_DIR" >&2
      exit 1
    fi
    # Without the lock file nothing may run: two runs could touch the dump and the repository.
    if ! { exec 9>"$LOCK"; } 2>/dev/null; then
      echo "backup.sh: cannot open the lock file $LOCK" >&2
      exit 1
    fi
    ;;
esac

case "$mode" in
  loop) mode_loop ;;
  once) mode_once "$once_tag" ;;
  snapshot-info) mode_snapshot_info "$2" ;;
  restore) mode_restore "$2" ;;
esac
