#!/usr/bin/env bash
set -euo pipefail

: "${FORGE_COS_ACCESS_KEY:?FORGE_COS_ACCESS_KEY is required}"
: "${FORGE_COS_SECRET_KEY:?FORGE_COS_SECRET_KEY is required}"
: "${FORGE_COS_BUCKET:?FORGE_COS_BUCKET is required}"

region="${FORGE_COS_REGION:-ap-shanghai}"
endpoint="${FORGE_COS_ENDPOINT:-https://cos.${region}.myqcloud.com}"
evidence_file="${FORGE_COS_EVIDENCE_FILE:-}"
contract_file="${FORGE_COS_CONTRACT_FILE:-}"
tested_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
probe_key="_forge-contract/$(date -u +%Y%m%dT%H%M%SZ)-$$/probe.txt"
work_dir="$(mktemp -d)"
uploaded=0

cleanup() {
    if [[ "$uploaded" == "1" ]]; then
        aws s3api delete-object \
            --bucket "$FORGE_COS_BUCKET" \
            --key "$probe_key" \
            --endpoint-url "$endpoint" \
            >/dev/null 2>&1 || true
    fi
    rm -rf "$work_dir"
}
trap cleanup EXIT

if ! command -v aws >/dev/null 2>&1; then
    echo "aws CLI is required" >&2
    exit 1
fi
if [[ -n "$contract_file" && -z "$evidence_file" ]]; then
    echo "FORGE_COS_EVIDENCE_FILE is required when FORGE_COS_CONTRACT_FILE is set" >&2
    exit 1
fi

cat >"$work_dir/aws-config" <<'CONFIG'
[default]
region = ap-shanghai
s3 =
    addressing_style = virtual
CONFIG

export AWS_CONFIG_FILE="$work_dir/aws-config"
export AWS_ACCESS_KEY_ID="$FORGE_COS_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$FORGE_COS_SECRET_KEY"
export AWS_DEFAULT_REGION="$region"
export AWS_EC2_METADATA_DISABLED=true

result="passed"
head_bucket="failed"
list_objects="failed"
put_sse_s3="failed"
head_object="skipped"
get_object="skipped"
content_match="skipped"
get_versioning="failed"
list_versions="failed"
delete_object="skipped"

if aws s3api head-bucket --bucket "$FORGE_COS_BUCKET" --endpoint-url "$endpoint" >/dev/null 2>"$work_dir/error"; then
    head_bucket="passed"
else
    result="failed"
fi

if aws s3api list-objects-v2 --bucket "$FORGE_COS_BUCKET" --max-keys 1 --endpoint-url "$endpoint" >/dev/null 2>"$work_dir/error"; then
    list_objects="passed"
else
    result="failed"
fi

printf 'forge-cos-contract-probe' >"$work_dir/payload"
if aws s3 cp - "s3://$FORGE_COS_BUCKET/$probe_key" \
    --sse AES256 \
    --endpoint-url "$endpoint" \
    --only-show-errors \
    <"$work_dir/payload" >/dev/null 2>"$work_dir/error"; then
    put_sse_s3="passed"
    uploaded=1
else
    result="failed"
fi

if [[ "$uploaded" == "1" ]]; then
    if aws s3api head-object --bucket "$FORGE_COS_BUCKET" --key "$probe_key" --endpoint-url "$endpoint" >/dev/null 2>"$work_dir/error"; then
        head_object="passed"
    else
        head_object="failed"
        result="failed"
    fi

    if aws s3api get-object --bucket "$FORGE_COS_BUCKET" --key "$probe_key" "$work_dir/download" --endpoint-url "$endpoint" >/dev/null 2>"$work_dir/error"; then
        get_object="passed"
        if cmp -s "$work_dir/payload" "$work_dir/download"; then
            content_match="passed"
        else
            content_match="failed"
            result="failed"
        fi
    else
        get_object="failed"
        content_match="failed"
        result="failed"
    fi
fi

if aws s3api get-bucket-versioning --bucket "$FORGE_COS_BUCKET" --endpoint-url "$endpoint" >/dev/null 2>"$work_dir/error"; then
    get_versioning="passed"
else
    result="failed"
fi

if aws s3api list-object-versions --bucket "$FORGE_COS_BUCKET" --max-keys 1 --endpoint-url "$endpoint" >/dev/null 2>"$work_dir/error"; then
    list_versions="passed"
else
    result="failed"
fi

if [[ "$uploaded" == "1" ]]; then
    if aws s3api delete-object --bucket "$FORGE_COS_BUCKET" --key "$probe_key" --endpoint-url "$endpoint" >/dev/null 2>"$work_dir/error"; then
        delete_object="passed"
        uploaded=0
    else
        delete_object="failed"
        result="failed"
    fi
fi

if [[ -n "$evidence_file" ]]; then
    mkdir -p "$(dirname "$evidence_file")"
    python3 - "$evidence_file" "$tested_at" "$region" "$endpoint" "$FORGE_COS_BUCKET" "$head_bucket" "$list_objects" "$put_sse_s3" "$head_object" "$get_object" "$content_match" "$get_versioning" "$list_versions" "$delete_object" <<'PY'
import json
import pathlib
import sys

(
    output, tested_at, region, endpoint, bucket, head_bucket, list_objects,
    put_sse_s3, head_object, get_object, content_match, get_versioning,
    list_versions, delete_object,
) = sys.argv[1:]

pathlib.Path(output).write_text(json.dumps({
    "schema": "forge.storage.contract.v1",
    "provider": "tencent-cos",
    "target": {"region": region, "endpoint": endpoint, "bucket": bucket},
    "tested_at": tested_at,
    "suite": "scripts/test-cos-contract.sh",
    "capabilities": {
        "basic_object_io": {"state": "supported", "operations": {
            "head_bucket": head_bucket, "list_objects": list_objects,
            "put": put_sse_s3, "head": head_object, "get": get_object,
            "content_match": content_match, "delete": delete_object,
        }},
        "sse_s3": {"state": "supported", "algorithm": "AES256", "operation": put_sse_s3},
        "versioning_read": {"state": "observed", "get": get_versioning, "list": list_versions},
        "multipart_recovery": {"state": "not-tested"},
        "checksum": {"state": "not-tested"},
        "sse_kms": {"state": "not-tested"},
        "object_lock": {"state": "not-tested"},
        "retention": {"state": "not-tested"},
        "legal_hold": {"state": "not-tested"},
        "constrained_presign": {"state": "not-tested"},
        "temporary_credentials": {"state": "not-tested"},
    },
    "security": {
        "credentials_source": "environment",
        "probe_object_cleanup": delete_object,
        "secret_values_recorded": False,
    },
}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
    if command -v sha256sum >/dev/null 2>&1; then
        digest="$(sha256sum "$evidence_file" | awk '{print $1}')"
    else
        digest="$(shasum -a 256 "$evidence_file" | awk '{print $1}')"
    fi
    printf 'evidence_file=%s\nevidence_sha256=%s\n' "$evidence_file" "$digest"

    if [[ -n "$contract_file" && "$result" == "passed" ]]; then
        mkdir -p "$(dirname "$contract_file")"
        python3 - "$contract_file" "$tested_at" "$FORGE_COS_BUCKET" "$evidence_file" "$digest" <<'PY'
import json
import pathlib
import sys

output, tested_at, bucket, evidence_ref, evidence_digest = sys.argv[1:]
pathlib.Path(output).write_text(json.dumps({
    "profile": "tencent-cos",
    "level": "Target-tested",
    "target": f"tencent-cos/ap-shanghai/{bucket}",
    "evidence_ref": evidence_ref,
    "evidence_digest": evidence_digest,
    "tested_at": tested_at,
    "capabilities": {
        "basic_object_io": {"state": "supported", "evidence": evidence_ref},
        "sse_s3": {"state": "supported", "evidence": evidence_ref},
    },
}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
        printf 'contract_file=%s\n' "$contract_file"
    fi
fi

printf 'provider=tencent-cos region=%s bucket=%s result=%s\n' "$region" "$FORGE_COS_BUCKET" "$result"
printf 'head_bucket=%s list_objects=%s put_sse_s3=%s head_object=%s get_object=%s content_match=%s delete_object=%s\n' \
    "$head_bucket" "$list_objects" "$put_sse_s3" "$head_object" "$get_object" "$content_match" "$delete_object"
printf 'get_versioning=%s list_versions=%s advanced_capabilities=not-tested\n' "$get_versioning" "$list_versions"

[[ "$result" == "passed" ]]
