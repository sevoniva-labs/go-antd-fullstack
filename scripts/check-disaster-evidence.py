#!/usr/bin/env python3
"""Validate dated disaster-recovery evidence without inventing target results."""
import argparse
import hashlib
import json
import os
import sys
from pathlib import Path


SCENARIOS = {
    "node_failure",
    "network_partition",
    "database_failover",
    "mq_failure",
    "s3_failure",
    "site_failure",
    "backup_restore",
}
STATUSES = {"passed", "failed", "not_tested"}


def fail(message: str) -> int:
    print(message, file=sys.stderr)
    return 1


def verify_evidence_refs(record: dict, root: Path) -> None:
    digests = record.get("evidence_digests")
    if not isinstance(digests, dict):
        raise ValueError("evidence_digests must be an object when evidence root is supplied")
    root = root.resolve()
    for scenario in record["scenarios"]:
        if scenario.get("status") != "passed":
            continue
        for ref in scenario["evidence_refs"]:
            path = Path(ref)
            if path.is_absolute() or ".." in path.parts or not ref.strip():
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
    parser.add_argument("--file", default=os.environ.get("DR_EVIDENCE_FILE", ""))
    parser.add_argument("--evidence-root", default=os.environ.get("DR_EVIDENCE_ROOT", ""))
    parser.add_argument("--require-certified", action="store_true")
    args = parser.parse_args()
    if not args.file:
        print("disaster evidence file not supplied; format evidence not claimed")
        return 0

    path = Path(args.file)
    try:
        record = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return fail(f"cannot read disaster evidence: {exc}")
    if not isinstance(record, dict):
        return fail("disaster evidence must be a JSON object")

    metadata = record.get("target")
    if not isinstance(metadata, dict):
        return fail("disaster evidence target metadata is required")
    for key in ("product", "version", "tested_at", "cpu", "os", "runtime", "database", "message_queue", "object_storage"):
        if not isinstance(metadata.get(key), str) or not metadata[key].strip():
            return fail(f"disaster evidence target.{key} is required")
    targets = {}
    for key in ("rpo_target_seconds", "rto_target_seconds"):
        value = record.get(key)
        if not isinstance(value, (int, float)) or value <= 0:
            return fail(f"disaster evidence {key} must be positive")
        targets[key] = float(value)

    scenarios = record.get("scenarios")
    if not isinstance(scenarios, list):
        return fail("disaster evidence scenarios must be a list")
    names = [item.get("name") for item in scenarios if isinstance(item, dict)]
    if set(names) != SCENARIOS or len(names) != len(SCENARIOS):
        return fail("disaster evidence must contain each required scenario exactly once")

    failed_or_untested = []
    for item in scenarios:
        if not isinstance(item, dict) or item.get("status") not in STATUSES:
            return fail("each disaster scenario must have status passed, failed, or not_tested")
        status = item["status"]
        if status != "passed":
            failed_or_untested.append(item["name"])
            continue
        refs = item.get("evidence_refs")
        if not isinstance(refs, list) or not refs or not all(isinstance(ref, str) and ref.strip() for ref in refs):
            return fail(f"passed scenario {item['name']} requires evidence_refs")
        for key, target_key in (("observed_rpo_seconds", "rpo_target_seconds"), ("observed_rto_seconds", "rto_target_seconds")):
            value = item.get(key)
            if not isinstance(value, (int, float)) or value < 0:
                return fail(f"passed scenario {item['name']} requires non-negative {key}")
            if value > targets[target_key]:
                return fail(f"scenario {item['name']} exceeded {target_key}")

    if args.require_certified and not args.evidence_root:
        return fail("DR_EVIDENCE_ROOT is required for certified disaster evidence")
    if args.evidence_root:
        try:
            verify_evidence_refs(record, Path(args.evidence_root))
        except (OSError, ValueError) as exc:
            return fail(f"disaster evidence references are invalid: {exc}")

    if args.require_certified and failed_or_untested:
        return fail("disaster evidence is not certified; incomplete scenarios: " + ", ".join(sorted(failed_or_untested)))
    status = "certified" if not failed_or_untested else "not certified"
    print(f"disaster evidence format OK: {status}; target={metadata['product']}@{metadata['version']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
