#!/usr/bin/env bash
# backup-now.sh [--tag pre-upgrade] (design doc item 86, the notes in "Backups"): one backup now,
# through the backup service's `once` mode. It waits for the backup to finish (no polling, no
# timeout) and exits with its code. Runs from any folder: the stack is the one next to this script.
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

cd "$(dirname "$0")/.."
exec docker compose run --rm backup once "$@"
