# shellcheck shell=bash
# Shared by image-scan.sh and deps-scan.sh (docs/designs/devsecops.md "Blocking policy"). Sourced,
# never run: `. trivy-ignore.sh <name>`, where <name> is the caller's message prefix.
#
# write_trivy_ignore <file>: writes the Trivy ignore file for the live trivy ids of
# security/allow.json (allow.sh); the caller runs from the repo root. An id is <CVE id> (ignored
# wherever it is found) or <CVE id>@<path> (ignored only in the one result whose Target is that
# path: for a Go binary in an image, its path inside the image, such as usr/local/bin/gosu; for a
# manifest, its repo path). The path is written as the entry's `paths:`. A path is a clean relative
# path (no glob, no `..`, no leading or doubled slash). Any other id is refused, exit 1, rather
# than written into YAML. deps-scan.sh passes a scoped id through like image-scan.sh does: an image
# path never equals a manifest's Target there, so it ignores nothing.
trivy_name=${1:?trivy-ignore.sh needs a name}

write_trivy_ignore() {
  local file=$1 ids id cve scope
  ids=$(scripts/security/allow.sh trivy)
  {
    if [ -z "$ids" ]; then
      echo "vulnerabilities: []"
    else
      echo "vulnerabilities:"
      while read -r id; do
        cve=${id%%@*}
        if ! [[ "$cve" =~ ^[A-Za-z0-9._:-]+$ ]]; then
          echo "$trivy_name: security/allow.json has a trivy id that is not <CVE id> or <CVE id>@<path>" >&2
          exit 1
        fi
        echo "  - id: $cve"
        [ "$cve" != "$id" ] || continue
        scope=${id#*@}
        if ! [[ "$scope" =~ ^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$ ]] || [[ "/$scope/" == */../* ]]; then
          echo "$trivy_name: security/allow.json has a trivy id whose path is not a clean relative path (no glob, no .., no leading /)" >&2
          exit 1
        fi
        printf '    paths:\n      - "%s"\n' "$scope"
      done <<<"$ids"
    fi
  } >"$file"
}
