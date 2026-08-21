#!/usr/bin/env python3
"""Validate non-secret runtime evidence for domestic middleware adapters."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import sys
from typing import Any

ALLOWED_LEVELS = {"Adapter slot", "Experimental", "Target-tested", "Not certified"}
ALLOWED_PROVIDERS = {"kafka", "nacos", "otel", "prometheus", "rocketmq"}
ALLOWED_STATES = {"failed", "not_tested", "observed", "passed"}
REQUIRED_TARGET = ("product", "version", "architecture", "os", "runtime", "driver", "image")
SENSITIVE_KEY = re.compile(
    r"(?:password|secret|token|private[_-]?key|access[_-]?key|credential|authorization)",
    re.IGNORECASE,
)
SHA256_REF = re.compile(r"^[a-f0-9]{64}$")


def fail(message: str) -> int:
    print(message, file=sys.stderr)
    return 1


def ensure_safe(value: Any, path: str = "record") -> None:
    if isinstance(value, dict):
        for key, child in value.items():
            # Check names are control labels and may legitimately describe an
            # authentication-token policy; target and metadata keys may not.
            if not path.startswith("record.checks") and SENSITIVE_KEY.search(str(key)):
                raise ValueError(f"sensitive field is not allowed: {path}.{key}")
            ensure_safe(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            ensure_safe(child, f"{path}[{index}]")


def safe_reference(root: pathlib.Path, reference: str) -> pathlib.Path:
    path = pathlib.Path(reference)
    if path.is_absolute() or ".." in path.parts:
        raise ValueError(f"unsafe evidence reference: {reference}")
    resolved = (root / path).resolve()
    if root.resolve() not in resolved.parents:
        raise ValueError(f"evidence reference escapes root: {reference}")
    if not resolved.is_file():
        raise ValueError(f"evidence reference does not exist: {reference}")
    return resolved


def validate(record: Any, evidence_root: pathlib.Path | None, require_target_tested: bool) -> str:
    if not isinstance(record, dict):
        raise ValueError("middleware evidence must be a JSON object")
    ensure_safe(record)
    if record.get("kind") != "forge-middleware-runtime-evidence":
        raise ValueError("middleware evidence kind must be forge-middleware-runtime-evidence")
    level = record.get("level")
    if level not in ALLOWED_LEVELS:
        raise ValueError("middleware evidence level is invalid")
    provider = record.get("provider")
    if provider not in ALLOWED_PROVIDERS:
        raise ValueError("middleware evidence provider is invalid")
    if record.get("status") != "passed":
        raise ValueError("middleware runtime evidence status must be passed")
    target = record.get("target")
    if not isinstance(target, dict):
        raise ValueError("middleware evidence target metadata is required")
    for key in REQUIRED_TARGET:
        if not isinstance(target.get(key), str) or not target[key].strip():
            raise ValueError(f"middleware evidence target.{key} is required")
    image = target["image"]
    if not re.search(r"@sha256:[a-f0-9]{64}$", image):
        raise ValueError("middleware evidence target.image must use a lowercase sha256 digest")
    checks = record.get("checks")
    if not isinstance(checks, dict) or not checks:
        raise ValueError("middleware evidence checks must be a non-empty object")
    for name, state in checks.items():
        if not isinstance(name, str) or not name.strip() or state not in ALLOWED_STATES:
            raise ValueError("middleware evidence checks must map names to allowed states")

    if require_target_tested or level == "Target-tested":
        if evidence_root is None:
            raise ValueError("FORGE_MIDDLEWARE_EVIDENCE_ROOT is required for Target-tested evidence")
        incomplete = sorted(name for name, state in checks.items() if state != "passed")
        if incomplete:
            raise ValueError("Target-tested middleware evidence has incomplete checks: " + ", ".join(incomplete))
        references = record.get("evidence_refs")
        digests = record.get("evidence_digests")
        if not isinstance(references, list) or not references:
            raise ValueError("Target-tested middleware evidence requires evidence_refs")
        if not isinstance(digests, dict):
            raise ValueError("Target-tested middleware evidence requires evidence_digests")
        for reference in references:
            if not isinstance(reference, str) or not SHA256_REF.fullmatch(str(digests.get(reference, ""))):
                raise ValueError("Target-tested middleware evidence has invalid reference digest")
            path = safe_reference(evidence_root, reference)
            digest = hashlib.sha256(path.read_bytes()).hexdigest()
            if digest != digests[reference]:
                raise ValueError(f"middleware evidence digest mismatch: {reference}")
    return str(level)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", default=os.getenv("FORGE_MIDDLEWARE_EVIDENCE_FILE", ""))
    parser.add_argument("--evidence-root", default=os.getenv("FORGE_MIDDLEWARE_EVIDENCE_ROOT", ""))
    parser.add_argument("--require-target-tested", action="store_true")
    args = parser.parse_args()
    if not args.file:
        print("middleware evidence file not supplied; target runtime certification not claimed")
        return 0
    try:
        record = json.loads(pathlib.Path(args.file).read_text(encoding="utf-8"))
        level = validate(record, pathlib.Path(args.evidence_root) if args.evidence_root else None, args.require_target_tested)
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        return fail(f"middleware evidence validation failed: {exc}")
    print(f"middleware evidence format OK: {level}; provider={record['provider']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
