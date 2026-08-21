#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

command -v aws >/dev/null 2>&1 || { echo "aws CLI is required" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
command -v openssl >/dev/null 2>&1 || { echo "openssl is required" >&2; exit 1; }

: "${FORGE_COS_ACCESS_KEY:?set FORGE_COS_ACCESS_KEY in the local environment}"
: "${FORGE_COS_SECRET_KEY:?set FORGE_COS_SECRET_KEY in the local environment}"
: "${FORGE_COS_BUCKET:?set FORGE_COS_BUCKET to a disposable test bucket}"
: "${FORGE_COS_REGION:?set FORGE_COS_REGION explicitly}"
: "${FORGE_COS_ENDPOINT:?set FORGE_COS_ENDPOINT explicitly}"

# The contract-facing FORGE_COS_* names are the source-of-truth inputs. Map
# them to the AWS CLI names unless a caller deliberately supplied temporary
# AWS credentials (for example, an STS session) for this invocation.
export AWS_EC2_METADATA_DISABLED=true
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-$FORGE_COS_ACCESS_KEY}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-$FORGE_COS_SECRET_KEY}"

REQUIRE_ADVANCED=${FORGE_COS_REQUIRE_ADVANCED:-false}
ALLOW_MUTATING=${FORGE_COS_ALLOW_MUTATING_ADVANCED:-false}
PREFIX=${FORGE_COS_PREFIX:-forge-advanced-$(date -u +%Y%m%dT%H%M%SZ)-$$}
EVIDENCE_FILE=${FORGE_COS_ADVANCED_EVIDENCE_FILE:-}
COMPATIBILITY_EVIDENCE_FILE=${FORGE_S3_COMPATIBILITY_EVIDENCE_FILE:-}
COMPATIBILITY_EVIDENCE_LEVEL=${FORGE_S3_EVIDENCE_LEVEL:-Not certified}
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/forge-cos-advanced.XXXXXX")
CREATED_KEYS=()
FAILURES=()
declare -A STATUS=()

cleanup() {
  versions_file="$TMP_DIR/versions.json"
  delete_file="$TMP_DIR/delete.json"
  if aws_cmd list-object-versions --bucket "$FORGE_COS_BUCKET" --prefix "$PREFIX/" >"$versions_file" 2>/dev/null; then
    python3 - "$versions_file" "$delete_file" <<'PY'
import json
import pathlib
import sys

source, target = sys.argv[1:]
data = json.loads(pathlib.Path(source).read_text())
objects = []
for field in ("Versions", "DeleteMarkers"):
    for item in data.get(field, []):
        objects.append({"Key": item["Key"], "VersionId": item["VersionId"]})
pathlib.Path(target).write_text(json.dumps({"Objects": objects, "Quiet": True}) + "\n")
PY
    object_count=$(python3 - "$delete_file" <<'PY'
import json
import pathlib
import sys

print(len(json.loads(pathlib.Path(sys.argv[1]).read_text()).get("Objects", [])))
PY
)
    if [[ "$object_count" != 0 ]]; then
      aws_cmd delete-objects --bucket "$FORGE_COS_BUCKET" --delete "file://$delete_file" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if [[ -n "$COMPATIBILITY_EVIDENCE_FILE" ]]; then
  : "${FORGE_S3_PROVIDER:?FORGE_S3_PROVIDER is required for compatibility evidence}"
  : "${FORGE_S3_TARGET_PRODUCT:?FORGE_S3_TARGET_PRODUCT is required for compatibility evidence}"
  : "${FORGE_S3_TARGET_VERSION:?FORGE_S3_TARGET_VERSION is required for compatibility evidence}"
  : "${FORGE_S3_TARGET_ARCHITECTURE:?FORGE_S3_TARGET_ARCHITECTURE is required for compatibility evidence}"
  : "${FORGE_S3_TARGET_OS:?FORGE_S3_TARGET_OS is required for compatibility evidence}"
  : "${FORGE_S3_TARGET_RUNTIME:?FORGE_S3_TARGET_RUNTIME is required for compatibility evidence}"
  : "${FORGE_S3_TARGET_DRIVER:?FORGE_S3_TARGET_DRIVER is required for compatibility evidence}"
  : "${EVIDENCE_FILE:?FORGE_COS_ADVANCED_EVIDENCE_FILE is required for compatibility evidence}"
fi

aws_cmd() {
  AWS_PAGER='' aws s3api "$@" --endpoint-url "$FORGE_COS_ENDPOINT" --region "$FORGE_COS_REGION"
}

mark_passed() { STATUS["$1"]=passed; }
mark_not_tested() { STATUS["$1"]=${2:-not-tested}; }
mark_failed() {
  STATUS["$1"]=${2:-failed}
  FAILURES+=("$1")
}

json_field() {
  local field=$1
  python3 -c '
import json
import sys

value = json.load(sys.stdin)
for part in sys.argv[1].split("."):
    if not isinstance(value, dict) or part not in value:
        raise SystemExit(1)
    value = value[part]
if value is None:
    raise SystemExit(1)
print(value)
' "$field"
}

payload="$TMP_DIR/payload.bin"
printf 'forge advanced s3 contract\n' >"$payload"
base_key="$PREFIX/base-object"
if aws_cmd put-object --bucket "$FORGE_COS_BUCKET" --key "$base_key" --body "$payload" >/dev/null 2>"$TMP_DIR/base.err"; then
  CREATED_KEYS+=("$base_key")
  mark_passed basic_object_write
else
  mark_failed basic_object_write
fi

checksum_b64=$(openssl dgst -sha256 -binary "$payload" | base64 | tr -d '\n')
checksum_key="$PREFIX/checksum-object"
if checksum_output=$(aws_cmd put-object --bucket "$FORGE_COS_BUCKET" --key "$checksum_key" --body "$payload" --checksum-algorithm SHA256 --checksum-sha256 "$checksum_b64" 2>"$TMP_DIR/checksum.err"); then
  CREATED_KEYS+=("$checksum_key")
  head_output=$(aws_cmd head-object --bucket "$FORGE_COS_BUCKET" --key "$checksum_key" --checksum-mode ENABLED 2>"$TMP_DIR/checksum-head.err" || true)
  observed_checksum=$(printf '%s' "$head_output" | json_field ChecksumSHA256 2>/dev/null || true)
  if [[ -n "$observed_checksum" && "$observed_checksum" == "$checksum_b64" ]]; then
    mark_passed checksum
  else
    mark_failed checksum
  fi
else
  mark_failed checksum
fi

versioning=$(aws_cmd get-bucket-versioning --bucket "$FORGE_COS_BUCKET" 2>"$TMP_DIR/versioning.err" || true)
if [[ "$(printf '%s' "$versioning" | json_field Status 2>/dev/null || true)" == Enabled ]]; then
  mark_passed versioning
else
  mark_not_tested versioning "not-enabled"
fi

multipart_key="$PREFIX/multipart-object"
part_file="$TMP_DIR/multipart.part"
dd if=/dev/zero of="$part_file" bs=1048576 count=6 >/dev/null 2>&1
upload_json=$(aws_cmd create-multipart-upload --bucket "$FORGE_COS_BUCKET" --key "$multipart_key" --content-type application/octet-stream 2>"$TMP_DIR/multipart-create.err" || true)
upload_id=$(printf '%s' "$upload_json" | json_field UploadId 2>/dev/null || true)
if [[ -n "$upload_id" ]]; then
  part_json=$(aws_cmd upload-part --bucket "$FORGE_COS_BUCKET" --key "$multipart_key" --upload-id "$upload_id" --part-number 1 --body "$part_file" 2>"$TMP_DIR/multipart-part.err" || true)
  etag=$(printf '%s' "$part_json" | json_field ETag 2>/dev/null || true)
  if [[ -n "$etag" ]] && aws_cmd abort-multipart-upload --bucket "$FORGE_COS_BUCKET" --key "$multipart_key" --upload-id "$upload_id" >/dev/null 2>"$TMP_DIR/multipart-abort.err"; then
    mark_passed multipart_recovery
  else
    mark_failed multipart_recovery
    aws_cmd abort-multipart-upload --bucket "$FORGE_COS_BUCKET" --key "$multipart_key" --upload-id "$upload_id" >/dev/null 2>&1 || true
  fi
else
  mark_failed multipart_recovery
fi

if [[ "${STATUS[basic_object_write]:-failed}" == passed ]]; then
  presigned_url=$(aws s3 presign "s3://$FORGE_COS_BUCKET/$base_key" --endpoint-url "$FORGE_COS_ENDPOINT" --region "$FORGE_COS_REGION" --expires-in 900 2>"$TMP_DIR/presign.err" || true)
  if [[ -n "$presigned_url" ]] && curl --fail --silent --show-error --max-time 15 "$presigned_url" -o /dev/null 2>"$TMP_DIR/presign-fetch.err"; then
    mark_passed constrained_presign
  else
    mark_failed constrained_presign
  fi
else
  mark_not_tested constrained_presign "base-object-unavailable"
fi

if [[ -n "${FORGE_COS_KMS_KEY_ID:-}" ]]; then
  kms_key="$PREFIX/sse-kms-object"
  if aws_cmd put-object --bucket "$FORGE_COS_BUCKET" --key "$kms_key" --body "$payload" --server-side-encryption aws:kms --sse-kms-key-id "$FORGE_COS_KMS_KEY_ID" >/dev/null 2>"$TMP_DIR/sse-kms.err"; then
    CREATED_KEYS+=("$kms_key")
    mark_passed sse_kms
  else
    mark_failed sse_kms
  fi
else
  mark_not_tested sse_kms "kms-key-unavailable"
fi

lock_config=$(aws_cmd get-object-lock-configuration --bucket "$FORGE_COS_BUCKET" 2>"$TMP_DIR/object-lock.err" || true)
if [[ "$(printf '%s' "$lock_config" | json_field ObjectLockConfiguration.ObjectLockEnabled 2>/dev/null || true)" == Enabled ]]; then
  mark_passed object_lock_configuration
  if [[ "$ALLOW_MUTATING" == true ]]; then
    hold_key="$PREFIX/legal-hold-object"
    hold_output=$(aws_cmd put-object --bucket "$FORGE_COS_BUCKET" --key "$hold_key" --body "$payload" 2>"$TMP_DIR/legal-hold-put.err" || true)
    hold_version=$(printf '%s' "$hold_output" | json_field VersionId 2>/dev/null || true)
    if [[ -n "$hold_version" ]]; then CREATED_KEYS+=("$hold_key"); fi
    if [[ -n "$hold_version" ]] && aws_cmd put-object-legal-hold --bucket "$FORGE_COS_BUCKET" --key "$hold_key" --version-id "$hold_version" --legal-hold Status=ON >/dev/null 2>"$TMP_DIR/legal-hold-on.err" && aws_cmd get-object-legal-hold --bucket "$FORGE_COS_BUCKET" --key "$hold_key" --version-id "$hold_version" >/dev/null 2>"$TMP_DIR/legal-hold-get.err" && aws_cmd put-object-legal-hold --bucket "$FORGE_COS_BUCKET" --key "$hold_key" --version-id "$hold_version" --legal-hold Status=OFF >/dev/null 2>"$TMP_DIR/legal-hold-off.err"; then
      mark_passed legal_hold
    else
      mark_failed legal_hold
    fi

    retention_key="$PREFIX/retention-object"
    retention_output=$(aws_cmd put-object --bucket "$FORGE_COS_BUCKET" --key "$retention_key" --body "$payload" 2>"$TMP_DIR/retention-put.err" || true)
    retention_version=$(printf '%s' "$retention_output" | json_field VersionId 2>/dev/null || true)
    if [[ -n "$retention_version" ]]; then CREATED_KEYS+=("$retention_key"); fi
    retain_until=$(date -u -v+1H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+1 hour' '+%Y-%m-%dT%H:%M:%SZ')
    if [[ -n "$retention_version" ]] && aws_cmd put-object-retention --bucket "$FORGE_COS_BUCKET" --key "$retention_key" --version-id "$retention_version" --retention "Mode=GOVERNANCE,RetainUntilDate=$retain_until" >/dev/null 2>"$TMP_DIR/retention-set.err"; then
      retention_check=$(aws_cmd get-object-retention --bucket "$FORGE_COS_BUCKET" --key "$retention_key" --version-id "$retention_version" 2>"$TMP_DIR/retention-get.err" || true)
      if [[ "$(printf '%s' "$retention_check" | json_field Retention.Mode 2>/dev/null || true)" == GOVERNANCE ]]; then
        mark_passed retention
      else
        mark_failed retention
      fi
    else
      mark_failed retention
    fi
  else
    mark_not_tested legal_hold "mutating-test-disabled"
    mark_not_tested retention "mutating-test-disabled"
  fi
  mark_passed object_lock
else
  mark_not_tested object_lock "bucket-not-configured"
  mark_not_tested retention "bucket-not-configured"
  mark_not_tested legal_hold "bucket-not-configured"
fi

if [[ -n "${FORGE_COS_TEMP_ACCESS_KEY:-}" && -n "${FORGE_COS_TEMP_SECRET_KEY:-}" && -n "${FORGE_COS_SESSION_TOKEN:-}" ]]; then
  if AWS_ACCESS_KEY_ID="$FORGE_COS_TEMP_ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$FORGE_COS_TEMP_SECRET_KEY" AWS_SESSION_TOKEN="$FORGE_COS_SESSION_TOKEN" aws_cmd head-bucket --bucket "$FORGE_COS_BUCKET" >/dev/null 2>"$TMP_DIR/temporary-credential.err"; then
    mark_passed temporary_credential
  else
    mark_failed temporary_credential
  fi
else
  mark_not_tested temporary_credential "temporary-credentials-unavailable"
fi

for capability in checksum multipart_recovery constrained_presign sse_kms object_lock retention legal_hold temporary_credential; do
  : "${STATUS[$capability]:=not-tested}"
done

printf 'cos advanced contract: '
for capability in checksum multipart_recovery constrained_presign sse_kms object_lock retention legal_hold temporary_credential; do
  printf '%s=%s ' "$capability" "${STATUS[$capability]}"
done
printf '\n'

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  python3 - "$EVIDENCE_FILE" "$FORGE_COS_BUCKET" "$FORGE_COS_REGION" "$FORGE_COS_ENDPOINT" "$(git rev-parse HEAD)" "${STATUS[checksum]}" "${STATUS[multipart_recovery]}" "${STATUS[constrained_presign]}" "${STATUS[sse_kms]}" "${STATUS[object_lock]}" "${STATUS[retention]}" "${STATUS[legal_hold]}" "${STATUS[temporary_credential]}" <<'PY'
import json
import pathlib
import sys
from datetime import datetime, timezone

path, bucket, region, endpoint, commit, checksum, multipart, presign, sse_kms, object_lock, retention, legal_hold, temporary = sys.argv[1:]
payload = {
    "kind": "s3-advanced-contract",
    "status": "failed" if any(value == "failed" for value in (checksum, multipart, presign, sse_kms, object_lock, retention, legal_hold, temporary)) else "observed",
    "target": {"provider": "s3-compatible", "bucket": bucket, "region": region, "endpoint": endpoint},
    "source_commit": commit,
    "checked_at": datetime.now(timezone.utc).isoformat(),
    "capabilities": {"checksum": checksum, "multipart_recovery": multipart, "constrained_presign": presign, "sse_kms": sse_kms, "object_lock": object_lock, "retention": retention, "legal_hold": legal_hold, "temporary_credential": temporary},
}
pathlib.Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
fi

if [[ -n "$COMPATIBILITY_EVIDENCE_FILE" ]]; then
  compatibility_ref=${FORGE_S3_EVIDENCE_REF:-$(basename "$EVIDENCE_FILE")}
  compatibility_digest=$(shasum -a 256 "$EVIDENCE_FILE" | awk '{print $1}')
  mkdir -p "$(dirname "$COMPATIBILITY_EVIDENCE_FILE")"
  python3 - "$COMPATIBILITY_EVIDENCE_FILE" "$FORGE_S3_PROVIDER" "$COMPATIBILITY_EVIDENCE_LEVEL" "$FORGE_S3_TARGET_PRODUCT" "$FORGE_S3_TARGET_VERSION" "$FORGE_S3_TARGET_ARCHITECTURE" "$FORGE_S3_TARGET_OS" "$FORGE_S3_TARGET_RUNTIME" "$FORGE_S3_TARGET_DRIVER" "$FORGE_COS_ENDPOINT" "$FORGE_COS_REGION" "$FORGE_COS_BUCKET" "$compatibility_ref" "$compatibility_digest" "${STATUS[basic_object_write]:-not-tested}" "${STATUS[checksum]:-not-tested}" "${STATUS[multipart_recovery]:-not-tested}" "${STATUS[constrained_presign]:-not-tested}" "${STATUS[sse_kms]:-not-tested}" "${STATUS[object_lock]:-not-tested}" "${STATUS[retention]:-not-tested}" "${STATUS[legal_hold]:-not-tested}" "${STATUS[temporary_credential]:-not-tested}" <<'PY'
import json
import pathlib
import sys
from datetime import datetime, timezone

(
    path,
    provider,
    level,
    product,
    version,
    architecture,
    operating_system,
    runtime,
    driver,
    endpoint,
    region,
    bucket,
    evidence_ref,
    evidence_digest,
    basic,
    checksum,
    multipart,
    presign,
    sse_kms,
    object_lock,
    retention,
    legal_hold,
    temporary,
) = sys.argv[1:]

def state(value):
    if value == "passed":
        return "passed"
    if value == "failed":
        return "failed"
    return "not_tested"

raw = {
    "basic_object_io": basic,
    "checksum": checksum,
    "multipart_recovery": multipart,
    "constrained_presign": presign,
    "sse_kms": sse_kms,
    "object_lock": object_lock,
    "retention": retention,
    "legal_hold": legal_hold,
    "temporary_credential": temporary,
}
capabilities = {}
claims = []
for name, value in raw.items():
    capability_state = state(value)
    capabilities[name] = {"state": capability_state}
    if capability_state == "passed":
        capabilities[name]["evidence_ref"] = evidence_ref
        claims.append(name)

document = {
    "kind": "forge-s3-compatibility-evidence",
    "level": level,
    "provider": provider,
    "target": {
        "product": product,
        "version": version,
        "architecture": architecture,
        "os": operating_system,
        "runtime": runtime,
        "driver": driver,
        "endpoint": endpoint,
        "region": region,
        "bucket": bucket,
    },
    "tested_at": datetime.now(timezone.utc).isoformat(),
    "tested_capabilities": claims,
    "capabilities": capabilities,
    "evidence": {evidence_ref: evidence_digest},
}
pathlib.Path(path).write_text(json.dumps(document, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
fi

if [[ "$REQUIRE_ADVANCED" == true ]]; then
  for capability in checksum multipart_recovery constrained_presign sse_kms object_lock retention legal_hold temporary_credential; do
    [[ "${STATUS[$capability]}" == passed ]] || FAILURES+=("$capability")
  done
fi

if ((${#FAILURES[@]} > 0)); then
  printf 'cos advanced contract failed capabilities: %s\n' "${FAILURES[*]}" >&2
  exit 1
fi
