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
[ -f .env ] || die "no .env next to compose.yaml"
[ "$(grep -c '^APP_VERSION=' .env)" -eq 1 ] || die ".env must have exactly one APP_VERSION= line"
current=$(sed -n 's/^APP_VERSION=//p' .env)

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
