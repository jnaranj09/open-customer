#!/usr/bin/env bash
# upgrade.sh <version> (design doc item 86, the five steps in "Upgrades and the migration guard"):
# checks .env against the running app, pulls the new images, takes a pre-upgrade backup, then
# sets APP_VERSION in .env and restarts the stack. Any failure stops it; until step 4, .env and
# the running containers stay on the old version. Runs from any folder: the stack is the one next
# to this script.
set -euo pipefail

usage() {
  echo "usage: upgrade.sh <version>   (for example 1.2.3)" >&2
  exit 2
}

die() {
  echo "upgrade.sh: $*" >&2
  exit 1
}

[ $# -eq 1 ] || usage
new=$1
[[ $new =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]] || {
  echo "upgrade.sh: the version must look like 1.2.3" >&2
  exit 2
}

cd "$(dirname "$0")/.."
# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. scripts/lib.sh
check_env_file
current=$(env_app_version)

# 1. A retried upgrade may have changed .env already; the pre-upgrade snapshot would then carry
# the wrong version tag.
running=$(docker compose exec app open-customer version) || die "cannot read the running app's version (is app up?)"
running=$(printf '%s' "$running" | tr -d '[:space:]')
[ "$running" = "$current" ] ||
  die "APP_VERSION in .env is '$current' but the running app is '$running'; fix .env first"

# 2. The new images; .env is not changed yet.
APP_VERSION=$new docker compose pull || die "pull failed; nothing changed"

# 3. The backup the migration guard asks for.
scripts/backup-now.sh --tag pre-upgrade ||
  die "the pre-upgrade backup failed; .env and the running containers stay on $current"

# 4 and 5.
set_app_version "$new" || die "cannot set APP_VERSION in .env; the containers stay on $current"
docker compose up -d || die "docker compose up -d failed; .env is already on $new"
echo "upgrade.sh: upgraded from $current to $new"
