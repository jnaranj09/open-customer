#!/usr/bin/env bash
# The release workflow's first step after checkout (docs/designs/devsecops.md section 4): refuses a
# v* tag whose commit is not an ancestor of main, so a tag on a side branch never builds, scans,
# pushes or signs anything. The repo's tag ruleset only limits who can create tags; this is the
# check that tests the commit. Limit: an ancestor of main has not necessarily passed CI.
#
# Fails closed. It is red when GITHUB_SHA is missing or not a commit in the clone, when the clone
# is shallow (the ancestry cannot be decided; check out with fetch-depth: 0), when the fetch of
# main fails, and when the commit is not an ancestor. Needs the clone's `origin` to answer for main
# (a public repo needs no credential). Exit 0 only for an ancestor, 1 otherwise.
set -euo pipefail

die() {
  echo "release-on-main: $*" >&2
  exit 1
}

if [ "$#" -ne 0 ]; then
  echo "usage: release-on-main.sh (env GITHUB_SHA; run inside the checkout)" >&2
  exit 2
fi
sha=${GITHUB_SHA:-}
[[ $sha =~ ^[0-9a-f]{40}$ ]] || die "GITHUB_SHA is missing or is not a full commit id"

shallow=$(git rev-parse --is-shallow-repository) || die "cannot tell whether the clone is shallow"
[ "$shallow" = false ] || die "the clone is shallow, so the ancestry of $sha cannot be decided (checkout with fetch-depth: 0)"

git fetch --quiet --no-tags origin '+refs/heads/main:refs/remotes/origin/main' || die "cannot fetch main from origin"
[ "$(git rev-parse --is-shallow-repository)" = false ] || die "the clone is shallow after the fetch"
git rev-parse --verify --quiet "refs/remotes/origin/main^{commit}" >/dev/null || die "origin/main is not a commit after the fetch"
git cat-file -e "$sha^{commit}" 2>/dev/null || die "$sha is not a commit in this clone"

rc=0
git merge-base --is-ancestor "$sha" refs/remotes/origin/main || rc=$?
case "$rc" in
  0) echo "release-on-main: $sha is on main" ;;
  1) die "$sha is not an ancestor of main: a release is built only from a commit on main" ;;
  *) die "git merge-base failed (exit $rc), so the ancestry of $sha is unknown" ;;
esac
