#!/usr/bin/env bash
# pull.sh [<version>] (docs/designs/devsecops.md section 4): brings the app and backup images of
# .env's APP_VERSION onto this PC through the signature check (pull_verified in lib.sh: cosign
# verify of both, `docker compose pull`, then each pulled image must be the verified digest). Run
# it before the first `docker compose up -d` (deploy/README.md section 6), and before any
# `docker compose` command by hand on a PC that does not have those images yet: compose pulls a
# missing image without the check. upgrade.sh, restore.sh and backup-now.sh do this themselves.
# A <version> must equal .env's APP_VERSION, so the later `up -d` runs exactly what this checked.
# Runs from any folder: the stack is the one next to this script.
set -euo pipefail

usage() {
  echo "usage: pull.sh [<version>]   (default: APP_VERSION in .env)" >&2
  exit 2
}

die() {
  echo "pull.sh: $*" >&2
  exit 1
}

[ $# -le 1 ] || usage
if [ $# -eq 1 ] && ! [[ $1 =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "pull.sh: the version must look like 1.2.3" >&2
  exit 2
fi

cd "$(dirname "$0")/.."
# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. scripts/lib.sh
check_env_file
version=$(env_app_version)
if [ $# -eq 1 ] && [ "$1" != "$version" ]; then
  die "APP_VERSION in .env is '$version', not '$1'; set it in .env first, so" \
    "docker compose up -d runs the images this checks"
fi
pull_verified "$version" || die "the images of $version are not verified; do not start the stack"
echo "pull.sh: the app and backup images $version are signed by the release workflow and pulled"
