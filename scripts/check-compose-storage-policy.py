#!/usr/bin/env python3
"""Prevent the default Compose profiles from acquiring MinIO-only behavior."""

from __future__ import annotations

import pathlib
import re
import sys


ROOT = pathlib.Path(__file__).resolve().parents[1]
BASE_FILES = (ROOT / "deploy/compose/standard.yaml", ROOT / "deploy/compose/full.yaml")
LOCAL_OVERLAY = ROOT / "deploy/compose/local-s3-minio.yaml"
EXTERNAL_ENDPOINT = re.compile(r"FORGE_STORAGE_ENDPOINT:\s+\$\{FORGE_STORAGE_ENDPOINT:\?[^}]+\}")


def fail(message: str) -> None:
    print(f"compose storage policy failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    forbidden = ("MINIO_ROOT", "S3_MC_IMAGE", "s3-init:", "s3_data:", "mc alias")
    for path in BASE_FILES:
        text = path.read_text(encoding="utf-8")
        if not EXTERNAL_ENDPOINT.search(text):
            fail(f"{path}: external S3 endpoint must be required")
        for token in forbidden:
            if token in text:
                fail(f"{path}: default profile contains MinIO-only token {token!r}")
        if not re.search(r"FORGE_STORAGE_PATH_STYLE:\s+\$\{FORGE_STORAGE_PATH_STYLE:-false\}", text):
            fail(f"{path}: path-style behavior must remain provider-configurable")

    overlay = LOCAL_OVERLAY.read_text(encoding="utf-8")
    for token in ("DEVELOPMENT ONLY", "S3_IMAGE", "S3_MC_IMAGE", "MINIO_ROOT_USER", "s3-init:"):
        if token not in overlay:
            fail(f"{LOCAL_OVERLAY}: explicit local overlay is missing {token!r}")
    print("compose storage policy passed: default profiles are external S3, MinIO is explicit local overlay")


if __name__ == "__main__":
    main()
