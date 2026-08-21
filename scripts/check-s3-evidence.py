#!/usr/bin/env python3
"""Validate the provider-neutral S3 compatibility evidence contract."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import Any


LEVELS = {
    "Built-in",
    "Profile",
    "Adapter slot",
    "Experimental",
    "Target-tested",
    "Not certified",
}
PROFILES = {
    "generic-s3",
    "aws-s3",
    "minio",
    "ceph-rgw",
    "alibaba-oss",
    "tencent-cos",
    "huawei-obs",
}
CAPABILITIES = {
    "basic_object_io",
    "checksum",
    "multipart_recovery",
    "constrained_presign",
    "sse_s3",
    "sse_kms",
    "versioning",
    "object_lock",
    "retention",
    "legal_hold",
    "temporary_credential",
}
STATES = {"passed", "failed", "not_tested", "observed"}
SENSITIVE_KEY = re.compile(
    r"(?:password|passwd|secret|token|private[_-]?key|raw[_-]?key|"
    r"access[_-]?key|secret[_-]?id|pin)",
    re.IGNORECASE,
)
SHA256 = re.compile(r"^[0-9a-f]{64}$")
TARGET_FIELDS = {
    "product",
    "version",
    "architecture",
    "os",
    "runtime",
    "driver",
    "endpoint",
    "region",
    "bucket",
}


class EvidenceError(ValueError):
    """Raised when an evidence document is unsafe or incomplete."""


def _require_string(value: Any, field: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise EvidenceError(f"{field} is required")
    return value.strip()


def _scan_sensitive(value: Any, path: str = "$") -> None:
    if isinstance(value, dict):
        for key, child in value.items():
            if SENSITIVE_KEY.search(str(key)):
                raise EvidenceError(f"sensitive field is forbidden: {path}.{key}")
            _scan_sensitive(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            _scan_sensitive(child, f"{path}[{index}]")


def _safe_ref(value: Any, field: str) -> str:
    ref = _require_string(value, field)
    path = Path(ref)
    if path.is_absolute() or "\\" in ref or any(part in {"", ".", ".."} for part in path.parts):
        raise EvidenceError(f"{field} must be a safe relative path")
    return ref


def _validate_timestamp(value: Any) -> str:
    timestamp = _require_string(value, "tested_at")
    try:
        datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
    except ValueError as exc:
        raise EvidenceError("tested_at must be an ISO-8601 timestamp") from exc
    return timestamp


def _validate_digest(value: Any, field: str) -> str:
    digest = _require_string(value, field)
    if not SHA256.fullmatch(digest):
        raise EvidenceError(f"{field} must be a lowercase SHA-256 digest")
    return digest


def validate(data: Any, evidence_root: Path | None = None, require_target_tested: bool = False) -> None:
    if not isinstance(data, dict):
        raise EvidenceError("evidence document must be a JSON object")
    _scan_sensitive(data)
    if data.get("kind") != "forge-s3-compatibility-evidence":
        raise EvidenceError("kind must be forge-s3-compatibility-evidence")

    level = _require_string(data.get("level"), "level")
    if level not in LEVELS:
        raise EvidenceError(f"unsupported evidence level: {level}")
    provider = _require_string(data.get("provider"), "provider")
    if provider not in PROFILES:
        raise EvidenceError(f"unsupported S3 provider profile: {provider}")
    _validate_timestamp(data.get("tested_at"))

    target = data.get("target")
    if not isinstance(target, dict):
        raise EvidenceError("target is required")
    for field in TARGET_FIELDS:
        _require_string(target.get(field), f"target.{field}")

    capabilities = data.get("capabilities")
    if not isinstance(capabilities, dict) or not capabilities:
        raise EvidenceError("capabilities must be a non-empty object")
    unknown = set(capabilities) - CAPABILITIES
    if unknown:
        raise EvidenceError(f"unknown S3 capabilities: {', '.join(sorted(unknown))}")
    for capability, result in capabilities.items():
        if not isinstance(result, dict):
            raise EvidenceError(f"capability {capability} must be an object")
        state = _require_string(result.get("state"), f"capabilities.{capability}.state")
        if state not in STATES:
            raise EvidenceError(f"unsupported state for {capability}: {state}")
        if state == "passed":
            _safe_ref(result.get("evidence_ref"), f"capabilities.{capability}.evidence_ref")

    claims = data.get("tested_capabilities", [])
    if not isinstance(claims, list) or any(not isinstance(item, str) for item in claims):
        raise EvidenceError("tested_capabilities must be a list of strings")
    if len(set(claims)) != len(claims) or any(item not in CAPABILITIES for item in claims):
        raise EvidenceError("tested_capabilities contains an unknown or duplicate capability")
    for capability in claims:
        result = capabilities.get(capability)
        if not isinstance(result, dict) or result.get("state") != "passed":
            raise EvidenceError(f"claimed capability is not passed: {capability}")

    evidence = data.get("evidence", {})
    if not isinstance(evidence, dict):
        raise EvidenceError("evidence must be an object mapping relative paths to digests")
    digest_map: dict[str, str] = {}
    for ref, digest in evidence.items():
        safe = _safe_ref(ref, "evidence reference")
        digest_map[safe] = _validate_digest(digest, f"evidence.{safe}")

    if level == "Target-tested":
        if not claims:
            raise EvidenceError("Target-tested evidence must declare tested_capabilities")
        if not evidence_root:
            raise EvidenceError("Target-tested evidence requires an evidence root")
        root = evidence_root.resolve()
        for capability in claims:
            ref = _safe_ref(capabilities[capability].get("evidence_ref"), f"capabilities.{capability}.evidence_ref")
            if ref not in digest_map:
                raise EvidenceError(f"missing digest for evidence reference: {ref}")
            evidence_path = (root / ref).resolve()
            if root not in evidence_path.parents:
                raise EvidenceError(f"evidence reference escapes the evidence root: {ref}")
            if not evidence_path.is_file():
                raise EvidenceError(f"evidence file does not exist: {ref}")
            actual = hashlib.sha256(evidence_path.read_bytes()).hexdigest()
            if actual != digest_map[ref]:
                raise EvidenceError(f"evidence digest mismatch: {ref}")
    elif require_target_tested:
        raise EvidenceError(f"evidence level is {level}, Target-tested is required")


def _load(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise EvidenceError(f"evidence file does not exist: {path}") from exc
    except json.JSONDecodeError as exc:
        raise EvidenceError(f"invalid evidence JSON: {exc}") from exc


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--file", default=os.environ.get("FORGE_S3_EVIDENCE_FILE", ""))
    parser.add_argument("--evidence-root", default=os.environ.get("FORGE_S3_EVIDENCE_ROOT", ""))
    parser.add_argument("--require-target-tested", action="store_true")
    args = parser.parse_args()
    if not args.file:
        if args.require_target_tested:
            print("FORGE_S3_EVIDENCE_FILE is required", file=sys.stderr)
            return 1
        print("s3 evidence file not supplied; target compatibility not claimed")
        return 0
    try:
        validate(
            _load(Path(args.file)),
            Path(args.evidence_root) if args.evidence_root else None,
            args.require_target_tested,
        )
    except EvidenceError as exc:
        print(f"s3 evidence validation failed: {exc}", file=sys.stderr)
        return 1
    print(f"s3 evidence format OK: {Path(args.file)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
