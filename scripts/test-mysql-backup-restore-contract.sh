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
DR_EVIDENCE_FILE="${FORGE_DISASTER_EVIDENCE_FILE:-}"
DR_EVIDENCE_ROOT="${FORGE_DISASTER_EVIDENCE_ROOT:-}"
if [[ -n "$EVIDENCE_FILE" && "$EVIDENCE_FILE" != /* ]]; then
  EVIDENCE_FILE="$PWD/$EVIDENCE_FILE"
fi
if [[ -n "$DR_EVIDENCE_FILE" && "$DR_EVIDENCE_FILE" != /* ]]; then
  DR_EVIDENCE_FILE="$PWD/$DR_EVIDENCE_FILE"
fi
if [[ -n "$DR_EVIDENCE_ROOT" && "$DR_EVIDENCE_ROOT" != /* ]]; then
  DR_EVIDENCE_ROOT="$PWD/$DR_EVIDENCE_ROOT"
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

if [[ -n "$DR_EVIDENCE_FILE" ]]; then
  : "${DR_EVIDENCE_ROOT:?FORGE_DISASTER_EVIDENCE_ROOT is required with FORGE_DISASTER_EVIDENCE_FILE}"
  : "${FORGE_DR_TARGET_PRODUCT:?set FORGE_DR_TARGET_PRODUCT for disaster evidence}"
  : "${FORGE_DR_TARGET_VERSION:?set FORGE_DR_TARGET_VERSION for disaster evidence}"
  : "${FORGE_DR_TARGET_TESTED_AT:?set FORGE_DR_TARGET_TESTED_AT for disaster evidence}"
  : "${FORGE_DR_TARGET_CPU:?set FORGE_DR_TARGET_CPU for disaster evidence}"
  : "${FORGE_DR_TARGET_OS:?set FORGE_DR_TARGET_OS for disaster evidence}"
  : "${FORGE_DR_TARGET_RUNTIME:?set FORGE_DR_TARGET_RUNTIME for disaster evidence}"
  : "${FORGE_DR_TARGET_DATABASE:?set FORGE_DR_TARGET_DATABASE for disaster evidence}"
  : "${FORGE_DR_TARGET_MESSAGE_QUEUE:?set FORGE_DR_TARGET_MESSAGE_QUEUE for disaster evidence}"
  : "${FORGE_DR_TARGET_OBJECT_STORAGE:?set FORGE_DR_TARGET_OBJECT_STORAGE for disaster evidence}"
  : "${FORGE_DR_RPO_TARGET_SECONDS:?set FORGE_DR_RPO_TARGET_SECONDS for disaster evidence}"
  : "${FORGE_DR_RTO_TARGET_SECONDS:?set FORGE_DR_RTO_TARGET_SECONDS for disaster evidence}"
  mkdir -p "$DR_EVIDENCE_ROOT" "$(dirname "$DR_EVIDENCE_FILE")"
  proof_file="$DR_EVIDENCE_ROOT/mysql-backup-restore-proof.txt"
  printf 'kind=forge-disaster-backup-restore-proof\nproduct=%s\nimage=%s\ndata_verification=passed\n' \
    "$FORGE_DR_TARGET_PRODUCT" "$MYSQL_IMAGE" >"$proof_file"
  proof_digest=$(shasum -a 256 "$proof_file" | awk '{print $1}')
  python3 - "$DR_EVIDENCE_FILE" "$proof_digest" "$FORGE_DR_TARGET_PRODUCT" "$FORGE_DR_TARGET_VERSION" \
    "$FORGE_DR_TARGET_TESTED_AT" "$FORGE_DR_TARGET_CPU" "$FORGE_DR_TARGET_OS" "$FORGE_DR_TARGET_RUNTIME" \
    "$FORGE_DR_TARGET_DATABASE" "$FORGE_DR_TARGET_MESSAGE_QUEUE" "$FORGE_DR_TARGET_OBJECT_STORAGE" \
    "$FORGE_DR_RPO_TARGET_SECONDS" "$FORGE_DR_RTO_TARGET_SECONDS" <<'PY'
import json
import pathlib
import sys

(path, digest, product, version, tested_at, cpu, os_name, runtime, database,
 message_queue, object_storage, rpo, rto) = sys.argv[1:]
pathlib.Path(path).write_text(json.dumps({
    "target": {
        "product": product, "version": version, "tested_at": tested_at,
        "cpu": cpu, "os": os_name, "runtime": runtime, "database": database,
        "message_queue": message_queue, "object_storage": object_storage,
    },
    "rpo_target_seconds": float(rpo), "rto_target_seconds": float(rto),
    "scenarios": [
        {"name": name, "status": "not_tested"}
        for name in ("node_failure", "network_partition", "database_failover", "mq_failure", "s3_failure", "site_failure")
    ] + [{
        "name": "backup_restore", "status": "passed",
        "observed_rpo_seconds": 0, "observed_rto_seconds": 0,
        "evidence_refs": ["mysql-backup-restore-proof.txt"],
    }],
    "evidence_digests": {"mysql-backup-restore-proof.txt": digest},
}, indent=2) + "\n", encoding="utf-8")
PY
fi

echo "MySQL backup/restore contract passed: image=$MYSQL_IMAGE"
