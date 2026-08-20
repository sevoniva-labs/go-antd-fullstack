#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/apisix-runtime-contract.yaml"
APISIX_IMAGE="${FORGE_APISIX_IMAGE:?FORGE_APISIX_IMAGE must be set to an immutable image digest}"
ETCD_IMAGE="${FORGE_APISIX_ETCD_IMAGE:?FORGE_APISIX_ETCD_IMAGE must be set to an immutable image digest}"
NGINX_IMAGE="${FORGE_APISIX_NGINX_IMAGE:?FORGE_APISIX_NGINX_IMAGE must be set to an immutable image digest}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
DATA_PORT="${FORGE_APISIX_DATA_PORT:-19080}"
ADMIN_PORT="${FORGE_APISIX_ADMIN_PORT:-19180}"
EVIDENCE_FILE="${FORGE_APISIX_EVIDENCE_FILE:-}"
ADMIN_KEY="${FORGE_APISIX_CONTRACT_ADMIN_KEY:-forge-contract-admin-key}"
CONFIG_FILE="$(mktemp "$ROOT/apisix-contract-config.XXXXXX")"
BACKEND_CONFIG="$(mktemp "$ROOT/apisix-contract-backend.XXXXXX")"

for image in "$APISIX_IMAGE" "$ETCD_IMAGE" "$NGINX_IMAGE"; do
  if [[ ! "$image" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
    echo "all APISIX contract images must use immutable @sha256 digests" >&2
    exit 1
  fi
done

compose() {
  # shellcheck disable=SC2086
  COMPOSE_PROJECT_NAME=forge-apisix-contract APISIX_IMAGE="$APISIX_IMAGE" \
    ETCD_IMAGE="$ETCD_IMAGE" NGINX_IMAGE="$NGINX_IMAGE" APISIX_CONFIG="$CONFIG_FILE" \
    APISIX_BACKEND_CONFIG="$BACKEND_CONFIG" APISIX_CONTRACT_DATA_PORT="$DATA_PORT" \
    APISIX_CONTRACT_ADMIN_PORT="$ADMIN_PORT" sh -c "$COMPOSE_CMD \"\$@\"" -- "$@"
}

cleanup() {
  compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$CONFIG_FILE" "$BACKEND_CONFIG"
}
trap cleanup EXIT

cat >"$CONFIG_FILE" <<EOF
deployment:
  role: traditional
  role_traditional:
    config_provider: etcd
  etcd:
    host:
      - http://etcd:2379
    prefix: /apisix
    timeout: 10
  admin:
    allow_admin:
      - 0.0.0.0/0
    admin_key:
      - name: contract
        key: ${ADMIN_KEY}
        role: admin
apisix:
  node_listen: 9080
  enable_ipv6: false
  enable_admin: true
  enable_admin_cors: false
  enable_control: false
EOF

cat >"$BACKEND_CONFIG" <<'EOF'
server {
  listen 8080;
  server_name _;
  location / {
    default_type application/json;
    return 200 '{"backend":"forge-apisix-contract"}';
  }
}
EOF

compose -f "$COMPOSE_FILE" up -d

ADMIN_URL="http://127.0.0.1:${ADMIN_PORT}"
DATA_URL="http://127.0.0.1:${DATA_PORT}"
for _ in $(seq 1 45); do
  if curl --connect-timeout 1 --max-time 3 --fail --silent --show-error -H "X-API-KEY: ${ADMIN_KEY}" \
    "$ADMIN_URL/apisix/admin/routes" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --connect-timeout 1 --max-time 3 --fail --silent --show-error -H "X-API-KEY: ${ADMIN_KEY}" \
  "$ADMIN_URL/apisix/admin/routes" >/dev/null

UNAUTHORIZED_STATUS="$(curl --connect-timeout 1 --max-time 3 --silent --output /dev/null --write-out '%{http_code}' \
  "$ADMIN_URL/apisix/admin/routes/forge-contract")"
if [[ "$UNAUTHORIZED_STATUS" != "401" && "$UNAUTHORIZED_STATUS" != "403" ]]; then
  echo "APISIX Admin API accepted an unauthenticated request: status=$UNAUTHORIZED_STATUS" >&2
  exit 1
fi

route_payload='{"uri":"/forge-contract","methods":["GET"],"upstream":{"type":"roundrobin","nodes":{"backend:8080":1}}}'
ROUTE_STATUS="$(curl --connect-timeout 1 --max-time 3 --silent --output /dev/null --write-out '%{http_code}' \
  -X PUT "$ADMIN_URL/apisix/admin/routes/forge-contract" \
  -H "X-API-KEY: ${ADMIN_KEY}" -H 'Content-Type: application/json' \
  --data "$route_payload")"
if [[ "$ROUTE_STATUS" != "200" && "$ROUTE_STATUS" != "201" ]]; then
  echo "APISIX route creation failed: status=$ROUTE_STATUS" >&2
  exit 1
fi

RESPONSE_FILE="$(mktemp "$ROOT/apisix-contract-response.XXXXXX")"
trap 'rm -f "$RESPONSE_FILE"; cleanup' EXIT
DATA_STATUS="$(curl --connect-timeout 1 --max-time 3 --silent --show-error --output "$RESPONSE_FILE" --write-out '%{http_code}' \
  "$DATA_URL/forge-contract")"
if [[ "$DATA_STATUS" != "200" ]] || ! grep -q 'forge-apisix-contract' "$RESPONSE_FILE"; then
  echo "APISIX route forwarding failed: status=$DATA_STATUS" >&2
  cat "$RESPONSE_FILE" >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  cat >"$EVIDENCE_FILE" <<EOF
{
  "apisix_image": "${APISIX_IMAGE}",
  "etcd_image": "${ETCD_IMAGE}",
  "nginx_image": "${NGINX_IMAGE}",
  "admin_unauthorized_status": ${UNAUTHORIZED_STATUS},
  "route_write_status": ${ROUTE_STATUS},
  "route_forward_status": ${DATA_STATUS}
}
EOF
fi

echo "APISIX runtime contract passed: image=$APISIX_IMAGE"
