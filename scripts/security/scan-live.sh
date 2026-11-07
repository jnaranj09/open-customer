#!/usr/bin/env bash
# make scan-live URL=https://... (docs/designs/devsecops.md section 9): a manual scan the deployer
# runs from their own machine against their own deployment, behind any proxy. The tools run in
# Docker images; the host needs Docker, the repository checkout, make, bash 4 or newer, jq and the
# usual coreutils (checked first; a missing one exits 2 naming it). It uses no key, token or
# account, never logs in, and never runs in GitHub (it exits 2 when GITHUB_ACTIONS=true). It runs,
# all signed out:
#   - a header check: curl (CURL_IMAGE) fetches the URL and each security header the app sets
#     (internal/httpapi/middleware.go) must be there with exactly the app's value:
#     X-Content-Type-Options: nosniff, Referrer-Policy: same-origin, the fixed Content-Security-
#     Policy, and, on https, Strict-Transport-Security: max-age=31536000. A missing header or a
#     wrong value is a finding. Only these headers go to the report, never the whole response.
#   - the ZAP baseline (a passive spider on the URL, no attacks) with security/zap-rules.tsv.
#   - Nuclei with the pinned templates (NUCLEI_TEMPLATES, checked against NUCLEI_TEMPLATES_SHA256,
#     downloaded once into NUCLEI_CACHE_DIR). These send non-intrusive but active probes, default
#     credential checks on other products' paths among them; the intrusive, dos and fuzz tags and
#     the out-of-band (interactsh) templates are off. Its ssl templates check the TLS of an https
#     URL.
# The ZAP and Nuclei handling is the same as make dast's (scripts/security/lib.sh): ZAP exit 0 and
# 2 pass, any other exit, an unreadable report or an alert from a rule security/zap-rules.tsv does
# not list is red; Nuclei exits 0 with findings, so its JSONL lines are counted, and a run that
# loaded no templates is red. A tool that fails to run is red, never clean. Every check runs
# before the script exits, so one run reports them all.
#
# The URL is https://host[:port][/path], the one argument or, when there is none, the environment
# variable SCAN_URL (how make passes it, so no shell ever reads it). Plain http is refused, except as a test hook: with NETWORK
# set (a Docker network, or container:<name>, that every tool container joins) http is accepted for
# the host localhost only. That is how the scan was run against a throwaway local stack, because
# the app answers plain http only on localhost; it is not a way to scan a deployment over http.
#
# The allow-list is the deployer's own file deploy/.scan-live.allow (git-ignored), never
# security/allow.json. One entry per line, `<tool> <id> # <reason>`; blank lines and lines
# starting with # are ignored. Tools: zap (a rule number, or <rule number>@<url regex> to allow
# the rule on matching URLs only), nuclei (a template id), header (a lower-case header name from
# the list above). A malformed line is refused with its line number, before anything runs.
#
# Reports go to REPORT_DIR (default security/reports/scan-live-<UTC timestamp>, git-ignored). An
# existing REPORT_DIR that is not empty is refused (exit 2) rather than cleared: the script never
# deletes a path the caller typed, and old files could pass for this run's results. The scan sends no cookie and the app sets none signed out, so no session value can reach the report.
#
# Exit 0 clean, 1 on any finding or failed check, 2 on bad arguments, a malformed allow file or
# GITHUB_ACTIONS=true. (make turns any failure into its own exit 2 and prints the script's code in
# its "Error N" line.)
set -euo pipefail

# shellcheck source=/dev/null
. "$(dirname "$0")/lib.sh" scan-live

usage="usage: scan-live.sh <https://host[:port][/path]> (env ZAP_IMAGE, NUCLEI_IMAGE, NUCLEI_TEMPLATES, NUCLEI_TEMPLATES_SHA256, CURL_IMAGE; optional NETWORK, REPORT_DIR, NUCLEI_CACHE_DIR)"
if [ "${GITHUB_ACTIONS:-}" = true ]; then
  echo "scan-live: never runs in GitHub (it scans a real deployment from the deployer's machine)" >&2
  exit 2
fi
require_host_tools
if [ "$#" -eq 0 ]; then
  set -- "${SCAN_URL:-}"
fi
if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "$usage" >&2
  exit 2
fi
require_env ZAP_IMAGE NUCLEI_IMAGE NUCLEI_TEMPLATES NUCLEI_TEMPLATES_SHA256 CURL_IMAGE
check_nuclei_pins

url=$1
network=${NETWORK:-}
url_re='^(https?)://([A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?|\[[0-9A-Fa-f:]+\])(:[0-9]{1,5})?(/[A-Za-z0-9._~%:+,=;/-]*)?$'
if ! [[ "$url" =~ $url_re ]]; then
  echo "scan-live: '$url' is not https://host[:port][/path]" >&2
  exit 2
fi
origin=${BASH_REMATCH[1]}://${BASH_REMATCH[2]}${BASH_REMATCH[4]}
if [ "${BASH_REMATCH[1]}" = http ] && { [ -z "$network" ] || [ "${BASH_REMATCH[2]}" != localhost ]; }; then
  echo "scan-live: '$url' is plain http; use https (http is accepted only for localhost with NETWORK set, a test hook)" >&2
  exit 2
fi
if [ -n "$network" ] && ! [[ "$network" =~ ^[A-Za-z0-9][A-Za-z0-9_.:-]*$ ]]; then
  echo "scan-live: NETWORK '$network' is not a Docker network name or container:<name>" >&2
  exit 2
fi

cd "$(dirname "$0")/../.."

# The header names the header check covers, as allow-file ids (lower case).
header_ids="x-content-type-options referrer-policy content-security-policy strict-transport-security"
allow_file=deploy/.scan-live.allow

# scope_problem <url regex>: prints what is wrong with a scoped zap id's regex, or nothing. The
# same checks as the Go guard TestSecurityAllowList: it must end in $, compile (grep -E on an empty
# input; ZAP uses Python's re, so this catches typos, not every difference) and match none of the
# probe URLs, so it cannot be a catch-all. ZAP matches from the start of the URL (re.match). The
# probes are the guard's, on localhost, and the same paths on the origin of the scanned URL.
scope_problem() {
  local re=$1 base probe rc=0
  [[ "$re" == *'$' ]] || {
    echo 'the url regex must end in $'
    return 0
  }
  printf '' | grep -E -e "$re" >/dev/null 2>&1 || rc=$?
  if [ "$rc" -gt 1 ]; then
    echo "the url regex does not compile"
    return 0
  fi
  for base in http://localhost:8080 "$origin"; do
    for probe in "$base/" "$base/_app/x.js" "$base/api/v1/zz-scope-probe"; do
      if grep -Eq -e "^($re)" <<<"$probe"; then
        echo "the url regex matches $probe, so it is a catch-all"
        return 0
      fi
    done
  done
}

# The allow file, validated before anything runs. zap_ids is one per line, nuclei_ids comma
# separated (-exclude-id), header_allow space separated.
zap_ids='' nuclei_ids='' header_allow=''
bad_allow() {
  echo "scan-live: $allow_file line $1: $2" >&2
  exit 2
}
if [ -f "$allow_file" ]; then
  line_no=0
  line_re='^[[:space:]]*([^[:space:]#]+)[[:space:]]+([^[:space:]#][^[:space:]]*)([[:space:]]+#(.*))?[[:space:]]*$'
  while IFS= read -r line || [ -n "$line" ]; do
    line_no=$((line_no + 1))
    [[ "$line" =~ ^[[:space:]]*(#.*)?$ ]] && continue
    [[ "$line" =~ $line_re ]] || bad_allow "$line_no" "expected <tool> <id> # <reason>"
    tool=${BASH_REMATCH[1]} id=${BASH_REMATCH[2]} reason=${BASH_REMATCH[4]:-}
    case "$tool" in
      zap | nuclei | header) ;;
      *) bad_allow "$line_no" "unknown tool '$tool' (zap, nuclei or header)" ;;
    esac
    [[ "$reason" =~ [^[:space:]] ]] || bad_allow "$line_no" "needs a reason ($tool $id # <reason>)"
    case "$tool" in
      zap)
        valid_zap_id "$id" || bad_allow "$line_no" "'$id' is not <rule number> or <rule number>@<url regex>"
        if [[ "$id" == *@* ]] && problem=$(scope_problem "${id#*@}") && [ -n "$problem" ]; then
          bad_allow "$line_no" "$problem"
        fi
        zap_ids+="$id"$'\n'
        ;;
      nuclei)
        valid_nuclei_id "$id" || bad_allow "$line_no" "'$id' is not a template id"
        nuclei_ids+="${nuclei_ids:+,}$id"
        ;;
      header)
        case " $header_ids " in
          *" $id "*) header_allow+=" $id" ;;
          *) bad_allow "$line_no" "'$id' is not a checked header ($header_ids)" ;;
        esac
        ;;
    esac
  done <"$allow_file"
fi

report=${REPORT_DIR:-security/reports/scan-live-$(date -u +%Y%m%dT%H%M%SZ)}
templates_cache=${NUCLEI_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/open-customer/nuclei}
# Named after the release and its pinned sha256, so a new sha256 for the same release never reuses
# a folder unpacked from another tarball.
templates=$templates_cache/nuclei-templates-$NUCLEI_TEMPLATES-$NUCLEI_TEMPLATES_SHA256
if [ -e "$report" ] && { [ ! -d "$report" ] || [ -n "$(ls -A "$report")" ]; }; then
  echo "scan-live: report folder $report already exists and is not empty (remove it or set another REPORT_DIR)" >&2
  exit 2
fi
# One container name per run (the PID), so two runs at once never remove each other's container.
scanner=oc-scan-live-$$

cleanup() {
  docker rm -f -v "$scanner" >/dev/null 2>&1 || true
}
# A leftover with this name (a reused PID after a killed run) goes before the first tool starts.
cleanup
trap cleanup EXIT

net=()
[ -z "$network" ] || net=(--network "$network")
# scan_container <docker run args...>: a one-off tool container on the host's network, or on NETWORK.
scan_container() {
  docker run --rm --name "$scanner" "${net[@]}" "$@"
}

fetch_nuclei_templates "$templates_cache" "$templates"

# ZAP runs as its own user, so its work folder is writable by all.
mkdir -p "$report/zap" "$report/nuclei"
chmod a+rwx "$report/zap" "$report/nuclei"
zap_rules_file security/zap-rules.tsv "$report/zap/rules.tsv" "$zap_ids" "allow-listed in $allow_file"
echo "scan-live: scanning $url; reports in $report"

# The header check. The response goes to a variable and only the checked headers to the report.
echo "== scan-live: headers"
expected=(
  "x-content-type-options|X-Content-Type-Options|nosniff"
  "referrer-policy|Referrer-Policy|same-origin"
  "content-security-policy|Content-Security-Policy|frame-ancestors 'none'; base-uri 'self'; object-src 'none'; form-action 'self'"
)
[ "${url%%:*}" != https ] || expected+=("strict-transport-security|Strict-Transport-Security|max-age=31536000")
: >"$report/headers.txt"
rc=0
response=$(scan_container "$CURL_IMAGE" -sS -D - -o /dev/null --max-time 30 "$url") || rc=$?
if [ "$rc" -ne 0 ]; then
  red "header check: cannot fetch $url (curl exit $rc)"
else
  response=$(tr -d '\r' <<<"$response")
  checked=0
  for entry in "${expected[@]}"; do
    IFS='|' read -r id header want <<<"$entry"
    # Every instance of the header, in order: a second, weaker value is a finding too.
    mapfile -t got < <(awk -v n="$id" 'tolower(substr($0, 1, length(n) + 1)) == n ":" { sub(/^[^:]*:[ \t]*/, ""); sub(/[ \t]+$/, ""); print }' <<<"$response")
    if [ "${#got[@]}" -eq 0 ]; then
      echo "$header: (missing)" >>"$report/headers.txt"
    else
      for value in "${got[@]}"; do
        printf '%s: %s\n' "$header" "$value" >>"$report/headers.txt"
      done
    fi
    case " $header_allow " in
      *" $id "*)
        echo "scan-live: header $header: allow-listed in $allow_file"
        continue
        ;;
    esac
    checked=$((checked + 1))
    if [ "${#got[@]}" -eq 0 ]; then
      red "header $header is missing (want '$want')"
    fi
    for value in "${got[@]}"; do
      [ "$value" = "$want" ] || red "header $header is '$value', want '$want'"
    done
  done
  echo "scan-live: headers: checked $checked of ${#expected[@]} ($report/headers.txt)"
fi

run_zap "$report" baseline zap-baseline.py -t "$url"
run_nuclei "$report" "$templates" "$nuclei_ids" "$url"

finish "$report"
