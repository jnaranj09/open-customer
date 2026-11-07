#!/usr/bin/env bash
# make release-sign IMAGE=<repository>:<tag> (docs/designs/devsecops.md section 4, decision D11):
# the release workflow's last step for each of the two images it built and scanned. It pushes the
# local image, takes the digest the registry answered, writes a CycloneDX SBOM of that image with
# Trivy (the TRIVY_IMAGE pin, no network), and signs it and attests the SBOM keylessly with cosign
# on <repository>@<digest>, never on the tag. deploy/scripts/upgrade.sh verifies the signature.
#
# The digest: `docker push` prints `<tag>: digest: sha256:<64 hex> size: <n>` for the manifest
# the registry stored. The script takes exactly that line, and also requires the local image's
# RepoDigests to hold <repository>@<digest>, so the digest names the very image that was scanned
# and described in the SBOM. No digest, two digests or a mismatch is red before cosign runs.
#
# Needs a registry login (docker and cosign read the same Docker config) and, for cosign, the
# GitHub OIDC token of a job with `id-token: write`; it cannot sign outside the release workflow.
# Exit 2 on bad arguments, 1 on any failed step.
set -euo pipefail

usage="usage: release-sign.sh <repository>:<tag> (env TRIVY_IMAGE; cosign on PATH)"
if [ "$#" -ne 1 ]; then
  echo "$usage" >&2
  exit 2
fi
ref=$1
if ! [[ $ref =~ ^[a-z0-9][a-z0-9._/-]*(:[0-9]+)?(/[a-z0-9._-]+)*:[A-Za-z0-9._-]+$ ]]; then
  echo "release-sign: '$ref' is not <repository>:<tag>" >&2
  exit 2
fi
if [ -z "${TRIVY_IMAGE:-}" ]; then
  echo "release-sign: TRIVY_IMAGE is not set (run through make release-sign)" >&2
  exit 2
fi

die() {
  echo "release-sign: $*" >&2
  exit 1
}

command -v cosign >/dev/null || die "cosign is not on PATH (sigstore/cosign-installer)"

repo=${ref%:*}
tag=${ref##*:}
container=oc-release-sign-sbom
work=$(mktemp -d)
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

docker image inspect "$ref" >/dev/null 2>&1 || die "$ref is not in the local Docker"

# 1. The push, and the digest the registry answered.
push_out=$(docker push "$ref" 2>&1) || {
  printf '%s\n' "$push_out" >&2
  die "docker push $ref failed"
}
printf '%s\n' "$push_out"
digest=
while read -r line; do
  if [[ $line =~ ^"$tag":\ digest:\ (sha256:[0-9a-f]{64})\ size:\ [0-9]+$ ]]; then
    [ -z "$digest" ] || [ "$digest" = "${BASH_REMATCH[1]}" ] || die "docker push $ref printed two digests"
    digest=${BASH_REMATCH[1]}
  fi
done <<<"$push_out"
[ -n "$digest" ] || die "docker push $ref printed no digest; nothing is signed"
docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$ref" | grep -qxF "$repo@$digest" ||
  die "the local $ref does not record $repo@$digest; nothing is signed"
signed=$repo@$digest
echo "release-sign: pushed $signed"

# 2. The SBOM of the image just pushed (its saved copy, scanned with no network).
docker save -o "$work/image.tar" "$ref" || die "cannot save $ref for the SBOM"
docker run --rm --name "$container" --network none -u "$(id -u):$(id -g)" -v "$work:/scan" \
  "$TRIVY_IMAGE" image --cache-dir /scan/cache --format cyclonedx --output /scan/sbom.json \
  --no-progress --skip-version-check --input /scan/image.tar ||
  die "trivy could not write the SBOM of $ref"
[ -s "$work/sbom.json" ] || die "trivy wrote no SBOM for $ref"
grep -q '"bomFormat": *"CycloneDX"' "$work/sbom.json" || die "the SBOM of $ref is not CycloneDX"

# 3. The signature and the SBOM attestation, on the digest.
cosign sign --yes "$signed" || die "cosign sign $signed failed"
cosign attest --yes --type cyclonedx --predicate "$work/sbom.json" "$signed" ||
  die "cosign attest $signed failed"
echo "release-sign: signed $signed and attested its CycloneDX SBOM"
