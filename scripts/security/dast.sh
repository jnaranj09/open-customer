#!/usr/bin/env bash
# make dast (docs/designs/devsecops.md section 6 and "Blocking policy"): the running-app scan.
# Starts the app image (IMAGE, built by make image) and a throwaway Postgres (PG_IMAGE) on the
# internal network oc-dast-net, with no published port. Every scanner and request runs inside the
# app container's network namespace (--network container:oc-dast-app) against
# http://localhost:8080, which is PUBLIC_URL: the app accepts plain http only on localhost
# (internal/platform/config), and every non-GET must send exactly that Origin.
#
# Before any scan it completes setup with a made-up owner (setup-link, POST /api/v1/setup), logs in
# (POST /api/v1/auth/login) and checks GET /api/v1/me answers 200 with that cookie; if not, it
# stops red, so the authenticated scan can never silently scan only the public routes. Then:
#   - ZAP API scan from api/openapi.yaml (passive and active), once with only the Origin header
#     and once with the session cookie too. The signed-in pass leaves POST /auth/logout out so the
#     session lives; GET /api/v1/me is checked again after it and a dead session is red.
#   - ZAP baseline (passive, spider) on /.
#   - Nuclei with the pinned templates (NUCLEI_TEMPLATES, downloaded once into NUCLEI_CACHE_DIR).
#
# security/zap-rules.tsv sets each ZAP rule to FAIL or WARN; the live zap ids of
# security/allow.json (allow.sh) become IGNORE in the copy ZAP gets, and the live nuclei ids go to
# -exclude-id. ZAP exits 0 clean, 1 on a FAIL rule, 2 on WARN rules only, 3 on its own error: 0
# and 2 pass, anything else is red. Nuclei exits 0 with findings, so its JSONL lines are counted,
# and a run that loaded no templates is red. Every scan runs before the script exits, so one run
# reports them all. Reports go to REPORT_DIR (default dast-report, git-ignored).
#
# Exit 2 on bad arguments, 1 when the setup fails or any scan is red.
set -euo pipefail

usage="usage: dast.sh (env IMAGE, PG_IMAGE, ZAP_IMAGE, NUCLEI_IMAGE, NUCLEI_TEMPLATES, CURL_IMAGE; optional REPORT_DIR, NUCLEI_CACHE_DIR)"
if [ "$#" -ne 0 ]; then
  echo "$usage" >&2
  exit 2
fi
for var in IMAGE PG_IMAGE ZAP_IMAGE NUCLEI_IMAGE NUCLEI_TEMPLATES CURL_IMAGE; do
  if [ -z "${!var:-}" ]; then
    echo "dast: $var is not set (run through make dast)" >&2
    exit 2
  fi
done
if ! [[ "$NUCLEI_TEMPLATES" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "dast: NUCLEI_TEMPLATES must be a release tag like v1.2.3, got '$NUCLEI_TEMPLATES'" >&2
  exit 2
fi

cd "$(dirname "$0")/../.."

net=oc-dast-net
pg=oc-dast-pg
app=oc-dast-app
scanner=oc-dast-scanner
# PUBLIC_URL inside the app container, and the one Origin it accepts.
origin=http://localhost:8080
# The throwaway database's password and the made-up owner; they live only for this run.
# The owner's password is random so no password-shaped literal sits in the repo.
pg_password=dast-only
owner=dast-duena
owner_password=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')

report=${REPORT_DIR:-dast-report}
templates_cache=${NUCLEI_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/open-customer/nuclei}
templates=$templates_cache/nuclei-templates-$NUCLEI_TEMPLATES

cleanup() {
  docker rm -f -v "$scanner" "$app" "$pg" >/dev/null 2>&1 || true
  docker network rm "$net" >/dev/null 2>&1 || true
}
trap cleanup EXIT
# A run killed before its trap ran leaves these behind.
cleanup

red=0
red() {
  echo "dast: $*" >&2
  red=$((red + 1))
}
die() {
  echo "dast: $*" >&2
  exit 1
}

# in_app <docker run args...>: a one-off container in the app's network namespace.
in_app() {
  docker run --rm --name "$scanner" --network "container:$app" "$@"
}
# app_curl <curl args...>
app_curl() {
  in_app "$CURL_IMAGE" -sS "$@"
}

docker image inspect "$IMAGE" >/dev/null 2>&1 || die "$IMAGE is not built (run make image)"

# The pinned templates, downloaded once. A failed download is red, never a scan with no templates.
if [ ! -d "$templates" ]; then
  mkdir -p "$templates_cache"
  tarball=nuclei-templates-$NUCLEI_TEMPLATES.tar.gz
  docker run --rm -u "$(id -u):$(id -g)" -v "$(cd "$templates_cache" && pwd):/cache" "$CURL_IMAGE" \
    -fsSL -o "/cache/$tarball" \
    "https://github.com/projectdiscovery/nuclei-templates/archive/refs/tags/$NUCLEI_TEMPLATES.tar.gz" ||
    die "cannot download the nuclei templates $NUCLEI_TEMPLATES"
  unpack=$(mktemp -d "$templates_cache/unpack.XXXXXX")
  tar -xzf "$templates_cache/$tarball" -C "$unpack" --strip-components=1 ||
    die "cannot unpack the nuclei templates $NUCLEI_TEMPLATES"
  mv "$unpack" "$templates"
  rm -f "$templates_cache/$tarball"
fi

# The report folder: ZAP runs as its own user, so its work folder is writable by all.
rm -rf "$report"
mkdir -p "$report/zap" "$report/nuclei"
chmod a+rwx "$report/zap" "$report/nuclei"
report_abs=$(cd "$report" && pwd)
# ZAP's OpenAPI import needs an absolute server URL; the spec's is relative (/api/v1).
sed 's|^  - url: /api/v1$|  - url: http://localhost:8080/api/v1|' api/openapi.yaml >"$report/zap/openapi.yaml"
grep -q '^  - url: http://localhost:8080/api/v1$' "$report/zap/openapi.yaml" ||
  die "api/openapi.yaml has no servers url /api/v1 to point at the app"
# The signed-in pass gets the spec without POST /auth/logout: ZAP's import and active scan send
# every operation, and a logout would end the session for the rest of the scan. The signed-out
# pass still scans it.
awk '/^  \/auth\/logout:$/ { skip = 1; next } skip && /^  [^ ]/ { skip = 0 } !skip' \
  "$report/zap/openapi.yaml" >"$report/zap/openapi-session.yaml"
if grep -q '/auth/logout' "$report/zap/openapi-session.yaml" || ! grep -q '^  /auth/password:$' "$report/zap/openapi-session.yaml"; then
  die "cannot drop /auth/logout from the spec for the signed-in scan"
fi

# The rules file ZAP gets: security/zap-rules.tsv with each live zap id of the allow-list set to
# IGNORE (an id the file does not list is added). An id that is not a number is refused.
allowed=$(scripts/security/allow.sh zap)
while read -r id; do
  [ -z "$id" ] || [[ "$id" =~ ^[0-9]+$ ]] || die "security/allow.json has a zap id that is not a rule number"
done <<<"$allowed"
awk -F '\t' -v OFS='\t' -v ids="$allowed" '
  BEGIN { n = split(ids, list, "\n"); for (i = 1; i <= n; i++) if (list[i] != "") allow[list[i]] = 1 }
  /^#/ || NF == 0 { print; next }
  $1 in allow { print $1, "IGNORE", "allow-listed in security/allow.json"; seen[$1] = 1; next }
  { print }
  END { for (id in allow) if (!(id in seen)) print id, "IGNORE", "allow-listed in security/allow.json" }
' security/zap-rules.tsv >"$report/zap/rules.tsv"

docker network create --internal "$net" >/dev/null
docker run -d --name "$pg" --network "$net" \
  -e POSTGRES_USER=open_customer -e POSTGRES_DB=open_customer -e POSTGRES_PASSWORD="$pg_password" \
  "$PG_IMAGE" >/dev/null
# Over TCP: on a first start the image's init server answers on the socket only, then restarts.
tries=0
until docker exec "$pg" pg_isready -q -h 127.0.0.1 -U open_customer -d open_customer; do
  tries=$((tries + 1))
  [ "$tries" -lt 60 ] || die "$pg did not become ready in 60 s"
  sleep 1
done

# A fresh database has nothing to back up, so the migration guard is off.
docker run -d --name "$app" --network "$net" \
  -e PUBLIC_URL="$origin" -e DB_HOST="$pg" -e DB_PASSWORD="$pg_password" \
  -e VAPID_SUBJECT=mailto:dast@localhost -e ALLOW_MIGRATE_WITHOUT_BACKUP=true \
  "$IMAGE" >/dev/null
tries=0
until app_curl -f -o /dev/null "$origin/readyz" 2>/dev/null; do
  tries=$((tries + 1))
  if [ "$tries" -ge 60 ]; then
    docker logs "$app" 2>&1 | tail -20 >&2
    die "$app did not become ready in 60 s"
  fi
  sleep 1
done

# Setup and login with the made-up owner; the session check must pass before any scan.
link=$(docker exec "$app" open-customer setup-link) || die "setup-link failed"
token=${link#*#}
setup_body=$(printf '{"token":"%s","username":"%s","password":"%s","settings":{"name":"Estudio DAST","timezone":"America/Costa_Rica","locale":"es","currency":"USD","country_code":"CR","accent_color":"#aa3366"}}' \
  "$token" "$owner" "$owner_password")
status=$(app_curl -o /dev/null -w '%{http_code}' -X POST -H "Origin: $origin" -H 'Content-Type: application/json' \
  --data "$setup_body" "$origin/api/v1/setup") || die "POST /api/v1/setup did not answer"
[ "$status" = 201 ] || die "POST /api/v1/setup answered $status, want 201"
login=$(app_curl -o /dev/null -w '%{http_code}\n%header{set-cookie}' -X POST -H "Origin: $origin" \
  -H 'Content-Type: application/json' --data "$(printf '{"username":"%s","password":"%s"}' "$owner" "$owner_password")" \
  "$origin/api/v1/auth/login") || die "POST /api/v1/auth/login did not answer"
[ "$(head -n 1 <<<"$login")" = 200 ] || die "POST /api/v1/auth/login answered $(head -n 1 <<<"$login"), want 200"
cookie=$(sed -n '2s/^\(oc_session=[^;]*\).*/\1/p' <<<"$login")
[ -n "$cookie" ] || die "POST /api/v1/auth/login set no oc_session cookie"

# session_status: the status GET /api/v1/me answers with the scan's headers (none when curl fails).
session_status() {
  app_curl -o /dev/null -w '%{http_code}' -H "Origin: $origin" -H "Cookie: $cookie" "$origin/api/v1/me" || echo none
}
status=$(session_status)
[ "$status" = 200 ] || die "GET /api/v1/me answered $status with the session cookie, want 200; nothing was scanned"
echo "dast: signed in as $owner; scanning $origin"

# replacer <index> <header> <value>: ZAP options that set one request header on every request.
replacer() {
  local n=$1 name=$2 value=$3
  printf -- '-config replacer.full_list(%s).description=%s -config replacer.full_list(%s).enabled=true -config replacer.full_list(%s).matchtype=REQ_HEADER -config replacer.full_list(%s).matchstr=%s -config replacer.full_list(%s).regex=false -config replacer.full_list(%s).replacement=%s' \
    "$n" "$name" "$n" "$n" "$n" "$name" "$n" "$n" "$value"
}
origin_opts=$(replacer 0 Origin "$origin")
cookie_opts="$origin_opts $(replacer 1 Cookie "$cookie")"

# zap <name> <script> <script args...>: one ZAP run; 0 and 2 (WARN rules only) pass. ZAP's rule
# 100000 is one id for 4xx (Informational) and 5xx (Low) answers, so the rules file cannot fail
# only on 5xx: a 5xx in the JSON report is red here (an app that answers 500 to a bad request
# crashed past its boundary).
zap() {
  local name=$1 rc=0 errors
  shift
  echo "== dast: zap $name"
  in_app -v "$report_abs/zap:/zap/wrk:rw" "$ZAP_IMAGE" "$@" -c rules.tsv \
    -J "$name.json" -r "$name.html" || rc=$?
  case "$rc" in
    0 | 2) ;;
    1)
      red "zap $name: blocking findings (see $report/zap/$name.html)"
      return 0
      ;;
    *)
      red "zap could not scan ($name, exit $rc)"
      return 0
      ;;
  esac
  if ! errors=$(jq '[.site[]?.alerts[]? | select(.pluginid == "100000" and .riskcode != "0") | .count | tonumber] | add // 0' \
    "$report/zap/$name.json"); then
    red "zap $name wrote no readable JSON report"
  elif [ "$errors" -gt 0 ]; then
    red "zap $name: the app answered $errors requests with a 5xx (see $report/zap/$name.html)"
  else
    echo "dast: zap $name: no blocking findings"
  fi
}

zap api-public zap-api-scan.py -t /zap/wrk/openapi.yaml -f openapi -z "$origin_opts"
zap api-session zap-api-scan.py -t /zap/wrk/openapi-session.yaml -f openapi -z "$cookie_opts"
status=$(session_status)
[ "$status" = 200 ] || red "the session ended during the authenticated scan (GET /api/v1/me answered $status): part of it ran signed out"
zap baseline zap-baseline.py -t "$origin/" -z "$origin_opts"

echo "== dast: nuclei"
excludes=()
nuclei_ids=$(scripts/security/allow.sh nuclei | paste -sd, -)
[ -z "$nuclei_ids" ] || excludes=(-exclude-id "$nuclei_ids")
rc=0
in_app -u "$(id -u):$(id -g)" -e HOME=/tmp -v "$(cd "$templates" && pwd):/templates:ro" -v "$report_abs/nuclei:/out" \
  "$NUCLEI_IMAGE" -u "$origin" -t /templates -duc -etags intrusive,dos,fuzz -s low,medium,high,critical \
  "${excludes[@]}" -jle /out/nuclei.jsonl -se /out/nuclei.sarif -nc 2>&1 | tee "$report/nuclei/nuclei.log" || rc=$?
loaded=$(sed -n 's/.*Templates loaded for current scan: \([0-9]*\).*/\1/p' "$report/nuclei/nuclei.log" | head -n 1)
findings=0
[ ! -s "$report/nuclei/nuclei.jsonl" ] || findings=$(grep -c . "$report/nuclei/nuclei.jsonl")
if [ "$rc" -ne 0 ]; then
  red "nuclei could not scan (exit $rc)"
elif [ "${loaded:-0}" -eq 0 ]; then
  red "nuclei loaded no templates"
elif [ "$findings" -gt 0 ]; then
  red "nuclei: $findings findings (see $report/nuclei/nuclei.jsonl)"
else
  echo "dast: nuclei: no findings ($loaded templates)"
fi

if [ "$red" -gt 0 ]; then
  echo "dast: $red checks red; reports in $report" >&2
  exit 1
fi
echo "dast: ok"
