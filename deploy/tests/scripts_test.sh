#!/usr/bin/env bash
# Tests for the operator scripts in deploy/scripts (design doc item 86, "Backups" and "Upgrades and
# the migration guard"). Run by make test-deploy (scripts/test-deploy.sh) on the host. Each test
# runs a script from a temp copy of deploy/ with its own .env, and a fake `docker` first on PATH
# that appends each call to a log and answers from env vars:
#   FAKE_APP_VERSION    what `compose exec app open-customer version` prints
#   FAKE_SNAPSHOT_JSON  what `snapshot-info` prints
#   FAKE_FAIL_ON        a call containing this text exits FAKE_FAIL_CODE (default 1); every other
#                       call exits 0, so `run --rm <COSIGN_IMAGE> verify` passes unless
#                       FAKE_FAIL_ON=verify
#   FAKE_VERIFY_JSON    what a passing `run --rm <COSIGN_IMAGE> verify <ref>` prints on stdout
#                       (default: cosign's JSON with the made-up digest of <ref>'s repository; it
#                       also prints cosign's "Verification for" lines on stderr, as cosign does)
#   FAKE_PULLED_DIGEST  the digest `image inspect --format <RepoDigests> <repo>:<tag>` prints as
#                       <repo>@<digest> (default: the same made-up digest of <repo>)
# The made-up digest of a repository is sha256 of "made-up <repository>".
# Only `compose config is valid` uses the real docker, and it starts nothing. The real deploy/.env
# is never read or copied. All data here is made up.
set -uo pipefail

# lib.sh is shellchecked on its own (make lint runs shellcheck without -x).
# shellcheck source=/dev/null
. "$(dirname "$0")/lib.sh"

DEPLOY=$(cd "$(dirname "$0")/.." && pwd)
ROOT=$(mktemp -d)
trap 'rm -rf "$ROOT"' EXIT
# The scripts must not see a version from the caller's shell, only from .env.
unset APP_VERSION FAKE_APP_VERSION FAKE_SNAPSHOT_JSON FAKE_FAIL_ON FAKE_FAIL_CODE FAKE_VERIFY_JSON \
  FAKE_PULLED_DIGEST

mkdir -p "$ROOT/bin"
cat >"$ROOT/bin/docker" <<'EOF'
#!/usr/bin/env bash
# The fake docker: logs "<args> | APP_VERSION=<the caller's env>" and answers from FAKE_*.
echo "$* | APP_VERSION=${APP_VERSION-unset}" >>"$FAKE_LOG"
if [ -n "${FAKE_FAIL_ON:-}" ]; then
  case "$*" in *"$FAKE_FAIL_ON"*) exit "${FAKE_FAIL_CODE:-1}" ;; esac
fi
# made_up_digest <repository>
made_up_digest() {
  echo "sha256:$(printf 'made-up %s\n' "$1" | sha256sum | cut -d' ' -f1)"
}
case "$*" in
  "compose exec app open-customer version") echo "${FAKE_APP_VERSION:-}" ;;
  "compose --profile restore run --rm restore snapshot-info "*) echo "${FAKE_SNAPSHOT_JSON:-}" ;;
  "run --rm "*" verify "*)
    ref=$5
    echo "Verification for $ref --" >&2
    echo "The following checks were performed on each of these signatures:" >&2
    if [ -n "${FAKE_VERIFY_JSON+set}" ]; then
      printf '%s\n' "$FAKE_VERIFY_JSON"
    else
      printf '[{"critical":{"identity":{"docker-reference":"%s"},"image":{"docker-manifest-digest":"%s"},"type":"https://sigstore.dev/cosign/sign/v1"},"optional":{}}]\n' \
        "$ref" "$(made_up_digest "${ref%:*}")"
    fi
    ;;
  "image inspect --format "*RepoDigests*)
    ref=${*: -1}
    echo "${ref%:*}@${FAKE_PULLED_DIGEST:-$(made_up_digest "${ref%:*}")}"
    ;;
esac
exit 0
EOF
chmod +x "$ROOT/bin/docker"

# A sed that sends TERM to the script running it while it writes the temp copy of .env, for the
# trap test; it is first on PATH only in that test.
mkdir -p "$ROOT/sigbin"
real_sed=$(command -v sed)
cat >"$ROOT/sigbin/sed" <<SED
#!/usr/bin/env bash
case "\$*" in "s/^APP_VERSION=.*/"*) kill -TERM "\$PPID" ;; esac
exec "$real_sed" "\$@"
SED
chmod +x "$ROOT/sigbin/sed"

# A sed that writes nothing and exits 0 for the APP_VERSION edit, for the empty-temp-file test;
# it is first on PATH only in that test.
mkdir -p "$ROOT/emptybin"
cat >"$ROOT/emptybin/sed" <<SED
#!/usr/bin/env bash
case "\$*" in "s/^APP_VERSION=.*/"*) exit 0 ;; esac
exec "$real_sed" "\$@"
SED
chmod +x "$ROOT/emptybin/sed"

env_file='APP_VERSION=1.0.0
PUBLIC_URL=https://example.test
DB_PASSWORD=test-only
# a comment line stays as is
BACKUP_TARGET=/backup-target-for-tests'

# no_temp_left: no .env.XXXXXX copy (it holds every secret) stays next to .env.
no_temp_left() {
  local left
  left=$(find "$W/deploy" -maxdepth 1 -name '.env.*')
  assert_eq "temp files left in deploy/" "" "$left"
}

# setup: a fresh copy of the deploy folder in $W with a test .env, an empty log in $LOG.
setup() {
  W=$(mktemp -d "$ROOT/run.XXXXXX")
  mkdir -p "$W/deploy"
  cp -r "$DEPLOY/scripts" "$DEPLOY/compose.yaml" "$W/deploy/"
  printf '%s\n' "$env_file" >"$W/deploy/.env"
  LOG=$W/docker.log
  : >"$LOG"
  export FAKE_LOG=$LOG
}

# run <script> [args...]: runs deploy/scripts/<script> from another folder, with the fake docker.
run() {
  local script=$1
  shift
  (cd / && PATH="$ROOT/bin:$PATH" "$W/deploy/scripts/$script" "$@")
}

# calls: the logged docker calls without the env suffix.
calls() {
  sed 's/ | APP_VERSION=.*//' "$LOG"
}

# has_call <text>: some docker call contains <text>.
has_call() {
  calls | grep -qF -- "$1"
}

no_call() {
  if has_call "$1"; then
    fail "unexpected docker call containing '$1'"
  fi
}

# line_of <text>: the log line number of the first call containing <text> (empty when none; the
# `|| true` keeps pipefail from ending the test when grep finds nothing).
line_of() {
  calls | grep -nF -- "$1" | head -n1 | cut -d: -f1 || true
}

env_value() {
  sed -n "s/^$1=//p" "$W/deploy/.env"
}

# The pinned cosign image upgrade.sh verifies with, and the two images it must verify at 1.1.0.
cosign_image=$(sed -n 's/^COSIGN_IMAGE=//p' "$DEPLOY/scripts/lib.sh")
verify_app="run --rm $cosign_image verify ghcr.io/jnaranj09/open-customer:1.1.0 "
verify_backup="run --rm $cosign_image verify ghcr.io/jnaranj09/open-customer-backup:1.1.0 "

# A digest no fake image has.
other_digest=sha256:$(printf 'made-up other image\n' | sha256sum | cut -d' ' -f1)

snapshot_json='{"id":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","time":"2026-11-01T03:00:00.123456789Z","app":"0.9.0"}'

test_missing_args() {
  setup
  local calls_made
  run upgrade.sh >/dev/null 2>&1 && fail "upgrade.sh without a version exited 0"
  run restore.sh </dev/null >/dev/null 2>&1 && fail "restore.sh without a snapshot exited 0"
  run backup-now.sh --tag >/dev/null 2>&1 && fail "backup-now.sh --tag without a value exited 0"
  run backup-now.sh --tag other >/dev/null 2>&1 && fail "backup-now.sh --tag other exited 0"
  run upgrade.sh not-a-version >/dev/null 2>&1 && fail "upgrade.sh with a bad version exited 0"
  run restore.sh 'latest; rm' </dev/null >/dev/null 2>&1 && fail "restore.sh with a bad id exited 0"
  calls_made=$(wc -l <"$LOG")
  assert_eq "docker calls for bad args" 0 "$calls_made"
}

test_backup_now_tag_and_code() {
  setup
  local rc=0
  FAKE_FAIL_ON='compose run' FAKE_FAIL_CODE=3 run backup-now.sh --tag pre-upgrade || rc=$?
  assert_eq "exit code" 3 "$rc"
  assert_eq "docker call" "compose run --rm backup once --tag pre-upgrade" "$(calls)"
}

test_upgrade_versions_disagree() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  FAKE_APP_VERSION=1.0.1 run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  no_call pull
  no_call "up -d"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
  grep -q "fix .env first" "$W/err" || fail "the message does not say to fix .env first"
}

test_upgrade_backup_fails() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  FAKE_APP_VERSION=1.0.0 FAKE_FAIL_ON='backup once' run upgrade.sh 1.1.0 >/dev/null 2>&1 || rc=$?
  assert_ne "exit code" 0 "$rc"
  has_call "run --rm backup once --tag pre-upgrade" || fail "no pre-upgrade backup was tried"
  no_call "up -d"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
}

test_upgrade_happy_path() {
  setup
  local v p da db b u want
  FAKE_APP_VERSION=1.0.0 run upgrade.sh 1.1.0 >/dev/null
  v=$(line_of "compose exec app open-customer version")
  p=$(line_of "compose pull")
  da=$(line_of "RepoDigests}}{{println .}}{{end}} ghcr.io/jnaranj09/open-customer:1.1.0")
  db=$(line_of "RepoDigests}}{{println .}}{{end}} ghcr.io/jnaranj09/open-customer-backup:1.1.0")
  b=$(line_of "compose run --rm backup once --tag pre-upgrade")
  u=$(line_of "compose up -d")
  if [ -z "$v" ] || [ -z "$p" ] || [ -z "$da" ] || [ -z "$db" ] || [ -z "$b" ] || [ -z "$u" ]; then
    fail "missing a step: $(calls)"
  fi
  if [ "$v" -ge "$p" ] || [ "$p" -ge "$da" ] || [ "$p" -ge "$db" ] || [ "$da" -ge "$b" ] ||
    [ "$db" -ge "$b" ] || [ "$b" -ge "$u" ]; then
    fail "wrong order: $(calls)"
  fi
  grep -qxF "compose pull | APP_VERSION=1.1.0" "$LOG" || fail "pull did not get APP_VERSION=1.1.0"
  want=$(printf '%s\n' "$env_file" | sed 's/^APP_VERSION=.*/APP_VERSION=1.1.0/')
  assert_eq ".env" "$want" "$(cat "$W/deploy/.env")"
}

test_upgrade_env_not_writable() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  chmod 400 "$W/deploy/.env"
  FAKE_APP_VERSION=1.0.0 run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  no_call "up -d"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
  grep -q "cannot set APP_VERSION" "$W/err" || fail "no message about .env: $(cat "$W/err")"
  no_temp_left
}

test_upgrade_term_leaves_no_temp() {
  setup
  local rc=0
  FAKE_APP_VERSION=1.0.0 PATH="$ROOT/sigbin:$PATH" run upgrade.sh 1.1.0 >/dev/null 2>&1 || rc=$?
  assert_eq "exit code" 143 "$rc"
  no_call "up -d"
  no_temp_left
}

test_upgrade_empty_temp_never_replaces_env() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  FAKE_APP_VERSION=1.0.0 PATH="$ROOT/emptybin:$PATH" run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" ||
    rc=$?
  assert_ne "exit code" 0 "$rc"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
  no_call "up -d"
  grep -q "cannot set APP_VERSION" "$W/err" || fail "no message about .env: $(cat "$W/err")"
  no_temp_left
}

test_upgrade_verifies_before_pull() {
  setup
  local v a b p want_id want_issuer want_repo want_trigger
  [ -n "$cosign_image" ] || fail "deploy/scripts/lib.sh has no COSIGN_IMAGE"
  FAKE_APP_VERSION=1.0.0 run upgrade.sh 1.1.0 >"$W/out" 2>&1 || fail "upgrade failed: $(cat "$W/out")"
  v=$(line_of "compose exec app open-customer version")
  a=$(line_of "$verify_app")
  b=$(line_of "$verify_backup")
  p=$(line_of "compose pull")
  if [ -z "$a" ] || [ -z "$b" ]; then fail "both images were not verified: $(calls)"; fi
  if [ -z "$v" ] || [ -z "$p" ]; then fail "missing the version check or the pull: $(calls)"; fi
  if [ "$v" -ge "$a" ] || [ "$v" -ge "$b" ] || [ "$a" -ge "$p" ] || [ "$b" -ge "$p" ]; then
    fail "wrong order: $(calls)"
  fi
  # The exact signer: the release workflow of this repository on a v* tag, through GitHub OIDC.
  want_id='--certificate-identity-regexp ^https://github\.com/jnaranj09/open-customer/\.github/workflows/release\.yml@refs/tags/v'
  want_issuer='--certificate-oidc-issuer https://token.actions.githubusercontent.com'
  want_repo='--certificate-github-workflow-repository jnaranj09/open-customer'
  want_trigger='--certificate-github-workflow-trigger push'
  [ "$(calls | grep -cF -- "$want_id $want_issuer $want_repo $want_trigger")" -eq 2 ] ||
    fail "a verify call does not pin the signer: $(calls)"
  # Quiet on success: neither cosign's stderr nor its JSON reaches the operator.
  if grep -q -e "signature" -e "Verification for" -e "docker-manifest-digest" "$W/out"; then
    fail "verify printed on success: $(cat "$W/out")"
  fi
}

test_upgrade_signature_check_fails() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  FAKE_APP_VERSION=1.0.0 FAKE_FAIL_ON=verify run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
  no_call pull
  no_call "backup once"
  no_call "up -d"
  grep -qF "ghcr.io/jnaranj09/open-customer:1.1.0" "$W/err" || fail "the message does not name the image: $(cat "$W/err")"
  grep -q "Sigstore" "$W/err" || fail "the message does not say what to check: $(cat "$W/err")"
}

test_upgrade_backup_image_signature_fails() {
  setup
  local rc=0
  FAKE_APP_VERSION=1.0.0 FAKE_FAIL_ON="verify ghcr.io/jnaranj09/open-customer-backup:" \
    run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  no_call pull
  no_call "backup once"
  grep -qF "ghcr.io/jnaranj09/open-customer-backup:1.1.0" "$W/err" || fail "the message does not name the backup image: $(cat "$W/err")"
}

test_upgrade_compose_without_backup_image() {
  setup
  local rc=0
  sed -i 's|open-customer-backup:|open-customer:|' "$W/deploy/compose.yaml"
  FAKE_APP_VERSION=1.0.0 run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  no_call verify
  no_call pull
  grep -q "compose.yaml must name the app image and its -backup image" "$W/err" ||
    fail "the message does not name compose.yaml: $(cat "$W/err")"
}

# The digest cosign verified for each image, as upgrade.sh must see it after the pull.
test_upgrade_pulled_digest_differs() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  FAKE_APP_VERSION=1.0.0 FAKE_PULLED_DIGEST=$other_digest run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  has_call "compose pull" || fail "the pull did not run: $(calls)"
  no_call "backup once"
  no_call "up -d"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
  grep -qF "ghcr.io/jnaranj09/open-customer:1.1.0" "$W/err" || fail "the message does not name the image: $(cat "$W/err")"
  grep -q "not the digest whose signature was verified" "$W/err" || fail "the message does not say why: $(cat "$W/err")"
}

test_upgrade_verify_output_without_digest() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  FAKE_APP_VERSION=1.0.0 FAKE_VERIFY_JSON='[]' run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  no_call pull
  no_call "backup once"
  no_call "up -d"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
  grep -q "no verified digest" "$W/err" || fail "the message does not say the digest is missing: $(cat "$W/err")"
}

test_upgrade_verify_output_two_digests() {
  setup
  local rc=0 json
  json="[{\"critical\":{\"image\":{\"docker-manifest-digest\":\"$other_digest\"}}},{\"critical\":{\"image\":{\"docker-manifest-digest\":\"sha256:$(printf '%064d' 0)\"}}}]"
  FAKE_APP_VERSION=1.0.0 FAKE_VERIFY_JSON=$json run upgrade.sh 1.1.0 >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  no_call pull
  no_call "backup once"
  grep -q "more than one digest" "$W/err" || fail "the message does not name two digests: $(cat "$W/err")"
}

test_restore_stops_on_no() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  printf 'n\n' | FAKE_SNAPSHOT_JSON=$snapshot_json run restore.sh latest >"$W/out" 2>&1 || rc=$?
  assert_eq "exit code" 1 "$rc"
  grep -qF "Restore snapshot 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef from 2026-11-01T03:00:00.123456789Z (app 0.9.0)? [y/N]" "$W/out" ||
    fail "no confirmation question: $(cat "$W/out")"
  has_call "snapshot-info latest" || fail "snapshot-info was not called"
  no_call stop
  no_call "restore restore"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
}

test_restore_failure_leaves_stopped() {
  setup
  local before rc=0
  before=$(cat "$W/deploy/.env")
  printf 'y\n' | FAKE_SNAPSHOT_JSON=$snapshot_json FAKE_FAIL_ON='restore restore' \
    run restore.sh latest >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  has_call "compose stop app backup" || fail "app and backup were not stopped"
  no_call "up -d"
  grep -q "app and backup are stopped" "$W/err" || fail "the message does not name the stopped services"
  assert_eq ".env" "$before" "$(cat "$W/deploy/.env")"
}

test_restore_env_not_writable() {
  setup
  local rc=0
  chmod 400 "$W/deploy/.env"
  printf 'y\n' | FAKE_SNAPSHOT_JSON=$snapshot_json run restore.sh latest >/dev/null 2>"$W/err" || rc=$?
  assert_ne "exit code" 0 "$rc"
  has_call "restore restore" || fail "the restore did not run"
  no_call "up -d"
  grep -q "app and backup are stopped" "$W/err" || fail "the message does not name the stopped services"
  no_temp_left
}

test_restore_happy_path() {
  setup
  local want s r
  printf 'y\n' | FAKE_SNAPSHOT_JSON=$snapshot_json run restore.sh latest >/dev/null
  has_call "compose --profile restore run --rm restore restore 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" ||
    fail "restore did not get the snapshot's id: $(calls)"
  s=$(line_of "compose stop app backup")
  r=$(line_of "restore restore")
  if [ -z "$s" ] || [ -z "$r" ]; then fail "missing the stop or the restore: $(calls)"; fi
  [ "$s" -lt "$r" ] || fail "restore ran before stop"
  assert_eq "last call" "compose up -d" "$(calls | tail -n1)"
  want=$(printf '%s\n' "$env_file" | sed 's/^APP_VERSION=.*/APP_VERSION=0.9.0/')
  assert_eq ".env" "$want" "$(cat "$W/deploy/.env")"
}

test_restore_snapshot_without_version() {
  setup
  local rc=0
  printf 'y\n' | FAKE_SNAPSHOT_JSON='{"id":"0123456789abcdef","time":"2026-11-01T03:00:00Z","app":null}' \
    run restore.sh latest >/dev/null 2>&1 || rc=$?
  assert_ne "exit code" 0 "$rc"
  no_call stop
}

# The real docker compose, on a copy of deploy/ with .env.example as .env: interpolation, the
# required variables and the file's shape. `config` only renders the file; it starts nothing.
test_compose_config() {
  setup
  local rendered services
  docker compose version >/dev/null 2>&1 || fail "docker compose is not available"
  mkdir -p "$W/target"
  sed "s|^BACKUP_TARGET=.*|BACKUP_TARGET=$W/target|" "$DEPLOY/.env.example" >"$W/deploy/.env"
  rendered=$(cd "$W/deploy" && docker compose config) || fail "docker compose config failed"
  services=$(cd "$W/deploy" && docker compose config --services | sort | tr '\n' ' ')
  assert_eq "services" "app backup cloudflared db " "$services"
  services=$(cd "$W/deploy" && docker compose --profile restore config --services | sort | tr '\n' ' ')
  assert_eq "services with the restore profile" "app backup cloudflared db restore " "$services"
  if grep -qE '^ +ports:' <<<"$rendered"; then
    fail "a service publishes ports"
  fi
  # A required variable left empty stops Compose with its name.
  sed -i 's/^TUNNEL_TOKEN=.*/TUNNEL_TOKEN=/' "$W/deploy/.env"
  (cd "$W/deploy" && docker compose config -q) 2>"$W/err" && fail "config passed without TUNNEL_TOKEN"
  grep -q "TUNNEL_TOKEN is required in .env" "$W/err" || fail "the error does not name TUNNEL_TOKEN"
}

t "each script without its arg exits non-zero" test_missing_args
t "backup-now passes the tag and the exit code" test_backup_now_tag_and_code
t "upgrade stops when .env and the running app disagree" test_upgrade_versions_disagree
t "upgrade stops when the pre-upgrade backup fails" test_upgrade_backup_fails
t "upgrade happy path order" test_upgrade_happy_path
t "upgrade verifies before it pulls" test_upgrade_verifies_before_pull
t "upgrade stops when the signature check fails" test_upgrade_signature_check_fails
t "upgrade stops when the backup image's signature check fails" test_upgrade_backup_image_signature_fails
t "upgrade stops when compose.yaml does not name the two release images" test_upgrade_compose_without_backup_image
t "upgrade stops when a pulled image is not the digest that was verified" test_upgrade_pulled_digest_differs
t "upgrade stops when cosign's output has no digest" test_upgrade_verify_output_without_digest
t "upgrade stops when cosign's output has two digests" test_upgrade_verify_output_two_digests
t "upgrade stops when .env cannot be written" test_upgrade_env_not_writable
t "a TERM during the .env edit leaves no temp copy" test_upgrade_term_leaves_no_temp
t "an empty temp copy never replaces .env" test_upgrade_empty_temp_never_replaces_env
t "restore asks first and stops on no" test_restore_stops_on_no
t "restore failure leaves app and backup stopped" test_restore_failure_leaves_stopped
t "restore stops with app and backup stopped when .env cannot be written" test_restore_env_not_writable
t "restore happy path sets APP_VERSION from the snapshot" test_restore_happy_path
t "restore refuses a snapshot without an app version" test_restore_snapshot_without_version
t "compose config is valid" test_compose_config
finish
