# Shared by the operator scripts (sourced, never run; design doc item 86): the checks on
# deploy/.env and the one edit upgrade.sh and restore.sh make to it, and the signature check every
# script runs before it brings a release image onto this PC (pull_verified, ensure_release_images). The caller has done `cd "$(dirname "$0")/.."` and defines die <message> (prints
# one line and exits non-zero).
# shellcheck shell=bash

# The temp copy of .env (it holds every secret) is removed on any exit; bash also runs the EXIT
# trap when INT or TERM ends the script (test: a TERM during the .env edit leaves no temp copy). A
# SIGKILL can still leave one, so .gitignore covers deploy/.env.*.
env_tmp=
trap 'if [ -n "$env_tmp" ]; then rm -f "$env_tmp"; fi' EXIT

# check_env_file: .env exists and has exactly one APP_VERSION= line.
check_env_file() {
  [ -f .env ] || die "no .env next to compose.yaml"
  [ "$(grep -c '^APP_VERSION=' .env)" -eq 1 ] || die ".env must have exactly one APP_VERSION= line"
}

# env_app_version: the APP_VERSION value in .env.
env_app_version() {
  sed -n 's/^APP_VERSION=//p' .env
}

# set_app_version <version>: rewrites the APP_VERSION line of .env in place (same file, so its
# owner and mode stay); every other line is kept as is. Returns non-zero when .env was not
# changed. Each step is checked by hand: a caller's `||` turns `set -e` off in here, and an empty
# temp file must never replace .env.
set_app_version() {
  env_tmp=$(umask 077 && mktemp .env.XXXXXX) || return 1
  if ! sed "s/^APP_VERSION=.*/APP_VERSION=$1/" .env >"$env_tmp" ||
    ! grep -qx "APP_VERSION=$1" "$env_tmp" ||
    ! cat "$env_tmp" >.env; then
    rm -f "$env_tmp"
    env_tmp=
    return 1
  fi
  rm -f "$env_tmp"
  env_tmp=
}

# Signature check (docs/designs/devsecops.md section 4, decision D11). cosign runs from its pinned
# image, so a deployer installs nothing. Its version is also `cosign-release` in
# .github/workflows/release.yml (make check-pins); Dependabot cannot read this file, so a bump is
# made by hand, both copies in one commit. A fork that publishes its own images changes
# SIGNER_IDENTITY_REGEXP and SIGNER_REPOSITORY to its own repository (deploy/README.md section 9).
COSIGN_IMAGE=ghcr.io/sigstore/cosign/cosign:v3.1.4@sha256:0f06154836d2de34f887a5c273506d52ca43edaa1d9e53e6510fa78a48f69c2d
SIGNER_IDENTITY_REGEXP='^https://github\.com/jnaranj09/open-customer/\.github/workflows/release\.yml@refs/tags/v'
SIGNER_ISSUER=https://token.actions.githubusercontent.com
# The certificate's GitHub claims: the repository the workflow ran in, and the event (a tag push).
SIGNER_REPOSITORY=jnaranj09/open-customer
SIGNER_TRIGGER=push

# release_images <version>: the app and backup images compose.yaml runs at <version>, one per line
# (the `image: <repository>:${APP_VERSION...}` lines). Returns non-zero unless there are exactly
# two.
release_images() {
  local repos repo
  repos=$(sed -n 's/^ *image: *\([^ ]*\):[$][{]APP_VERSION.*/\1/p' compose.yaml | sort -u)
  [ -n "$repos" ] && [ "$(wc -l <<<"$repos")" -eq 2 ] || return 1
  for repo in $repos; do
    echo "$repo:$1"
  done
}

# verify_image <repository:tag>: exit 0 only when cosign finds a signature on <repository:tag> made
# by this repository's release workflow on a v* tag push, and prints <repository>@<digest>: the one
# digest cosign verified (`critical.image.docker-manifest-digest` in its JSON on stdout). The caller
# checks the pulled image against that digest and never resolves the tag again. Nothing else is
# printed on success; on failure cosign's stderr and one line naming the image go to stderr. No
# digest, or more than one distinct digest, in cosign's output is a failure. Needs network to the
# registry and to Sigstore.
verify_image() {
  local repo=${1%:*} out err digests
  err=$(mktemp) || return 1
  if ! out=$(docker run --rm "$COSIGN_IMAGE" verify "$1" \
    --certificate-identity-regexp "$SIGNER_IDENTITY_REGEXP" \
    --certificate-oidc-issuer "$SIGNER_ISSUER" \
    --certificate-github-workflow-repository "$SIGNER_REPOSITORY" \
    --certificate-github-workflow-trigger "$SIGNER_TRIGGER" 2>"$err"); then
    cat "$err" >&2
    rm -f "$err"
    echo "the signature check of $1 failed: it is not signed by the release workflow of" \
      "$SIGNER_REPOSITORY. Check the network to the registry and to Sigstore, that the version" \
      "exists, and, for a fork, the SIGNER_ lines in scripts/lib.sh" >&2
    return 1
  fi
  rm -f "$err"
  digests=$(grep -oE '"docker-manifest-digest": *"sha256:[0-9a-f]{64}"' <<<"$out" |
    grep -oE 'sha256:[0-9a-f]{64}' | sort -u || true)
  if [ -z "$digests" ]; then
    echo "the signature check of $1 gave no verified digest; nothing was pulled" >&2
    return 1
  fi
  if [ "$(wc -l <<<"$digests")" -ne 1 ]; then
    echo "the signature check of $1 gave more than one digest; nothing was pulled" >&2
    return 1
  fi
  echo "$repo@$digests"
}

# pulled_is_verified <repository:tag> <repository@digest>: exit 0 when the local <repository:tag>
# (just pulled) records <repository@digest> among its RepoDigests, so what `up -d` runs is the
# image whose signature verify_image checked.
pulled_is_verified() {
  docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$1" | grep -qxF "$2"
}

# pull_verified <version>: the one way a script brings release images onto this PC. verify_image
# for the app and the backup image at <version> (a failure pulls nothing), then
# `APP_VERSION=<version> docker compose pull`, then pulled_is_verified for each, so the local
# images are the digests whose signatures were checked; the tags are never resolved again.
# compose.yaml sets no pull_policy (guard TestComposeHardening), so a later `up -d` or `run` uses
# these local images. Returns non-zero with the reason on stderr; the caller says what is left as
# it was.
pull_verified() {
  local images image ref pair verified=()
  images=$(release_images "$1") || {
    echo "compose.yaml must name the app image and its -backup image with the tag \${APP_VERSION}" >&2
    return 1
  }
  # The two refs hold no spaces (release_images builds them from compose.yaml's image: lines).
  for image in $images; do
    ref=$(verify_image "$image") || return 1
    verified+=("$image $ref")
  done
  APP_VERSION=$1 docker compose pull || {
    echo "docker compose pull of $1 failed" >&2
    return 1
  }
  for pair in "${verified[@]}"; do
    image=${pair% *}
    ref=${pair#* }
    pulled_is_verified "$image" "$ref" || {
      echo "the pulled $image is not the digest whose signature was verified ($ref): the" \
        "registry tag changed after the signature check. Run the command again; if it repeats," \
        "look into the registry" >&2
      return 1
    }
  done
}

# ensure_release_images <version>: exit 0 when the app and backup images at <version> are on this
# PC (a local image is trusted: it came through pull_verified or a local build), touching no
# network; otherwise pull_verified <version>. Every script runs it before a compose command that
# would otherwise pull a missing release image unchecked.
ensure_release_images() {
  local images image
  images=$(release_images "$1") || {
    echo "compose.yaml must name the app image and its -backup image with the tag \${APP_VERSION}" >&2
    return 1
  }
  for image in $images; do
    docker image inspect "$image" >/dev/null 2>&1 || {
      pull_verified "$1"
      return
    }
  done
}
