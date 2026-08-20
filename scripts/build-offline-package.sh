#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT="${OFFLINE_BUNDLE_DIR:-$ROOT/.evidence/offline-bundle}"
IMAGES_LOCK="${OFFLINE_IMAGES_LOCK:-}"
SIGNATURE_FILE="${OFFLINE_SIGNATURE_FILE:-}"
SBOM_FILE="${OFFLINE_SBOM_FILE:-}"
REQUIRE_CERTIFIED="${OFFLINE_REQUIRE_CERTIFIED:-false}"

if [[ "$OUTPUT" != /* ]]; then
  OUTPUT="$ROOT/$OUTPUT"
fi
if [[ "$IMAGES_LOCK" != /* ]]; then
  IMAGES_LOCK="$PWD/$IMAGES_LOCK"
fi
if [[ -n "$SIGNATURE_FILE" && "$SIGNATURE_FILE" != /* ]]; then
  SIGNATURE_FILE="$PWD/$SIGNATURE_FILE"
fi
if [[ -n "$SBOM_FILE" && "$SBOM_FILE" != /* ]]; then
  SBOM_FILE="$PWD/$SBOM_FILE"
fi
if [[ -z "$IMAGES_LOCK" || ! -s "$IMAGES_LOCK" ]]; then
  echo "OFFLINE_IMAGES_LOCK must point to an approved internal image lock" >&2
  exit 1
fi
if [[ -e "$OUTPUT" ]]; then
  echo "offline bundle already exists: $OUTPUT (choose another OFFLINE_BUNDLE_DIR)" >&2
  exit 1
fi

if rg -n -I 'docker\.io|ghcr\.io|quay\.io|registry-1\.docker\.io' "$IMAGES_LOCK"; then
  echo "public OCI source found in OFFLINE_IMAGES_LOCK" >&2
  exit 1
fi
if ! awk 'NF && $0 !~ /@sha256:[0-9a-f]{64}$/ {bad=1} END {exit bad}' "$IMAGES_LOCK"; then
  echo "every offline image must be pinned by a lowercase sha256 digest" >&2
  exit 1
fi
if [[ "$REQUIRE_CERTIFIED" == "true" ]]; then
  [[ -n "$SIGNATURE_FILE" && -s "$SIGNATURE_FILE" ]] || {
    echo "OFFLINE_SIGNATURE_FILE is required for a certified offline bundle" >&2
    exit 1
  }
  [[ -n "$SBOM_FILE" && -s "$SBOM_FILE" ]] || {
    echo "OFFLINE_SBOM_FILE is required for a certified offline bundle" >&2
    exit 1
  }
  [[ -n "${OFFLINE_SOURCE_MIRROR_REF:-}" ]] || {
    echo "OFFLINE_SOURCE_MIRROR_REF is required for a certified offline bundle" >&2
    exit 1
  }
  [[ -n "${OFFLINE_APPROVAL_REF:-}" ]] || {
    echo "OFFLINE_APPROVAL_REF is required for a certified offline bundle" >&2
    exit 1
  }
fi

mkdir -p "$(dirname "$OUTPUT")"
STAGING="$(mktemp -d "$(dirname "$OUTPUT")/.offline-staging.XXXXXX")"
cleanup() {
  if [[ -n "${STAGING:-}" && -d "$STAGING" ]]; then
    rm -rf "$STAGING"
  fi
}
trap cleanup EXIT

cd "$ROOT"
git archive --format=tar HEAD | tar -xf - -C "$STAGING"
cp "$IMAGES_LOCK" "$STAGING/images.lock"
if [[ -n "$SBOM_FILE" ]]; then
  [[ -s "$SBOM_FILE" ]] || {
    echo "OFFLINE_SBOM_FILE is not a readable non-empty file" >&2
    exit 1
  }
  cp "$SBOM_FILE" "$STAGING/sbom.cdx.json"
fi

commit="$(git rev-parse HEAD)"
utc_now="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
version_or_unknown() {
  if command -v "$1" >/dev/null 2>&1; then
    "$@" 2>/dev/null || printf 'unavailable'
  else
    printf 'unavailable'
  fi
}

cat >"$STAGING/provenance.txt" <<EOF
kind=forge-offline-bundle
source_commit=$commit
created_at_utc=$utc_now
source_archive=git archive HEAD (uncommitted worktree changes excluded)
image_lock=caller-supplied approved lock; every entry is digest-pinned
source_mirror_ref=${OFFLINE_SOURCE_MIRROR_REF:-not-supplied}
approval_ref=${OFFLINE_APPROVAL_REF:-not-supplied}
sbom=$(if [[ -n "$SBOM_FILE" ]]; then printf 'sbom.cdx.json'; else printf 'not-supplied'; fi)
go_version=$(version_or_unknown go version)
node_version=$(version_or_unknown node --version)
pnpm_version=$(version_or_unknown corepack pnpm --version)
source_policy=domestic-or-internal-only; no silent public fallback
release_status=$(if [[ "$REQUIRE_CERTIFIED" == "true" ]]; then printf 'Target-tested'; else printf 'Not certified until target signing, air-gapped install, upgrade, rollback, and recovery evidence is attached'; fi)
EOF

if [[ -n "$SIGNATURE_FILE" ]]; then
  [[ -s "$SIGNATURE_FILE" ]] || {
    echo "OFFLINE_SIGNATURE_FILE is not a readable non-empty file" >&2
    exit 1
  }
  cp "$SIGNATURE_FILE" "$STAGING/release-signature"
fi

(
  cd "$STAGING"
  find . -type f ! -name manifest.sha256 -print | LC_ALL=C sort | while IFS= read -r path; do
    shasum -a 256 "$path"
  done
) >"$STAGING/manifest.sha256"

mv "$STAGING" "$OUTPUT"
STAGING=""
echo "offline bundle built: $OUTPUT"
