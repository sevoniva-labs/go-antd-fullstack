#!/usr/bin/env bash
set -euo pipefail

: "${FORGE_S3_PROFILE:?FORGE_S3_PROFILE is required}"
: "${FORGE_S3_ACCESS_KEY:?FORGE_S3_ACCESS_KEY is required}"
: "${FORGE_S3_SECRET_KEY:?FORGE_S3_SECRET_KEY is required}"
: "${FORGE_S3_BUCKET:?FORGE_S3_BUCKET is required}"

profile="${FORGE_S3_PROFILE,,}"
region="${FORGE_S3_REGION:-us-east-1}"
endpoint="${FORGE_S3_ENDPOINT:-}"
evidence_file="${FORGE_S3_EVIDENCE_FILE:-}"
contract_file="${FORGE_S3_CONTRACT_FILE:-}"
allow_http="${FORGE_S3_ALLOW_HTTP:-false}"
tested_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
probe_key="_forge-contract/${profile}/$(date -u +%Y%m%dT%H%M%SZ)-$$/probe.txt"
work_dir="$(mktemp -d)"
uploaded=0
result=passed

cleanup() {
    local status=$?
    if [[ "$uploaded" == 1 ]]; then
        if ! aws_cmd delete-object --bucket "$FORGE_S3_BUCKET" --key "$probe_key" >/dev/null 2>"$work_dir/cleanup.err"; then
            printf 'S3 contract cleanup failed; inspect %s\n' "$work_dir/cleanup.err" >&2
            status=1
        fi
    fi
    rm -rf "$work_dir"
    exit "$status"
}
trap cleanup EXIT

case "$profile" in
    generic-s3|aws-s3|minio|ceph-rgw|alibaba-oss|tencent-cos|huawei-obs) ;;
    *) echo "unsupported S3 profile: $FORGE_S3_PROFILE" >&2; exit 1 ;;
esac
if [[ -n "$contract_file" && -z "$evidence_file" ]]; then
    echo "FORGE_S3_EVIDENCE_FILE is required when FORGE_S3_CONTRACT_FILE is set" >&2
    exit 1
fi
if [[ "$endpoint" == http://* && "$allow_http" != true ]]; then
    echo "HTTP S3 endpoints require FORGE_S3_ALLOW_HTTP=true for disposable local tests" >&2
    exit 1
fi
command -v aws >/dev/null 2>&1 || { echo "aws CLI is required" >&2; exit 1; }

export AWS_ACCESS_KEY_ID="$FORGE_S3_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$FORGE_S3_SECRET_KEY"
export AWS_SESSION_TOKEN="${FORGE_S3_SESSION_TOKEN:-}"
export AWS_PAGER=""
aws_args=(--region "$region")
if [[ -n "$endpoint" ]]; then aws_args+=(--endpoint-url "$endpoint"); fi
aws_cmd() { aws s3api "$@" "${aws_args[@]}"; }

payload="$work_dir/payload"
printf 'forge s3 contract %s\n' "$tested_at" >"$payload"
sha256="$(shasum -a 256 "$payload" | awk '{print $1}')"
checksum="$(printf '%s' "$sha256" | xxd -r -p | base64)"
head_bucket=failed
list_objects=failed
put_sse_s3=failed
head_object=failed
get_object=failed
content_match=failed
delete_object=failed
versioning=not-tested

if aws_cmd head-bucket --bucket "$FORGE_S3_BUCKET" >/dev/null 2>"$work_dir/head-bucket.err"; then head_bucket=passed; else result=failed; fi
if aws_cmd list-objects-v2 --bucket "$FORGE_S3_BUCKET" --prefix "_forge-contract/" --max-keys 1 >/dev/null 2>"$work_dir/list.err"; then list_objects=passed; else result=failed; fi
if aws_cmd put-object --bucket "$FORGE_S3_BUCKET" --key "$probe_key" --body "$payload" --content-type text/plain --checksum-algorithm SHA256 --checksum-sha256 "$checksum" --server-side-encryption AES256 >/dev/null 2>"$work_dir/put.err"; then
    put_sse_s3=passed
    uploaded=1
else
    result=failed
fi
if [[ "$uploaded" == 1 ]]; then
    if aws_cmd head-object --bucket "$FORGE_S3_BUCKET" --key "$probe_key" >/dev/null 2>"$work_dir/head.err"; then head_object=passed; else result=failed; fi
    if aws_cmd get-object --bucket "$FORGE_S3_BUCKET" --key "$probe_key" "$work_dir/download" >/dev/null 2>"$work_dir/get.err"; then
        get_object=passed
        if cmp -s "$payload" "$work_dir/download"; then content_match=passed; else result=failed; fi
    else result=failed; fi
fi
if aws_cmd get-bucket-versioning --bucket "$FORGE_S3_BUCKET" >/dev/null 2>"$work_dir/versioning.err"; then versioning=observed; fi
if [[ "$uploaded" == 1 ]] && aws_cmd delete-object --bucket "$FORGE_S3_BUCKET" --key "$probe_key" >/dev/null 2>"$work_dir/delete.err"; then
    delete_object=passed
    uploaded=0
else
    result=failed
fi

if [[ -n "$evidence_file" ]]; then
    mkdir -p "$(dirname "$evidence_file")"
    python3 - "$evidence_file" "$profile" "$region" "$endpoint" "$FORGE_S3_BUCKET" "$tested_at" "$(git rev-parse HEAD)" "$result" "$head_bucket" "$list_objects" "$put_sse_s3" "$head_object" "$get_object" "$content_match" "$delete_object" "$versioning" <<'PY'
import json
import pathlib
import sys

path, profile, region, endpoint, bucket, tested_at, commit, result, head_bucket, list_objects, put_sse_s3, head_object, get_object, content_match, delete_object, versioning = sys.argv[1:]
basic = all(value == "passed" for value in (head_bucket, list_objects, head_object, get_object, content_match, delete_object))
payload = {
    "kind": "s3-foundation-contract",
    "status": result,
    "profile": profile,
    "target": {"region": region, "endpoint": endpoint, "bucket": bucket},
    "tested_at": tested_at,
    "source_commit": commit,
    "capabilities": {
        "basic_object_io": {"state": "supported" if basic else "failed", "evidence": path},
        "sse_s3": {"state": "supported" if put_sse_s3 == "passed" else "failed", "evidence": path},
        "versioning": {"state": versioning, "evidence": path},
    },
}
pathlib.Path(path).write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
fi

if [[ -n "$contract_file" && "$result" == passed ]]; then
    digest="$(shasum -a 256 "$evidence_file" | awk '{print $1}')"
    mkdir -p "$(dirname "$contract_file")"
    python3 - "$contract_file" "$profile" "$region" "$FORGE_S3_BUCKET" "$tested_at" "$evidence_file" "$digest" "$head_bucket" "$list_objects" "$put_sse_s3" "$head_object" "$get_object" "$content_match" "$delete_object" <<'PY'
import json
import pathlib
import sys

path, profile, region, bucket, tested_at, evidence_ref, digest, head_bucket, list_objects, put_sse_s3, head_object, get_object, content_match, delete_object = sys.argv[1:]
basic = all(value == "passed" for value in (head_bucket, list_objects, head_object, get_object, content_match, delete_object))
pathlib.Path(path).write_text(json.dumps({
    "profile": profile,
    "level": "Target-tested",
    "target": f"{profile}/{region}/{bucket}",
    "evidence_ref": evidence_ref,
    "evidence_digest": digest,
    "tested_at": tested_at,
    "capabilities": {
        "basic_object_io": {"state": "supported" if basic else "failed", "evidence": evidence_ref},
        "sse_s3": {"state": "supported" if put_sse_s3 == "passed" else "failed", "evidence": evidence_ref},
    },
}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
fi

printf 'provider=%s region=%s bucket=%s result=%s\n' "$profile" "$region" "$FORGE_S3_BUCKET" "$result"
printf 'head_bucket=%s list_objects=%s put_sse_s3=%s head_object=%s get_object=%s content_match=%s delete_object=%s versioning=%s\n' \
    "$head_bucket" "$list_objects" "$put_sse_s3" "$head_object" "$get_object" "$content_match" "$delete_object" "$versioning"
[[ "$result" == passed ]]
