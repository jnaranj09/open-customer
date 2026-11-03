#!/usr/bin/env bash
# Stop hook (principle P1): the turn cannot end with red tests.
#
# - Runs the project's test command from the repo root, whatever folder the agent is in.
# - Red tests: exit 2, so the test output goes back to the agent and the turn continues.
# - When the payload says `"stop_hook_active": true` this stop was already blocked once: stand down
#   (exit 0) so a test the agent cannot fix never loops forever. The red result is still in the
#   transcript, and the agent must report it.
set -uo pipefail

cd "$(dirname "$0")/../.." || exit 2

# Use the pinned Node (web/package.json engines.node) when fnm manages Node on this machine.
if command -v fnm >/dev/null 2>&1; then
  NODE_PIN=$(sed -n 's/.*"node": "\(.*\)".*/\1/p' web/package.json 2>/dev/null)
  if [ -n "$NODE_PIN" ]; then
    eval "$(fnm env --shell bash)"
    fnm use --install-if-missing "$NODE_PIN" >/dev/null 2>&1 || true
  fi
fi

PAYLOAD=$(cat)
if printf '%s' "$PAYLOAD" | grep -q '"stop_hook_active"[[:space:]]*:[[:space:]]*true'; then
  exit 0
fi

OUT=$(make test 2>&1)
CODE=$?
if [ "$CODE" -ne 0 ]; then
  printf '%s\n' "$OUT" | tail -n 60 >&2
  echo "run-tests-on-stop: tests are red (exit $CODE). Fix them, or report them as red; never skip or loosen a test." >&2
  exit 2
fi
exit 0
