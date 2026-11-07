#!/usr/bin/env bash
# make check-pins (design doc "Pins"): files that cannot read the Makefile repeat a pin; fail if any
# copy differs from its source. PG_IMAGE and GITLEAKS_IMAGE come from make, which reads them from
# security/tools.Dockerfile. Scans only the files below, each once it exists; never docs/ or *.md.
# The cosign pin is the exception to "each once it exists": its two copies are checked as a pair.
set -euo pipefail

cd "$(dirname "$0")/.."

: "${PG_IMAGE:?set by make}"
: "${GOLANGCI_LINT_VERSION:?set by make}"
: "${GITLEAKS_IMAGE:?set by make}"

fail=0
bad() {
  echo "check-pins: $1" >&2
  fail=1
}

node_pin=$(sed -n 's/.*"node": "\(.*\)".*/\1/p' web/package.json)
pnpm_pin=$(sed -n 's/.*"packageManager": "pnpm@\(.*\)".*/\1/p' web/package.json)
go_pin=$(sed -n 's/^toolchain go\(.*\)$/\1/p' go.mod)
[ -n "$node_pin" ] || bad "web/package.json has no engines.node"
[ -n "$pnpm_pin" ] || bad "web/package.json has no packageManager pnpm@<version>"
[ -n "$go_pin" ] || bad "go.mod has no toolchain line"

# PG_IMAGE is <tag>@sha256:<digest>, and every postgres: reference, digest included, equals it: a
# copy with another digest, or none, pulls a different image under the same tag.
[[ "$PG_IMAGE" =~ ^postgres:[0-9][A-Za-z0-9._-]*@sha256:[0-9a-f]{64}$ ]] ||
  bad "PG_IMAGE (security/tools.Dockerfile) '$PG_IMAGE' is not postgres:<tag>@sha256:<digest>"
for f in deploy/compose.yaml deploy/backup/Dockerfile .github/workflows/ci.yml Dockerfile; do
  [ -f "$f" ] || continue
  while read -r ref; do
    [ "$ref" = "$PG_IMAGE" ] || bad "$f uses $ref, PG_IMAGE (security/tools.Dockerfile) is $PG_IMAGE"
  done < <(grep -oE 'postgres:[0-9][A-Za-z0-9._-]*(@sha256:[0-9a-f]*)?' "$f" || true)
done

if [ -f Dockerfile ]; then
  node_tag=$(grep -oE 'FROM node:[^ ]+' Dockerfile | sed 's/FROM node:\(.*\)-alpine.*/\1/' || true)
  go_tag=$(grep -oE 'FROM golang:[^ ]+' Dockerfile | sed 's/FROM golang:\(.*\)-alpine.*/\1/' || true)
  pnpm_tag=$(grep -oE 'pnpm@[0-9][A-Za-z0-9.-]*' Dockerfile | sed 's/pnpm@//' || true)
  [ "$node_tag" = "$node_pin" ] || bad "Dockerfile node:$node_tag, web/package.json engines.node is $node_pin"
  [ "$go_tag" = "$go_pin" ] || bad "Dockerfile golang:$go_tag, go.mod toolchain is go$go_pin"
  [ "$pnpm_tag" = "$pnpm_pin" ] || bad "Dockerfile pnpm@$pnpm_tag, web/package.json packageManager is pnpm@$pnpm_pin"
fi

if [ -f .github/workflows/ci.yml ]; then
  # The version: line right after the golangci-lint action's `with:`.
  ci_lint=$(grep -A3 'golangci/golangci-lint-action' .github/workflows/ci.yml | sed -n 's/^ *version: *//p' | head -1)
  [ "$ci_lint" = "$GOLANGCI_LINT_VERSION" ] ||
    bad "ci.yml golangci-lint version '$ci_lint', Makefile GOLANGCI_LINT_VERSION is $GOLANGCI_LINT_VERSION"
  # The GITLEAKS_VERSION env of the gitleaks action step equals GITLEAKS_IMAGE's tag (without its v
  # and its digest).
  ci_gitleaks=$(grep -A6 'gitleaks/gitleaks-action' .github/workflows/ci.yml | sed -n 's/^ *GITLEAKS_VERSION: *\([^ #]*\).*/\1/p' | head -1)
  gitleaks_tag=${GITLEAKS_IMAGE%@*}
  [ "$ci_gitleaks" = "${gitleaks_tag##*:v}" ] ||
    bad "ci.yml gitleaks GITLEAKS_VERSION '$ci_gitleaks', GITLEAKS_IMAGE (security/tools.Dockerfile) is $GITLEAKS_IMAGE"
fi

# The cosign version upgrade.sh verifies with (COSIGN_IMAGE in deploy/scripts/lib.sh, which
# deployers have without the Makefile) equals the `cosign-release` the release workflow signs with.
# Dependabot reads neither, so both are bumped by hand in one commit.
if [ -f .github/workflows/release.yml ] || [ -f deploy/scripts/lib.sh ]; then
  release_cosign=$(sed -n 's/^ *cosign-release: *\([^ #]*\).*/\1/p' .github/workflows/release.yml 2>/dev/null | head -1 || true)
  lib_cosign=$(sed -n 's/^COSIGN_IMAGE=[^:]*:\([^@]*\)@.*/\1/p' deploy/scripts/lib.sh 2>/dev/null | head -1 || true)
  [ -n "$lib_cosign" ] || bad "deploy/scripts/lib.sh has no COSIGN_IMAGE=<image>:<tag>@sha256:<digest>"
  [ -n "$release_cosign" ] || bad ".github/workflows/release.yml has no cosign-release"
  [ "$release_cosign" = "$lib_cosign" ] ||
    bad "release.yml cosign-release '$release_cosign', deploy/scripts/lib.sh COSIGN_IMAGE is cosign '$lib_cosign'"
fi

[ "$fail" -eq 0 ] && echo "check-pins: ok"
exit "$fail"
