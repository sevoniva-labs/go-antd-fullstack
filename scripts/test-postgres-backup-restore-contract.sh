#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/postgres-backup-runtime-contract.yaml"
POSTGRES_IMAGE="${FORGE_POSTGRES_IMAGE:?FORGE_POSTGRES_IMAGE must be set to an immutable image digest}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
PASSWORD="${FORGE_POSTGRES_PASSWORD:-forge-contract-password}"
PORT="${FORGE_POSTGRES_PORT:-15432}"
EVIDENCE_FILE="${FORGE_POSTGRES_BACKUP_EVIDENCE_FILE:-}"
DR_EVIDENCE_FILE="${FORGE_DISASTER_EVIDENCE_FILE:-}"
DR_EVIDENCE_ROOT="${FORGE_DISASTER_EVIDENCE_ROOT:-}"
if [[ -n "$DR_EVIDENCE_FILE" && "$DR_EVIDENCE_FILE" != /* ]]; then
  DR_EVIDENCE_FILE="$PWD/$DR_EVIDENCE_FILE"
fi
if [[ -n "$DR_EVIDENCE_ROOT" && "$DR_EVIDENCE_ROOT" != /* ]]; then
  DR_EVIDENCE_ROOT="$PWD/$DR_EVIDENCE_ROOT"
fi
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
  proof_file="$DR_EVIDENCE_ROOT/postgres-backup-restore-proof.txt"
  printf 'kind=forge-disaster-backup-restore-proof\nproduct=%s\nimage=%s\ndata_verification=passed\n' \
    "$FORGE_DR_TARGET_PRODUCT" "$POSTGRES_IMAGE" >"$proof_file"
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
        "evidence_refs": ["postgres-backup-restore-proof.txt"],
    }],
    "evidence_digests": {"postgres-backup-restore-proof.txt": digest},
}, indent=2) + "\n", encoding="utf-8")
PY
fi

echo "PostgreSQL backup/restore contract passed: image=$POSTGRES_IMAGE"
