#!/usr/bin/env bash
# make deps-scan (docs/designs/devsecops.md section 4 and "Blocking policy"; threat model T74 and
# T75): Trivy fs on the two dependency manifests, web/pnpm-lock.yaml (development dependencies
# included: the build and the test tools run on CI and on the developer's machine) and go.mod
# (govulncheck in make vuln stays the reachability check). A high or critical finding with a fixed
# version blocks; one with no fix is not shown, so an upstream advisory nobody can act on never
# turns every PR red. The live trivy ids of security/allow.json (allow.sh) go to a temp
# .trivyignore.yaml, as in make image-scan (trivy-ignore.sh); an id scoped to an image path ignores
# nothing here.
#
# Trivy downloads its vulnerability database into TRIVY_CACHE_DIR (default: the same cache dir
# make image-scan uses), so it needs network; without the database it exits 1, which is red. Trivy
# exits 1 on its own errors whatever --exit-code says, so findings use exit code 5 and any other
# non-zero exit is red. Each manifest is mounted alone at /src/<repo path>, and the JSON report must
# list both (go.mod as gomod, web/pnpm-lock.yaml as pnpm), so a file Trivy did not read never reads
# as clean; an exit that disagrees with the report is red. Exit 2 on bad arguments, 1 when red.
set -euo pipefail

if [ "$#" -ne 0 ]; then
  echo "usage: deps-scan.sh (no arguments; env TRIVY_IMAGE; optional TRIVY_CACHE_DIR)" >&2
  exit 2
fi
if [ -z "${TRIVY_IMAGE:-}" ]; then
  echo "deps-scan: TRIVY_IMAGE is not set (run through make deps-scan)" >&2
  exit 2
fi

cd "$(dirname "$0")/../.."

# Trivy's exit code for findings (--exit-code); see the header for why it is not 1.
findings_exit=5
# Each manifest and the Trivy type its result must carry.
manifests=(go.mod web/pnpm-lock.yaml)
declare -A manifest_type=([go.mod]=gomod [web/pnpm-lock.yaml]=pnpm)

cache=${TRIVY_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/open-customer/trivy}
mkdir -p "$cache"
container=oc-deps-scan-trivy
work=$(mktemp -d)
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
mkdir "$work/out"

# The ignore file: the live trivy ids of the allow-list, plain or scoped to one path
# (trivy-ignore.sh).
. scripts/security/trivy-ignore.sh deps-scan
write_trivy_ignore "$work/.trivyignore.yaml"

red=0
red() {
  echo "deps-scan: $*" >&2
  red=$((red + 1))
}

mounts=()
for f in "${manifests[@]}"; do
  [ -f "$f" ] || {
    echo "deps-scan: $f is missing" >&2
    exit 1
  }
  mounts+=(-v "$PWD/$f:/src/$f:ro")
done

echo "== deps-scan: trivy fs ${manifests[*]}"
rc=0
docker run --rm --name "$container" -u "$(id -u):$(id -g)" \
  -v "$cache:/cache" "${mounts[@]}" -v "$work:/scan:ro" -v "$work/out:/out" \
  "$TRIVY_IMAGE" fs --cache-dir /cache --ignorefile /scan/.trivyignore.yaml \
  --scanners vuln --severity HIGH,CRITICAL --ignore-unfixed --include-dev-deps \
  --no-progress --skip-version-check --exit-code "$findings_exit" \
  --format json --output /out/deps.json /src || rc=$?
report=$work/out/deps.json
case "$rc" in
  0 | "$findings_exit") ;;
  *)
    echo "deps-scan: trivy could not scan the dependency manifests (exit $rc)" >&2
    exit 1
    ;;
esac

found=0
for f in "${manifests[@]}"; do
  if ! jq -e --arg f "$f" --arg t "${manifest_type[$f]}" \
    '[.Results[]? | select(.Target == $f and .Type == $t)] | length == 1' "$report" >/dev/null 2>&1; then
    red "trivy read no ${manifest_type[$f]} manifest at $f"
    continue
  fi
  jq -r --arg f "$f" '.Results[] | select(.Target == $f) | .Vulnerabilities[]? |
    "\(.VulnerabilityID) (\(.Severity)): \(.PkgName) \(.InstalledVersion), fixed in \(.FixedVersion)"' "$report"
  n=$(jq --arg f "$f" '[.Results[] | select(.Target == $f) | .Vulnerabilities[]?] | length' "$report")
  if [ "$n" -gt 0 ]; then
    red "$f: high or critical findings with a fix"
    found=1
  else
    echo "deps-scan: $f: no blocking findings"
  fi
done
if [ "$rc" -eq "$findings_exit" ] && [ "$found" -eq 0 ]; then
  red "trivy exited $rc (findings) but its report lists none"
fi
if [ "$rc" -eq 0 ] && [ "$found" -eq 1 ]; then
  red "trivy exited 0 but its report lists findings"
fi

if [ "$red" -gt 0 ]; then
  exit 1
fi
echo "deps-scan: ok"
