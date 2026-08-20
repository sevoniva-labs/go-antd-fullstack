#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/otel-runtime-contract.yaml"
OTEL_IMAGE="${FORGE_OTEL_IMAGE:?FORGE_OTEL_IMAGE must be set to an immutable image digest}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
HTTP_PORT="${FORGE_OTEL_HTTP_PORT:-14318}"
HEALTH_PORT="${FORGE_OTEL_HEALTH_PORT:-13133}"
EVIDENCE_FILE="${FORGE_OTEL_EVIDENCE_FILE:-}"
CONFIG_FILE="$(mktemp "$ROOT/otel-contract-config.XXXXXX")"
RESPONSE_FILE="$(mktemp "$ROOT/otel-contract-response.XXXXXX")"

if [[ ! "$OTEL_IMAGE" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "FORGE_OTEL_IMAGE must use an immutable @sha256 digest" >&2
  exit 1
fi

cleanup() {
  # shellcheck disable=SC2086
  OTEL_IMAGE="$OTEL_IMAGE" OTEL_CONTRACT_CONFIG="$CONFIG_FILE" \
    OTEL_CONTRACT_HTTP_PORT="$HTTP_PORT" OTEL_CONTRACT_HEALTH_PORT="$HEALTH_PORT" \
    sh -c "$COMPOSE_CMD \"\$@\"" -- -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$CONFIG_FILE" "$RESPONSE_FILE"
}
trap cleanup EXIT

cat >"$CONFIG_FILE" <<'EOF'
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  debug:
    verbosity: basic

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions:
    - health_check
  pipelines:
    traces:
      receivers:
        - otlp
      exporters:
        - debug
EOF

compose() {
  # shellcheck disable=SC2086
  OTEL_IMAGE="$OTEL_IMAGE" OTEL_CONTRACT_CONFIG="$CONFIG_FILE" \
    OTEL_CONTRACT_HTTP_PORT="$HTTP_PORT" OTEL_CONTRACT_HEALTH_PORT="$HEALTH_PORT" \
    sh -c "$COMPOSE_CMD \"\$@\"" -- "$@"
}

compose -f "$COMPOSE_FILE" up -d

HEALTH_URL="http://127.0.0.1:${HEALTH_PORT}/"
for _ in $(seq 1 30); do
  if curl --fail --silent --show-error "$HEALTH_URL" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --fail --silent --show-error "$HEALTH_URL" >/dev/null

TRACE_PAYLOAD='{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"forge-otel-contract"}}]},"scopeSpans":[{"scope":{"name":"forge-otel-contract"},"spans":[{"traceId":"0123456789abcdef0123456789abcdef","spanId":"0123456789abcdef","name":"forge-contract","kind":1,"startTimeUnixNano":"1700000000000000000","endTimeUnixNano":"1700000001000000000"}]}]}]}'
HTTP_STATUS="$(curl --silent --show-error --output "$RESPONSE_FILE" --write-out '%{http_code}' \
  -X POST "http://127.0.0.1:${HTTP_PORT}/v1/traces" \
  -H 'Content-Type: application/json' \
  --data "$TRACE_PAYLOAD")"
if [[ "$HTTP_STATUS" != "200" ]]; then
  echo "OTLP HTTP trace export failed with status $HTTP_STATUS" >&2
  cat "$RESPONSE_FILE" >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  cat >"$EVIDENCE_FILE" <<EOF
{
  "image": "${OTEL_IMAGE}",
  "health_endpoint": "${HEALTH_URL}",
  "otlp_http_endpoint": "http://127.0.0.1:${HTTP_PORT}/v1/traces",
  "health_check": true,
  "otlp_http_trace_status": ${HTTP_STATUS}
}
EOF
fi

echo "OTel runtime contract passed: image=$OTEL_IMAGE"
