#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
COMPOSE_FILE="$ROOT_DIR/deploy/compose/mail-runtime-contract.yaml"
PROJECT_NAME=${FORGE_MAIL_PROJECT_NAME:-forge-mail-contract}
SMTP_PORT=${FORGE_MAIL_SMTP_PORT:-1025}
UI_PORT=${FORGE_MAIL_UI_PORT:-8025}
EVIDENCE_FILE=${FORGE_MAIL_EVIDENCE_FILE:-$ROOT_DIR/.evidence/mail-runtime-contract.json}
IMAGE='docker.m.daocloud.io/axllent/mailpit@sha256:81370195cd4a0eab9604d17c2617a7525b0486f9365555253b6c5376c6350f1a'

mkdir -p "$(dirname "$EVIDENCE_FILE")"
compose=(docker compose --project-name "$PROJECT_NAME" -f "$COMPOSE_FILE")
cleanup() {
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

"${compose[@]}" up -d --quiet-pull
ready=false
for _ in $(seq 1 60); do
  if curl --fail --silent --show-error --max-time 2 "http://127.0.0.1:${UI_PORT}/api/v1/info" >/dev/null; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  echo "mail runtime did not become ready" >&2
  exit 1
fi

FORGE_MAIL_SMTP_PORT="$SMTP_PORT" FORGE_MAIL_UI_PORT="$UI_PORT" python3 - <<'PY'
import json
import os
import smtplib
import urllib.request
from email.message import EmailMessage

smtp_port = int(os.environ["FORGE_MAIL_SMTP_PORT"])
ui_port = int(os.environ["FORGE_MAIL_UI_PORT"])
message = EmailMessage()
message["From"] = "forge-contract@example.com"
message["To"] = "forge-recipient@example.com"
message["Subject"] = "Forge mail contract"
message.set_content("forge mail contract body")
with smtplib.SMTP("127.0.0.1", smtp_port, timeout=10) as client:
    client.send_message(message)
with urllib.request.urlopen(f"http://127.0.0.1:{ui_port}/api/v1/messages", timeout=10) as response:
    data = json.load(response)
if not data.get("messages"):
    raise SystemExit("mail API did not expose the submitted message")
PY

ARCHITECTURE=$(docker image inspect "$IMAGE" --format '{{.Architecture}}')
OS=$(docker image inspect "$IMAGE" --format '{{.Os}}')
GENERATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
cat >"$EVIDENCE_FILE" <<EOF
{
  "schema": "forge-mail-runtime-evidence",
  "level": "Experimental",
  "provider": "mailpit",
  "version": "v1.21.8",
  "architecture": "$ARCHITECTURE",
  "os": "$OS",
  "runtime": "docker-compose",
  "image": "$IMAGE",
  "generated_at": "$GENERATED_AT",
  "checks": [
    {"name": "smtp_accept", "status": "passed"},
    {"name": "message_visible_in_http_api", "status": "passed"}
  ]
}
EOF
python3 "$ROOT_DIR/scripts/check-mail-evidence.py" --file "$EVIDENCE_FILE"
echo "mail runtime contract passed: $EVIDENCE_FILE"
