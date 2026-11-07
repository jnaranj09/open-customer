#!/usr/bin/env bash
# make sast (docs/designs/devsecops.md section 2, decision D2): Semgrep with only the rules written
# in security/semgrep, one file per rule, each with an annotated fixture in security/semgrep/testdata.
# No registry rule is ever fetched: the container has --network none, --metrics=off and
# --disable-version-check, and --config names the repo directory.
#
# Two runs, both in the pinned image: `semgrep --test` (every rule file has a fixture with at least
# one `ruleid:` line, and every annotation matches what the rule reports), then the scan of the
# repo (what is skipped is listed in .semgrepignore). The live `semgrep` ids of security/allow.json
# (allow.sh) become --exclude-rule; an id must be the plain id of a rule in security/semgrep (Semgrep
# prefixes it with the config path: security.semgrep.<id>, which the script adds), so a typo or a
# removed rule is refused instead of silently excluding nothing.
#
# Fail closed: any exit other than 0 (clean) and 1 (findings, --error) is red; so is a JSON report
# that is missing, lists an error, lists no scanned Go, TypeScript and Svelte file (a scan of 0
# files reads as clean otherwise), or disagrees with the exit code. SARIF_DIR=<dir> (relative to the
# repo root) also writes <dir>/semgrep.sarif from the same run. Exit 2 on bad arguments, 1 when red.
set -euo pipefail

if [ "$#" -ne 0 ]; then
  echo "usage: sast.sh (no arguments; env SEMGREP_IMAGE; optional SARIF_DIR)" >&2
  exit 2
fi
if [ -z "${SEMGREP_IMAGE:-}" ]; then
  echo "sast: SEMGREP_IMAGE is not set (run through make sast)" >&2
  exit 2
fi

cd "$(dirname "$0")/../.."

rules_dir=security/semgrep
tests_dir=$rules_dir/testdata
# The prefix Semgrep puts before a rule id when --config is this directory, run from /src.
id_prefix=security.semgrep.

sarif_dir=
if [ -n "${SARIF_DIR:-}" ]; then
  mkdir -p "$SARIF_DIR"
  sarif_dir=$(cd "$SARIF_DIR" && pwd)
fi

test_container=oc-sast-test
scan_container=oc-sast-scan
work=$(mktemp -d)
cleanup() {
  docker rm -f "$test_container" "$scan_container" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
mkdir "$work/out"
chmod 777 "$work/out"

red=0
red() {
  echo "sast: $*" >&2
  red=$((red + 1))
}

# The ids of the rules in security/semgrep (a line "  - id: <id>" of each rule file).
rule_ids=$(sed -n 's/^  - id: \([a-z0-9-]*\)$/\1/p' "$rules_dir"/*.yaml | LC_ALL=C sort)

exclude=()
allowed=$(scripts/security/allow.sh semgrep)
if [ -n "$allowed" ]; then
  while read -r id; do
    if ! [[ "$id" =~ ^[a-z0-9-]+$ ]] || ! grep -qxF -- "$id" <<<"$rule_ids"; then
      echo "sast: security/allow.json has a semgrep id that is not the id of a rule in $rules_dir" >&2
      exit 1
    fi
    exclude+=(--exclude-rule "$id_prefix$id")
  done <<<"$allowed"
fi

# semgrep <container name> <output dir mounted at /out> <semgrep args...>: one run in the image.
semgrep() {
  local name=$1
  shift
  docker run --rm --name "$name" --network none -u "$(id -u):$(id -g)" -e HOME=/tmp \
    -v "$PWD:/src:ro" -v "$work/out:/out" -w /src \
    "$SEMGREP_IMAGE" semgrep "$@" --metrics=off --disable-version-check
}

echo "== sast: semgrep --test $rules_dir"
rc=0
semgrep "$test_container" --test --json --config "$rules_dir" "$tests_dir" >"$work/out/test.json" || rc=$?
if [ "$rc" -ne 0 ] || ! jq -e . "$work/out/test.json" >/dev/null 2>&1; then
  red "semgrep --test failed or its fixtures do not match the rules (exit $rc)"
  jq -r '.results[]?.checks | to_entries[] | select(.value.passed != true) | "  rule \(.key): fixture lines do not match"' \
    "$work/out/test.json" 2>/dev/null || true
else
  # Every rule file is tested, with fixtures that expect at least one finding.
  tested=$(jq -r '[.results[]?.checks | to_entries[] | select(.value.passed == true and ([.value.matches[]?.expected_lines[]?] | length) > 0) | .key] | sort[]' "$work/out/test.json")
  if [ "$tested" != "$rule_ids" ] || [ -z "$rule_ids" ] ||
    [ "$(jq '[.config_missing_tests, .config_with_errors] | flatten | length' "$work/out/test.json")" -ne 0 ]; then
    red "every rule in $rules_dir needs a fixture in $tests_dir with a ruleid: line (rules: $(tr '\n' ' ' <<<"$rule_ids")| tested: $(tr '\n' ' ' <<<"$tested"))"
  else
    echo "sast: $(wc -l <<<"$rule_ids") rules, each tested"
  fi
fi

echo "== sast: semgrep scan"
args=(scan --config "$rules_dir" "${exclude[@]}" --error --json-output=/out/semgrep.json)
[ -z "$sarif_dir" ] || args+=(--sarif-output=/out/semgrep.sarif)
rc=0
semgrep "$scan_container" "${args[@]}" . || rc=$?
report=$work/out/semgrep.json
if { [ "$rc" -ne 0 ] && [ "$rc" -ne 1 ]; } || ! jq -e . "$report" >/dev/null 2>&1; then
  red "semgrep could not scan the repo (exit $rc)"
else
  nerr=$(jq '.errors | length' "$report")
  nres=$(jq '.results | length' "$report")
  if [ "$nerr" -ne 0 ]; then
    jq -r '.errors[] | "  \(.type // "error"): \(.path // "")"' "$report" >&2
    red "semgrep reported $nerr errors"
  fi
  for ext in go ts svelte; do
    if [ "$(jq --arg e ".$ext" '[.paths.scanned[]? | select(endswith($e))] | length' "$report")" -eq 0 ]; then
      red "semgrep scanned no .$ext file (check .semgrepignore)"
    fi
  done
  if [ "$nres" -gt 0 ]; then
    jq -r '.results[] | "  \(.path):\(.start.line) \(.check_id | sub("^security\\.semgrep\\."; "")): \(.extra.message | gsub("\\s+"; " "))"' "$report" >&2
    red "$nres findings"
  elif [ "$rc" -eq 1 ]; then
    red "semgrep exited 1 (findings) but its report lists none"
  fi
  if [ "$nres" -eq 0 ] && [ "$rc" -eq 0 ] && [ "$nerr" -eq 0 ]; then
    echo "sast: $(jq '.paths.scanned | length' "$report") files scanned, no findings"
  fi
fi
if [ -n "$sarif_dir" ]; then
  if [ -s "$work/out/semgrep.sarif" ]; then
    cp "$work/out/semgrep.sarif" "$sarif_dir/semgrep.sarif"
  else
    red "semgrep wrote no SARIF file"
  fi
fi

if [ "$red" -gt 0 ]; then
  echo "sast: $red checks red" >&2
  exit 1
fi
echo "sast: ok"
