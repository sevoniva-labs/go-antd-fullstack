#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

: "${FORGE_LDAP_IMAGE:?set FORGE_LDAP_IMAGE to an approved immutable LDAP image digest}"
: "${FORGE_SSO_IMAGE:?set FORGE_SSO_IMAGE to an approved immutable SSO image digest}"
: "${FORGE_LDAP_ADMIN_PASSWORD:?set FORGE_LDAP_ADMIN_PASSWORD in the local environment}"
: "${FORGE_LDAP_USER_PASSWORD:?set FORGE_LDAP_USER_PASSWORD in the local environment}"
: "${FORGE_SSO_ADMIN_PASSWORD:?set FORGE_SSO_ADMIN_PASSWORD in the local environment}"

COMPOSE_CMD=${FORGE_COMPOSE_CMD:-docker compose}
read -r -a COMPOSE <<<"$COMPOSE_CMD"
PROJECT=${FORGE_IDENTITY_PROJECT:-forge-identity-contract-$$}
COMPOSE_FILE=${FORGE_IDENTITY_COMPOSE_FILE:-deploy/compose/identity-dev.yaml}
BASE_DN=${FORGE_LDAP_BASE_DN:-dc=forge,dc=local}
TEST_LOGIN=${FORGE_LDAP_TEST_LOGIN:-forge.user}

compose() {
  "${COMPOSE[@]}" -p "$PROJECT" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  compose down -v >/dev/null 2>&1 || true
}

trap cleanup EXIT

compose config >/dev/null
compose up -d >/dev/null 2>&1

ldap_probe() {
  compose exec -T ldap ldapsearch -x \
    -H "ldap://127.0.0.1:1389" \
    -D "cn=admin,${BASE_DN}" \
    -w "$FORGE_LDAP_ADMIN_PASSWORD" \
    -b "$BASE_DN" \
    "(uid=${TEST_LOGIN})" dn uid >/dev/null 2>&1
}

sso_probe() {
  compose exec -T sso /bin/bash -c '
    exec 3<>/dev/tcp/127.0.0.1/9000
    printf "GET /health/ready HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3
    timeout 3 cat <&3
  ' 2>/dev/null | rg -q "200 OK"
}

oidc_probe() {
  compose exec -T sso /bin/bash -c '
    exec 3<>/dev/tcp/127.0.0.1/8080
    printf "GET /realms/master/.well-known/openid-configuration HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3
    timeout 3 cat <&3
  ' 2>/dev/null | rg -q "200 OK"
}

ldap_ok=false
sso_ok=false
oidc_ok=false
for _ in $(seq 1 90); do
  if [[ "$ldap_ok" != true ]] && ldap_probe; then ldap_ok=true; fi
  if [[ "$sso_ok" != true ]] && sso_probe; then sso_ok=true; fi
  if [[ "$oidc_ok" != true ]] && oidc_probe; then oidc_ok=true; fi
  if [[ "$ldap_ok" == true && "$sso_ok" == true && "$oidc_ok" == true ]]; then break; fi
  sleep 1
done

if [[ "$ldap_ok" != true || "$sso_ok" != true || "$oidc_ok" != true ]]; then
  printf 'identity contract failed: ldap=%s sso=%s oidc=%s\n' "$ldap_ok" "$sso_ok" "$oidc_ok" >&2
  compose logs --tail=80 >&2 || true
  exit 1
fi

if [[ -n "${FORGE_IDENTITY_EVIDENCE_FILE:-}" ]]; then
  mkdir -p "$(dirname "$FORGE_IDENTITY_EVIDENCE_FILE")"
  python3 - "$FORGE_IDENTITY_EVIDENCE_FILE" "$PROJECT" "$FORGE_LDAP_IMAGE" "$FORGE_SSO_IMAGE" "$(git rev-parse HEAD)" <<'PY'
import json
import os
import platform
import pathlib
import sys
from datetime import datetime, timezone

path, project, ldap_image, sso_image, commit = sys.argv[1:]
payload = {
    "kind": "identity-runtime-contract",
    "status": "passed",
    "level": "Experimental",
    "profile": os.environ.get("FORGE_IDENTITY_PROFILE", "local-disposable"),
    "architecture": platform.machine(),
    "project": project,
    "ldap_image": ldap_image,
    "sso_image": sso_image,
    "source_commit": commit,
    "checked_at": datetime.now(timezone.utc).isoformat(),
    "checks": ["ldap-user-bind-search", "sso-management-health", "oidc-discovery"],
}
pathlib.Path(path).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY
fi

printf 'identity runtime contract passed: ldap=%s sso=%s\n' "$FORGE_LDAP_IMAGE" "$FORGE_SSO_IMAGE"
