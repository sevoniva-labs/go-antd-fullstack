#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${FORGE_NACOS_IMAGE:?set FORGE_NACOS_IMAGE to an approved immutable Nacos image digest}"
: "${FORGE_NACOS_AUTH_TOKEN:?set FORGE_NACOS_AUTH_TOKEN in the local environment}"
: "${FORGE_NACOS_AUTH_IDENTITY_KEY:?set FORGE_NACOS_AUTH_IDENTITY_KEY in the local environment}"
: "${FORGE_NACOS_AUTH_IDENTITY_VALUE:?set FORGE_NACOS_AUTH_IDENTITY_VALUE in the local environment}"

COMPOSE_CMD=${FORGE_COMPOSE_CMD:-docker compose}
read -r -a COMPOSE <<<"$COMPOSE_CMD"
PROJECT=${FORGE_NACOS_PROJECT:-forge-nacos-contract-$$}
COMPOSE_FILE=${FORGE_NACOS_COMPOSE_FILE:-deploy/compose/nacos-dev.yaml}
BASE_URL=${FORGE_NACOS_BASE_URL:-http://127.0.0.1:8848}
CONSOLE_BASE_URL=${FORGE_NACOS_CONSOLE_BASE_URL:-http://127.0.0.1:18080}
EVIDENCE_FILE=${FORGE_NACOS_EVIDENCE_FILE:-}
MIDDLEWARE_EVIDENCE_FILE=${FORGE_MIDDLEWARE_EVIDENCE_FILE:-}
NACOS_USERNAME=${FORGE_NACOS_USERNAME:-}
NACOS_PASSWORD=${FORGE_NACOS_PASSWORD:-}

if [[ -n "$NACOS_USERNAME" || -n "$NACOS_PASSWORD" ]] && [[ -z "$NACOS_USERNAME" || -z "$NACOS_PASSWORD" ]]; then
  echo "FORGE_NACOS_USERNAME and FORGE_NACOS_PASSWORD must be supplied together" >&2
  exit 1
fi

export NACOS_IMAGE="$FORGE_NACOS_IMAGE"
export NACOS_AUTH_ENABLE=true
export NACOS_AUTH_TOKEN="$FORGE_NACOS_AUTH_TOKEN"
export NACOS_AUTH_IDENTITY_KEY="$FORGE_NACOS_AUTH_IDENTITY_KEY"
export NACOS_AUTH_IDENTITY_VALUE="$FORGE_NACOS_AUTH_IDENTITY_VALUE"

validate_token() {
  python3 - "$FORGE_NACOS_AUTH_TOKEN" <<'PY'
import base64
import binascii
import sys

try:
    decoded = base64.b64decode(sys.argv[1], validate=True)
except (binascii.Error, ValueError) as exc:
    raise SystemExit(f"Nacos auth token is not strict Base64: {exc}")

if len(decoded) < 32:
    raise SystemExit("Nacos auth token must decode to at least 32 bytes")
PY
}

compose() {
  "${COMPOSE[@]}" -p "$PROJECT" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  compose down -v >/dev/null 2>&1 || true
}

validate_token
compose config >/dev/null
compose up -d >/dev/null 2>&1

probe_ok() {
  local base_url=$1
  local path=$2
  local response_file=$3
  local http_code

  http_code=$(curl -sS --connect-timeout 2 --max-time 5 \
    -o "$response_file" -w '%{http_code}' "${base_url}${path}" 2>/dev/null || true)
  [[ "$http_code" == "200" ]] && rg -iq '(^|[^[:alpha:]])ok([^[:alpha:]]|$)' "$response_file"
}

probe_anonymous_config_rejected() {
  local http_code

  http_code=$(curl -sS --connect-timeout 2 --max-time 5 \
    -o /dev/null -w '%{http_code}' \
    "${BASE_URL}/nacos/v1/cs/configs?dataId=forge-contract&group=DEFAULT_GROUP" 2>/dev/null || true)
  [[ "$http_code" == "401" || "$http_code" == "403" ]]
}

authenticated_config_round_trip() {
  local access_token data_id group content login_response published published_ok fetched fetched_content deleted deleted_ok

  # Nacos 3 requires an initial administrator before password login can succeed.
  # Ignore the bootstrap response when the server was already initialized.
  curl -sS --connect-timeout 2 --max-time 5 \
    -X POST "${BASE_URL}/nacos/v3/auth/user/admin" \
    --data-urlencode "username=${NACOS_USERNAME}" \
    --data-urlencode "password=${NACOS_PASSWORD}" >/dev/null 2>&1 || true

  login_response=$(curl -sS --fail --connect-timeout 2 --max-time 5 \
    -X POST "${BASE_URL}/nacos/v3/auth/user/login" \
    --data-urlencode "username=${NACOS_USERNAME}" \
    --data-urlencode "password=${NACOS_PASSWORD}" 2>/dev/null || true)
  access_token=$(printf '%s' "$login_response" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("accessToken", ""))' 2>/dev/null || true)
  [[ -n "$access_token" ]] || return 1

  data_id="forge-contract-${PROJECT}"
  group="DEFAULT_GROUP"
  content="forge-nacos-contract-${PROJECT}"
  published=$(curl -sS --fail --connect-timeout 2 --max-time 5 \
    -X POST "${BASE_URL}/nacos/v3/admin/cs/config?dataId=${data_id}&groupName=${group}&content=${content}" \
    -H "accessToken: ${access_token}" 2>/dev/null || true)
  published_ok=$(printf '%s' "$published" | python3 -c 'import json,sys; data=json.load(sys.stdin); print(str(data.get("code") == 0 and data.get("data") is True).lower())' 2>/dev/null || true)
  [[ "$published_ok" == true ]] || return 1

  # The pinned Nacos 3 target exposes the verified management read path here.
  # Do not claim client-runtime reads until a target-tested client API contract exists.
  fetched=$(curl -sS --fail --connect-timeout 2 --max-time 5 \
    "${BASE_URL}/nacos/v3/admin/cs/config?dataId=${data_id}&groupName=${group}" \
    -H "accessToken: ${access_token}" 2>/dev/null || true)
  fetched_content=$(printf '%s' "$fetched" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("data", {}).get("content", ""))' 2>/dev/null || true)
  [[ "$fetched_content" == "$content" ]] || return 1

  deleted=$(curl -sS --fail --connect-timeout 2 --max-time 5 \
    -X DELETE \
    "${BASE_URL}/nacos/v3/admin/cs/config?dataId=${data_id}&groupName=${group}" \
    -H "accessToken: ${access_token}" 2>/dev/null || true)
  deleted_ok=$(printf '%s' "$deleted" | python3 -c 'import json,sys; data=json.load(sys.stdin); print(str(data.get("code") == 0 and data.get("data") is True).lower())' 2>/dev/null || true)
  [[ "$deleted_ok" == true ]]
}

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"; cleanup' EXIT

console_ok=false
server_ok=false
anonymous_rejected=false
config_round_trip=not-tested
for _ in $(seq 1 120); do
  if [[ "$console_ok" != true ]] && probe_ok "$CONSOLE_BASE_URL" "/v3/console/health/readiness" "$tmp_dir/console.json"; then
    console_ok=true
  fi
  if [[ "$server_ok" != true ]] && probe_ok "$BASE_URL" "/nacos/v3/admin/core/state/readiness" "$tmp_dir/server.json"; then
    server_ok=true
  fi
  if [[ "$anonymous_rejected" != true ]] && probe_anonymous_config_rejected; then
    anonymous_rejected=true
  fi
  if [[ "$console_ok" == true && "$server_ok" == true && "$anonymous_rejected" == true ]]; then
    break
  fi
  sleep 1
done

if [[ "$console_ok" != true || "$server_ok" != true || "$anonymous_rejected" != true ]]; then
  printf 'Nacos contract failed: console_readiness=%s server_readiness=%s anonymous_config_rejected=%s\n' \
    "$console_ok" "$server_ok" "$anonymous_rejected" >&2
  compose ps >&2 || true
  exit 1
fi

if [[ -n "$NACOS_USERNAME" ]]; then
  if authenticated_config_round_trip; then
    config_round_trip=passed
  else
    printf 'Nacos contract failed: authenticated configuration round trip failed\n' >&2
    exit 1
  fi
fi

if [[ -n "$EVIDENCE_FILE" ]]; then
  mkdir -p "$(dirname "$EVIDENCE_FILE")"
  python3 - "$EVIDENCE_FILE" "$PROJECT" "$FORGE_NACOS_IMAGE" "$(git rev-parse HEAD)" "$config_round_trip" <<'PY'
import json
import pathlib
import sys
from datetime import datetime, timezone

path, project, image, commit, config_round_trip = sys.argv[1:]
checks = [
    "console-readiness",
    "server-readiness",
    "anonymous-config-rejected",
    "base64-auth-token-minimum-32-bytes",
]
if config_round_trip == "passed":
    checks.append("authenticated-config-round-trip")
payload = {
    "kind": "nacos-runtime-contract",
    "status": "passed",
    "project": project,
    "nacos_image": image,
    "source_commit": commit,
    "checked_at": datetime.now(timezone.utc).isoformat(),
    "checks": checks,
}
pathlib.Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
fi

if [[ -n "$MIDDLEWARE_EVIDENCE_FILE" ]]; then
  middleware_checks=$(python3 - "$config_round_trip" <<'PY'
import json
import sys

checks = {
    "console-readiness": "passed",
    "server-readiness": "passed",
    "anonymous-config-rejected": "passed",
    "base64-auth-token-minimum-32-bytes": "passed",
    "authenticated-config-round-trip": sys.argv[1] if sys.argv[1] in {"passed", "not_tested"} else "failed",
}
print(json.dumps(checks, separators=(",", ":")))
PY
  )
  FORGE_MIDDLEWARE_EVIDENCE_FILE="$MIDDLEWARE_EVIDENCE_FILE" \
  FORGE_MIDDLEWARE_PROVIDER=nacos \
  FORGE_MIDDLEWARE_IMAGE="$FORGE_NACOS_IMAGE" \
  FORGE_MIDDLEWARE_ENDPOINT="$BASE_URL" \
  FORGE_MIDDLEWARE_CHECKS="$middleware_checks" \
  python3 scripts/write-middleware-evidence.py
fi

printf 'Nacos runtime contract passed: image=%s\n' "$FORGE_NACOS_IMAGE"
