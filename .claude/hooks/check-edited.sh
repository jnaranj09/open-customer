#!/usr/bin/env bash
# PostToolUse hook (principle P1): after an Edit/Write, run the fast checks for the ONE edited file
# and hand any error back to the agent, so it fixes type and lint errors in the same turn.
#
# - Reads the edited path from the hook payload on stdin (`tool_input.file_path`).
# - A file type with no block below, a deleted file or an empty payload: nothing to check, exit 0.
# - A tool that is not installed FAILS (exit 2) and says so. A check that silently skips is a gate
#   that lies: "no errors" must mean the check ran.
# - Exit 2 sends stderr back to the agent.
#
# Blocks: Go, the web app (TS/JS/Svelte), shell. The web block calls the package's own `check`
# script (svelte-kit sync + svelte-check), because plain tsc cannot type-check .svelte files.
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
if command -v jq >/dev/null 2>&1; then
  FILE=$(printf '%s' "$PAYLOAD" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
elif command -v python3 >/dev/null 2>&1; then
  FILE=$(printf '%s' "$PAYLOAD" | python3 -c 'import json,sys
try: print(json.load(sys.stdin).get("tool_input", {}).get("file_path", ""))
except Exception: print("")')
else
  echo "check-edited: need jq or python3 to read the hook payload" >&2
  exit 2
fi

[ -n "$FILE" ] || exit 0
[ -f "$FILE" ] || exit 0

missing() {
  echo "check-edited: $1: finish TASKS.md T1 (the gate) before editing $FILE" >&2
  exit 2
}

need() {
  command -v "$1" >/dev/null 2>&1 || missing "tool $1 not installed"
}

# Absolute folder of the edited file; walks below go up from here to `/`.
HERE=$(cd "$(dirname "$FILE")" && pwd)

FAIL=0
case "$FILE" in
  # Generated files are checked by `make check-generated`, not edited by hand.
  */internal/httpapi/gen/* | */internal/db/sqlc/* | */web/src/lib/api/schema.d.ts) exit 0 ;;
  # --- go ---
  *.go)
    need gofmt
    need go
    need make
    if [ -n "$(gofmt -l "$FILE")" ]; then
      echo "gofmt: $FILE is not formatted (run gofmt -w)" >&2
      FAIL=1
    fi
    # web/embed.go needs web/build to compile; the stub never overwrites a real build.
    [ -f web/build/index.html ] || make web-stub >&2 || missing "make web-stub failed"
    (cd "$HERE" && go vet .) >&2 || FAIL=1
    ;;
  # --- end go ---
  # --- web ---
  *.ts | *.js | *.mjs | *.cjs | *.svelte)
    # The package that owns the file: nearest package.json at or above it.
    PKG=$HERE
    while [ "$PKG" != "/" ] && [ ! -f "$PKG/package.json" ]; do PKG=$(dirname "$PKG"); done
    [ -f "$PKG/package.json" ] || exit 0
    need pnpm
    [ -x "$PKG/node_modules/.bin/eslint" ] || missing "eslint not installed in $PKG (run pnpm install)"
    (cd "$PKG" && ./node_modules/.bin/eslint --cache "$FILE") >&2 || FAIL=1
    (cd "$PKG" && pnpm run --silent check) >&2 || FAIL=1
    ;;
  # --- end web ---
  # --- shell ---
  *.sh | *.bash)
    need shellcheck
    shellcheck "$FILE" >&2 || FAIL=1
    ;;
  # --- end shell ---
  *) exit 0 ;;
esac

if [ "$FAIL" -ne 0 ]; then
  echo "check-edited: $FILE has errors (above). Fix them before moving on." >&2
  exit 2
fi
exit 0
