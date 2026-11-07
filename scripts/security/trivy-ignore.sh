# shellcheck shell=bash
# Shared by image-scan.sh and deps-scan.sh (docs/designs/devsecops.md "Blocking policy"). Sourced,
# never run: `. trivy-ignore.sh <name>`, where <name> is the caller's message prefix.
#
# write_trivy_ignore <file> [<repository>]: writes the Trivy ignore file for the live trivy ids of
# security/allow.json (allow.sh); the caller runs from the repo root. An id is one of three forms:
#   <CVE id>                       ignored wherever it is found;
#   <CVE id>@<path>                ignored only in the one result whose Target is that path: for a Go
#                                  binary in an image, its path inside the image, such as
#                                  usr/local/bin/gosu; for a manifest, its repo path. The path is
#                                  written as the entry's `paths:`. A path is a clean relative path
#                                  (no glob, no `..`, no leading or doubled slash);
#   <CVE id>@image:<repository>    ignored only when the scan is of the image whose repository (the
#                                  ref without tag and digest) is <repository>, such as
#                                  CVE-2026-76844@image:cloudflare/cloudflared. It is how an OS
#                                  package finding is scoped: its Target is "<tar> (<os>)", which a
#                                  path cannot name. <repository> is lowercase [a-z0-9._/-] only.
# The image-scoped ids are written only when <repository> is given and equal: image-scan.sh passes
# the repository of the image it is about to scan; deps-scan.sh passes none, so it skips them. Any
# id of another shape is refused, exit 1, rather than written into YAML; every id is checked on
# every call, whether or not it is written.
trivy_name=${1:?trivy-ignore.sh needs a name}

write_trivy_ignore() {
  local file=$1 repo=${2:-} ids id cve scope body=""
  ids=$(scripts/security/allow.sh trivy)
  while read -r id; do
    [ -n "$id" ] || continue
    cve=${id%%@*}
    if ! [[ "$cve" =~ ^[A-Za-z0-9._:-]+$ ]]; then
      echo "$trivy_name: security/allow.json has a trivy id that is not <CVE id>, <CVE id>@<path> or <CVE id>@image:<repository>" >&2
      exit 1
    fi
    if [ "$cve" = "$id" ]; then
      body+="  - id: $cve"$'\n'
      continue
    fi
    scope=${id#*@}
    if [[ "$scope" == image:* ]]; then
      if ! [[ "${scope#image:}" =~ ^[a-z0-9._/-]+$ ]]; then
        echo "$trivy_name: security/allow.json has a trivy id whose image repository is not lowercase [a-z0-9._/-] (no tag, no digest)" >&2
        exit 1
      fi
      if [ -n "$repo" ] && [ "${scope#image:}" = "$repo" ]; then
        body+="  - id: $cve"$'\n'
      fi
      continue
    fi
    if ! [[ "$scope" =~ ^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$ ]] || [[ "/$scope/" == */../* ]]; then
      echo "$trivy_name: security/allow.json has a trivy id whose path is not a clean relative path (no glob, no .., no leading /)" >&2
      exit 1
    fi
    body+="  - id: $cve"$'\n'"    paths:"$'\n'"      - \"$scope\""$'\n'
  done <<<"$ids"
  if [ -z "$body" ]; then
    echo "vulnerabilities: []" >"$file"
  else
    printf 'vulnerabilities:\n%s' "$body" >"$file"
  fi
}
