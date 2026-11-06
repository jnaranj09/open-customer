#!/usr/bin/env bash
# Prints the live allow-list ids for one tool, one per line (docs/designs/devsecops.md "Blocking
# policy", decision D4): the entries of security/allow.json whose tool matches and whose expires
# date is today (UTC) or later. Each scan wrapper turns them into its tool's own ignore form. The
# guard TestSecurityAllowList (internal/guards) checks the file's entries; this script only reads.
# Exit 2 on a missing or unknown tool name; any other failure (no jq, bad JSON) exits non-zero.
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
jq -r --arg tool "$1" --arg today "$today" \
	'.[] | select(.tool == $tool and .expires >= $today) | .id' "$file"
