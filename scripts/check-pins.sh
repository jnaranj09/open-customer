#!/usr/bin/env bash
# make check-pins (design doc "Pins"): files that cannot read the Makefile repeat a pin; fail if any
# copy differs from its source. Scans only the files below, each once it exists; never docs/ or *.md.
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
  bad "Makefile PG_IMAGE '$PG_IMAGE' is not postgres:<tag>@sha256:<digest>"
for f in deploy/compose.yaml deploy/backup/Dockerfile .github/workflows/ci.yml Dockerfile; do
  [ -f "$f" ] || continue
  while read -r ref; do
    [ "$ref" = "$PG_IMAGE" ] || bad "$f uses $ref, Makefile PG_IMAGE is $PG_IMAGE"
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
    bad "ci.yml gitleaks GITLEAKS_VERSION '$ci_gitleaks', Makefile GITLEAKS_IMAGE is $GITLEAKS_IMAGE"
fi

[ "$fail" -eq 0 ] && echo "check-pins: ok"
exit "$fail"
