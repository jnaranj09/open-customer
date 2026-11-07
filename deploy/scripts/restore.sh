#!/usr/bin/env bash
# restore.sh <snapshot-id|latest> (design doc item 86, the restore steps in "Upgrades and the
# migration guard"): shows the snapshot and asks first, makes sure the images of .env's and of the
# snapshot's version are on this PC (pulled through the signature check when not; a failure stops
# before anything is stopped), stops app and backup, restores the database and the media folder,
# sets APP_VERSION in .env to the snapshot's version and starts the stack. A failure after the
# stop leaves app and backup stopped and says so. Runs from any folder: the stack is the one next
# to this script.
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
# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. scripts/lib.sh
check_env_file

# 0. snapshot-info and the restore run the backup image of .env's version; on a new PC it is not
# here yet, and compose would pull it unchecked.
ensure_release_images "$(env_app_version)" ||
  die "the images of .env's APP_VERSION are not verified; nothing was stopped"

# 1. One JSON line {"id","time","app"} from the backup image (jq is not needed on the host).
info=$(docker compose --profile restore run --rm restore snapshot-info "$1") ||
  die "cannot read snapshot $1; nothing was stopped"
info=$(printf '%s\n' "$info" | tail -n1)
id=$(sed -n 's/.*"id":"\([0-9a-f]*\)".*/\1/p' <<<"$info")
time=$(sed -n 's/.*"time":"\([0-9T:.Z+-]*\)".*/\1/p' <<<"$info")
version=$(sed -n 's/.*"app":"\([^"]*\)".*/\1/p' <<<"$info")
if ! [[ $id =~ ^[0-9a-f]{8,64}$ ]] || [ -z "$time" ]; then
  die "unexpected snapshot-info output; nothing was stopped"
fi
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]] ||
  die "snapshot $id has no app version tag; nothing was stopped"

printf 'Restore snapshot %s from %s (app %s)? [y/N] ' "$id" "$time" "$version"
answer=
read -r answer || true
[ "$answer" = y ] || die "cancelled; nothing was stopped"

# The images `up -d` will run at the snapshot's version: checked before anything is stopped or
# changed, so a failed signature check leaves the stack as it was.
ensure_release_images "$version" ||
  die "the images of app $version are not verified; nothing was stopped"

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
