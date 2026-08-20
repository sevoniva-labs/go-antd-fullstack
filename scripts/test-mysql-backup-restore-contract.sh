#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/mysql-runtime-contract.yaml"
MYSQL_IMAGE="${FORGE_MYSQL_IMAGE:?FORGE_MYSQL_IMAGE must be set to an immutable image digest}"
MYSQL_ROOT_PASSWORD="${FORGE_MYSQL_ROOT_PASSWORD:?FORGE_MYSQL_ROOT_PASSWORD must be set for the disposable contract}"
MYSQL_PASSWORD="${FORGE_MYSQL_PASSWORD:?FORGE_MYSQL_PASSWORD must be set for the disposable contract}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
MYSQL_PORT="${FORGE_MYSQL_BACKUP_PORT:-13307}"
EVIDENCE_FILE="${FORGE_MYSQL_BACKUP_EVIDENCE_FILE:-}"
if [[ -n "$EVIDENCE_FILE" && "$EVIDENCE_FILE" != /* ]]; then
  EVIDENCE_FILE="$PWD/$EVIDENCE_FILE"
fi

if [[ ! "$MYSQL_IMAGE" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "FORGE_MYSQL_IMAGE must use an immutable @sha256 digest" >&2
  exit 1
fi

mkdir -p "$ROOT/.evidence"
DUMP_FILE="$(mktemp "$ROOT/.evidence/mysql-backup-contract.XXXXXX")"

compose() {
  # shellcheck disable=SC2086
  MYSQL_IMAGE="$MYSQL_IMAGE" MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
    MYSQL_PASSWORD="$MYSQL_PASSWORD" MYSQL_CONTRACT_PORT="$MYSQL_PORT" \
    COMPOSE_PROJECT_NAME=forge-mysql-backup-contract \
    sh -c "$COMPOSE_CMD \"\$@\"" -- "$@"
}

cleanup() {
  compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$DUMP_FILE"
}
trap cleanup EXIT

cd "$ROOT"
compose -f "$COMPOSE_FILE" up -d
for _ in $(seq 1 45); do
  if compose -f "$COMPOSE_FILE" exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot \
    "-p$MYSQL_ROOT_PASSWORD" --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
compose -f "$COMPOSE_FILE" exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot \
  "-p$MYSQL_ROOT_PASSWORD" --silent >/dev/null

compose -f "$COMPOSE_FILE" exec -T mysql mysql -uroot "-p$MYSQL_ROOT_PASSWORD" -Nse \
  "DROP DATABASE IF EXISTS forge_contract; CREATE DATABASE forge_contract;"
compose -f "$COMPOSE_FILE" exec -T mysql mysql -uroot "-p$MYSQL_ROOT_PASSWORD" -D forge_contract <<'SQL'
CREATE TABLE backup_contract (id INT PRIMARY KEY, payload VARCHAR(255) NOT NULL);
INSERT INTO backup_contract (id, payload) VALUES (1, 'mysql-backup-restore-contract');
SQL

compose -f "$COMPOSE_FILE" exec -T mysql mysqldump -uroot "-p$MYSQL_ROOT_PASSWORD" \
  --single-transaction --routines --triggers forge_contract >"$DUMP_FILE"
[[ -s "$DUMP_FILE" ]] || { echo "mysqldump produced an empty backup" >&2; exit 1; }

compose -f "$COMPOSE_FILE" exec -T mysql mysql -uroot "-p$MYSQL_ROOT_PASSWORD" -Nse \
  "DROP DATABASE forge_contract; CREATE DATABASE forge_contract;"
compose -f "$COMPOSE_FILE" exec -T mysql mysql -uroot "-p$MYSQL_ROOT_PASSWORD" \
  forge_contract <"$DUMP_FILE"

PAYLOAD="$(compose -f "$COMPOSE_FILE" exec -T mysql mysql -uroot "-p$MYSQL_ROOT_PASSWORD" \
  -D forge_contract -Nse 'SELECT payload FROM backup_contract WHERE id = 1;' | tr -d '\r')"
if [[ "$PAYLOAD" != "mysql-backup-restore-contract" ]]; then
  echo "MySQL backup restore data verification failed: $PAYLOAD" >&2
  exit 1
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  cat >"$EVIDENCE_FILE" <<EOF
{
  "kind": "mysql-backup-restore-contract",
  "image": "${MYSQL_IMAGE}",
  "source_commit": "$(git rev-parse HEAD)",
  "backup_format": "mysqldump-single-transaction",
  "backup_restore_status": "passed",
  "data_verification": "passed"
}
EOF
fi

echo "MySQL backup/restore contract passed: image=$MYSQL_IMAGE"
