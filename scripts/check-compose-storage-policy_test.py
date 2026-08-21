#!/usr/bin/env python3

import pathlib
import subprocess
import sys


ROOT = pathlib.Path(__file__).resolve().parents[1]
result = subprocess.run(
    [sys.executable, str(ROOT / "scripts/check-compose-storage-policy.py")],
    cwd=ROOT,
    text=True,
    capture_output=True,
)
assert result.returncode == 0, result.stderr
assert "external S3" in result.stdout
print("compose storage policy tests passed")
