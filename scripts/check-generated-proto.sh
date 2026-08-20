#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/forge-proto.XXXXXX")
trap 'rm -rf "$TEMP_DIR"' EXIT

.tools/bin/buf generate \
  --template buf.gen.yaml \
  --output "$TEMP_DIR" \
  --path api/proto/forge

if ! diff -ru api/gen/go "$TEMP_DIR/api/gen/go" \
  || ! diff -ru api/gen/openapi "$TEMP_DIR/api/gen/openapi"; then
  echo "generated Proto files are stale; run make proto-generate and commit the result" >&2
  exit 1
fi

echo "generated Proto and OpenAPI files are current"
