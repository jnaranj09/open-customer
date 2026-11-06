# Shared by upgrade.sh and restore.sh (sourced, never run; design doc item 86): the checks on
# deploy/.env and the one edit both scripts make to it. The caller has done
# `cd "$(dirname "$0")/.."` and defines die <message> (prints one line and exits non-zero).
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
