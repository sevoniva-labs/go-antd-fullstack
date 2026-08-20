#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/postgres-backup-runtime-contract.yaml"
POSTGRES_IMAGE="${FORGE_POSTGRES_IMAGE:?FORGE_POSTGRES_IMAGE must be set to an immutable image digest}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
PASSWORD="${FORGE_POSTGRES_PASSWORD:-forge-contract-password}"
PORT="${FORGE_POSTGRES_PORT:-15432}"
EVIDENCE_FILE="${FORGE_POSTGRES_BACKUP_EVIDENCE_FILE:-}"
mkdir -p "$ROOT/.evidence"
DUMP_FILE="$(mktemp "$ROOT/.evidence/postgres-backup-contract.XXXXXX")"

if [[ ! "$POSTGRES_IMAGE" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "FORGE_POSTGRES_IMAGE must use an immutable @sha256 digest" >&2
  exit 1
fi

compose() {
  # shellcheck disable=SC2086
  COMPOSE_PROJECT_NAME=forge-postgres-backup-contract POSTGRES_IMAGE="$POSTGRES_IMAGE" \
    FORGE_POSTGRES_PASSWORD="$PASSWORD" FORGE_POSTGRES_DUMP_FILE="$DUMP_FILE" \
    FORGE_POSTGRES_PORT="$PORT" sh -c "$COMPOSE_CMD \"\$@\"" -- "$@"
}

cleanup() {
  compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$DUMP_FILE"
}
trap cleanup EXIT

compose -f "$COMPOSE_FILE" up -d
for _ in $(seq 1 45); do
  if compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U forge -d postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U forge -d postgres >/dev/null

compose -f "$COMPOSE_FILE" exec -T postgres psql -v ON_ERROR_STOP=1 -U forge -d postgres <<'SQL'
DROP DATABASE IF EXISTS forge_contract;
CREATE DATABASE forge_contract;
SQL
compose -f "$COMPOSE_FILE" exec -T postgres psql -v ON_ERROR_STOP=1 -U forge -d forge_contract <<'SQL'
CREATE TABLE backup_contract (id integer PRIMARY KEY, payload text NOT NULL);
INSERT INTO backup_contract (id, payload) VALUES (1, 'backup-restore-contract');
SQL

compose -f "$COMPOSE_FILE" exec -T postgres pg_dump -Fc -U forge -d forge_contract >"$DUMP_FILE"
[[ -s "$DUMP_FILE" ]] || { echo "pg_dump produced an empty backup" >&2; exit 1; }

compose -f "$COMPOSE_FILE" exec -T postgres psql -v ON_ERROR_STOP=1 -U forge -d postgres <<'SQL'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'forge_contract' AND pid <> pg_backend_pid();
DROP DATABASE forge_contract;
CREATE DATABASE forge_contract;
SQL
compose -f "$COMPOSE_FILE" exec -T postgres pg_restore --clean --if-exists --no-owner -U forge -d forge_contract /tmp/forge-contract.dump

PAYLOAD="$(compose -f "$COMPOSE_FILE" exec -T postgres psql -At -U forge -d forge_contract -c 'SELECT payload FROM backup_contract WHERE id = 1;' | tr -d '\r')"
if [[ "$PAYLOAD" != "backup-restore-contract" ]]; then
  echo "backup restore data verification failed: $PAYLOAD" >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  cat >"$EVIDENCE_FILE" <<EOF
{
  "kind": "postgres-backup-restore-contract",
  "image": "${POSTGRES_IMAGE}",
  "source_commit": "$(git rev-parse HEAD)",
  "backup_format": "custom",
  "backup_restore_status": "passed",
  "data_verification": "passed"
}
EOF
fi

echo "PostgreSQL backup/restore contract passed: image=$POSTGRES_IMAGE"
