#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${PROMETHEUS_IMAGE:?set PROMETHEUS_IMAGE to an approved immutable Prometheus image digest}"
: "${PROMETHEUS_VERSION:?set PROMETHEUS_VERSION explicitly for evidence}"
COMPOSE_CMD=${FORGE_COMPOSE_CMD:-docker compose}
read -r -a COMPOSE <<<"$COMPOSE_CMD"
PROJECT=${FORGE_PROMETHEUS_PROJECT:-forge-prometheus-contract-$$}
PORT=${PROMETHEUS_PORT:-19090}
COMPOSE_FILE=${FORGE_PROMETHEUS_COMPOSE_FILE:-deploy/compose/prometheus-runtime-contract.yaml}

compose() {
  "${COMPOSE[@]}" -p "$PROJECT" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  compose down -v >/dev/null 2>&1 || true
}

trap cleanup EXIT
compose config >/dev/null
compose up -d >/dev/null 2>&1

for _ in $(seq 1 120); do
  if curl -fsS "http://127.0.0.1:${PORT}/-/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${PORT}/-/ready" >/dev/null

query_file=$(mktemp)
trap 'rm -f "$query_file"; cleanup' EXIT
check_self_scrape() {
  python3 - "$query_file" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1], encoding="utf-8"))
if data.get("status") != "success" or data.get("data", {}).get("resultType") != "vector":
    raise SystemExit("Prometheus query API did not return a vector")
results = data["data"].get("result", [])
if not any(item.get("value", [None, None])[1] == "1" for item in results):
    raise SystemExit("Prometheus self-scrape up metric was not observed")
PY
}
for _ in $(seq 1 60); do
  curl -fsS --get --data-urlencode 'query=up{job="prometheus"}' "http://127.0.0.1:${PORT}/api/v1/query" >"$query_file" || true
  if check_self_scrape; then
    break
  fi
  sleep 1
done
check_self_scrape

if [[ -n "${FORGE_MIDDLEWARE_EVIDENCE_FILE:-}" ]]; then
  target_arch="${PROMETHEUS_PLATFORM##*/}"
  target_os="${PROMETHEUS_PLATFORM%%/*}"
  FORGE_MIDDLEWARE_EVIDENCE_FILE="$FORGE_MIDDLEWARE_EVIDENCE_FILE" \
  FORGE_MIDDLEWARE_PROVIDER=prometheus \
  FORGE_MIDDLEWARE_PRODUCT=prometheus \
  FORGE_MIDDLEWARE_VERSION="$PROMETHEUS_VERSION" \
  FORGE_MIDDLEWARE_ARCHITECTURE="$target_arch" \
  FORGE_MIDDLEWARE_OS="$target_os" \
  FORGE_MIDDLEWARE_RUNTIME=docker-compose \
  FORGE_MIDDLEWARE_DRIVER=prometheus-http-api \
  FORGE_MIDDLEWARE_IMAGE="$PROMETHEUS_IMAGE" \
  FORGE_MIDDLEWARE_CHECKS='{"readiness":"passed","query-api":"passed","self-scrape":"passed"}' \
    python3 scripts/write-middleware-evidence.py
  python3 scripts/check-middleware-evidence.py --file "$FORGE_MIDDLEWARE_EVIDENCE_FILE"
fi

printf 'Prometheus runtime contract passed: %s (%s)\n' "$PROMETHEUS_IMAGE" "$PROMETHEUS_VERSION"
