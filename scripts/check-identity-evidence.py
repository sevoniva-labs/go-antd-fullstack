#!/usr/bin/env python3
"""Validate non-secret LDAP/OIDC runtime evidence without certifying production."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import pathlib
import re
import sys
from typing import Any


ALLOWED_LEVELS = {"Built-in", "Profile", "Adapter slot", "Experimental", "Target-tested", "Not certified"}
DIGEST_RE = re.compile(r"@sha256:[0-9a-f]{64}$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40,64}$")
SENSITIVE_KEY_RE = re.compile(r"(password|secret|token|access[_-]?key|credential|private[_-]?key)", re.IGNORECASE)


def fail(message: str) -> None:
    print(f"identity evidence check failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def scan_keys(value: Any, path: str = "$") -> None:
    if isinstance(value, dict):
        for key, child in value.items():
            if SENSITIVE_KEY_RE.search(str(key)):
                fail(f"sensitive key at {path}.{key}")
            scan_keys(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            scan_keys(child, f"{path}[{index}]")


def validate(path: pathlib.Path) -> str:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"{path}: invalid JSON: {exc}")
    if not isinstance(payload, dict):
        fail(f"{path}: root must be an object")
    scan_keys(payload)

    required = (
        "kind",
        "status",
        "level",
        "profile",
        "architecture",
        "project",
        "source_commit",
        "checked_at",
        "checks",
        "ldap_image",
        "sso_image",
    )
    missing = [key for key in required if key not in payload]
    if missing:
        fail(f"{path}: missing {', '.join(missing)}")
    if payload["kind"] != "identity-runtime-contract":
        fail(f"{path}: kind must be identity-runtime-contract")
    if payload["status"] != "passed":
        fail(f"{path}: status must be passed")
    if payload["level"] not in ALLOWED_LEVELS:
        fail(f"{path}: unsupported level {payload['level']!r}")
    for key in ("profile", "architecture", "project"):
        if not isinstance(payload[key], str) or not payload[key].strip():
            fail(f"{path}: {key} must be a non-empty string")
    if not isinstance(payload["source_commit"], str) or not COMMIT_RE.fullmatch(payload["source_commit"]):
        fail(f"{path}: source_commit must be a full hexadecimal commit")
    if not isinstance(payload["checked_at"], str):
        fail(f"{path}: checked_at must be an ISO-8601 string")
    try:
        dt.datetime.fromisoformat(payload["checked_at"].replace("Z", "+00:00"))
    except ValueError:
        fail(f"{path}: checked_at is not ISO-8601")
    if (
        not isinstance(payload["checks"], list)
        or not payload["checks"]
        or not all(isinstance(item, str) and item for item in payload["checks"])
    ):
        fail(f"{path}: checks must be a non-empty string list")
    for key in ("ldap_image", "sso_image"):
        if not isinstance(payload[key], str) or not DIGEST_RE.search(payload[key]):
            fail(f"{path}: {key} must use an immutable sha256 digest")
    return str(payload["level"])


def evidence_files(args: argparse.Namespace) -> list[pathlib.Path]:
    if args.file:
        return [pathlib.Path(args.file)]
    if not args.evidence_root:
        return []
    root = pathlib.Path(args.evidence_root)
    if not root.exists():
        return []
    return sorted(root.glob("identity-runtime*.json"))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file")
    parser.add_argument("--evidence-root")
    parser.add_argument("--require-target-tested", action="store_true")
    args = parser.parse_args()
    files = evidence_files(args)
    if not files:
        if args.require_target_tested:
            fail("no identity evidence file found")
        print("identity evidence check skipped: no evidence file")
        return
    levels = {validate(path) for path in files}
    if args.require_target_tested and "Target-tested" not in levels:
        fail("Target-tested identity evidence is required")
    print(f"identity evidence check passed: {len(files)} file(s), levels={','.join(sorted(levels))}")


if __name__ == "__main__":
    main()
