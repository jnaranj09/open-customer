#!/usr/bin/env bash
# The weekly rescan's tag (docs/designs/devsecops.md section 4; .github/workflows/security-weekly.yml):
# prints the newest release tag of the checkout, vX.Y.Z by version order, or nothing when there is
# none. A pre-release tag (v1.3.0-rc1, anything after X.Y.Z) is skipped: deployers upgrade to the
# newest release, so that is the one the rescan must cover, and `--sort=-v:refname` alone sorts
# v1.3.0-rc1 above v1.2.0 (and above v1.3.0 itself). Run inside a clone that has the tags. Exit 0
# with or without a release tag; non-zero only when git fails.
set -euo pipefail

if [ "$#" -ne 0 ]; then
  echo "usage: latest-release.sh (run inside the checkout)" >&2
  exit 2
fi
# The list is read whole before it is filtered, so no early exit can break the git pipe.
tags=$(git tag --list 'v*' --sort=-v:refname)
awk '/^v[0-9]+\.[0-9]+\.[0-9]+$/ { print; exit }' <<<"$tags"
