#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

snapshot() {
  if [[ ! -d api/gen/go ]]; then
    return
  fi
  find api/gen/go -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
}

before=$(snapshot)
.tools/bin/buf generate --path api/proto/forge
after=$(snapshot)
if [[ "$before" != "$after" ]]; then
  echo "generated Proto files are stale; run make proto-generate and commit the result" >&2
  exit 1
fi

echo "generated Proto files are current"
