# Shared by upgrade.sh and restore.sh (sourced, never run; design doc item 86): the checks on
# deploy/.env and the one edit both scripts make to it, and the signature check upgrade.sh runs
# before it pulls. The caller has done `cd "$(dirname "$0")/.."` and defines die <message> (prints
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
# made by hand, both copies in one commit. A fork that publishes its own images changes the two
# SIGNER_ lines to its own repository (deploy/README.md section 9).
COSIGN_IMAGE=ghcr.io/sigstore/cosign/cosign:v3.1.4@sha256:0f06154836d2de34f887a5c273506d52ca43edaa1d9e53e6510fa78a48f69c2d
SIGNER_IDENTITY_REGEXP='^https://github\.com/jnaranj09/open-customer/\.github/workflows/release\.yml@refs/tags/v'
SIGNER_ISSUER=https://token.actions.githubusercontent.com

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

# verify_image <image:tag>: exit 0 only when cosign finds a signature on <image:tag> made by this
# repository's release workflow on a v* tag. Quiet on success; on failure cosign's output and one
# line naming the image go to stderr. Needs network to the registry and to Sigstore.
verify_image() {
  local out
  if ! out=$(docker run --rm "$COSIGN_IMAGE" verify "$1" \
    --certificate-identity-regexp "$SIGNER_IDENTITY_REGEXP" \
    --certificate-oidc-issuer "$SIGNER_ISSUER" 2>&1); then
    printf '%s\n' "$out" >&2
    echo "the signature check of $1 failed: it is not signed by the release workflow of" \
      "$SIGNER_IDENTITY_REGEXP. Check the network to the registry and to Sigstore, that the" \
      "version exists, and, for a fork, the SIGNER_ lines in scripts/lib.sh" >&2
    return 1
  fi
}
