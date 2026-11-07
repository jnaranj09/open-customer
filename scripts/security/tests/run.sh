#!/usr/bin/env bash
# Tests for the scan wrappers in scripts/security (docs/designs/devsecops.md, decision D10), and for
# scripts/check-pins.sh's image digest checks. Run by make test-sec on the host. Each test runs a wrapper from a temp copy of the repo parts it reads
# (scripts/security, security/allow.json, deploy/compose.yaml; made-up Dockerfiles and a workflow
# for config-scan.sh), with a fake `docker` first on PATH
# that appends each call to a log and answers from env vars:
#   FAKE_FINDINGS   space-separated scan names (app backup postgres cloudflared) whose Trivy run
#                   finds something: the fake exits with the run's --exit-code value, as Trivy does
#   FAKE_FAIL_ON    a call containing this text exits FAKE_FAIL_CODE (default 1)
# and, for dast.sh, by the image a `docker run` names:
#   curl-test:1     answers the app's routes: /readyz ok, setup 201, login 200 with a made-up
#                   session cookie, /api/v1/me the next status of FAKE_ME_STATUS (space-separated,
#                   the last one repeats; default 200)
#   zap-test:1      exits 3 unless given `-c rules.tsv`; writes FAKE_ZAP_REPORT (default: no
#                   alerts) as its -J report, exits
#                   FAKE_ZAP_CODE (default 0) and copies the rules file it was given to
#                   $FAKE_LOG.zaprules
#   nuclei-test:1   prints FAKE_NUCLEI_TEMPLATES (default 5) as its loaded-templates count, writes
#                   one finding to its JSONL output when FAKE_NUCLEI_FINDINGS is set, exits 0
#   curl-test:1 with `-o /cache/<file>` (the templates download) writes a made-up tarball there
# and, for config-scan.sh:
#   trivy-cfg-test:1  reports each file mounted at /src/<path> as a scanned Dockerfile (all but
#                   deploy/backup/Dockerfile when FAKE_TRIVY_CONFIG_EMPTY is set) in a JSON report at
#                   its --output, copies its ignore file to $FAKE_LOG.cfgignore, and gives each path
#                   in FAKE_TRIVY_CONFIG_FINDINGS (space-separated) a DS-0002 finding unless the
#                   ignore file drops it, as Trivy does: an entry `- id: DS-0002` with no `paths:`,
#                   or with `paths:` listing that path. It exits FAKE_TRIVY_CONFIG_CODE when set,
#                   else the run's --exit-code value when a finding is left, else 0
#   zizmor-test:1   copies its --config file to $FAKE_LOG.zizmor, prints a made-up SARIF report
#                   with --format sarif (exit 0), and otherwise exits FAKE_ZIZMOR_CODE (default 0)
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
  FAKE_NUCLEI_FINDINGS FAKE_TRIVY_CONFIG_CODE FAKE_TRIVY_CONFIG_EMPTY FAKE_TRIVY_CONFIG_FINDINGS FAKE_ZIZMOR_CODE MODE PULL SARIF_DIR \
  REPORT_DIR NUCLEI_CACHE_DIR

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
  local url='' out='' prev=''
  for a in "$@"; do
    case "$a" in http*) url=$a ;; esac
    [ "$prev" != -o ] || out=$a
    prev=$a
  done
  case "$out" in
    /cache/*) # the templates download: a made-up tarball in the mounted cache
      printf 'made-up templates tarball\n' >"$(mount_of /cache "$@")/${out#/cache/}"
      return 0
      ;;
  esac
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
        case " $* " in
          *" -c rules.tsv "*) ;;
          *)
            echo "fake zap: no -c rules.tsv" >&2
            exit 3
            ;;
        esac
        wrk=$(mount_of /zap/wrk "$@")
        cp "$wrk/rules.tsv" "$FAKE_LOG.zaprules"
        prev=''
        for a in "$@"; do
          [ "$prev" != -J ] || printf '%s' "${FAKE_ZAP_REPORT:-{\"site\":[]\}}" >"$wrk/$a"
          prev=$a
        done
        exit "${FAKE_ZAP_CODE:-0}"
        ;;
      *" trivy-cfg-test:1 "*)
        out_file='' code=0 prev='' targets=()
        for a in "$@"; do
          [ "$prev" != --output ] || out_file=$a
          [ "$prev" != --exit-code ] || code=$a
          if [ "$prev" = -v ]; then
            case "$a" in *:/src/*:ro) t=${a#*:/src/} && targets+=("${t%:ro}") ;; esac
          fi
          prev=$a
        done
        ignore=$(mount_of /scan "$@")/.trivyignore.yaml
        cp "$ignore" "$FAKE_LOG.cfgignore"
        # The paths DS-0002 is dropped for, one per line; "*" for an entry without paths.
        dropped=$(awk '/^  - id: /{ if (open) print "*"; open = ($3 == "DS-0002"); paths = 0; next }
          /^    paths:/{ next }
          /^      - /{ if (open) { p = $2; gsub(/"/, "", p); print p; paths = 1 } next }
          END{ if (open && !paths) print "*" }' "$ignore")
        results='[]' left=0
        for t in "${targets[@]}"; do
          [ -z "${FAKE_TRIVY_CONFIG_EMPTY:-}" ] || [ "$t" != deploy/backup/Dockerfile ] || continue
          m='[]'
          case " ${FAKE_TRIVY_CONFIG_FINDINGS:-} " in
            *" $t "*)
              if ! grep -qxF -e '*' -e "$t" <<<"$dropped"; then
                m='[{"ID":"DS-0002","Severity":"HIGH","Title":"made up"}]'
                left=1
              fi
              ;;
          esac
          results=$(jq -c --arg t "$t" --argjson m "$m" '. + [{Target: $t, Type: "dockerfile", Misconfigurations: $m}]' <<<"$results")
        done
        jq -n --argjson r "$results" '{Results: $r}' >"$(mount_of /out "$@")/${out_file#/out/}"
        if [ -n "${FAKE_TRIVY_CONFIG_CODE:-}" ]; then exit "$FAKE_TRIVY_CONFIG_CODE"; fi
        [ "$left" -eq 0 ] || exit "$code"
        exit 0
        ;;
      *" zizmor-test:1 "*)
        prev=''
        for a in "$@"; do
          if [ "$prev" = --config ]; then
            cp "$(mount_of "$(dirname "$a")" "$@")/$(basename "$a")" "$FAKE_LOG.zizmor"
          fi
          prev=$a
        done
        case " $* " in
          *" --format sarif "*)
            echo '{"version":"2.1.0","runs":[]}'
            exit 0
            ;;
        esac
        exit "${FAKE_ZIZMOR_CODE:-0}"
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
  mkdir -p "$W/deploy/backup" "$W/.github/workflows"
  printf 'FROM scratch\n' >"$W/Dockerfile"
  printf 'FROM scratch\n' >"$W/deploy/backup/Dockerfile"
  printf 'name: made-up\n' >"$W/.github/workflows/ci.yml"
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

# config_scan: runs config-scan.sh from another folder with the fake docker; stdout and stderr go
# to $W/out.
config_scan() {
  (cd / && PATH="$ROOT/bin:$PATH" TRIVY_IMAGE=trivy-cfg-test:1 ZIZMOR_IMAGE=zizmor-test:1 \
    "$W/scripts/security/config-scan.sh" "$@") >"$W/out" 2>&1
}

# pins_setup: a fresh copy, in $W, of the files scripts/check-pins.sh reads. CHECK_PINS overrides
# the script copied (default: the repo's).
pins_setup() {
  W=$(mktemp -d "$ROOT/pins.XXXXXX")
  mkdir -p "$W/scripts" "$W/web" "$W/deploy/backup" "$W/.github/workflows"
  cp "${CHECK_PINS:-$REPO/scripts/check-pins.sh}" "$W/scripts/check-pins.sh"
  cp "$REPO/web/package.json" "$W/web/"
  cp "$REPO/go.mod" "$REPO/Dockerfile" "$W/"
  cp "$REPO/deploy/compose.yaml" "$W/deploy/"
  cp "$REPO/deploy/backup/Dockerfile" "$W/deploy/backup/"
  cp "$REPO/.github/workflows/ci.yml" "$W/.github/workflows/"
}

# The Makefile's pins, as make check-pins passes them.
make_pin() {
  sed -n "s/^$1 := //p" "$REPO/Makefile"
}

# check_pins: runs the copied check-pins.sh with the Makefile's pins (PG_PIN overrides PG_IMAGE);
# stdout and stderr go to $W/out.
check_pins() {
  (cd / && PG_IMAGE="${PG_PIN:-$(make_pin PG_IMAGE)}" GOLANGCI_LINT_VERSION="$(make_pin GOLANGCI_LINT_VERSION)" \
    GITLEAKS_IMAGE="$(make_pin GITLEAKS_IMAGE)" "$W/scripts/check-pins.sh") >"$W/out" 2>&1
}

# A digest that is not the pinned one.
other_digest=sha256:$(printf 'made-up image\n' | sha256sum | cut -d' ' -f1)

# The sha256 of the fake curl's made-up templates tarball.
fake_tarball_sha=$(printf 'made-up templates tarball\n' | sha256sum | cut -d' ' -f1)

# dast: runs dast.sh from another folder with the fake docker; stdout and stderr go to $W/out. The
# templates cache already holds the pinned templates unless EMPTY_CACHE is set. TEMPLATES_SHA
# overrides the pinned tarball sha256 (default: the fake tarball's).
dast() {
  mkdir -p "$W/nuclei-cache"
  [ -n "${EMPTY_CACHE:-}" ] || mkdir -p "$W/nuclei-cache/nuclei-templates-v10.5.1"
  (cd / && PATH="$ROOT/bin:$PATH" IMAGE=open-customer:1.2.3 PG_IMAGE=postgres:18.6-alpine \
    ZAP_IMAGE=zap-test:1 NUCLEI_IMAGE=nuclei-test:1 NUCLEI_TEMPLATES=v10.5.1 CURL_IMAGE=curl-test:1 \
    NUCLEI_TEMPLATES_SHA256="${TEMPLATES_SHA:-$fake_tarball_sha}" \
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
  FAKE_FAIL_ON=nuclei-templates EMPTY_CACHE=1 dast && fail "a failed templates download exited 0"
  out_has "cannot download the nuclei templates"
}

test_templates_checksum_mismatch_red() {
  setup
  TEMPLATES_SHA=$(printf 'other\n' | sha256sum | cut -d' ' -f1) EMPTY_CACHE=1 dast &&
    fail "a templates tarball with the wrong sha256 exited 0"
  out_has "the nuclei templates tarball v10.5.1 does not match NUCLEI_TEMPLATES_SHA256"
  [ ! -d "$W/nuclei-cache/nuclei-templates-v10.5.1" ] || fail "a tarball with the wrong sha256 was unpacked"
  assert_eq "nuclei runs after a checksum mismatch" 0 "$(count_calls 'nuclei-test:1')"
}

test_zap_unknown_rule_red() {
  setup
  FAKE_ZAP_REPORT='{"site":[{"alerts":[{"pluginid":"10021","riskcode":"1","count":"1"},{"pluginid":"99999","riskcode":"3","count":"1"}]}]}' \
    dast && fail "an alert from a rule zap-rules.tsv does not list exited 0"
  out_has "rules not in security/zap-rules.tsv: 99999"
}

test_zap_scope_regex_kept_byte_for_byte() {
  setup
  jq -n '[{"tool": "zap", "id": "40018@.*/api/v1/client\\.errors\\?x=\\d$", "reason": "made up", "expires": "2099-01-01"}]' \
    >"$W/security/allow.json"
  dast || fail "dast exited non-zero: $(cat "$W/out")"
  grep -qxF "$(printf '40018\tOUTOFSCOPE\t%s' '.*/api/v1/client\.errors\?x=\d$')" "$LOG.zaprules" ||
    fail "the OUTOFSCOPE regex changed on its way to zap: $(grep -P '^40018\tOUTOFSCOPE' "$LOG.zaprules")"
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

test_config_clean_run() {
  setup
  config_scan || fail "a clean config scan exited non-zero: $(cat "$W/out")"
  assert_eq "trivy config runs" 1 "$(count_calls 'trivy-cfg-test:1 config')"
  has_call "/Dockerfile:/src/Dockerfile:ro" || fail "the app Dockerfile was not mounted at its repo path"
  has_call "/deploy/backup/Dockerfile:/src/deploy/backup/Dockerfile:ro" || fail "the backup Dockerfile was not mounted at its repo path"
  assert_eq "trivy config runs with the blocking flags" 1 \
    "$(count_calls '--skip-check-update --skip-version-check --ignorefile /scan/.trivyignore.yaml --severity HIGH,CRITICAL --exit-code 5')"
  assert_eq "zizmor runs" 1 "$(count_calls 'zizmor-test:1 --offline')"
  assert_eq "scanner runs with a network" 0 "$(grep -E 'trivy-cfg-test:1|zizmor-test:1' "$LOG" | grep -vc -- '--network none')"
  has_call "rm -f oc-config-scan-trivy oc-config-scan-zizmor" || fail "the containers were not removed"
}

test_config_bad_args() {
  setup
  local rc=0
  config_scan extra || rc=$?
  assert_eq "exit code with an argument" 2 "$rc"
  rc=0
  (cd / && PATH="$ROOT/bin:$PATH" ZIZMOR_IMAGE=zizmor-test:1 "$W/scripts/security/config-scan.sh") >"$W/out" 2>&1 || rc=$?
  assert_eq "exit code without TRIVY_IMAGE" 2 "$rc"
  assert_eq "docker calls for bad args" 0 "$(wc -l <"$LOG")"
}

# zizmor exits 11 to 14 for its highest finding: informational, low, medium, high.
test_zizmor_medium_and_high_red() {
  local code
  for code in 13 14; do
    setup
    FAKE_ZIZMOR_CODE=$code config_scan && fail "zizmor exit $code exited 0"
    out_has "zizmor: medium or high findings (exit $code)"
  done
}

test_zizmor_low_and_info_reported() {
  local code
  for code in 11 12; do
    setup
    FAKE_ZIZMOR_CODE=$code config_scan || fail "zizmor exit $code exited non-zero: $(cat "$W/out")"
    out_has "zizmor: informational or low findings only (exit $code), reported, not blocking"
  done
}

test_zizmor_error_red() {
  local code
  for code in 1 2 3 125; do
    setup
    FAKE_ZIZMOR_CODE=$code config_scan && fail "zizmor exit $code (an error) exited 0"
    out_has "zizmor could not scan .github/workflows (exit $code)"
  done
}

test_trivy_config_findings_red() {
  setup
  FAKE_TRIVY_CONFIG_FINDINGS=deploy/backup/Dockerfile config_scan && fail "a Trivy config finding exited 0"
  out_has "config-scan: deploy/backup/Dockerfile: high or critical misconfigurations"
  out_has "DS-0002 (HIGH): made up"
}

# The owner-approved shape: DS-0002 allowed on the backup Dockerfile only.
scoped_allow() {
  jq -n --arg id "$1" '[{"tool": "trivy-config", "id": $id, "reason": "made up", "expires": "2099-01-01"}]' \
    >"$W/security/allow.json"
}

test_trivy_config_scoped_allow_drops_that_file() {
  setup
  scoped_allow DS-0002@deploy/backup/Dockerfile
  FAKE_TRIVY_CONFIG_FINDINGS=deploy/backup/Dockerfile config_scan ||
    fail "a finding the scoped id allows on that file exited non-zero: $(cat "$W/out")"
  assert_eq "ignore file" "$(printf 'misconfigurations:\n  - id: DS-0002\n    paths:\n      - "deploy/backup/Dockerfile"')" \
    "$(cat "$LOG.cfgignore")"
}

test_trivy_config_scoped_allow_other_file_red() {
  setup
  scoped_allow DS-0002@deploy/backup/Dockerfile
  FAKE_TRIVY_CONFIG_FINDINGS='Dockerfile deploy/backup/Dockerfile' config_scan &&
    fail "the same finding on the app Dockerfile exited 0"
  out_has "config-scan: Dockerfile: high or critical misconfigurations"
  grep -qF "config-scan: deploy/backup/Dockerfile: high" "$W/out" && fail "the scoped file is still red: $(cat "$W/out")"
  return 0
}

test_trivy_config_malformed_scoped_id_refused() {
  local id
  for id in 'DS-0002@' 'DS-0002@/src/Dockerfile' 'DS-0002@../Dockerfile' 'DS-0002@deploy/*/Dockerfile' \
    'DS-0002@deploy/backup/Dockerfile@x' 'DS-0002@deploy/compose.yaml' '@deploy/backup/Dockerfile' \
    'DS-0002@deploy//backup/Dockerfile' 'DS 0002@deploy/backup/Dockerfile'; do
    setup
    scoped_allow "$id"
    config_scan && fail "the trivy-config id '$id' was accepted"
    out_has "security/allow.json has a trivy-config id that is not <id> or <id>@<a scanned Dockerfile>"
    assert_eq "trivy runs with the bad id '$id'" 0 "$(count_calls 'trivy-cfg-test:1')"
  done
}

test_trivy_config_error_red() {
  setup
  # Trivy exits 1 on its own errors whatever --exit-code says.
  FAKE_TRIVY_CONFIG_CODE=1 config_scan && fail "a Trivy config error (exit 1) exited 0"
  out_has "trivy could not scan the Dockerfiles (exit 1)"
}

test_trivy_config_nothing_scanned_red() {
  setup
  FAKE_TRIVY_CONFIG_EMPTY=1 config_scan && fail "a Trivy config run that scanned no Dockerfile exited 0"
  out_has "trivy scanned no Dockerfile at deploy/backup/Dockerfile"
}

test_config_allow_reaches_tools() {
  setup
  cat >"$W/security/allow.json" <<'JSON'
[
  {"tool": "trivy-config", "id": "DS-9001", "reason": "made up", "expires": "2099-01-01"},
  {"tool": "trivy-config", "id": "DS-9002", "reason": "made up, expired", "expires": "2000-01-01"},
  {"tool": "zizmor", "id": "made-up-audit", "reason": "made up", "expires": "2099-01-01"},
  {"tool": "trivy", "id": "DS-9003", "reason": "made up, other tool", "expires": "2099-01-01"}
]
JSON
  config_scan || fail "config scan exited non-zero: $(cat "$W/out")"
  grep -qx '  - id: DS-9001' "$LOG.cfgignore" || fail "the live trivy-config id is not ignored: $(cat "$LOG.cfgignore")"
  grep -q 'DS-9002\|DS-9003' "$LOG.cfgignore" && fail "an expired or other-tool id is ignored"
  grep -qx '  made-up-audit:' "$LOG.zizmor" || fail "the live zizmor id is not in the zizmor config: $(cat "$LOG.zizmor")"
  grep -qx '    disable: true' "$LOG.zizmor" || fail "the zizmor rule is not disabled: $(cat "$LOG.zizmor")"
}

test_config_allow_bad_id_refused() {
  setup
  jq -n '[{"tool": "zizmor", "id": "two words", "reason": "made up", "expires": "2099-01-01"}]' >"$W/security/allow.json"
  config_scan && fail "a zizmor id that is not a plain token was accepted"
  out_has "security/allow.json has a zizmor id that is not a plain token"
  assert_eq "zizmor runs with the bad id" 0 "$(count_calls 'zizmor-test:1')"
}

test_config_sarif_written() {
  setup
  SARIF_DIR=$W/sarif config_scan || fail "config scan exited non-zero: $(cat "$W/out")"
  has_call "zizmor-test:1 --offline --format sarif" || fail "zizmor was not run for SARIF"
  jq -e '.version == "2.1.0"' "$W/sarif/zizmor.sarif" >/dev/null || fail "no zizmor SARIF file: $(cat "$W/out")"
}

test_pins_clean() {
  pins_setup
  check_pins || fail "check-pins on the repo's files exited non-zero: $(cat "$W/out")"
}

# Each copy of the Postgres pin in turn gets another digest; check-pins must name that file.
test_pins_postgres_digest_drift_red() {
  local pin f
  pin=$(make_pin PG_IMAGE)
  for f in deploy/compose.yaml deploy/backup/Dockerfile .github/workflows/ci.yml; do
    pins_setup
    grep -qF "$pin" "$W/$f" || fail "$f does not hold the pin $pin"
    sed -i "s|${pin#*@}|$other_digest|" "$W/$f"
    check_pins && fail "a postgres digest drift in $f exited 0"
    out_has "check-pins: $f uses ${pin%@*}@$other_digest"
  done
}

test_pins_postgres_digest_missing_red() {
  local pin
  pin=$(make_pin PG_IMAGE)
  pins_setup
  sed -i "s|$pin|${pin%@*}|" "$W/deploy/compose.yaml"
  check_pins && fail "a postgres copy without its digest exited 0"
  out_has "check-pins: deploy/compose.yaml uses ${pin%@*},"
}

test_pins_pg_image_without_digest_red() {
  local pin
  pin=$(make_pin PG_IMAGE)
  pins_setup
  PG_PIN=${pin%@*} check_pins && fail "a PG_IMAGE without a digest exited 0"
  out_has "is not postgres:<tag>@sha256:<digest>"
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
t "a nuclei templates tarball with the wrong sha256 is red" test_templates_checksum_mismatch_red
t "an alert from a rule zap-rules.tsv does not list is red" test_zap_unknown_rule_red
t "a scoped zap regex reaches zap byte for byte" test_zap_scope_regex_kept_byte_for_byte
t "an allow-listed zap rule is IGNORE in the rules file" test_zap_allow_is_ignore
t "an allow-listed nuclei template reaches -exclude-id" test_nuclei_allow_excluded
t "a scoped zap id is OUTOFSCOPE for its URL and leaves the rule FAIL" test_zap_scoped_allow_is_out_of_scope
t "a malformed scoped zap id is refused" test_zap_malformed_scoped_id_refused
t "config scan: a clean run scans both Dockerfiles and the workflows offline" test_config_clean_run
t "config scan: an argument or a missing image exits 2" test_config_bad_args
t "zizmor medium or high findings (exit 13, 14) are red" test_zizmor_medium_and_high_red
t "zizmor informational or low findings (exit 11, 12) are reported, not red" test_zizmor_low_and_info_reported
t "a zizmor error (exit 1, 2, 3) or a zizmor that cannot start is red" test_zizmor_error_red
t "a Trivy config finding is red" test_trivy_config_findings_red
t "a Trivy config error (exit 1) is red" test_trivy_config_error_red
t "a Trivy config run that scanned no Dockerfile is red" test_trivy_config_nothing_scanned_red
t "a scoped trivy-config id drops the finding on that file" test_trivy_config_scoped_allow_drops_that_file
t "a scoped trivy-config id leaves the same finding on the app Dockerfile red" test_trivy_config_scoped_allow_other_file_red
t "a malformed scoped trivy-config id is refused" test_trivy_config_malformed_scoped_id_refused
t "allow-listed trivy-config and zizmor ids reach the tools" test_config_allow_reaches_tools
t "a zizmor allow id that is not a plain token is refused" test_config_allow_bad_id_refused
t "SARIF_DIR gets the zizmor SARIF file" test_config_sarif_written
t "check-pins: the repo's pins agree" test_pins_clean
t "check-pins: a postgres digest that drifts in any copy is red" test_pins_postgres_digest_drift_red
t "check-pins: a postgres copy without its digest is red" test_pins_postgres_digest_missing_red
t "check-pins: a PG_IMAGE without a digest is red" test_pins_pg_image_without_digest_red
finish
