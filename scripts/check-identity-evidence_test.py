#!/usr/bin/env python3

import json
import pathlib
import subprocess
import sys
import tempfile


ROOT = pathlib.Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-identity-evidence.py"


def run(path: pathlib.Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(CHECKER), "--file", str(path), *args],
        cwd=ROOT,
        text=True,
        capture_output=True,
    )


def payload() -> dict:
    return {
        "kind": "identity-runtime-contract",
        "status": "passed",
        "level": "Experimental",
        "profile": "local-disposable",
        "architecture": "arm64",
        "project": "forge-test",
        "source_commit": "a" * 40,
        "checked_at": "2026-08-21T00:00:00+00:00",
        "checks": ["ldap-user-bind-search", "sso-management-health", "oidc-discovery"],
        "ldap_image": "harbor.internal/openldap@sha256:" + "1" * 64,
        "sso_image": "harbor.internal/keycloak@sha256:" + "2" * 64,
    }


with tempfile.TemporaryDirectory() as directory:
    path = pathlib.Path(directory) / "identity-runtime.json"
    path.write_text(json.dumps(payload()), encoding="utf-8")
    result = run(path)
    assert result.returncode == 0, result.stderr

    invalid = payload()
    invalid["ldap_image"] = "harbor.internal/openldap:latest"
    path.write_text(json.dumps(invalid), encoding="utf-8")
    result = run(path)
    assert result.returncode != 0

    invalid = payload()
    invalid["ldap_admin_password"] = "must-not-be-recorded"
    path.write_text(json.dumps(invalid), encoding="utf-8")
    result = run(path)
    assert result.returncode != 0

    result = run(path, "--require-target-tested")
    assert result.returncode != 0

print("identity evidence tests passed")
