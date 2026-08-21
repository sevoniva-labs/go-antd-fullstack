#!/usr/bin/env python3
import json
import subprocess
import sys
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts" / "check-mail-evidence.py"


def record():
    return {
        "schema": "forge-mail-runtime-evidence",
        "level": "Experimental",
        "provider": "mailpit",
        "version": "v1.21.8",
        "architecture": "arm64",
        "os": "linux",
        "runtime": "docker-compose",
        "image": "docker.m.daocloud.io/axllent/mailpit@sha256:" + "a" * 64,
        "generated_at": "2026-08-21T00:00:00Z",
        "checks": [
            {"name": "smtp_accept", "status": "passed"},
            {"name": "message_visible_in_http_api", "status": "passed"},
        ],
    }


def run(path, *extra):
    return subprocess.run([sys.executable, str(CHECKER), "--file", str(path), *extra], capture_output=True, text=True)


with tempfile.TemporaryDirectory() as directory:
    path = Path(directory) / "mail.json"
    path.write_text(json.dumps(record()), encoding="utf-8")
    valid = run(path)
    assert valid.returncode == 0, valid.stderr
    invalid_record = record()
    invalid_record["checks"][0]["status"] = "failed"
    path.write_text(json.dumps(invalid_record), encoding="utf-8")
    invalid = run(path)
    assert invalid.returncode != 0, invalid.stdout

print("mail evidence checker tests passed")
