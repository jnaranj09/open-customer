#!/usr/bin/env bash
# Tests for the scan wrappers in scripts/security (docs/designs/devsecops.md, decision D10). Run by
# make test-sec on the host. Each test runs a wrapper from a temp copy of the repo parts it reads
# (scripts/security, security/allow.json, deploy/compose.yaml), with a fake `docker` first on PATH
# that appends each call to a log and answers from env vars:
#   FAKE_FINDINGS   space-separated scan names (app backup postgres cloudflared) whose Trivy run
#                   finds something: the fake exits with the run's --exit-code value, as Trivy does
#   FAKE_FAIL_ON    a call containing this text exits FAKE_FAIL_CODE (default 1)
# The fake also copies the Trivy ignore file it was given to $FAKE_LOG.ignore. No real container
# runs and nothing is pulled. All data here is made up.
set -uo pipefail

# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. "$(dirname "$0")/../../../deploy/tests/lib.sh"

REPO=$(cd "$(dirname "$0")/../../.." && pwd)
ROOT=$(mktemp -d)
trap 'rm -rf "$ROOT"' EXIT
unset FAKE_FINDINGS FAKE_FAIL_ON FAKE_FAIL_CODE MODE PULL SARIF_DIR

mkdir -p "$ROOT/bin"
cat >"$ROOT/bin/docker" <<'FAKE'
#!/usr/bin/env bash
# The fake docker: logs "<args>" and answers from FAKE_*.
echo "$*" >>"$FAKE_LOG"
if [ -n "${FAKE_FAIL_ON:-}" ]; then
  case "$*" in *"$FAKE_FAIL_ON"*) exit "${FAKE_FAIL_CODE:-1}" ;; esac
fi
case "$1" in
  save) # docker save -o <file> <image>
    : >"$3"
    ;;
  run)
    shift
    scan_dir='' name='' code=0
    while [ "$#" -gt 0 ]; do
      case "$1" in
        -v)
          case "$2" in *:/scan:ro) scan_dir=${2%:/scan:ro} ;; esac
          shift
          ;;
        --input)
          name=$(basename "$2" .tar)
          shift
          ;;
        --exit-code)
          code=$2
          shift
          ;;
      esac
      shift
    done
    if [ -n "$scan_dir" ] && [ -f "$scan_dir/.trivyignore.yaml" ]; then
      cp "$scan_dir/.trivyignore.yaml" "$FAKE_LOG.ignore"
    fi
    case " ${FAKE_FINDINGS:-} " in *" $name "*) exit "$code" ;; esac
    ;;
esac
exit 0
FAKE
chmod +x "$ROOT/bin/docker"

# setup: a fresh copy of the repo parts the wrappers read in $W, an empty log in $LOG.
setup() {
  W=$(mktemp -d "$ROOT/run.XXXXXX")
  mkdir -p "$W/scripts" "$W/security" "$W/deploy"
  cp -r "$REPO/scripts/security" "$W/scripts/"
  cp "$REPO/deploy/compose.yaml" "$W/deploy/"
  echo '[]' >"$W/security/allow.json"
  LOG=$W/docker.log
  : >"$LOG"
  export FAKE_LOG=$LOG
}

# scan <args...>: runs image-scan.sh from another folder with the fake docker; stdout and stderr go
# to $W/out.
scan() {
  (cd / && PATH="$ROOT/bin:$PATH" TRIVY_IMAGE=trivy-test:1 PG_IMAGE=postgres:18.6-alpine \
    VERSION=1.2.3 TRIVY_CACHE_DIR="$W/cache" "$W/scripts/security/image-scan.sh" "$@") >"$W/out" 2>&1
}

has_call() {
  grep -qF -- "$1" "$LOG"
}

# count_calls <text>: how many docker calls contain <text>.
count_calls() {
  grep -cF -- "$1" "$LOG"
}

out_has() {
  grep -qF -- "$1" "$W/out" || fail "output does not contain '$1': $(cat "$W/out")"
}

cloudflared=$(grep -oE 'cloudflare/cloudflared:[^ ]+' "$REPO/deploy/compose.yaml")

test_unknown_mode() {
  setup
  local rc=0
  scan nightly || rc=$?
  assert_eq "exit code for MODE=nightly" 2 "$rc"
  rc=0
  scan || rc=$?
  assert_eq "exit code with no MODE" 2 "$rc"
  rc=0
  PULL=yes scan push || rc=$?
  assert_eq "exit code for PULL=yes" 2 "$rc"
  assert_eq "docker calls for bad args" 0 "$(wc -l <"$LOG")"
}

test_push_clean_scans_four_images() {
  setup
  scan push || fail "a clean push scan exited non-zero: $(cat "$W/out")"
  has_call "save -o" || fail "no image was saved"
  local image
  for image in open-customer:1.2.3 open-customer-backup:1.2.3 postgres:18.6-alpine "$cloudflared"; do
    has_call " $image" || fail "$image was not scanned"
  done
  assert_eq "trivy runs" 4 "$(count_calls 'run --rm')"
  assert_eq "trivy runs with the blocking flags" 4 \
    "$(count_calls 'trivy-test:1 image --cache-dir /cache --ignorefile /scan/.trivyignore.yaml --severity HIGH,CRITICAL --ignore-unfixed')"
  has_call "pull -q postgres:18.6-alpine" || fail "the postgres image was not pulled"
  has_call "pull -q $cloudflared" || fail "the cloudflared image was not pulled"
  has_call "pull -q open-customer:1.2.3" && fail "the local app image was pulled"
  return 0
}

test_push_app_finding_red() {
  setup
  FAKE_FINDINGS=app scan push && fail "an app image finding exited 0"
  out_has "open-customer:1.2.3"
}

test_push_postgres_finding_reported() {
  setup
  FAKE_FINDINGS=postgres scan push || fail "a postgres finding in push mode exited non-zero: $(cat "$W/out")"
  out_has "postgres:18.6-alpine: findings (report only in push mode)"
}

test_release_cloudflared_finding_red() {
  setup
  FAKE_FINDINGS=cloudflared scan release && fail "a cloudflared finding in release mode exited 0"
  out_has "$cloudflared"
}

test_tool_cannot_start_red() {
  setup
  FAKE_FAIL_ON='/scan/postgres.tar' FAKE_FAIL_CODE=125 scan push && fail "a scanner that could not start exited 0"
  out_has "trivy could not scan postgres:18.6-alpine (exit 125)"
}

test_trivy_error_red_on_report_only_image() {
  setup
  # Trivy exits 1 on its own errors (no database, bad image) whatever --exit-code says.
  FAKE_FAIL_ON='/scan/cloudflared.tar' FAKE_FAIL_CODE=1 scan push && fail "a Trivy error on a report-only image exited 0"
  out_has "trivy could not scan $cloudflared (exit 1)"
}

test_allow_reaches_ignore_file() {
  setup
  cat >"$W/security/allow.json" <<'JSON'
[
  {"tool": "trivy", "id": "CVE-2099-0001", "reason": "made up", "expires": "2099-01-01"},
  {"tool": "trivy", "id": "CVE-2000-0002", "reason": "made up, expired", "expires": "2000-01-01"},
  {"tool": "zizmor", "id": "CVE-2099-0003", "reason": "made up, other tool", "expires": "2099-01-01"}
]
JSON
  scan push || fail "scan exited non-zero: $(cat "$W/out")"
  [ -f "$LOG.ignore" ] || fail "trivy was given no ignore file"
  grep -qx '  - id: CVE-2099-0001' "$LOG.ignore" || fail "the live trivy id is not ignored: $(cat "$LOG.ignore")"
  grep -q 'CVE-2000-0002\|CVE-2099-0003' "$LOG.ignore" && fail "an expired or other-tool id is ignored"
  return 0
}

test_pull_scans_released_images() {
  setup
  PULL=1 scan release || fail "a clean pull scan exited non-zero: $(cat "$W/out")"
  local image
  for image in ghcr.io/jnaranj09/open-customer:1.2.3 ghcr.io/jnaranj09/open-customer-backup:1.2.3; do
    has_call "pull -q $image" || fail "$image was not pulled"
    has_call "save -o" || fail "no image was saved"
    has_call " $image" || fail "$image was not scanned"
  done
  has_call "image inspect open-customer:1.2.3" && fail "a pull scan looked for the local build"
  return 0
}

test_missing_image_red() {
  setup
  FAKE_FAIL_ON='pull -q ghcr.io/jnaranj09/open-customer-backup:1.2.3' PULL=1 scan release &&
    fail "a missing released image exited 0"
  out_has "cannot pull ghcr.io/jnaranj09/open-customer-backup:1.2.3"
}

test_local_build_missing_red() {
  setup
  FAKE_FAIL_ON='image inspect open-customer-backup:1.2.3' scan push && fail "a missing local build exited 0"
  out_has "open-customer-backup:1.2.3 is not built (run make image)"
}

test_sarif_written_per_image() {
  setup
  SARIF_DIR=$W/sarif scan push || fail "scan exited non-zero: $(cat "$W/out")"
  assert_eq "sarif runs" 4 "$(count_calls '--format sarif --output /sarif/')"
  has_call "--output /sarif/app.sarif" || fail "no SARIF for the app image"
}

t "an unknown MODE exits 2" test_unknown_mode
t "push mode: a clean scan covers the four images" test_push_clean_scans_four_images
t "push mode: an app image finding is red" test_push_app_finding_red
t "push mode: a postgres finding is reported, not red" test_push_postgres_finding_reported
t "release mode: a cloudflared finding is red" test_release_cloudflared_finding_red
t "a tool that cannot start is red" test_tool_cannot_start_red
t "a Trivy error is red even for a report-only image" test_trivy_error_red_on_report_only_image
t "an allow-listed CVE reaches the ignore file" test_allow_reaches_ignore_file
t "PULL=1 scans the released images from compose.yaml" test_pull_scans_released_images
t "a released image that cannot be pulled is red" test_missing_image_red
t "a local image make image did not build is red" test_local_build_missing_red
t "SARIF_DIR gets one SARIF file per image" test_sarif_written_per_image
finish
