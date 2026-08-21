#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${CLAMAV_IMAGE:?set CLAMAV_IMAGE to an approved immutable ClamAV image digest}"
COMPOSE_CMD=${FORGE_COMPOSE_CMD:-docker compose}
read -r -a COMPOSE <<<"$COMPOSE_CMD"
PROJECT=${FORGE_CLAMAV_PROJECT:-forge-clamav-contract-$$}
PORT=${CLAMAV_PORT:-13310}
COMPOSE_FILE=${FORGE_CLAMAV_COMPOSE_FILE:-deploy/compose/clamav-runtime-contract.yaml}

compose() {
  "${COMPOSE[@]}" -p "$PROJECT" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  compose down -v >/dev/null 2>&1 || true
}

trap cleanup EXIT
compose config >/dev/null
compose up -d >/dev/null 2>&1
for _ in $(seq 1 180); do
  if compose exec -T clamav clamdscan --ping 1 >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
compose exec -T clamav clamdscan --ping 1 >/dev/null

FORGE_CLAMAV_ADDRESS="127.0.0.1:${PORT}" go test ./internal/adapters/malwarescanner -run '^TestClamAVRuntimeContract$' -count=1

if [[ -n "${FORGE_MALWARE_EVIDENCE_FILE:-}" ]]; then
  mkdir -p "$(dirname "$FORGE_MALWARE_EVIDENCE_FILE")"
  python3 - "$FORGE_MALWARE_EVIDENCE_FILE" "$CLAMAV_IMAGE" "${CLAMAV_PLATFORM:-linux/amd64}" <<'PY'
import json
import pathlib
import platform
import subprocess
import sys
from datetime import datetime, timezone

path, image, target_platform = sys.argv[1:]
payload = {
    "kind": "forge-malware-runtime-evidence",
    "status": "passed",
    "level": "Experimental",
    "provider": "clamav",
    "image": image,
    "target_platform": target_platform,
    "runner_architecture": platform.machine(),
    "source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
    "checked_at": datetime.now(timezone.utc).isoformat(),
    "checks": ["clamd-ping", "clean-payload", "eicar-malicious-payload", "chunked-instream"],
}
pathlib.Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
fi

printf 'ClamAV runtime contract passed: %s on %s\n' "$CLAMAV_IMAGE" "${CLAMAV_PLATFORM:-linux/amd64}"
