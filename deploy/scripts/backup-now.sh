#!/usr/bin/env bash
# backup-now.sh [--tag pre-upgrade] (design doc item 86, the notes in "Backups"): one backup now,
# through the backup service's `once` mode, after ensure_release_images (lib.sh) for .env's
# version. It waits for the backup to finish (no polling, no timeout) and exits with its code.
# Runs from any folder: the stack is the one next to this script.
set -euo pipefail

usage() {
  echo "usage: backup-now.sh [--tag pre-upgrade]" >&2
  exit 2
}

case $# in
  0) ;;
  2) if [ "$1" != --tag ] || [ "$2" != pre-upgrade ]; then usage; fi ;;
  *) usage ;;
esac

die() {
  echo "backup-now.sh: $*" >&2
  exit 1
}

cd "$(dirname "$0")/.."
# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. scripts/lib.sh
check_env_file
# The backup image of .env's version: compose would pull it unchecked if it were not here.
ensure_release_images "$(env_app_version)" ||
  die "the images of .env's APP_VERSION are not verified; no backup was taken"
exec docker compose run --rm backup once "$@"
