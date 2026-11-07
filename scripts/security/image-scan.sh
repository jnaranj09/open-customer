#!/usr/bin/env bash
# make image-scan (docs/designs/devsecops.md section 4 and "Blocking policy", decision D5): Trivy
# on the four images a deployer runs: the app and backup images, PG_IMAGE and the cloudflared
# image of deploy/compose.yaml. A high or critical finding with a fixed version blocks; in MODE
# push only the app and backup images block and the two third-party images report only; in MODE
# release all four block. Without PULL the app and backup images are the local build of make image
# (open-customer:$VERSION, open-customer-backup:$VERSION); with PULL=1 they are the released images,
# the ghcr names deploy/compose.yaml gives with the tag $VERSION (the weekly rescan). Every other
# image is pulled; an image that cannot be pulled is red. SARIF_DIR=<dir> (relative to the repo
# root) also writes <dir>/<app|backup|postgres|cloudflared>.sarif.
#
# Each image is saved to a tar and scanned with `trivy image --input`, so the scanner never gets the
# Docker socket. The live trivy ids of security/allow.json (allow.sh) go to a temp
# .trivyignore.yaml (trivy-ignore.sh): <CVE id> ignores the CVE in every image, <CVE id>@<path in
# the image> (usr/local/bin/gosu) only in the result whose Target is that path. Trivy exits 1 on its own errors (no database, unreadable image) whatever
# --exit-code says, so findings use exit code 5 and any exit other than 0 or 5 is red, for every
# image and mode: a scan that did not run never reads as clean. Every image is scanned before the
# script exits, so one run reports them all. Exit 2 on bad arguments, 1 when any image is red.
set -euo pipefail

usage="usage: image-scan.sh push|release (env TRIVY_IMAGE, PG_IMAGE, VERSION; optional PULL=1, SARIF_DIR, TRIVY_CACHE_DIR)"
if [ "$#" -ne 1 ]; then
  echo "$usage" >&2
  exit 2
fi
mode=$1
case "$mode" in
  push | release) ;;
  *)
    echo "image-scan: unknown MODE '$mode' (push or release)" >&2
    exit 2
    ;;
esac
case "${PULL:-}" in
  "" | 0) pull=0 ;;
  1) pull=1 ;;
  *)
    echo "image-scan: PULL must be 1 or empty, got '$PULL'" >&2
    exit 2
    ;;
esac
for var in TRIVY_IMAGE PG_IMAGE VERSION; do
  if [ -z "${!var:-}" ]; then
    echo "image-scan: $var is not set (run through make image-scan)" >&2
    exit 2
  fi
done

cd "$(dirname "$0")/../.."

# Trivy's exit code for findings (--exit-code); see the header for why it is not 1.
findings_exit=5
compose=deploy/compose.yaml

cloudflared=$(grep -oE 'cloudflare/cloudflared:[^ "]+' "$compose" | sort -u)
if [ -z "$cloudflared" ] || [ "$(wc -l <<<"$cloudflared")" -ne 1 ]; then
  echo "image-scan: $compose must name exactly one cloudflare/cloudflared:<tag> image" >&2
  exit 1
fi
if [ "$pull" -eq 1 ]; then
  # The app and backup services: `image: <repository>:${APP_VERSION...}`; sorted, the app comes first.
  mapfile -t repos < <(sed -n 's/^ *image: *\([^ ]*\):[$][{]APP_VERSION.*/\1/p' "$compose" | sort -u)
  if [ "${#repos[@]}" -ne 2 ] || [ "${repos[1]}" != "${repos[0]}-backup" ]; then
    echo "image-scan: $compose must name the app image and its -backup image with the tag \${APP_VERSION}" >&2
    exit 1
  fi
  app=${repos[0]}:$VERSION
  backup=${repos[1]}:$VERSION
else
  app=open-customer:$VERSION
  backup=open-customer-backup:$VERSION
fi

cache=${TRIVY_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/open-customer/trivy}
mkdir -p "$cache"
sarif_mount=()
if [ -n "${SARIF_DIR:-}" ]; then
  mkdir -p "$SARIF_DIR"
  sarif_mount=(-v "$(cd "$SARIF_DIR" && pwd):/sarif")
fi

container=oc-image-scan-trivy
work=$(mktemp -d)
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

# The ignore file: the live trivy ids of the allow-list, plain or scoped to one path
# (trivy-ignore.sh).
. scripts/security/trivy-ignore.sh image-scan
write_trivy_ignore "$work/.trivyignore.yaml"

red=0
red() {
  echo "image-scan: $*" >&2
  red=$((red + 1))
}

# trivy <name> [extra flags...]: one Trivy run on $work/<name>.tar.
trivy() {
  local name=$1
  shift
  docker run --rm --name "$container" -u "$(id -u):$(id -g)" \
    -v "$cache:/cache" -v "$work:/scan:ro" "${sarif_mount[@]}" \
    "$TRIVY_IMAGE" image --cache-dir /cache --ignorefile /scan/.trivyignore.yaml \
    --severity HIGH,CRITICAL --ignore-unfixed --no-progress --skip-version-check --scanners vuln \
    --input "/scan/$name.tar" "$@"
}

# scan_one <name> <image> <local|pull> <block|report>
scan_one() {
  local name=$1 image=$2 source=$3 policy=$4 rc=0
  if [ "$source" = local ]; then
    if ! docker image inspect "$image" >/dev/null 2>&1; then
      red "$image is not built (run make image)"
      return 0
    fi
  elif ! docker pull -q "$image" >/dev/null; then
    red "cannot pull $image"
    return 0
  fi
  if ! docker save -o "$work/$name.tar" "$image"; then
    red "cannot save $image for the scan"
    return 0
  fi
  echo "== image-scan: $image"
  trivy "$name" --exit-code "$findings_exit" || rc=$?
  case "$rc" in
    0) echo "image-scan: $image: no blocking findings" ;;
    "$findings_exit")
      if [ "$policy" = block ]; then
        red "$image: high or critical findings with a fix (MODE $mode)"
      else
        echo "image-scan: $image: findings (report only in push mode)"
      fi
      ;;
    *) red "trivy could not scan $image (exit $rc)" ;;
  esac
  if [ "${#sarif_mount[@]}" -gt 0 ]; then
    rc=0
    trivy "$name" --format sarif --output "/sarif/$name.sarif" >/dev/null || rc=$?
    [ "$rc" -eq 0 ] || red "trivy could not write the SARIF file for $image (exit $rc)"
  fi
  rm -f "$work/$name.tar"
}

own_source=local
[ "$pull" -eq 1 ] && own_source=pull
third_policy=report
[ "$mode" = release ] && third_policy=block

scan_one app "$app" "$own_source" block
scan_one backup "$backup" "$own_source" block
scan_one postgres "$PG_IMAGE" pull "$third_policy"
scan_one cloudflared "$cloudflared" pull "$third_policy"

if [ "$red" -gt 0 ]; then
  echo "image-scan: $red of 4 images red (MODE $mode)" >&2
  exit 1
fi
echo "image-scan: ok (MODE $mode)"
