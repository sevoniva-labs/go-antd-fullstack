#!/usr/bin/env python3
"""Validate target crypto-device evidence without treating software crypto as certification."""
import argparse
import hashlib
import json
import os
import sys
from pathlib import Path


LEVELS = {"Adapter slot", "Experimental", "Target-tested", "Not certified"}
CONTROL_STATUSES = {"passed", "failed", "not_tested"}
REQUIRED_TARGET_FIELDS = ("product", "version", "firmware", "architecture", "os", "runtime", "tested_at")
REQUIRED_CONTROLS = ("key_rotation", "backup_restore", "dual_control", "audit", "tls")
SENSITIVE_KEYS = {"password", "pin", "private_key", "raw_key", "secret", "secret_key", "token"}


def fail(message: str) -> int:
    print(message, file=sys.stderr)
    return 1


def reject_secrets(value, path: str = "record") -> None:
    if isinstance(value, dict):
        for key, child in value.items():
            normalized = str(key).lower().replace("-", "_")
            if normalized in SENSITIVE_KEYS:
                raise ValueError(f"secret-bearing field is forbidden: {path}.{key}")
            reject_secrets(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            reject_secrets(child, f"{path}[{index}]")


def verify_evidence_refs(record: dict, root: Path) -> None:
    refs = record.get("evidence_refs")
    digests = record.get("evidence_digests")
    if not isinstance(refs, list) or not refs or not all(isinstance(ref, str) and ref.strip() for ref in refs):
        raise ValueError("Target-tested crypto evidence requires non-empty evidence_refs")
    if not isinstance(digests, dict):
        raise ValueError("evidence_digests must be an object for Target-tested crypto evidence")
    root = root.resolve()
    for ref in refs:
        path = Path(ref)
        if path.is_absolute() or ".." in path.parts:
            raise ValueError(f"evidence reference must be a relative safe path: {ref!r}")
        evidence_path = (root / path).resolve()
        try:
            evidence_path.relative_to(root)
        except ValueError as exc:
            raise ValueError(f"evidence reference escapes evidence root: {ref!r}") from exc
        if not evidence_path.is_file():
            raise ValueError(f"evidence reference does not exist as a file: {ref!r}")
        expected = digests.get(ref)
        if not isinstance(expected, str) or len(expected) != 64 or any(c not in "0123456789abcdef" for c in expected):
            raise ValueError(f"evidence_digests[{ref!r}] must be a lowercase SHA-256 digest")
        actual = hashlib.sha256(evidence_path.read_bytes()).hexdigest()
        if actual != expected:
            raise ValueError(f"evidence digest mismatch: {ref!r}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", default=os.environ.get("FORGE_CRYPTO_EVIDENCE_FILE", ""))
    parser.add_argument("--evidence-root", default=os.environ.get("FORGE_CRYPTO_EVIDENCE_ROOT", ""))
    parser.add_argument("--require-target-tested", action="store_true")
    args = parser.parse_args()
    if not args.file:
        if args.require_target_tested:
            return fail("FORGE_CRYPTO_EVIDENCE_FILE is required for Target-tested crypto evidence")
        print("crypto evidence file not supplied; target certification not claimed")
        return 0

    path = Path(args.file)
    try:
        record = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return fail(f"cannot read crypto evidence: {exc}")
    if not isinstance(record, dict):
        return fail("crypto evidence must be a JSON object")
    try:
        reject_secrets(record)
    except ValueError as exc:
        return fail(f"crypto evidence is unsafe: {exc}")
    if record.get("kind") != "forge-crypto-device-evidence":
        return fail("crypto evidence kind must be forge-crypto-device-evidence")

    level = record.get("level")
    if level not in LEVELS:
        return fail("crypto evidence level must be Adapter slot|Experimental|Target-tested|Not certified")
    target = record.get("target")
    if not isinstance(target, dict):
        return fail("crypto evidence target metadata is required")
    for key in REQUIRED_TARGET_FIELDS:
        if not isinstance(target.get(key), str) or not target[key].strip():
            return fail(f"crypto evidence target.{key} is required")
    if not isinstance(record.get("provider"), str) or not record["provider"].strip():
        return fail("crypto evidence provider is required")
    if not isinstance(record.get("key_source"), str) or not record["key_source"].strip():
        return fail("crypto evidence key_source is required")
    algorithms = record.get("algorithms")
    if not isinstance(algorithms, list) or not algorithms or not all(isinstance(item, str) and item.strip() for item in algorithms):
        return fail("crypto evidence algorithms must be a non-empty list")

    controls = record.get("controls")
    if not isinstance(controls, dict):
        return fail("crypto evidence controls are required")
    incomplete = []
    for key in REQUIRED_CONTROLS:
        status = controls.get(key)
        if status not in CONTROL_STATUSES:
            return fail(f"crypto evidence controls.{key} must be passed, failed, or not_tested")
        if status != "passed":
            incomplete.append(key)

    if level == "Target-tested":
        if not args.evidence_root:
            return fail("FORGE_CRYPTO_EVIDENCE_ROOT is required for Target-tested crypto evidence")
        if incomplete:
            return fail("Target-tested crypto evidence has incomplete controls: " + ", ".join(incomplete))
        try:
            verify_evidence_refs(record, Path(args.evidence_root))
        except (OSError, ValueError) as exc:
            return fail(f"crypto evidence references are invalid: {exc}")
    elif args.evidence_root and record.get("evidence_refs"):
        try:
            verify_evidence_refs(record, Path(args.evidence_root))
        except (OSError, ValueError) as exc:
            return fail(f"crypto evidence references are invalid: {exc}")

    if args.require_target_tested and level != "Target-tested":
        return fail(f"crypto evidence is {level}; Target-tested is required")
    print(f"crypto evidence format OK: {level}; target={target['product']}@{target['version']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
