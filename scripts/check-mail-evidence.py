#!/usr/bin/env python3
import argparse
import json
import re
import sys
from pathlib import Path

LEVELS = {"Built-in", "Profile", "Adapter slot", "Experimental", "Target-tested", "Not certified"}
IMAGE_DIGEST = re.compile(r"^.+@sha256:[0-9a-f]{64}$")
REQUIRED_CHECKS = {"smtp_accept", "message_visible_in_http_api"}


def validate(record, require_target_tested=False):
    errors = []
    if record.get("schema") != "forge-mail-runtime-evidence":
        errors.append("schema must be forge-mail-runtime-evidence")
    level = record.get("level")
    if level not in LEVELS:
        errors.append("level is invalid")
    if require_target_tested and level != "Target-tested":
        errors.append("Target-tested evidence is required")
    if record.get("provider") != "mailpit":
        errors.append("provider must be mailpit")
    for key in ("version", "architecture", "os", "runtime", "generated_at"):
        if not isinstance(record.get(key), str) or not record[key].strip():
            errors.append(f"{key} is required")
    if not isinstance(record.get("image"), str) or not IMAGE_DIGEST.fullmatch(record["image"]):
        errors.append("image must be an immutable sha256 digest")
    checks = record.get("checks")
    if not isinstance(checks, list):
        errors.append("checks must be a list")
    else:
        names = set()
        for check in checks:
            if not isinstance(check, dict) or not isinstance(check.get("name"), str):
                errors.append("each check must have a name")
                continue
            name = check["name"]
            if name in names:
                errors.append(f"duplicate check {name}")
            names.add(name)
            if check.get("status") != "passed":
                errors.append(f"check {name} did not pass")
        missing = REQUIRED_CHECKS - names
        if missing:
            errors.append("missing checks: " + ",".join(sorted(missing)))
    return errors


def validate_file(path, require_target_tested=False):
    try:
        record = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return [f"{path}: {exc}"]
    errors = validate(record, require_target_tested)
    return [f"{path}: {error}" for error in errors]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", type=Path)
    parser.add_argument("--evidence-root", type=Path, default=Path(".evidence"))
    parser.add_argument("--require-target-tested", action="store_true")
    args = parser.parse_args()
    if args.file:
        files = [args.file]
    else:
        files = sorted(args.evidence_root.glob("mail-runtime-contract*.json")) if args.evidence_root.exists() else []
        if not files:
            print("mail evidence: no optional runtime evidence found; skipped")
            return 0
    errors = [error for path in files for error in validate_file(path, args.require_target_tested)]
    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    print(f"mail evidence valid: {len(files)} file(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
