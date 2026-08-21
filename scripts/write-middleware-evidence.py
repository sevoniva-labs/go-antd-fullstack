#!/usr/bin/env python3
"""Write a non-secret standard runtime evidence record from explicit environment inputs."""

from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys
from datetime import datetime, timezone

LEVELS = {"Adapter slot", "Experimental", "Target-tested", "Not certified"}
REQUIRED = {
    "FORGE_MIDDLEWARE_PRODUCT": "product",
    "FORGE_MIDDLEWARE_VERSION": "version",
    "FORGE_MIDDLEWARE_ARCHITECTURE": "architecture",
    "FORGE_MIDDLEWARE_OS": "os",
    "FORGE_MIDDLEWARE_RUNTIME": "runtime",
    "FORGE_MIDDLEWARE_DRIVER": "driver",
}


def main() -> int:
    path = os.getenv("FORGE_MIDDLEWARE_EVIDENCE_FILE", "")
    if not path:
        return 0
    missing = [name for name in (*REQUIRED, "FORGE_MIDDLEWARE_PROVIDER", "FORGE_MIDDLEWARE_IMAGE") if not os.getenv(name, "").strip()]
    if missing:
        print("middleware evidence metadata is missing: " + ", ".join(missing), file=sys.stderr)
        return 1
    level = os.getenv("FORGE_MIDDLEWARE_EVIDENCE_LEVEL", "Experimental")
    if level not in LEVELS:
        print("middleware evidence level is invalid", file=sys.stderr)
        return 1
    try:
        checks = json.loads(os.environ.get("FORGE_MIDDLEWARE_CHECKS", "{}"))
    except json.JSONDecodeError as exc:
        print(f"middleware evidence checks are invalid JSON: {exc}", file=sys.stderr)
        return 1
    if not isinstance(checks, dict) or not checks:
        print("middleware evidence checks must be a non-empty object", file=sys.stderr)
        return 1
    try:
        source_commit = os.environ.get("FORGE_MIDDLEWARE_SOURCE_COMMIT", "") or subprocess.check_output(
            ["git", "rev-parse", "HEAD"], text=True
        ).strip()
    except (OSError, subprocess.CalledProcessError) as exc:
        print(f"middleware evidence source commit is unavailable: {exc}", file=sys.stderr)
        return 1
    target = {env_name: os.environ[env_name] for env_name in REQUIRED}
    target = {key.removeprefix("FORGE_MIDDLEWARE_").lower(): value for key, value in target.items()}
    target["image"] = os.environ["FORGE_MIDDLEWARE_IMAGE"]
    if os.getenv("FORGE_MIDDLEWARE_ENDPOINT", "").strip():
        target["endpoint"] = os.environ["FORGE_MIDDLEWARE_ENDPOINT"]
    payload = {
        "kind": "forge-middleware-runtime-evidence",
        "status": "passed",
        "level": level,
        "provider": os.environ["FORGE_MIDDLEWARE_PROVIDER"],
        "target": target,
        "source_commit": source_commit,
        "checked_at": datetime.now(timezone.utc).isoformat(),
        "checks": checks,
    }
    output = pathlib.Path(path)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
