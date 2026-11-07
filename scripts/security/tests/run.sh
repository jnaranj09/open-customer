#!/usr/bin/env bash
# Tests for the scan wrappers in scripts/security (docs/designs/devsecops.md, decision D10). Run by
# make test-sec on the host. Each test runs a wrapper from a temp copy of the repo parts it reads
# (scripts/security, security/allow.json, deploy/compose.yaml), with a fake `docker` first on PATH
# that appends each call to a log and answers from env vars:
#   FAKE_FINDINGS   space-separated scan names (app backup postgres cloudflared) whose Trivy run
#                   finds something: the fake exits with the run's --exit-code value, as Trivy does
#   FAKE_FAIL_ON    a call containing this text exits FAKE_FAIL_CODE (default 1)
# and, for dast.sh, by the image a `docker run` names:
#   curl-test:1     answers the app's routes: /readyz ok, setup 201, login 200 with a made-up
#                   session cookie, /api/v1/me the next status of FAKE_ME_STATUS (space-separated,
#                   the last one repeats; default 200)
#   zap-test:1      writes FAKE_ZAP_REPORT (default: no alerts) as its -J report, exits
#                   FAKE_ZAP_CODE (default 0) and copies the rules file it was given to
#                   $FAKE_LOG.zaprules
#   nuclei-test:1   prints FAKE_NUCLEI_TEMPLATES (default 5) as its loaded-templates count, writes
#                   one finding to its JSONL output when FAKE_NUCLEI_FINDINGS is set, exits 0
# `docker exec ... setup-link` prints a made-up setup link. The fake also copies the Trivy ignore
# file it was given to $FAKE_LOG.ignore. No real container runs and nothing is pulled. All data
# here is made up.
set -uo pipefail

# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. "$(dirname "$0")/../../../deploy/tests/lib.sh"

REPO=$(cd "$(dirname "$0")/../../.." && pwd)
ROOT=$(mktemp -d)
trap 'rm -rf "$ROOT"' EXIT
unset FAKE_FINDINGS FAKE_FAIL_ON FAKE_FAIL_CODE FAKE_ME_STATUS FAKE_ZAP_CODE FAKE_ZAP_REPORT FAKE_NUCLEI_TEMPLATES \
  FAKE_NUCLEI_FINDINGS MODE PULL SARIF_DIR REPORT_DIR NUCLEI_CACHE_DIR

mkdir -p "$ROOT/bin"
cat >"$ROOT/bin/docker" <<'FAKE'
#!/usr/bin/env bash
# The fake docker: logs "<args>" and answers from FAKE_*.
echo "$*" >>"$FAKE_LOG"
if [ -n "${FAKE_FAIL_ON:-}" ]; then
  case "$*" in *"$FAKE_FAIL_ON"*) exit "${FAKE_FAIL_CODE:-1}" ;; esac
fi
# mount_of <container path> <args...>: the host side of `-v <host>:<container path>[:mode]`.
mount_of() {
  local want=$1 prev=''
  shift
  for a in "$@"; do
    if [ "$prev" = -v ]; then
      case "$a" in *:"$want" | *:"$want":*) echo "${a%%:"$want"*}" && return ;; esac
    fi
    prev=$a
  done
}
fake_curl() {
  local url=''
  for a in "$@"; do
    case "$a" in http*) url=$a ;; esac
  done
  case "$url" in
    */readyz) ;;
    */api/v1/setup) printf '201' ;;
    */api/v1/auth/login) printf '200\noc_session=made-up-session; Path=/; HttpOnly; SameSite=Lax' ;;
    */api/v1/me)
      local n statuses
      n=$(($(cat "$FAKE_LOG.me" 2>/dev/null || echo 0) + 1))
      echo "$n" >"$FAKE_LOG.me"
      read -ra statuses <<<"${FAKE_ME_STATUS:-200}"
      [ "$n" -le "${#statuses[@]}" ] || n=${#statuses[@]}
      printf '%s' "${statuses[$((n - 1))]}"
      ;;
  esac
}
case "$1" in
  save) # docker save -o <file> <image>
    : >"$3"
    ;;
  exec)
    case "$*" in *setup-link*) echo "http://localhost:8080/setup#made-up-token" ;; esac
    ;;
  run)
    case " $* " in
      *" curl-test:1 "*)
        fake_curl "$@"
        exit 0
        ;;
      *" zap-test:1 "*)
        wrk=$(mount_of /zap/wrk "$@")
        cp "$wrk/rules.tsv" "$FAKE_LOG.zaprules"
        prev=''
        for a in "$@"; do
          [ "$prev" != -J ] || printf '%s' "${FAKE_ZAP_REPORT:-{\"site\":[]\}}" >"$wrk/$a"
          prev=$a
        done
        exit "${FAKE_ZAP_CODE:-0}"
        ;;
      *" nuclei-test:1 "*)
        out=$(mount_of /out "$@")
        echo "[INF] Templates loaded for current scan: ${FAKE_NUCLEI_TEMPLATES:-5}" >&2
        if [ -n "${FAKE_NUCLEI_FINDINGS:-}" ]; then
          echo '{"template-id":"made-up-template","info":{"severity":"medium"}}' >"$out/nuclei.jsonl"
        fi
        exit 0
        ;;
    esac
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
  mkdir -p "$W/api"
  cp -r "$REPO/scripts/security" "$W/scripts/"
  cp "$REPO/deploy/compose.yaml" "$W/deploy/"
  cp "$REPO/security/zap-rules.tsv" "$W/security/"
  cp "$REPO/api/openapi.yaml" "$W/api/"
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

# dast: runs dast.sh from another folder with the fake docker and a templates cache that already
# holds the pinned templates; stdout and stderr go to $W/out.
dast() {
  mkdir -p "$W/nuclei-cache/nuclei-templates-v10.5.1"
  (cd / && PATH="$ROOT/bin:$PATH" IMAGE=open-customer:1.2.3 PG_IMAGE=postgres:18.6-alpine \
    ZAP_IMAGE=zap-test:1 NUCLEI_IMAGE=nuclei-test:1 NUCLEI_TEMPLATES=v10.5.1 CURL_IMAGE=curl-test:1 \
    REPORT_DIR="$W/report" NUCLEI_CACHE_DIR="$W/nuclei-cache" "$W/scripts/security/dast.sh" "$@") >"$W/out" 2>&1
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

test_dast_clean_run() {
  setup
  dast || fail "a clean dast run exited non-zero: $(cat "$W/out")"
  assert_eq "zap runs" 3 "$(count_calls 'zap-test:1')"
  assert_eq "nuclei runs" 1 "$(count_calls 'nuclei-test:1')"
  grep -qE '(^| )(-p|--publish)( |=)' "$LOG" && fail "a container published a port"
  assert_eq "scanners outside the app's network namespace" 0 \
    "$(grep -E 'zap-test:1|nuclei-test:1|curl-test:1 .*localhost' "$LOG" | grep -vc -- '--network container:oc-dast-app')"
  has_call "rm -f -v oc-dast-scanner oc-dast-app oc-dast-pg" || fail "the containers were not removed"
  has_call "network rm oc-dast-net" || fail "the network was not removed"
}

test_dast_bad_args() {
  setup
  local rc=0
  dast extra || rc=$?
  assert_eq "exit code with an argument" 2 "$rc"
  assert_eq "docker calls for bad args" 0 "$(wc -l <"$LOG")"
}

test_dast_session_check_fails() {
  setup
  FAKE_ME_STATUS=401 dast && fail "a failed session check exited 0"
  out_has "GET /api/v1/me answered 401"
  assert_eq "scans after a failed session check" 0 "$(count_calls 'zap-test:1')"
  assert_eq "nuclei runs after a failed session check" 0 "$(count_calls 'nuclei-test:1')"
}

test_dast_authenticated_scan_sends_session_headers() {
  setup
  dast || fail "a clean dast run exited non-zero: $(cat "$W/out")"
  has_call 'replacer.full_list(0).matchstr=Origin -config replacer.full_list(0).regex=false -config replacer.full_list(0).replacement=http://localhost:8080' ||
    fail "zap was not given the Origin header"
  has_call 'replacer.full_list(1).matchstr=Cookie -config replacer.full_list(1).regex=false -config replacer.full_list(1).replacement=oc_session=made-up-session' ||
    fail "the authenticated zap scan was not given the session cookie"
  assert_eq "zap runs with the session cookie" 1 "$(count_calls 'replacement=oc_session=made-up-session')"
}

test_dast_session_ends_during_scan() {
  setup
  FAKE_ME_STATUS='200 401' dast && fail "a session that ended during the authenticated scan exited 0"
  out_has "the session ended during the authenticated scan"
}

test_zap_exit_3_red() {
  setup
  FAKE_ZAP_CODE=3 dast && fail "zap exit 3 (zap error) exited 0"
  out_has "zap could not scan"
}

test_zap_exit_1_red() {
  setup
  FAKE_ZAP_CODE=1 dast && fail "zap exit 1 (a FAIL rule) exited 0"
  out_has "blocking findings"
}

test_zap_exit_2_passes() {
  setup
  FAKE_ZAP_CODE=2 dast || fail "zap exit 2 (WARN rules only) exited non-zero: $(cat "$W/out")"
}

test_zap_server_error_red() {
  setup
  FAKE_ZAP_REPORT='{"site":[{"alerts":[{"pluginid":"100000","riskcode":"0","count":"7"},{"pluginid":"100000","riskcode":"1","count":"2"}]}]}' \
    dast && fail "a 5xx answer during a zap scan exited 0"
  out_has "the app answered 2 requests with a 5xx"
}

test_zap_client_errors_pass() {
  setup
  FAKE_ZAP_REPORT='{"site":[{"alerts":[{"pluginid":"100000","riskcode":"0","count":"7"}]}]}' dast ||
    fail "4xx answers during a zap scan exited non-zero: $(cat "$W/out")"
}

test_zap_no_report_red() {
  setup
  FAKE_ZAP_REPORT='not json' dast && fail "a zap run without a readable report exited 0"
  out_has "wrote no readable JSON report"
}

test_nuclei_findings_exit_0_red() {
  setup
  FAKE_NUCLEI_FINDINGS=1 dast && fail "nuclei findings with exit 0 exited 0"
  out_has "nuclei: 1 findings"
}

test_nuclei_no_templates_red() {
  setup
  FAKE_NUCLEI_TEMPLATES=0 dast && fail "a nuclei run with no templates exited 0"
  out_has "nuclei loaded no templates"
}

test_templates_download_fails_red() {
  setup
  rm -rf "$W/nuclei-cache"
  mkdir -p "$W/nuclei-cache"
  (cd / && FAKE_FAIL_ON=nuclei-templates PATH="$ROOT/bin:$PATH" IMAGE=open-customer:1.2.3 PG_IMAGE=postgres:18.6-alpine \
    ZAP_IMAGE=zap-test:1 NUCLEI_IMAGE=nuclei-test:1 NUCLEI_TEMPLATES=v10.5.1 CURL_IMAGE=curl-test:1 \
    REPORT_DIR="$W/report" NUCLEI_CACHE_DIR="$W/nuclei-cache" "$W/scripts/security/dast.sh") >"$W/out" 2>&1 &&
    fail "a failed templates download exited 0"
  out_has "cannot download the nuclei templates"
}

test_zap_allow_is_ignore() {
  setup
  cat >"$W/security/allow.json" <<'JSON'
[
  {"tool": "zap", "id": "10021", "reason": "made up", "expires": "2099-01-01"},
  {"tool": "zap", "id": "10020", "reason": "made up, expired", "expires": "2000-01-01"},
  {"tool": "nuclei", "id": "10038", "reason": "made up, other tool", "expires": "2099-01-01"}
]
JSON
  dast || fail "dast exited non-zero: $(cat "$W/out")"
  [ -f "$LOG.zaprules" ] || fail "zap was given no rules file"
  grep -qP '^10021\tIGNORE\t' "$LOG.zaprules" || fail "the live zap id is not IGNORE: $(cat "$LOG.zaprules")"
  assert_eq "lines for the live id" 1 "$(grep -cP '^10021\t' "$LOG.zaprules")"
  grep -qP '^10020\tFAIL\t' "$LOG.zaprules" || fail "an expired zap id is not FAIL"
  grep -qP '^10038\tFAIL\t' "$LOG.zaprules" || fail "another tool's id changed the zap rules"
}

test_zap_scoped_allow_is_out_of_scope() {
  setup
  cat >"$W/security/allow.json" <<'JSON'
[
  {"tool": "zap", "id": "40018@.*/api/v1/client-errors$", "reason": "made up", "expires": "2099-01-01"}
]
JSON
  dast || fail "dast exited non-zero: $(cat "$W/out")"
  grep -qxP '40018\tOUTOFSCOPE\t\.\*/api/v1/client-errors\$' "$LOG.zaprules" ||
    fail "no OUTOFSCOPE line for the scoped id: $(grep -P '^40018\t' "$LOG.zaprules")"
  assert_eq "40018 FAIL lines" 1 "$(grep -cP '^40018\tFAIL\t' "$LOG.zaprules")"
  assert_eq "40018 IGNORE lines" 0 "$(grep -cP '^40018\tIGNORE' "$LOG.zaprules")"
}

test_zap_malformed_scoped_id_refused() {
  local id
  for id in '40018@' '40018@two words' 'x40018@.*/a$' '40018@a@b	tab'; do
    setup
    jq -n --arg id "$id" '[{"tool": "zap", "id": $id, "reason": "made up", "expires": "2099-01-01"}]' >"$W/security/allow.json"
    dast && fail "the zap id '$id' was accepted"
    out_has "security/allow.json has a zap id that is not <rule number> or <rule number>@<url regex>"
    assert_eq "zap runs with the bad id '$id'" 0 "$(count_calls 'zap-test:1')"
  done
}

test_nuclei_allow_excluded() {
  setup
  cat >"$W/security/allow.json" <<'JSON'
[
  {"tool": "nuclei", "id": "made-up-template", "reason": "made up", "expires": "2099-01-01"},
  {"tool": "nuclei", "id": "made-up-expired", "reason": "made up, expired", "expires": "2000-01-01"}
]
JSON
  dast || fail "dast exited non-zero: $(cat "$W/out")"
  has_call "-exclude-id made-up-template " || fail "the live nuclei id did not reach -exclude-id"
  has_call "made-up-expired" && fail "an expired nuclei id reached nuclei"
  return 0
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
t "dast: a clean run scans in the app's network namespace and cleans up" test_dast_clean_run
t "dast: an argument exits 2" test_dast_bad_args
t "dast stops when the session check fails" test_dast_session_check_fails
t "dast: the authenticated scan sends the Origin and session cookie" test_dast_authenticated_scan_sends_session_headers
t "dast: a session that ends during the authenticated scan is red" test_dast_session_ends_during_scan
t "zap exit 3 is red" test_zap_exit_3_red
t "zap exit 1 is red" test_zap_exit_1_red
t "zap exit 2 (WARN rules only) passes" test_zap_exit_2_passes
t "a 5xx answer in a zap scan is red" test_zap_server_error_red
t "4xx answers in a zap scan pass" test_zap_client_errors_pass
t "a zap run with no readable report is red" test_zap_no_report_red
t "nuclei findings with exit 0 are red" test_nuclei_findings_exit_0_red
t "a nuclei run that loads no templates is red" test_nuclei_no_templates_red
t "a failed nuclei templates download is red" test_templates_download_fails_red
t "an allow-listed zap rule is IGNORE in the rules file" test_zap_allow_is_ignore
t "an allow-listed nuclei template reaches -exclude-id" test_nuclei_allow_excluded
t "a scoped zap id is OUTOFSCOPE for its URL and leaves the rule FAIL" test_zap_scoped_allow_is_out_of_scope
t "a malformed scoped zap id is refused" test_zap_malformed_scoped_id_refused
finish
