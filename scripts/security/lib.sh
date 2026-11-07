# shellcheck shell=bash
# Shared by dast.sh and scan-live.sh (docs/designs/devsecops.md sections 6 and 9). Sourced, never
# run: `. lib.sh <name>`, where <name> is the caller's message prefix (dast, scan-live). The caller
# defines scan_container <docker run args...>, which runs one throwaway tool container on the
# network that reaches the target (dast: the app container's namespace; scan-live: the host's, or
# NETWORK). The scanning functions read the tool images they name (ZAP_IMAGE, NUCLEI_IMAGE,
# CURL_IMAGE, NUCLEI_TEMPLATES, NUCLEI_TEMPLATES_SHA256) from the environment.

name=${1:?lib.sh needs a name}
red=0
# red <message>: one check failed; the caller exits 1 at the end, after every scan ran.
red() {
  echo "$name: $*" >&2
  red=$((red + 1))
}
# die <message>: stop at once, red.
die() {
  echo "$name: $*" >&2
  exit 1
}

# require_env <variable>...: exit 2 when one is unset or empty.
require_env() {
  local var
  for var in "$@"; do
    if [ -z "${!var:-}" ]; then
      echo "$name: $var is not set (run through make $name)" >&2
      exit 2
    fi
  done
}

# check_nuclei_pins: exit 2 unless NUCLEI_TEMPLATES is a release tag and NUCLEI_TEMPLATES_SHA256 a
# sha256.
check_nuclei_pins() {
  if ! [[ "$NUCLEI_TEMPLATES" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "$name: NUCLEI_TEMPLATES must be a release tag like v1.2.3, got '$NUCLEI_TEMPLATES'" >&2
    exit 2
  fi
  if ! [[ "$NUCLEI_TEMPLATES_SHA256" =~ ^[0-9a-f]{64}$ ]]; then
    echo "$name: NUCLEI_TEMPLATES_SHA256 must be 64 lower-case hex digits" >&2
    exit 2
  fi
}

# finish <report dir>: the end of a run. Exits 1 when any check was red, else prints ok.
finish() {
  if [ "$red" -gt 0 ]; then
    echo "$name: $red checks red; reports in $1" >&2
    exit 1
  fi
  echo "$name: ok"
}

# fetch_nuclei_templates <cache dir> <templates dir>: the pinned templates, downloaded once into
# <cache dir> and unpacked to <templates dir> only when the tarball sha256 is the pinned
# NUCLEI_TEMPLATES_SHA256. A failed download or a mismatch is red, never a scan with no or other
# templates.
fetch_nuclei_templates() {
  local templates_cache=$1 templates=$2 tarball got unpack
  [ ! -d "$templates" ] || return 0
  mkdir -p "$templates_cache"
  tarball=nuclei-templates-$NUCLEI_TEMPLATES.tar.gz
  docker run --rm -u "$(id -u):$(id -g)" -v "$(cd "$templates_cache" && pwd):/cache" "$CURL_IMAGE" \
    -fsSL -o "/cache/$tarball" \
    "https://github.com/projectdiscovery/nuclei-templates/archive/refs/tags/$NUCLEI_TEMPLATES.tar.gz" ||
    die "cannot download the nuclei templates $NUCLEI_TEMPLATES"
  got=$(sha256sum "$templates_cache/$tarball" | cut -d' ' -f1)
  if [ "$got" != "$NUCLEI_TEMPLATES_SHA256" ]; then
    rm -f "$templates_cache/$tarball"
    die "the nuclei templates tarball $NUCLEI_TEMPLATES does not match NUCLEI_TEMPLATES_SHA256 (got $got)"
  fi
  unpack=$(mktemp -d "$templates_cache/unpack.XXXXXX")
  tar -xzf "$templates_cache/$tarball" -C "$unpack" --strip-components=1 ||
    die "cannot unpack the nuclei templates $NUCLEI_TEMPLATES"
  mv "$unpack" "$templates"
  rm -f "$templates_cache/$tarball"
}

# valid_zap_id <id>: a rule number, or <rule number>@<url regex> with no whitespace.
valid_zap_id() {
  [[ "$1" =~ ^[0-9]+(@[^[:space:]]+)?$ ]]
}

# zap_rules_file <rules.tsv> <dest> <ids> <comment>: copies the rules file to <dest> with the
# allow-listed zap ids (one per line, each valid_zap_id). A plain id (<rule number>) sets that rule
# to IGNORE with <comment> (a rule the file does not list is added). A scoped id
# (<rule number>@<url regex>) leaves the rule as the file sets it and adds a line
# `<rule number>\tOUTOFSCOPE\t<url regex>`: ZAP's packaged scans then drop only that rule's alerts
# whose URL the regex matches from its start (re.match). The ids reach awk through the
# environment: -v would turn a regex's \. into a plain dot.
zap_rules_file() {
  ids=$3 comment=$4 awk -F '\t' -v OFS='\t' '
    BEGIN {
      comment = ENVIRON["comment"]
      n = split(ENVIRON["ids"], list, "\n")
      for (i = 1; i <= n; i++) {
        if (list[i] == "") continue
        at = index(list[i], "@")
        if (at == 0) allow[list[i]] = 1
        else scoped[++s] = substr(list[i], 1, at - 1) OFS "OUTOFSCOPE" OFS substr(list[i], at + 1)
      }
    }
    /^#/ || NF == 0 { print; next }
    $1 in allow { print $1, "IGNORE", comment; seen[$1] = 1; next }
    { print }
    END {
      for (id in allow) if (!(id in seen)) print id, "IGNORE", comment
      for (i = 1; i <= s; i++) print scoped[i]
    }
  ' "$1" >"$2"
}

# run_zap <report dir> <report name> <zap script and args...>: one ZAP run with <report dir>/zap
# as its work folder (rules.tsv, the rules file ZAP gets, is in it); 0 and 2 (WARN rules only) pass. Then, from its JSON report: ZAP's rule 100000 is
# one id for 4xx (Informational) and 5xx (Low) answers, so the rules file cannot fail only on
# 5xx, and a 5xx is red here (an app that answers 500 to a bad request crashed past its
# boundary). An alert from a rule the rules file does not list is red too: ZAP would only WARN it,
# so a ZAP bump could add a High rule that passes silently.
run_zap() {
  local report=$1 zname=$2 rc=0 errors ids unknown known_rules report_abs
  shift 2
  report_abs=$(cd "$report" && pwd)
  known_rules=$(awk -F '\t' '!/^#/ && NF { print $1 }' "$report/zap/rules.tsv" | sort -u)
  echo "== $name: zap $zname"
  scan_container -v "$report_abs/zap:/zap/wrk:rw" "$ZAP_IMAGE" "$@" -c rules.tsv \
    -J "$zname.json" -r "$zname.html" || rc=$?
  case "$rc" in
    0 | 2) ;;
    1)
      red "zap $zname: blocking findings (see $report/zap/$zname.html)"
      return 0
      ;;
    *)
      red "zap could not scan ($zname, exit $rc)"
      return 0
      ;;
  esac
  if ! errors=$(jq '[.site[]?.alerts[]? | select(.pluginid == "100000" and .riskcode != "0") | .count | tonumber] | add // 0' \
    "$report/zap/$zname.json"); then
    red "zap $zname wrote no readable JSON report"
    return 0
  fi
  ids=$(jq -r '[.site[]?.alerts[]?.pluginid] | unique[]' "$report/zap/$zname.json" | sort -u)
  unknown=$(comm -23 <(printf '%s\n' "$ids" | sed '/^$/d') <(printf '%s\n' "$known_rules") | paste -sd' ' -)
  if [ -n "$unknown" ]; then
    red "zap $zname: alerts from rules not in security/zap-rules.tsv: $unknown (add each with its level)"
  elif [ "$errors" -gt 0 ]; then
    red "zap $zname: the app answered $errors requests with a 5xx (see $report/zap/$zname.html)"
  else
    echo "$name: zap $zname: no blocking findings"
  fi
}

# run_nuclei <report dir> <templates dir> <excluded ids> <url>: Nuclei with the templates in
# <templates dir>, writing <report dir>/nuclei, without the out-of-band
# (interactsh) templates (-ni) and without the intrusive, dos and fuzz tags, excluding the comma
# separated template ids in <excluded ids> (may be empty). It exits 0 with findings, so its JSONL
# lines are counted, and a run that loaded no templates is red.
run_nuclei() {
  local report=$1 templates=$2 nuclei_ids=$3 url=$4 rc=0 loaded broken findings=0 excludes=() report_abs
  report_abs=$(cd "$report" && pwd)
  echo "== $name: nuclei"
  [ -z "$nuclei_ids" ] || excludes=(-exclude-id "$nuclei_ids")
  scan_container -u "$(id -u):$(id -g)" -e HOME=/tmp -v "$(cd "$templates" && pwd):/templates:ro" -v "$report_abs/nuclei:/out" \
    "$NUCLEI_IMAGE" -u "$url" -t /templates -duc -ni -etags intrusive,dos,fuzz -s low,medium,high,critical \
    "${excludes[@]}" -jle /out/nuclei.jsonl -se /out/nuclei.sarif -nc 2>&1 | tee "$report/nuclei/nuclei.log" || rc=$?
  loaded=$(sed -n 's/.*Templates loaded for current scan: \([0-9]*\).*/\1/p' "$report/nuclei/nuclei.log" | head -n 1)
  broken=$(sed -n 's/.*Found \([0-9]*\) templates with runtime error.*/\1/p' "$report/nuclei/nuclei.log" | head -n 1)
  [ -z "$broken" ] || echo "$name: nuclei: $broken templates did not run (runtime error; nuclei -validate shows why)"
  [ ! -s "$report/nuclei/nuclei.jsonl" ] || findings=$(grep -c . "$report/nuclei/nuclei.jsonl")
  if [ "$rc" -ne 0 ]; then
    red "nuclei could not scan (exit $rc)"
  elif [ "${loaded:-0}" -eq 0 ]; then
    red "nuclei loaded no templates"
  elif [ "$findings" -gt 0 ]; then
    red "nuclei: $findings findings (see $report/nuclei/nuclei.jsonl)"
  else
    echo "$name: nuclei: no findings ($loaded templates)"
  fi
}
