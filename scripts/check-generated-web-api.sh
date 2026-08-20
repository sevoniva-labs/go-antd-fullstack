#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

GENERATED=web/packages/api-client/src/generated
if [[ ! -d "$GENERATED" ]]; then
  echo "generated TypeScript API client is missing" >&2
  exit 1
fi

TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/forge-api-client.XXXXXX")
trap 'rm -rf "$TEMP_DIR"' EXIT
GENERATED_CANDIDATE="$TEMP_DIR/generated"

snapshot() {
  local directory=$1
  find "$directory" -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
}

corepack pnpm --filter @forge/api-client exec openapi-ts \
  -i ../../../api/gen/openapi/openapi.yaml \
  -o "$GENERATED_CANDIDATE"

if ! diff -ru "$GENERATED" "$GENERATED_CANDIDATE" >/dev/null; then
  echo "generated TypeScript API client is stale; run make web-api-generate and commit the result" >&2
  exit 1
fi

echo "generated TypeScript API client is current"
