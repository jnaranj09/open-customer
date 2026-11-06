# Shared helpers for the deploy shell tests (sourced, never run). `t <name> <fn>` runs one test
# function in a subshell with `set -e`, so the first failed assertion ends that test only, and
# prints `ok <name>` or `FAIL <name>`. Background jobs a test leaves running are killed when it
# ends, so a failed test cannot block the next one. `finish` prints the totals and exits non-zero
# on any failure. The sourcing script must not use `set -e`.
# shellcheck shell=bash

tests_run=0
tests_failed=0

t() {
  local name=$1 fn=$2 rc
  tests_run=$((tests_run + 1))
  # Not in an `if` or `||`: there, bash would turn `set -e` off inside the subshell.
  (
    set -euo pipefail
    trap 'jobs -p | xargs -r kill 2>/dev/null || true' EXIT
    "$fn"
  )
  rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "ok $name"
  else
    tests_failed=$((tests_failed + 1))
    echo "FAIL $name"
  fi
}

finish() {
  echo "$tests_run tests, $tests_failed failed"
  [ "$tests_failed" -eq 0 ]
}

# fail <message>: the reason a test failed, on stderr.
fail() {
  echo "  $*" >&2
  return 1
}

# assert_eq <what> <want> <got>
assert_eq() {
  [ "$2" = "$3" ] || fail "$1: want '$2', got '$3'"
}

# assert_ne <what> <unwanted> <got>
assert_ne() {
  [ "$2" != "$3" ] || fail "$1: did not want '$2'"
}

# assert_prefix <what> <prefix> <got>
assert_prefix() {
  case "$3" in
    "$2"*) ;;
    *) fail "$1: want a value starting with '$2', got '$3'" ;;
  esac
}
