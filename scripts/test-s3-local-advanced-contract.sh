#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/compose/s3-advanced-runtime-contract.yaml"
MINIO_IMAGE="${FORGE_MINIO_IMAGE:?FORGE_MINIO_IMAGE must be set to an immutable image digest}"
ACCESS_KEY="${FORGE_S3_LOCAL_ACCESS_KEY:?FORGE_S3_LOCAL_ACCESS_KEY must be set for the disposable contract}"
SECRET_KEY="${FORGE_S3_LOCAL_SECRET_KEY:?FORGE_S3_LOCAL_SECRET_KEY must be set for the disposable contract}"
COMPOSE_CMD="${FORGE_COMPOSE_CMD:-docker compose}"
PORT="${FORGE_S3_LOCAL_PORT:-19000}"
REGION="${FORGE_S3_LOCAL_REGION:-us-east-1}"
BUCKET="${FORGE_S3_LOCAL_BUCKET:-forge-s3-advanced-$(date +%s)-$$}"
EVIDENCE_FILE="${FORGE_S3_LOCAL_EVIDENCE_FILE:-}"
COMPATIBILITY_EVIDENCE_FILE="${FORGE_S3_LOCAL_COMPATIBILITY_EVIDENCE_FILE:-}"
ENDPOINT="http://127.0.0.1:${PORT}"

if [[ ! "$MINIO_IMAGE" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "FORGE_MINIO_IMAGE must use an immutable @sha256 digest" >&2
  exit 1
fi
if [[ -n "$COMPATIBILITY_EVIDENCE_FILE" && -z "$EVIDENCE_FILE" ]]; then
  echo "FORGE_S3_LOCAL_EVIDENCE_FILE is required for compatibility evidence" >&2
  exit 1
fi

compose() {
  # shellcheck disable=SC2086
  COMPOSE_PROJECT_NAME=forge-s3-advanced-contract MINIO_IMAGE="$MINIO_IMAGE" \
    MINIO_ROOT_USER="$ACCESS_KEY" MINIO_ROOT_PASSWORD="$SECRET_KEY" \
    S3_CONTRACT_PORT="$PORT" sh -c "$COMPOSE_CMD \"\$@\"" -- "$@"
}

cleanup() {
  compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

compose -f "$COMPOSE_FILE" up -d
for _ in $(seq 1 40); do
  if curl --fail --silent --show-error "$ENDPOINT/minio/health/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --fail --silent --show-error "$ENDPOINT/minio/health/ready" >/dev/null

AWS_PAGER='' AWS_EC2_METADATA_DISABLED=true AWS_ACCESS_KEY_ID="$ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$SECRET_KEY" \
  aws s3api create-bucket --bucket "$BUCKET" --endpoint-url "$ENDPOINT" --region "$REGION" \
  --object-lock-enabled-for-bucket >/dev/null
AWS_PAGER='' AWS_EC2_METADATA_DISABLED=true AWS_ACCESS_KEY_ID="$ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$SECRET_KEY" \
  aws s3api put-bucket-versioning --bucket "$BUCKET" --endpoint-url "$ENDPOINT" --region "$REGION" \
  --versioning-configuration Status=Enabled >/dev/null

FORGE_COS_ACCESS_KEY="$ACCESS_KEY" \
FORGE_COS_SECRET_KEY="$SECRET_KEY" \
FORGE_COS_BUCKET="$BUCKET" \
FORGE_COS_REGION="$REGION" \
FORGE_COS_ENDPOINT="$ENDPOINT" \
FORGE_COS_ALLOW_MUTATING_ADVANCED=true \
FORGE_COS_ADVANCED_EVIDENCE_FILE="$EVIDENCE_FILE" \
FORGE_S3_COMPATIBILITY_EVIDENCE_FILE="$COMPATIBILITY_EVIDENCE_FILE" \
FORGE_S3_EVIDENCE_LEVEL="${FORGE_S3_LOCAL_EVIDENCE_LEVEL:-Target-tested}" \
FORGE_S3_PROVIDER="${FORGE_S3_LOCAL_PROVIDER:-generic-s3}" \
FORGE_S3_TARGET_PRODUCT="${FORGE_S3_LOCAL_TARGET_PRODUCT:-MinIO}" \
FORGE_S3_TARGET_VERSION="${FORGE_S3_LOCAL_TARGET_VERSION:-$MINIO_IMAGE}" \
FORGE_S3_TARGET_ARCHITECTURE="${FORGE_S3_LOCAL_TARGET_ARCHITECTURE:-$(uname -m)}" \
FORGE_S3_TARGET_OS="${FORGE_S3_LOCAL_TARGET_OS:-linux}" \
FORGE_S3_TARGET_RUNTIME="${FORGE_S3_LOCAL_TARGET_RUNTIME:-docker-compose}" \
FORGE_S3_TARGET_DRIVER="${FORGE_S3_LOCAL_TARGET_DRIVER:-aws-cli-s3api}" \
FORGE_S3_EVIDENCE_REF="${FORGE_S3_LOCAL_EVIDENCE_REF:-$(basename "$EVIDENCE_FILE")}" \
  bash "$ROOT/scripts/test-cos-advanced-contract.sh"

echo "Local S3 advanced runtime contract passed: image=$MINIO_IMAGE"
