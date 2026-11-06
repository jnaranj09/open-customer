#!/usr/bin/env bash
# restore.sh <snapshot-id|latest> (design doc item 86, the restore steps in "Upgrades and the
# migration guard"): shows the snapshot and asks first, stops app and backup, restores the
# database and the media folder, sets APP_VERSION in .env to the snapshot's version and starts the
# stack. A failure after the stop leaves app and backup stopped and says so. Runs from any folder:
# the stack is the one next to this script.
set -euo pipefail

usage() {
  echo "usage: restore.sh <snapshot-id|latest>" >&2
  exit 2
}

die() {
  echo "restore.sh: $*" >&2
  exit 1
}

# After step 2: nothing may start again on a half-restored database.
die_stopped() {
  die "$*; app and backup are stopped. Fix the cause and run restore.sh again"
}

[ $# -eq 1 ] || usage
[[ $1 =~ ^(latest|[0-9a-f]{8,64})$ ]] || {
  echo "restore.sh: the snapshot must be 'latest' or a snapshot id (hex)" >&2
  exit 2
}

cd "$(dirname "$0")/.."
[ -f .env ] || die "no .env next to compose.yaml"
[ "$(grep -c '^APP_VERSION=' .env)" -eq 1 ] || die ".env must have exactly one APP_VERSION= line"

# set_app_version <version>: rewrites the APP_VERSION line of .env in place (same file, so its
# owner and mode stay); every other line is kept as is.
set_app_version() {
  local tmp
  # Checked by hand: a caller's `||` turns `set -e` off in here, and an empty temp file must
  # never replace .env.
  tmp=$(umask 077 && mktemp .env.XXXXXX) || return 1
  if ! sed "s/^APP_VERSION=.*/APP_VERSION=$1/" .env >"$tmp" || ! grep -qx "APP_VERSION=$1" "$tmp"; then
    rm -f "$tmp"
    return 1
  fi
  cat "$tmp" >.env
  rm -f "$tmp"
}

# 1. One JSON line {"id","time","app"} from the backup image (jq is not needed on the host).
info=$(docker compose --profile restore run --rm restore snapshot-info "$1") ||
  die "cannot read snapshot $1; nothing was stopped"
info=$(printf '%s\n' "$info" | tail -n1)
id=$(sed -n 's/.*"id":"\([0-9a-f]*\)".*/\1/p' <<<"$info")
time=$(sed -n 's/.*"time":"\([0-9T:.Z+-]*\)".*/\1/p' <<<"$info")
version=$(sed -n 's/.*"app":"\([^"]*\)".*/\1/p' <<<"$info")
[[ $id =~ ^[0-9a-f]{8,64}$ ]] && [ -n "$time" ] || die "unexpected snapshot-info output; nothing was stopped"
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]] ||
  die "snapshot $id has no app version tag; nothing was stopped"

printf 'Restore snapshot %s from %s (app %s)? [y/N] ' "$id" "$time" "$version"
answer=
read -r answer || true
[ "$answer" = y ] || die "cancelled; nothing was stopped"

# 2. Nothing writes to the database and no hourly backup starts.
docker compose stop app backup ||
  die "docker compose stop app backup failed; nothing was restored, check which services still run"

# 3. Takes the backup lock, recreates the database and replaces the media folder.
docker compose --profile restore run --rm restore restore "$id" || die_stopped "the restore failed"

# 4. The restored schema and the app version match.
set_app_version "$version" || die_stopped "cannot set APP_VERSION in .env"

# 5. app on the restored version; backup recreated with it, so later snapshots carry the right tag.
docker compose up -d || die_stopped "docker compose up -d failed"
echo "restore.sh: restored snapshot $id (app $version)"
