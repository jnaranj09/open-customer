#!/usr/bin/env bash
# make config-scan (docs/designs/devsecops.md section 3 and "Blocking policy", decision D1): Trivy
# config on the two Dockerfiles (Dockerfile, deploy/backup/Dockerfile) and zizmor on
# .github/workflows. deploy/compose.yaml is checked by the Go guard TestComposeHardening instead,
# because neither tool reads Compose files.
#
# Both tools run with --network none: Trivy with the checks built into its pinned image
# (--skip-check-update, never a bundle downloaded at run time), zizmor with --offline. The live
# trivy-config and zizmor ids of security/allow.json (allow.sh) go to a temp .trivyignore.yaml and a
# temp zizmor config. A trivy-config id is a check id (the check is ignored in every Dockerfile) or
# <id>@<repo path of a scanned Dockerfile> (ignored in that file only, through the ignore entry's
# Trivy `paths:`); any other shape is refused.
#
# Every Dockerfile in the repo (Dockerfile, *.Dockerfile) must be one it scans or one it lists as
# never built (security/tools.Dockerfile, which only holds pins); any other is red.
#
# Trivy runs once on /src, where each Dockerfile is mounted at its repo path, so each result's
# Target is the repo path that `paths:` matches. A high or critical misconfiguration is red, named
# by file. Trivy exits 1 on its own errors whatever --exit-code says, so findings use exit code 5
# and any other non-zero exit is red; its JSON report must list every Dockerfile as scanned, so a
# file Trivy did not recognise never reads as clean, and an exit that disagrees with the report is
# red.
# zizmor exits 11 to 14 for its highest finding (informational, low, medium, high): 13 and 14 are
# red, 11 and 12 are reported only; 1 (error), 2 (bad arguments), 3 (no inputs) and any other
# exit are red. SARIF_DIR=<dir> (relative to the repo root) also writes <dir>/zizmor.sarif; the
# SARIF run always exits 0, so it never decides the result. Every check runs before the script
# exits. Exit 2 on bad arguments, 1 when any check is red.
set -euo pipefail

usage="usage: config-scan.sh (no arguments; env TRIVY_IMAGE, ZIZMOR_IMAGE; optional SARIF_DIR)"
if [ "$#" -ne 0 ]; then
  echo "$usage" >&2
  exit 2
fi
for var in TRIVY_IMAGE ZIZMOR_IMAGE; do
  if [ -z "${!var:-}" ]; then
    echo "config-scan: $var is not set (run through make config-scan)" >&2
    exit 2
  fi
done

cd "$(dirname "$0")/../.."

# Trivy's exit code for findings (--exit-code); see the header for why it is not 1.
findings_exit=5
# The Dockerfiles it scans, and the ones it does not because they are never built (the pin file
# Dependabot reads). Any other Dockerfile in the repo is red until it is in one of the two lists.
dockerfiles=(Dockerfile deploy/backup/Dockerfile)
never_built=(security/tools.Dockerfile)

sarif_dir=
if [ -n "${SARIF_DIR:-}" ]; then
  mkdir -p "$SARIF_DIR"
  sarif_dir=$(cd "$SARIF_DIR" && pwd)
fi

trivy_container=oc-config-scan-trivy
zizmor_container=oc-config-scan-zizmor
work=$(mktemp -d)
cleanup() {
  docker rm -f "$trivy_container" "$zizmor_container" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
mkdir "$work/out"

# allow_ids <tool> <token regex>: the tool's live allow-list ids; an id that does not match the
# regex is refused rather than written into YAML.
allow_ids() {
  local ids id
  ids=$(scripts/security/allow.sh "$1")
  [ -n "$ids" ] || return 0
  while read -r id; do
    if ! [[ "$id" =~ $2 ]]; then
      echo "config-scan: security/allow.json has a $1 id that is not a plain token" >&2
      return 1
    fi
    echo "$id"
  done <<<"$ids"
}

# trivy_ignore_entry <id>: the .trivyignore.yaml entry for one trivy-config id, or a refusal.
trivy_ignore_entry() {
  local id=$1 check path
  check=${id%%@*}
  if ! [[ "$check" =~ ^[A-Za-z0-9._:-]+$ ]]; then
    return 1
  fi
  echo "  - id: $check"
  [ "$check" != "$id" ] || return 0
  path=${id#*@}
  printf '%s\n' "${dockerfiles[@]}" | grep -qxF -- "$path" || return 1
  printf '    paths:\n      - "%s"\n' "$path"
}

trivy_ids=$(scripts/security/allow.sh trivy-config)
zizmor_ids=$(allow_ids zizmor '^[a-z0-9-]+$')
{
  if [ -z "$trivy_ids" ]; then
    echo "misconfigurations: []"
  else
    echo "misconfigurations:"
    while read -r id; do
      if ! trivy_ignore_entry "$id"; then
        echo "config-scan: security/allow.json has a trivy-config id that is not <id> or <id>@<a scanned Dockerfile> (${dockerfiles[*]})" >&2
        exit 1
      fi
    done <<<"$trivy_ids"
  fi
} >"$work/.trivyignore.yaml"
{
  if [ -z "$zizmor_ids" ]; then
    echo "rules: {}"
  else
    echo "rules:"
    while read -r id; do
      printf '  %s:\n    disable: true\n' "$id"
    done <<<"$zizmor_ids"
  fi
} >"$work/zizmor.yml"

red=0
red() {
  echo "config-scan: $*" >&2
  red=$((red + 1))
}

# trivy_config: one Trivy config run on every Dockerfile, each mounted at /src/<repo path>.
trivy_config() {
  local rc=0 mounts=() f n report=$work/out/trivy-config.json bad=0
  for f in "${dockerfiles[@]}"; do
    mounts+=(-v "$PWD/$f:/src/$f:ro")
  done
  echo "== config-scan: trivy config ${dockerfiles[*]}"
  docker run --rm --name "$trivy_container" --network none -u "$(id -u):$(id -g)" \
    "${mounts[@]}" -v "$work:/scan:ro" -v "$work/out:/out" \
    "$TRIVY_IMAGE" config --cache-dir /tmp/trivy --skip-check-update --skip-version-check \
    --ignorefile /scan/.trivyignore.yaml --severity HIGH,CRITICAL --exit-code "$findings_exit" \
    --format json --output /out/trivy-config.json /src || rc=$?
  case "$rc" in
    0 | "$findings_exit") ;;
    *)
      red "trivy could not scan the Dockerfiles (exit $rc)"
      return 0
      ;;
  esac
  for f in "${dockerfiles[@]}"; do
    if ! jq -e --arg f "$f" '[.Results[]? | select(.Target == $f and .Type == "dockerfile")] | length == 1' \
      "$report" >/dev/null 2>&1; then
      red "trivy scanned no Dockerfile at $f"
      bad=1
      continue
    fi
    jq -r --arg f "$f" '.Results[] | select(.Target == $f) | .Misconfigurations[]? | "\(.ID) (\(.Severity)): \(.Title)"' "$report"
    n=$(jq --arg f "$f" '[.Results[] | select(.Target == $f) | .Misconfigurations[]?] | length' "$report")
    if [ "$n" -gt 0 ]; then
      red "$f: high or critical misconfigurations"
      bad=1
    else
      echo "config-scan: $f: no high or critical misconfigurations"
    fi
  done
  if [ "$rc" -eq "$findings_exit" ] && [ "$bad" -eq 0 ]; then
    red "trivy exited $rc (findings) but its report lists none"
  fi
}

# zizmor [extra flags...]: one zizmor run on .github/workflows.
zizmor() {
  docker run --rm --name "$zizmor_container" --network none -u "$(id -u):$(id -g)" \
    -v "$PWD/.github:/src/.github:ro" -v "$work:/cfg:ro" -w /src \
    "$ZIZMOR_IMAGE" --offline "$@" --config /cfg/zizmor.yml .github/workflows
}

# Every file named Dockerfile or *.Dockerfile outside node_modules, build output and .git.
while IFS= read -r f; do
  f=${f#./}
  if ! printf '%s\n' "${dockerfiles[@]}" "${never_built[@]}" | grep -qxF -- "$f"; then
    red "$f is a Dockerfile config-scan.sh neither scans nor lists as never built"
  fi
done < <(find . \( -name node_modules -o -name .git -o -path ./web/build -o -path ./bin \) -prune -o \
  -type f \( -name Dockerfile -o -name '*.Dockerfile' \) -print | sort)

trivy_config

echo "== config-scan: zizmor .github/workflows"
rc=0
zizmor || rc=$?
case "$rc" in
  0) echo "config-scan: zizmor: no findings" ;;
  11 | 12) echo "config-scan: zizmor: informational or low findings only (exit $rc), reported, not blocking" ;;
  13 | 14) red "zizmor: medium or high findings (exit $rc)" ;;
  *) red "zizmor could not scan .github/workflows (exit $rc)" ;;
esac
if [ -n "$sarif_dir" ]; then
  rc=0
  zizmor --format sarif >"$sarif_dir/zizmor.sarif" || rc=$?
  [ "$rc" -eq 0 ] || red "zizmor could not write the SARIF file (exit $rc)"
fi

if [ "$red" -gt 0 ]; then
  echo "config-scan: $red checks red" >&2
  exit 1
fi
echo "config-scan: ok"
