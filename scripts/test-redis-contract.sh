#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${FORGE_REDIS_IMAGE:?set FORGE_REDIS_IMAGE to an approved immutable Redis image digest}"
: "${FORGE_REDIS_PASSWORD:?set FORGE_REDIS_PASSWORD in the local environment}"

COMPOSE_CMD=${FORGE_COMPOSE_CMD:-docker compose}
read -r -a COMPOSE <<<"$COMPOSE_CMD"
PROJECT=${FORGE_REDIS_PROJECT:-forge-redis-contract-$$}
COMPOSE_FILE=${FORGE_REDIS_COMPOSE_FILE:-deploy/compose/redis-dev.yaml}
EVIDENCE_FILE=${FORGE_REDIS_EVIDENCE_FILE:-}

export REDIS_IMAGE="$FORGE_REDIS_IMAGE"
export REDIS_PASSWORD="$FORGE_REDIS_PASSWORD"

compose() {
  "${COMPOSE[@]}" -p "$PROJECT" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  compose down -v >/dev/null 2>&1 || true
}

trap cleanup EXIT

compose config >/dev/null
compose up -d >/dev/null 2>&1

redis_exec() {
  compose exec -T redis sh -c "$1"
}

authenticated_ping() {
  redis_exec 'REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli --raw ping' 2>/dev/null | rg -q '^PONG$'
}

unauthenticated_ping_rejected() {
  local output

  output=$(redis_exec 'redis-cli --raw ping' 2>&1 || true)
  [[ "$output" == *NOAUTH* || "$output" == *"Authentication required"* ]]
}

ready=false
for _ in $(seq 1 90); do
  if authenticated_ping; then
    ready=true
    break
  fi
  sleep 1
done

if [[ "$ready" != true ]]; then
  printf 'Redis contract failed: authenticated ping did not become ready\n' >&2
  compose ps >&2 || true
  exit 1
fi

if ! unauthenticated_ping_rejected; then
  printf 'Redis contract failed: unauthenticated ping was accepted\n' >&2
  exit 1
fi

key="forge:contract:${PROJECT}"
if ! redis_exec "REDISCLI_AUTH=\"\$REDIS_PASSWORD\" redis-cli --raw set '$key' 'contract-value' EX 30" 2>/dev/null | rg -q '^OK$'; then
  printf 'Redis contract failed: authenticated write failed\n' >&2
  exit 1
fi

value=$(redis_exec "REDISCLI_AUTH=\"\$REDIS_PASSWORD\" redis-cli --raw get '$key'" 2>/dev/null)
if [[ "$value" != "contract-value" ]]; then
  printf 'Redis contract failed: authenticated read returned an unexpected value\n' >&2
  exit 1
fi

ttl=$(redis_exec "REDISCLI_AUTH=\"\$REDIS_PASSWORD\" redis-cli --raw ttl '$key'" 2>/dev/null)
if ! [[ "$ttl" =~ ^[0-9]+$ ]] || (( ttl <= 0 )); then
  printf 'Redis contract failed: key TTL was not positive\n' >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  python3 - "$EVIDENCE_FILE" "$PROJECT" "$FORGE_REDIS_IMAGE" "$(git rev-parse HEAD)" <<'PY'
import json
import pathlib
import sys
from datetime import datetime, timezone

path, project, image, commit = sys.argv[1:]
payload = {
    "kind": "redis-runtime-contract",
    "status": "passed",
    "project": project,
    "redis_image": image,
    "source_commit": commit,
    "checked_at": datetime.now(timezone.utc).isoformat(),
    "checks": [
        "authenticated-ping",
        "unauthenticated-ping-rejected",
        "authenticated-set-get",
        "positive-ttl",
    ],
}
pathlib.Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
fi

printf 'Redis runtime contract passed: image=%s\n' "$FORGE_REDIS_IMAGE"
