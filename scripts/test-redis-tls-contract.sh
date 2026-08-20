#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${FORGE_REDIS_IMAGE:?set FORGE_REDIS_IMAGE to an approved immutable Redis image digest}"
: "${FORGE_REDIS_TLS_PASSWORD:?set FORGE_REDIS_TLS_PASSWORD in the local environment}"

if [[ ! "$FORGE_REDIS_IMAGE" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "FORGE_REDIS_IMAGE must use an immutable @sha256 digest" >&2
  exit 1
fi
command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v openssl >/dev/null 2>&1 || { echo "openssl is required" >&2; exit 1; }

CONTAINER="forge-redis-tls-contract-$$"
EVIDENCE_FILE=${FORGE_REDIS_TLS_EVIDENCE_FILE:-}
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/forge-redis-tls.XXXXXX")
APP_PASSWORD='forge-redis-app-password'

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

umask 077
openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
  -keyout "$TMP_DIR/ca.key" -out "$TMP_DIR/ca.crt" \
  -subj '/CN=forge-redis-test-ca' >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes \
  -keyout "$TMP_DIR/server.key" -out "$TMP_DIR/server.csr" \
  -subj '/CN=localhost' >/dev/null 2>&1
printf 'subjectAltName=DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth\n' >"$TMP_DIR/server.ext"
openssl x509 -req -in "$TMP_DIR/server.csr" -CA "$TMP_DIR/ca.crt" -CAkey "$TMP_DIR/ca.key" \
  -CAcreateserial -days 1 -out "$TMP_DIR/server.crt" -extfile "$TMP_DIR/server.ext" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes \
  -keyout "$TMP_DIR/client.key" -out "$TMP_DIR/client.csr" \
  -subj '/CN=forge-redis-test-client' >/dev/null 2>&1
printf 'extendedKeyUsage=clientAuth\n' >"$TMP_DIR/client.ext"
openssl x509 -req -in "$TMP_DIR/client.csr" -CA "$TMP_DIR/ca.crt" -CAkey "$TMP_DIR/ca.key" \
  -CAcreateserial -days 1 -out "$TMP_DIR/client.crt" -extfile "$TMP_DIR/client.ext" >/dev/null 2>&1
printf 'user default on >%s ~* &* +@all\nuser app on >%s ~forge:* &* +@read +@write +expire\n' \
  "$FORGE_REDIS_TLS_PASSWORD" "$APP_PASSWORD" >"$TMP_DIR/users.acl"

docker run -d --name "$CONTAINER" \
  -e REDIS_PASSWORD="$FORGE_REDIS_TLS_PASSWORD" \
  -v "$TMP_DIR:/tls:ro" \
  "$FORGE_REDIS_IMAGE" sh -c \
  'exec redis-server --port 0 --tls-port 6379 --tls-cert-file /tls/server.crt --tls-key-file /tls/server.key --tls-ca-cert-file /tls/ca.crt --tls-auth-clients yes --aclfile /tls/users.acl --appendonly yes' \
  >/dev/null

redis_cli_default() {
  docker exec -e REDISCLI_AUTH="$FORGE_REDIS_TLS_PASSWORD" "$CONTAINER" redis-cli \
    --tls --cacert /tls/ca.crt --cert /tls/client.crt --key /tls/client.key --user default "$@"
}

redis_cli_app() {
  docker exec -e REDISCLI_AUTH="$APP_PASSWORD" "$CONTAINER" redis-cli \
    --tls --cacert /tls/ca.crt --cert /tls/client.crt --key /tls/client.key --user app "$@"
}

ready=false
for _ in $(seq 1 60); do
  if redis_cli_default --raw ping 2>/dev/null | rg -q '^PONG$'; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  docker logs "$CONTAINER" >&2 || true
  echo "Redis TLS contract failed: TLS ping did not become ready" >&2
  exit 1
fi

if docker exec "$CONTAINER" redis-cli --raw ping >/dev/null 2>&1; then
  echo "Redis TLS contract failed: plaintext connection was accepted" >&2
  exit 1
fi

if ! redis_cli_default --raw acl setuser app on ">$APP_PASSWORD" '~forge:*' '+@read' '+@write' '+expire' 2>/dev/null | rg -q '^OK$'; then
  echo "Redis TLS contract failed: ACL user setup failed" >&2
  exit 1
fi
if ! redis_cli_app --raw set forge:tls-contract tls-value EX 120 2>/dev/null | rg -q '^OK$'; then
  echo "Redis TLS contract failed: ACL-scoped write failed" >&2
  exit 1
fi
if [[ "$(redis_cli_app --raw get forge:tls-contract 2>/dev/null)" != "tls-value" ]]; then
  echo "Redis TLS contract failed: ACL-scoped read returned an unexpected value" >&2
  exit 1
fi
outside_result=$(redis_cli_app --raw set outside:key denied 2>&1 || true)
if [[ "$outside_result" != *NOPERM* ]]; then
  echo "Redis TLS contract failed: ACL key scope was not enforced" >&2
  exit 1
fi

if ! docker restart "$CONTAINER" >/dev/null; then
  echo "Redis TLS contract failed: restart did not complete" >&2
  exit 1
fi
ready=false
for _ in $(seq 1 60); do
  if redis_cli_default --raw ping 2>/dev/null | rg -q '^PONG$'; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true || "$(redis_cli_app --raw get forge:tls-contract 2>/dev/null)" != "tls-value" ]]; then
  echo "Redis TLS contract failed: TLS/ACL value did not recover after restart" >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  python3 - "$EVIDENCE_FILE" "$FORGE_REDIS_IMAGE" "$(git rev-parse HEAD)" <<'PY'
import json
import pathlib
import sys
from datetime import datetime, timezone

path, image, commit = sys.argv[1:]
payload = {
    "kind": "redis-tls-runtime-contract",
    "status": "passed",
    "redis_image": image,
    "source_commit": commit,
    "checked_at": datetime.now(timezone.utc).isoformat(),
    "checks": [
        "tls-certificate-chain",
        "plaintext-port-rejected",
        "password-authentication",
        "acl-key-scope",
        "tls-acl-value-recovery-after-restart",
    ],
}
pathlib.Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
fi

printf 'Redis TLS runtime contract passed: image=%s\n' "$FORGE_REDIS_IMAGE"
