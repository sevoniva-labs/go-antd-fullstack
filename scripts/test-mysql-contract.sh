#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/mysql-runtime-contract.yaml"
MYSQL_IMAGE="${FORGE_MYSQL_IMAGE:?FORGE_MYSQL_IMAGE must be set to an immutable image digest}"
MYSQL_ROOT_PASSWORD="${FORGE_MYSQL_ROOT_PASSWORD:?FORGE_MYSQL_ROOT_PASSWORD must be set for the disposable contract}"
MYSQL_PASSWORD="${FORGE_MYSQL_PASSWORD:?FORGE_MYSQL_PASSWORD must be set for the disposable contract}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
MYSQL_PORT="${FORGE_MYSQL_PORT:-13306}"
EVIDENCE_FILE="${FORGE_MYSQL_EVIDENCE_FILE:-}"

if [[ ! "$MYSQL_IMAGE" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "FORGE_MYSQL_IMAGE must use an immutable @sha256 digest" >&2
  exit 1
fi

compose() {
  # shellcheck disable=SC2086
  MYSQL_IMAGE="$MYSQL_IMAGE" MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
    MYSQL_PASSWORD="$MYSQL_PASSWORD" MYSQL_CONTRACT_PORT="$MYSQL_PORT" \
    sh -c "$COMPOSE_CMD \"\$@\"" -- "$@"
}

cleanup() {
  compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

cd "$ROOT"
compose -f "$COMPOSE_FILE" up -d

for _ in $(seq 1 40); do
  if compose -f "$COMPOSE_FILE" exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot \
    "-p$MYSQL_ROOT_PASSWORD" --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
compose -f "$COMPOSE_FILE" exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot \
  "-p$MYSQL_ROOT_PASSWORD" --silent >/dev/null

GOPROXY=https://goproxy.cn GOSUMDB='sum.golang.org https://goproxy.cn/sumdb/sum.golang.org' \
  FORGE_DATABASE_PROVIDER=mysql \
  FORGE_DATABASE_DSN="forge:${MYSQL_PASSWORD}@tcp(127.0.0.1:${MYSQL_PORT})/forge?parseTime=true&charset=utf8mb4" \
  go run ./cmd/migrate

SCHEMA_RESULT="$(compose -f "$COMPOSE_FILE" exec -T mysql mysql -uforge \
  "-p$MYSQL_PASSWORD" -D forge -Nse \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'forge' AND table_name IN ('users', 'organizations', 'audit_logs');")"
if [[ "$(tr -d '[:space:]' <<<"$SCHEMA_RESULT")" != "3" ]]; then
  echo "MySQL migration contract missing required tables: $SCHEMA_RESULT" >&2
  exit 1
fi

UPDATED_AT_RESULT="$(compose -f "$COMPOSE_FILE" exec -T mysql mysql -uforge \
  "-p$MYSQL_PASSWORD" -D forge -Nse \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'forge' AND table_name = 'organizations' AND column_name = 'updated_at';")"
if [[ "$(tr -d '[:space:]' <<<"$UPDATED_AT_RESULT")" != "1" ]]; then
  echo "MySQL schema contract missing organizations.updated_at" >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  cat >"$EVIDENCE_FILE" <<EOF
{
  "image": "${MYSQL_IMAGE}",
  "database_provider": "mysql",
  "migration_command": "go run ./cmd/migrate",
  "required_tables": 3,
  "organizations_updated_at": true
}
EOF
fi

echo "MySQL runtime contract passed: image=$MYSQL_IMAGE"
