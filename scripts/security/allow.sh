#!/usr/bin/env bash
# Prints the live allow-list ids for one tool, one per line (docs/designs/devsecops.md "Blocking
# policy", decision D4): the entries of security/allow.json whose tool matches and whose expires
# date is today (UTC) or later. Each scan wrapper turns them into its tool's own ignore form. The
# guard TestSecurityAllowList (internal/guards) checks the file's entries; this script only reads.
# Exit 2 on a missing or unknown tool name; exit 1 on an id of that tool with a line break; any
# other failure (no jq, bad JSON) exits non-zero.
set -euo pipefail

TOOLS="trivy trivy-config zizmor semgrep zap nuclei gitleaks"

if [ "$#" -ne 1 ]; then
	echo "usage: allow.sh <tool> (one of: $TOOLS)" >&2
	exit 2
fi
case " $TOOLS " in
*" $1 "*) ;;
*)
	echo "allow.sh: unknown tool '$1' (one of: $TOOLS)" >&2
	exit 2
	;;
esac
command -v jq >/dev/null || {
	echo "allow.sh: jq is not installed" >&2
	exit 1
}

file="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/security/allow.json"
today="$(date -u +%Y-%m-%d)"
# The ids go out one per line, so an id with a line break would reach a wrapper as two ids and one
# approved entry would silence two findings; the wrapper cannot see it, so it is refused here (exit
# 1), as the guard refuses it. Each wrapper refuses a space or a comma in its own ids.
ids=$(jq -r --arg tool "$1" --arg today "$today" \
	'.[] | select(.tool == $tool and .expires >= $today) | .id' "$file")
bad=$(jq -r --arg tool "$1" '[.[] | select(.tool == $tool and (.id | test("[\n\r\u000b\u000c\u0085\u2028\u2029]")))] | length' "$file")
if [ "$bad" != 0 ]; then
	echo "allow.sh: security/allow.json has a $1 id with a line break" >&2
	exit 1
fi
[ -z "$ids" ] || printf '%s\n' "$ids"
