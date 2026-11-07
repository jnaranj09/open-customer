#!/usr/bin/env bash
# Tests for the scan wrappers in scripts/security (docs/designs/devsecops.md, decision D10), and
# for scripts/check-pins.sh's image digest and cosign version checks. Run by make test-sec on the
# host. Each test runs a wrapper from a temp copy of the repo parts it reads (scripts/security,
# security/allow.json, deploy/compose.yaml; made-up Dockerfiles and a workflow for config-scan.sh),
# with a fake `docker` first on PATH that appends each call to a log and answers from env vars:
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
#   curl-test:1 with `-D -` (scan-live.sh's header check) prints a made-up response's headers: the
#                   four security headers with their right values (no Strict-Transport-Security
#                   for an http URL), except the headers named in FAKE_HEADERS_DROP
#                   (space-separated) and with FAKE_HEADERS_WRONG's header (one name) set to
#                   "wrong"; a Set-Cookie header with a made-up cookie is always in the answer
#   curl-test:1 with `-o /cache/<file>` (the templates download) writes a made-up tarball there
# and, for config-scan.sh:
#   trivy-cfg-test:1  reports each file mounted at /src/<path> as a scanned Dockerfile (all
#                   but deploy/backup/Dockerfile when FAKE_TRIVY_CONFIG_EMPTY is set) in a JSON
#                   report at its --output, copies its ignore file to $FAKE_LOG.cfgignore, and gives
#                   each path in FAKE_TRIVY_CONFIG_FINDINGS (space-separated) a DS-0002 finding
#                   unless the ignore file drops it, as Trivy does: an entry `- id: DS-0002` with no
#                   `paths:`, or with `paths:` listing that path. It exits FAKE_TRIVY_CONFIG_CODE
#                   when set, else the run's --exit-code value when a finding is left, else 0
#   zizmor-test:1   copies its --config file to $FAKE_LOG.zizmor, prints a made-up SARIF report
#                   with --format sarif (exit 0), and otherwise exits FAKE_ZIZMOR_CODE (default 0)
# and, for sast.sh:
#   semgrep-test:1  with --test, prints a Semgrep test report on stdout that lists every rule id of
#                   the copied security/semgrep/*.yaml as passed with one expected line (except
#                   FAKE_SEMGREP_UNTESTED, a rule id it leaves out; FAKE_SEMGREP_MISSING_TESTS and
#                   FAKE_SEMGREP_TEST_ERRORS add a config_missing_tests / config_with_errors entry;
#                   FAKE_SEMGREP_TEST_JSON replaces the whole output), exits FAKE_SEMGREP_TEST_CODE
#                   (default 0); otherwise writes a scan report to --json-output: one finding when
#                   FAKE_SEMGREP_FINDINGS is set, one error when FAKE_SEMGREP_ERRORS is set, and the
#                   scanned files FAKE_SEMGREP_SCANNED (space-separated; default one .go, .ts and
#                   .svelte file; set but empty: none). It writes a SARIF file to --sarif-output
#                   and exits FAKE_SEMGREP_CODE (default: 1 with a finding, else 0)
# and, for release-sign.sh:
#   push            prints `<tag>: digest: <FAKE_PUSH_DIGEST> size: 1234` (default: a made-up
#                   digest; none when FAKE_PUSH_NO_DIGEST is set), as docker push does, and a
#                   second such line with FAKE_PUSH_SECOND_DIGEST when that is set
#   image inspect   with RepoDigests in its --format, prints <repository>@FAKE_REPO_DIGEST
#                   (default: the push digest)
#   sbom-test:1     writes a made-up CycloneDX SBOM to sbom.json in the dir mounted at /scan
#                   (nothing when FAKE_SBOM_EMPTY is set)
# and a fake `cosign` that logs "cosign <args>" to the same log, honours FAKE_FAIL_ON and copies
# the --predicate file it was given to $FAKE_LOG.predicate.
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
  FAKE_NUCLEI_FINDINGS FAKE_HEADERS_DROP FAKE_HEADERS_WRONG GITHUB_ACTIONS NETWORK FAKE_TRIVY_CONFIG_CODE FAKE_TRIVY_CONFIG_EMPTY FAKE_TRIVY_CONFIG_FINDINGS FAKE_ZIZMOR_CODE FAKE_SEMGREP_TEST_CODE FAKE_SEMGREP_UNTESTED FAKE_SEMGREP_MISSING_TESTS FAKE_SEMGREP_TEST_ERRORS FAKE_SEMGREP_TEST_JSON FAKE_SEMGREP_FINDINGS \
  FAKE_SEMGREP_ERRORS FAKE_SEMGREP_SCANNED FAKE_SEMGREP_CODE FAKE_PUSH_DIGEST FAKE_PUSH_NO_DIGEST \
  FAKE_REPO_DIGEST FAKE_SBOM_EMPTY FAKE_PUSH_SECOND_DIGEST MODE PULL SARIF_DIR \
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
  case " $* " in
    *" -D - "*) # scan-live.sh's header check
      local h
      {
        printf 'HTTP/2 200\r\n'
        printf 'X-Content-Type-Options: nosniff\r\n'
        printf 'Referrer-Policy: same-origin\r\n'
        printf "Content-Security-Policy: frame-ancestors 'none'; base-uri 'self'; object-src 'none'; form-action 'self'\r\n"
        case "$url" in https://*) printf 'Strict-Transport-Security: max-age=31536000\r\n' ;; esac
        printf 'Set-Cookie: oc_session=made-up-cookie; Path=/; HttpOnly\r\n'
        printf '\r\n'
      } | while IFS= read -r h; do
        h=${h%$'\r'}
        case " ${FAKE_HEADERS_DROP:-} " in *" ${h%%:*} "*) continue ;; esac
        [ "${h%%:*}" != "${FAKE_HEADERS_WRONG:-}" ] || h="${h%%:*}: wrong"
        printf '%s\r\n' "$h"
      done
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
# The made-up digest a push answers with (FAKE_PUSH_DIGEST overrides it).
push_digest=${FAKE_PUSH_DIGEST:-sha256:$(printf 'made-up pushed image\n' | sha256sum | cut -d' ' -f1)}
case "$1" in
  save) # docker save -o <file> <image>
    : >"$3"
    ;;
  push) # docker push <repository>:<tag>
    echo "The push refers to repository [${2%:*}]"
    echo "0123456789ab: Pushed"
    [ -n "${FAKE_PUSH_NO_DIGEST:-}" ] || echo "${2##*:}: digest: $push_digest size: 1234"
    [ -z "${FAKE_PUSH_SECOND_DIGEST:-}" ] || echo "${2##*:}: digest: $FAKE_PUSH_SECOND_DIGEST size: 1234"
    ;;
  image)
    case "$*" in
      *RepoDigests*)
        ref=${*: -1}
        echo "${ref%:*}@${FAKE_REPO_DIGEST:-$push_digest}"
        ;;
    esac
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
        dropped=$(awk '/^  - id: /{ if (open && !paths) print "*"; open = ($3 == "DS-0002"); paths = 0; next }
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
      *" semgrep-test:1 "*)
        src=$(mount_of /src "$@")
        case " $* " in
          *" --test "*)
            ids=$(sed -n 's/^  - id: \([a-z0-9-]*\)$/\1/p' "$src"/security/semgrep/*.yaml | grep -vxF -- "${FAKE_SEMGREP_UNTESTED:-}")
            if [ -n "${FAKE_SEMGREP_TEST_JSON+x}" ]; then
              printf '%s\n' "$FAKE_SEMGREP_TEST_JSON"
              exit "${FAKE_SEMGREP_TEST_CODE:-0}"
            fi
            jq -n --arg ids "$ids" --arg missing "${FAKE_SEMGREP_MISSING_TESTS:-}" --arg errs "${FAKE_SEMGREP_TEST_ERRORS:-}" \
              '{config_missing_tests: ($missing | split(" ") | map(select(. != ""))),
              config_with_errors: ($errs | split(" ") | map(select(. != ""))),
              results: {"x.yaml": {checks: ($ids | split("\n") | map({key: ., value: {passed: true,
                matches: {"f": {expected_lines: [1], reported_lines: [1]}}, errors: []}}) | from_entries)}}}'
            exit "${FAKE_SEMGREP_TEST_CODE:-0}"
            ;;
        esac
        out=$(mount_of /out "$@")
        results='[]' errors='[]'
        if [ -n "${FAKE_SEMGREP_FINDINGS:-}" ]; then
          results='[{"check_id":"security.semgrep.ts-no-eval","path":"web/src/made-up.ts","start":{"line":7},"extra":{"message":"made up finding"}}]'
        fi
        if [ -n "${FAKE_SEMGREP_ERRORS:-}" ]; then
          errors='[{"type":"Syntax error","path":"web/src/made-up.ts"}]'
        fi
        scanned=${FAKE_SEMGREP_SCANNED-/src/a.go /src/b.ts /src/c.svelte}
        jq -n --argjson r "$results" --argjson e "$errors" --arg s "$scanned" \
          '{results: $r, errors: $e, paths: {scanned: ($s | split(" ") | map(select(. != "")))}}' >"$out/semgrep.json"
        case " $* " in *" --sarif-output=/out/semgrep.sarif "*) echo '{"version":"2.1.0","runs":[]}' >"$out/semgrep.sarif" ;; esac
        if [ -n "${FAKE_SEMGREP_CODE:-}" ]; then exit "$FAKE_SEMGREP_CODE"; fi
        [ -z "${FAKE_SEMGREP_FINDINGS:-}" ] || exit 1
        exit 0
        ;;
      *" sbom-test:1 "*)
        if [ -z "${FAKE_SBOM_EMPTY:-}" ]; then
          echo '{"bomFormat":"CycloneDX","specVersion":"1.6","components":[]}' >"$(mount_of /scan "$@")/sbom.json"
        fi
        exit 0
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

cat >"$ROOT/bin/cosign" <<'FAKE'
#!/usr/bin/env bash
# The fake cosign: logs "cosign <args>", honours FAKE_FAIL_ON, keeps the attested predicate.
echo "cosign $*" >>"$FAKE_LOG"
if [ -n "${FAKE_FAIL_ON:-}" ]; then
  case "cosign $*" in *"$FAKE_FAIL_ON"*) exit "${FAKE_FAIL_CODE:-1}" ;; esac
fi
prev=''
for a in "$@"; do
  [ "$prev" != --predicate ] || cp "$a" "$FAKE_LOG.predicate"
  prev=$a
done
exit 0
FAKE
chmod +x "$ROOT/bin/cosign"

# setup: a fresh copy of the repo parts the wrappers read in $W, an empty log in $LOG.
setup() {
  W=$(mktemp -d "$ROOT/run.XXXXXX")
  mkdir -p "$W/scripts" "$W/security" "$W/deploy"
  mkdir -p "$W/api"
  cp -r "$REPO/scripts/security" "$W/scripts/"
  cp "$REPO/deploy/compose.yaml" "$W/deploy/"
  cp "$REPO/security/zap-rules.tsv" "$W/security/"
  cp -r "$REPO/security/semgrep" "$W/security/"
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

# sast: runs sast.sh from another folder with the fake docker; stdout and stderr go to $W/out.
sast() {
  (cd / && PATH="$ROOT/bin:$PATH" SEMGREP_IMAGE=semgrep-test:1 "$W/scripts/security/sast.sh" "$@") >"$W/out" 2>&1
}

# pins_setup: a fresh copy, in $W, of the files scripts/check-pins.sh reads. CHECK_PINS overrides
# the script copied (default: the repo's).
pins_setup() {
  W=$(mktemp -d "$ROOT/pins.XXXXXX")
  mkdir -p "$W/scripts" "$W/web" "$W/deploy/backup" "$W/deploy/scripts" "$W/.github/workflows"
  cp "${CHECK_PINS:-$REPO/scripts/check-pins.sh}" "$W/scripts/check-pins.sh"
  cp "$REPO/deploy/scripts/lib.sh" "$W/deploy/scripts/"
  cp "$REPO/.github/workflows/release.yml" "$W/.github/workflows/"
  cp "$REPO/web/package.json" "$W/web/"
  cp "$REPO/go.mod" "$REPO/Dockerfile" "$W/"
  cp "$REPO/deploy/compose.yaml" "$W/deploy/"
  cp "$REPO/deploy/backup/Dockerfile" "$W/deploy/backup/"
  cp "$REPO/.github/workflows/ci.yml" "$W/.github/workflows/"
}

# The pins make check-pins passes: the images from security/tools.Dockerfile (by stage name, as the
# Makefile's tool_image reads them), the rest from the Makefile.
make_pin() {
  case "$1" in
    PG_IMAGE) awk '$1 == "FROM" && $4 == "postgres" { print $2 }' "$REPO/security/tools.Dockerfile" ;;
    GITLEAKS_IMAGE) awk '$1 == "FROM" && $4 == "gitleaks" { print $2 }' "$REPO/security/tools.Dockerfile" ;;
    *) sed -n "s/^$1 := //p" "$REPO/Makefile" ;;
  esac
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

# scan_live <args...>: runs scan-live.sh from another folder with the fake docker; stdout and
# stderr go to $W/out. Same templates cache and pins as dast; the report goes to $W/report unless
# DEFAULT_REPORT is set (the script then picks security/reports/scan-live-<UTC timestamp>).
scan_live() {
  mkdir -p "$W/nuclei-cache"
  [ -n "${EMPTY_CACHE:-}" ] || mkdir -p "$W/nuclei-cache/nuclei-templates-v10.5.1"
  (cd / && PATH="$ROOT/bin:$PATH" REPORT_DIR="$([ -n "${DEFAULT_REPORT:-}" ] || echo "$W/report")" \
    ZAP_IMAGE=zap-test:1 NUCLEI_IMAGE=nuclei-test:1 NUCLEI_TEMPLATES=v10.5.1 CURL_IMAGE=curl-test:1 \
    NUCLEI_TEMPLATES_SHA256="${TEMPLATES_SHA:-$fake_tarball_sha}" NUCLEI_CACHE_DIR="$W/nuclei-cache" \
    "$W/scripts/security/scan-live.sh" "$@") >"$W/out" 2>&1
}

test_scan_live_clean_run() {
  setup
  scan_live https://example.com || fail "a clean scan-live run exited non-zero: $(cat "$W/out")"
  assert_eq "header checks" 1 "$(count_calls 'curl-test:1 -sS -D -')"
  assert_eq "zap runs" 1 "$(count_calls 'zap-test:1')"
  assert_eq "zap script" 1 "$(count_calls 'zap-test:1 zap-baseline.py -t https://example.com')"
  assert_eq "nuclei runs" 1 "$(count_calls 'nuclei-test:1')"
  has_call "-ni " || fail "nuclei was not run without out-of-band templates"
  has_call "-etags intrusive,dos,fuzz" || fail "nuclei was not run without the intrusive tags"
  grep -qE '(^| )(-p|--publish)( |=)' "$LOG" && fail "a container published a port"
  has_call "--network" && fail "a scanner was given a network without NETWORK"
  has_call "rm -f -v oc-scan-live-scanner" || fail "the scanner container was not removed"
  out_has "scan-live: ok"
  [ -f "$W/report/zap/baseline.json" ] || fail "no zap report in the report folder"
  [ -f "$W/report/headers.txt" ] || fail "no header report in the report folder"
}

test_scan_live_report_has_no_cookie() {
  setup
  scan_live https://example.com || fail "scan-live exited non-zero: $(cat "$W/out")"
  grep -rqi 'made-up-cookie\|set-cookie' "$W/report" "$W/out" && fail "a cookie reached the report or the output"
  grep -qi 'Content-Security-Policy' "$W/report/headers.txt" || fail "the checked headers are not in headers.txt"
  return 0
}

test_scan_live_default_report_folder() {
  setup
  DEFAULT_REPORT=1 scan_live https://example.com || fail "scan-live exited non-zero: $(cat "$W/out")"
  compgen -G "$W/security/reports/scan-live-[0-9]*Z/headers.txt" >/dev/null ||
    fail "no report in security/reports/scan-live-<UTC timestamp>: $(ls "$W/security" 2>&1)"
}

test_scan_live_gitignore() {
  git -C "$REPO" check-ignore -q security/reports/scan-live-20260101T000000Z/headers.txt ||
    fail "security/reports/ is not git-ignored"
  git -C "$REPO" check-ignore -q deploy/.scan-live.allow || fail "deploy/.scan-live.allow is not git-ignored"
}

test_scan_live_make_target() {
  local out
  out=$(cd "$REPO" && make -n scan-live URL=https://example.com NETWORK=oc-made-up 2>&1) || fail "make -n scan-live failed: $out"
  grep -qF 'scripts/security/scan-live.sh "https://example.com"' <<<"$out" || fail "make does not pass URL: $out"
  grep -qF 'NETWORK=oc-made-up' <<<"$out" || fail "make does not pass NETWORK: $out"
  grep -q 'CURL_IMAGE=curlimages/curl:[^ ]*@sha256:' <<<"$out" || fail "make does not pass the pinned curl image: $out"
}

test_scan_live_without_url_exits_2() {
  setup
  local rc=0
  scan_live || rc=$?
  assert_eq "exit code with no URL" 2 "$rc"
  rc=0
  scan_live '' || rc=$?
  assert_eq "exit code with an empty URL" 2 "$rc"
  rc=0
  scan_live https://example.com extra || rc=$?
  assert_eq "exit code with two arguments" 2 "$rc"
  assert_eq "docker calls for bad args" 0 "$(wc -l <"$LOG")"
}

test_scan_live_in_github_actions_exits_2() {
  setup
  local rc=0
  GITHUB_ACTIONS=true scan_live https://example.com || rc=$?
  assert_eq "exit code in GitHub Actions" 2 "$rc"
  out_has "never runs in GitHub"
  assert_eq "docker calls in GitHub Actions" 0 "$(wc -l <"$LOG")"
}

test_scan_live_refuses_non_https_url() {
  setup
  local url rc
  for url in http://example.com http://example.com:8080/ http://localhost:8080 ftp://example.com example.com \
    'https://user:pass@example.com' 'https://example.com/a b' 'https://example.com/?q=1' 'https://'; do
    rc=0
    scan_live "$url" || rc=$?
    assert_eq "exit code for '$url'" 2 "$rc"
  done
  assert_eq "docker calls for refused URLs" 0 "$(wc -l <"$LOG")"
}

test_scan_live_http_only_on_localhost_with_network() {
  setup
  NETWORK=container:oc-made-up scan_live http://localhost:8080 || fail "http://localhost with NETWORK exited non-zero: $(cat "$W/out")"
  assert_eq "scanner containers on the given network" 0 \
    "$(grep -E 'zap-test:1|nuclei-test:1|curl-test:1 -sS' "$LOG" | grep -vc -- '--network container:oc-made-up')"
  assert_eq "zap runs" 1 "$(count_calls 'zap-test:1 zap-baseline.py -t http://localhost:8080')"
  local rc=0
  NETWORK=container:oc-made-up scan_live http://example.com || rc=$?
  assert_eq "exit code for http on another host, with NETWORK" 2 "$rc"
}

test_scan_live_missing_header_is_a_finding() {
  local h
  for h in X-Content-Type-Options Referrer-Policy Content-Security-Policy Strict-Transport-Security; do
    setup
    FAKE_HEADERS_DROP=$h scan_live https://example.com && fail "a response without $h exited 0"
    out_has "header $h is missing"
    assert_eq "zap runs after a missing $h" 1 "$(count_calls 'zap-test:1')"
    assert_eq "nuclei runs after a missing $h" 1 "$(count_calls 'nuclei-test:1')"
  done
}

test_scan_live_wrong_header_value_is_a_finding() {
  local h
  for h in X-Content-Type-Options Referrer-Policy Content-Security-Policy Strict-Transport-Security; do
    setup
    FAKE_HEADERS_WRONG=$h scan_live https://example.com && fail "a response with a wrong $h exited 0"
    out_has "header $h is 'wrong', want"
  done
}

test_scan_live_hsts_only_on_https() {
  setup
  FAKE_HEADERS_DROP=Strict-Transport-Security NETWORK=container:oc-made-up scan_live http://localhost:8080 ||
    fail "a plain-http scan wanted HSTS: $(cat "$W/out")"
}

test_scan_live_header_fetch_fails_red() {
  setup
  FAKE_FAIL_ON='curl-test:1 -sS -D -' FAKE_FAIL_CODE=7 scan_live https://example.com && fail "a header check that could not fetch exited 0"
  out_has "cannot fetch https://example.com"
  assert_eq "zap runs after a failed header fetch" 1 "$(count_calls 'zap-test:1')"
}

test_scan_live_nuclei_findings_exit_0_red() {
  setup
  FAKE_NUCLEI_FINDINGS=1 scan_live https://example.com && fail "nuclei findings with exit 0 exited 0"
  out_has "nuclei: 1 findings"
}

test_scan_live_nuclei_no_templates_red() {
  setup
  FAKE_NUCLEI_TEMPLATES=0 scan_live https://example.com && fail "a nuclei run with no templates exited 0"
  out_has "nuclei loaded no templates"
}

test_scan_live_tool_cannot_start_red() {
  setup
  local rc=0
  FAKE_FAIL_ON=nuclei-test:1 FAKE_FAIL_CODE=125 scan_live https://example.com || rc=$?
  assert_eq "exit code when nuclei cannot start" 1 "$rc"
  out_has "nuclei could not scan (exit 125)"
  rc=0
  FAKE_FAIL_ON=zap-test:1 FAKE_FAIL_CODE=125 scan_live https://example.com || rc=$?
  assert_eq "exit code when zap cannot start" 1 "$rc"
  out_has "zap could not scan (baseline, exit 125)"
}

test_scan_live_zap_exit_codes() {
  setup
  FAKE_ZAP_CODE=1 scan_live https://example.com && fail "zap exit 1 exited 0"
  out_has "blocking findings"
  FAKE_ZAP_CODE=3 scan_live https://example.com && fail "zap exit 3 exited 0"
  out_has "zap could not scan"
  FAKE_ZAP_CODE=2 scan_live https://example.com || fail "zap exit 2 (WARN only) exited non-zero: $(cat "$W/out")"
}

test_scan_live_zap_report_checks() {
  setup
  FAKE_ZAP_REPORT='not json' scan_live https://example.com && fail "an unreadable zap report exited 0"
  out_has "wrote no readable JSON report"
  FAKE_ZAP_REPORT='{"site":[{"alerts":[{"pluginid":"99999","riskcode":"3","count":"1"}]}]}' scan_live https://example.com &&
    fail "an alert from a rule zap-rules.tsv does not list exited 0"
  out_has "rules not in security/zap-rules.tsv: 99999"
}

test_scan_live_templates_checksum_mismatch_red() {
  setup
  TEMPLATES_SHA=$(printf 'other\n' | sha256sum | cut -d' ' -f1) EMPTY_CACHE=1 scan_live https://example.com &&
    fail "a templates tarball with the wrong sha256 exited 0"
  out_has "does not match NUCLEI_TEMPLATES_SHA256"
  assert_eq "nuclei runs after a checksum mismatch" 0 "$(count_calls 'nuclei-test:1')"
}

test_scan_live_allow_file_ids_are_excluded() {
  setup
  cat >"$W/deploy/.scan-live.allow" <<'ALLOW'
# The proxy adds its own CSP and HSTS.

zap 10021 # made up: the proxy sets nosniff
zap 40018@.*/api/v1/client-errors$ # made up: scoped
nuclei made-up-template # made up
nuclei other.template-2 # made up
header strict-transport-security # made up: the proxy sets it
ALLOW
  FAKE_HEADERS_DROP=Strict-Transport-Security scan_live https://example.com ||
    fail "scan-live with an allow-listed header exited non-zero: $(cat "$W/out")"
  grep -qP '^10021\tIGNORE\t' "$LOG.zaprules" || fail "the zap id is not IGNORE: $(cat "$LOG.zaprules")"
  grep -qxP '40018\tOUTOFSCOPE\t\.\*/api/v1/client-errors\$' "$LOG.zaprules" || fail "no OUTOFSCOPE line for the scoped id"
  has_call "-exclude-id made-up-template,other.template-2 " || fail "the nuclei ids did not reach -exclude-id"
  # Without the file the same response is red.
  rm "$W/deploy/.scan-live.allow"
  FAKE_HEADERS_DROP=Strict-Transport-Security scan_live https://example.com && fail "the header finding stayed allowed without the file"
  grep -qP '^10021\tFAIL\t' "$LOG.zaprules" || fail "zap 10021 is not FAIL without the file"
}

test_scan_live_ignores_security_allow_json() {
  setup
  cat >"$W/security/allow.json" <<'JSON'
[
  {"tool": "zap", "id": "10021", "reason": "made up", "expires": "2099-01-01"},
  {"tool": "nuclei", "id": "made-up-template", "reason": "made up", "expires": "2099-01-01"}
]
JSON
  scan_live https://example.com || fail "scan-live exited non-zero: $(cat "$W/out")"
  grep -qP '^10021\tFAIL\t' "$LOG.zaprules" || fail "security/allow.json reached the zap rules"
  has_call "made-up-template" && fail "security/allow.json reached nuclei"
  return 0
}

test_scan_live_malformed_allow_line_refused() {
  local line want
  while IFS='|' read -r line want; do
    setup
    printf '# a comment\n\nzap 10021 # fine\n%s\n' "$line" >"$W/deploy/.scan-live.allow"
    local rc=0
    scan_live https://example.com || rc=$?
    assert_eq "exit code for '$line'" 2 "$rc"
    out_has "deploy/.scan-live.allow line 4: $want"
    assert_eq "docker calls for '$line'" 0 "$(wc -l <"$LOG")"
  done <<'CASES'
zap 10021|needs a reason
zap 10021 #|needs a reason
zap # reason|expected <tool> <id> # <reason>
trivy CVE-2026-1 # reason|unknown tool 'trivy'
zap abc # reason|'abc' is not <rule number> or <rule number>@<url regex>
zap 40018@ # reason|'40018@' is not <rule number> or <rule number>@<url regex>
zap 10021 10020 # reason|expected <tool> <id> # <reason>
nuclei bad,id # reason|'bad,id' is not a template id
nuclei -exclude # reason|'-exclude' is not a template id
header Strict_Transport # reason|'Strict_Transport' is not a checked header
header server # reason|'server' is not a checked header
CASES
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

test_config_unlisted_dockerfile_red() {
  local f
  for f in web/extra.Dockerfile tools/Dockerfile; do
    setup
    mkdir -p "$W/$(dirname "$f")"
    printf 'FROM scratch\n' >"$W/$f"
    config_scan && fail "a Dockerfile config-scan does not list ($f) exited 0"
    out_has "config-scan: $f is a Dockerfile config-scan.sh neither scans nor lists as never built"
  done
}

test_config_tools_dockerfile_not_scanned() {
  setup
  mkdir -p "$W/security" "$W/web/node_modules/x"
  printf 'FROM scratch AS made-up\n' >"$W/security/tools.Dockerfile"
  printf 'FROM scratch\n' >"$W/web/node_modules/x/Dockerfile"
  config_scan || fail "the tools Dockerfile or a node_modules one made the scan red: $(cat "$W/out")"
  has_call "tools.Dockerfile" && fail "the never-built tools Dockerfile was scanned"
  return 0
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

# A scoped entry followed by another entry: the scope still holds (the fake must not read the
# scoped entry as a plain one when the next entry starts).
test_trivy_config_scoped_then_plain_entry() {
  setup
  jq -n '[{"tool": "trivy-config", "id": "DS-0002@deploy/backup/Dockerfile", "reason": "made up", "expires": "2099-01-01"},
    {"tool": "trivy-config", "id": "DS-9001", "reason": "made up", "expires": "2099-01-01"}]' >"$W/security/allow.json"
  FAKE_TRIVY_CONFIG_FINDINGS='Dockerfile deploy/backup/Dockerfile' config_scan &&
    fail "a scoped entry followed by another entry silenced the app Dockerfile"
  out_has "config-scan: Dockerfile: high or critical misconfigurations"
  grep -qx '  - id: DS-9001' "$LOG.cfgignore" || fail "the second entry is missing: $(cat "$LOG.cfgignore")"
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

test_sast_clean_run() {
  setup
  sast || fail "a clean sast run exited non-zero: $(cat "$W/out")"
  assert_eq "semgrep test runs" 1 "$(count_calls 'semgrep-test:1 semgrep --test')"
  assert_eq "semgrep scan runs" 1 "$(count_calls 'semgrep-test:1 semgrep scan --config security/semgrep')"
  assert_eq "semgrep runs with a network or telemetry" 0 \
    "$(grep 'semgrep-test:1' "$LOG" | grep -vc -e '--network none.*--metrics=off --disable-version-check')"
  has_call "semgrep scan --config security/semgrep --error" || fail "the scan does not exit non-zero on findings"
  has_call "rm -f oc-sast-test oc-sast-scan" || fail "the containers were not removed"
  out_has "sast: ok"
}

test_sast_findings_red() {
  setup
  FAKE_SEMGREP_FINDINGS=1 sast && fail "a semgrep finding exited 0"
  out_has "web/src/made-up.ts:7 ts-no-eval"
  out_has "sast: 1 findings"
}

test_sast_tool_error_red() {
  local code
  for code in 2 3 7; do
    setup
    FAKE_SEMGREP_CODE=$code sast && fail "a semgrep scan exit $code exited 0"
    out_has "semgrep could not scan the repo (exit $code)"
  done
}

test_sast_tool_cannot_start_red() {
  setup
  FAKE_FAIL_ON="semgrep-test:1" FAKE_FAIL_CODE=125 sast && fail "a semgrep that cannot start exited 0"
  out_has "semgrep --test failed"
  out_has "semgrep could not scan the repo (exit 125)"
}

test_sast_fixtures_that_fail_red() {
  setup
  FAKE_SEMGREP_TEST_CODE=1 sast && fail "a failing semgrep --test exited 0"
  out_has "semgrep --test failed or its fixtures do not match the rules"
}

test_sast_rule_without_fixture_red() {
  setup
  FAKE_SEMGREP_UNTESTED=ts-no-eval sast && fail "a rule without a fixture exited 0"
  out_has "every rule in security/semgrep needs a fixture"
}

test_sast_config_missing_tests_red() {
  setup
  FAKE_SEMGREP_MISSING_TESTS=made-up.yaml sast && fail "a rule file with no fixture file exited 0"
  out_has "every rule in security/semgrep needs a fixture"
}

test_sast_config_with_errors_red() {
  setup
  FAKE_SEMGREP_TEST_ERRORS=made-up.yaml sast && fail "a rule file with errors exited 0"
  out_has "every rule in security/semgrep needs a fixture"
}

test_sast_unparseable_test_report_red() {
  setup
  FAKE_SEMGREP_TEST_JSON='not json at all' sast && fail "a --test report that is not JSON (exit 0) exited 0"
  out_has "semgrep --test failed or its fixtures do not match the rules (exit 0)"
}

test_sast_exit_1_without_findings_red() {
  setup
  FAKE_SEMGREP_CODE=1 sast && fail "exit 1 with an empty report exited 0"
  out_has "exited 1 (findings) but its report lists none"
}

test_sast_findings_with_exit_0_red() {
  setup
  FAKE_SEMGREP_FINDINGS=1 FAKE_SEMGREP_CODE=0 sast && fail "findings with exit 0 exited 0"
  out_has "sast: 1 findings"
}

test_sast_scanned_nothing_red() {
  setup
  FAKE_SEMGREP_SCANNED='' sast && fail "a scan of 0 files exited 0"
  out_has "semgrep scanned no .go file"
  out_has "semgrep scanned no .ts file"
  out_has "semgrep scanned no .svelte file"
}

test_sast_scanned_no_svelte_red() {
  setup
  FAKE_SEMGREP_SCANNED="/src/a.go /src/b.ts" sast && fail "a scan with no .svelte file exited 0"
  out_has "semgrep scanned no .svelte file"
}

test_sast_report_error_red() {
  setup
  FAKE_SEMGREP_ERRORS=1 sast && fail "a semgrep report with an error exited 0"
  out_has "semgrep reported 1 errors"
}

test_sast_allow_excluded() {
  setup
  cat >"$W/security/allow.json" <<'JSON'
[
  {"tool": "semgrep", "id": "ts-no-eval", "reason": "made up", "expires": "2099-01-01"},
  {"tool": "semgrep", "id": "ts-no-inner-html", "reason": "made up, expired", "expires": "2000-01-01"},
  {"tool": "zizmor", "id": "no-slog-of-request-body", "reason": "made up, other tool", "expires": "2099-01-01"}
]
JSON
  sast || fail "sast exited non-zero: $(cat "$W/out")"
  has_call "--exclude-rule security.semgrep.ts-no-eval" || fail "the live semgrep id did not reach --exclude-rule"
  has_call "ts-no-inner-html" && fail "an expired id reached semgrep"
  has_call "no-slog-of-request-body" && fail "another tool's id reached semgrep"
  assert_eq "excluded rules" 1 "$(grep -o -e '--exclude-rule' "$LOG" | wc -l)"
}

test_sast_allow_bad_id_refused() {
  local id
  for id in "two words" 'ts-no-eval --config /tmp' 'security.semgrep.ts-no-eval' 'no-such-rule' '../x'; do
    setup
    jq -n --arg id "$id" '[{"tool": "semgrep", "id": $id, "reason": "made up", "expires": "2099-01-01"}]' >"$W/security/allow.json"
    sast && fail "the semgrep id '$id' was accepted"
    out_has "security/allow.json has a semgrep id that is not the id of a rule in security/semgrep"
    assert_eq "semgrep runs with the bad id '$id'" 0 "$(count_calls 'semgrep-test:1')"
  done
}

test_sast_bad_args() {
  setup
  local rc=0
  sast extra || rc=$?
  assert_eq "exit code with an argument" 2 "$rc"
  rc=0
  (cd / && PATH="$ROOT/bin:$PATH" "$W/scripts/security/sast.sh") >"$W/out" 2>&1 || rc=$?
  assert_eq "exit code without SEMGREP_IMAGE" 2 "$rc"
}

test_sast_sarif_written() {
  setup
  SARIF_DIR=$W/sarif sast || fail "sast exited non-zero: $(cat "$W/out")"
  has_call "--sarif-output=/out/semgrep.sarif" || fail "semgrep was not asked for SARIF"
  jq -e '.version == "2.1.0"' "$W/sarif/semgrep.sarif" >/dev/null || fail "no semgrep SARIF file: $(cat "$W/out")"
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

# release_sign <image:tag>: runs release-sign.sh from another folder with the fake docker and
# cosign; stdout and stderr go to $W/out.
release_sign() {
  (cd / && PATH="$ROOT/bin:$PATH" TRIVY_IMAGE=sbom-test:1 "$W/scripts/security/release-sign.sh" "$@") >"$W/out" 2>&1
}

fake_push_digest=sha256:$(printf 'made-up pushed image\n' | sha256sum | cut -d' ' -f1)
release_ref=ghcr.io/made-up/app:1.2.3

test_release_sign_signs_the_pushed_digest() {
  setup
  release_sign "$release_ref" || fail "release-sign exited non-zero: $(cat "$W/out")"
  has_call "push $release_ref" || fail "the image was not pushed: $(cat "$LOG")"
  has_call "--network none" || fail "the SBOM run is not offline: $(cat "$LOG")"
  has_call "sbom-test:1 image --cache-dir /scan/cache --format cyclonedx --output /scan/sbom.json" ||
    fail "no CycloneDX SBOM run: $(cat "$LOG")"
  has_call "cosign sign --yes ghcr.io/made-up/app@$fake_push_digest" || fail "the digest was not signed: $(cat "$LOG")"
  has_call "cosign attest --yes --type cyclonedx --predicate " || fail "no SBOM attestation: $(cat "$LOG")"
  has_call " ghcr.io/made-up/app@$fake_push_digest" || fail "the attestation is not on the digest"
  # Never the tag.
  if grep -E '^cosign ' "$LOG" | grep -qF ":1.2.3"; then fail "cosign got the tag: $(cat "$LOG")"; fi
  grep -qF '"bomFormat":"CycloneDX"' "$LOG.predicate" || fail "the attested predicate is not the SBOM"
  # The push comes first, the signature after the SBOM, the attestation last.
  [ "$(grep -nF "push $release_ref" "$LOG" | cut -d: -f1)" -lt "$(grep -nF "cosign sign" "$LOG" | cut -d: -f1)" ] ||
    fail "signed before the push: $(cat "$LOG")"
  [ "$(grep -nF "cosign sign" "$LOG" | cut -d: -f1)" -lt "$(grep -nF "cosign attest" "$LOG" | cut -d: -f1)" ] ||
    fail "attested before signing: $(cat "$LOG")"
}

test_release_sign_no_digest_red() {
  setup
  local rc=0
  FAKE_PUSH_NO_DIGEST=1 release_sign "$release_ref" || rc=$?
  assert_eq "exit code" 1 "$rc"
  out_has "no digest"
  if grep -q '^cosign ' "$LOG"; then fail "cosign ran without a digest: $(cat "$LOG")"; fi
}

test_release_sign_two_digests_red() {
  setup
  local rc=0
  FAKE_PUSH_SECOND_DIGEST=$other_digest release_sign "$release_ref" || rc=$?
  assert_eq "exit code" 1 "$rc"
  out_has "printed two digests"
  if grep -q '^cosign ' "$LOG"; then fail "cosign ran with two digests: $(cat "$LOG")"; fi
}

test_release_sign_digest_mismatch_red() {
  setup
  local rc=0
  FAKE_REPO_DIGEST=$other_digest release_sign "$release_ref" || rc=$?
  assert_eq "exit code" 1 "$rc"
  out_has "does not record"
  if grep -q '^cosign ' "$LOG"; then fail "cosign ran on a digest the local image does not have: $(cat "$LOG")"; fi
}

test_release_sign_push_fails_red() {
  setup
  local rc=0
  FAKE_FAIL_ON="push $release_ref" release_sign "$release_ref" || rc=$?
  assert_eq "exit code" 1 "$rc"
  if grep -q '^cosign ' "$LOG"; then fail "cosign ran after a failed push: $(cat "$LOG")"; fi
}

test_release_sign_sbom_missing_red() {
  setup
  local rc=0
  FAKE_SBOM_EMPTY=1 release_sign "$release_ref" || rc=$?
  assert_eq "exit code" 1 "$rc"
  out_has "SBOM"
  if grep -q '^cosign attest' "$LOG"; then fail "attested without an SBOM: $(cat "$LOG")"; fi
}

test_release_sign_cosign_fails_red() {
  local step rc
  for step in "cosign sign" "cosign attest"; do
    setup
    rc=0
    FAKE_FAIL_ON=$step release_sign "$release_ref" || rc=$?
    assert_eq "exit code when $step fails" 1 "$rc"
  done
}

test_release_sign_bad_args() {
  setup
  local rc=0
  release_sign || rc=$?
  assert_eq "exit code without an image" 2 "$rc"
  rc=0
  release_sign "ghcr.io/made-up/app@$fake_push_digest" || rc=$?
  assert_eq "exit code for a ref with no tag" 2 "$rc"
  rc=0
  (cd / && PATH="$ROOT/bin:$PATH" "$W/scripts/security/release-sign.sh" "$release_ref") >"$W/out" 2>&1 || rc=$?
  assert_eq "exit code without TRIVY_IMAGE" 2 "$rc"
  assert_eq "docker calls for bad args" 0 "$(wc -l <"$LOG")"
}

# The cosign release the workflow installs and the COSIGN_IMAGE tag in lib.sh drift apart, in
# either file; check-pins must name both.
test_pins_cosign_drift_red() {
  local f
  for f in .github/workflows/release.yml deploy/scripts/lib.sh; do
    pins_setup
    grep -qF "v3.1.4" "$W/$f" || fail "$f does not hold cosign v3.1.4"
    sed -i 's/v3\.1\.4/v3.1.3/' "$W/$f"
    check_pins && fail "a cosign version drift in $f exited 0"
    out_has "release.yml cosign-release"
    out_has "deploy/scripts/lib.sh COSIGN_IMAGE"
  done
}

test_pins_cosign_missing_red() {
  pins_setup
  sed -i '/^COSIGN_IMAGE=/d' "$W/deploy/scripts/lib.sh"
  check_pins && fail "a lib.sh without COSIGN_IMAGE exited 0"
  out_has "deploy/scripts/lib.sh has no COSIGN_IMAGE"
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
t "config scan: a Dockerfile it does not list is red" test_config_unlisted_dockerfile_red
t "config scan: the tools Dockerfile and node_modules are not scanned" test_config_tools_dockerfile_not_scanned
t "zizmor medium or high findings (exit 13, 14) are red" test_zizmor_medium_and_high_red
t "zizmor informational or low findings (exit 11, 12) are reported, not red" test_zizmor_low_and_info_reported
t "a zizmor error (exit 1, 2, 3) or a zizmor that cannot start is red" test_zizmor_error_red
t "a Trivy config finding is red" test_trivy_config_findings_red
t "a Trivy config error (exit 1) is red" test_trivy_config_error_red
t "a Trivy config run that scanned no Dockerfile is red" test_trivy_config_nothing_scanned_red
t "a scoped trivy-config id drops the finding on that file" test_trivy_config_scoped_allow_drops_that_file
t "a scoped trivy-config id leaves the same finding on the app Dockerfile red" test_trivy_config_scoped_allow_other_file_red
t "a malformed scoped trivy-config id is refused" test_trivy_config_malformed_scoped_id_refused
t "a scoped trivy-config id followed by another entry keeps its scope" test_trivy_config_scoped_then_plain_entry
t "allow-listed trivy-config and zizmor ids reach the tools" test_config_allow_reaches_tools
t "a zizmor allow id that is not a plain token is refused" test_config_allow_bad_id_refused
t "SARIF_DIR gets the zizmor SARIF file" test_config_sarif_written
t "sast: a clean run tests the rules, scans offline and cleans up" test_sast_clean_run
t "sast: an argument or a missing image exits 2" test_sast_bad_args
t "sast: a finding is red" test_sast_findings_red
t "scan-live: a clean run scans on the host network and cleans up" test_scan_live_clean_run
t "scan-live: no cookie reaches the report or the output" test_scan_live_report_has_no_cookie
t "scan-live: the report goes to security/reports/scan-live-<UTC timestamp>" test_scan_live_default_report_folder
t "scan-live: the report folder and the local allow file are git-ignored" test_scan_live_gitignore
t "scan-live: make passes URL, NETWORK and the pinned images" test_scan_live_make_target
t "scan-live without URL exits 2" test_scan_live_without_url_exits_2
t "scan-live exits 2 in GitHub Actions" test_scan_live_in_github_actions_exits_2
t "scan-live refuses a non-https URL" test_scan_live_refuses_non_https_url
t "scan-live: http is allowed only for localhost, and only with NETWORK" test_scan_live_http_only_on_localhost_with_network
t "a missing header is a finding" test_scan_live_missing_header_is_a_finding
t "scan-live: a wrong header value is a finding" test_scan_live_wrong_header_value_is_a_finding
t "scan-live: HSTS is expected on https only" test_scan_live_hsts_only_on_https
t "scan-live: a header check that cannot fetch is red" test_scan_live_header_fetch_fails_red
t "scan-live: nuclei findings with exit 0 are red" test_scan_live_nuclei_findings_exit_0_red
t "scan-live: a nuclei run with no templates is red" test_scan_live_nuclei_no_templates_red
t "scan-live: a tool that cannot start is red" test_scan_live_tool_cannot_start_red
t "scan-live: zap exit 1 and 3 are red, 2 passes" test_scan_live_zap_exit_codes
t "scan-live: an unreadable zap report or an unknown zap rule is red" test_scan_live_zap_report_checks
t "scan-live: a templates tarball with the wrong sha256 is red" test_scan_live_templates_checksum_mismatch_red
t "ids in deploy/.scan-live.allow are excluded" test_scan_live_allow_file_ids_are_excluded
t "scan-live: security/allow.json is not read" test_scan_live_ignores_security_allow_json
t "scan-live: a malformed line in the allow file is refused with its line number" test_scan_live_malformed_allow_line_refused
t "sast: a semgrep error (exit 2 and others) is red" test_sast_tool_error_red
t "sast: a semgrep that cannot start is red" test_sast_tool_cannot_start_red
t "sast: fixtures that do not match their rules are red" test_sast_fixtures_that_fail_red
t "sast: a rule with no fixture is red" test_sast_rule_without_fixture_red
t "sast: a rule file with no fixture file is red" test_sast_config_missing_tests_red
t "sast: a rule file with errors is red" test_sast_config_with_errors_red
t "sast: a --test report that is not JSON, with exit 0, is red" test_sast_unparseable_test_report_red
t "sast: exit 1 with no finding in the report is red" test_sast_exit_1_without_findings_red
t "sast: findings with exit 0 are red" test_sast_findings_with_exit_0_red
t "sast: a scan of 0 files is red" test_sast_scanned_nothing_red
t "sast: a scan with no Svelte file is red" test_sast_scanned_no_svelte_red
t "sast: an error in the report is red" test_sast_report_error_red
t "sast: an allow-listed rule is excluded" test_sast_allow_excluded
t "sast: a malformed or unknown semgrep allow id is refused" test_sast_allow_bad_id_refused
t "sast: SARIF_DIR gets the semgrep SARIF file" test_sast_sarif_written
t "release-sign: signs and attests the pushed digest, never the tag" test_release_sign_signs_the_pushed_digest
t "release-sign: a push with no digest is red and signs nothing" test_release_sign_no_digest_red
t "release-sign: a push that prints two digests is red and signs nothing" test_release_sign_two_digests_red
t "release-sign: a push digest the local image does not record is red" test_release_sign_digest_mismatch_red
t "release-sign: a failed push is red and signs nothing" test_release_sign_push_fails_red
t "release-sign: a missing SBOM is red and attests nothing" test_release_sign_sbom_missing_red
t "release-sign: a failed sign or attest is red" test_release_sign_cosign_fails_red
t "release-sign: a bad argument or a missing TRIVY_IMAGE exits 2" test_release_sign_bad_args
t "check-pins: the repo's pins agree" test_pins_clean
t "check-pins: a postgres digest that drifts in any copy is red" test_pins_postgres_digest_drift_red
t "check-pins: a postgres copy without its digest is red" test_pins_postgres_digest_missing_red
t "check-pins: a PG_IMAGE without a digest is red" test_pins_pg_image_without_digest_red
t "check-pins: a cosign version that drifts between release.yml and lib.sh is red" test_pins_cosign_drift_red
t "check-pins: a lib.sh without COSIGN_IMAGE is red" test_pins_cosign_missing_red
finish
