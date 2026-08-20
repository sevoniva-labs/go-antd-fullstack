#!/usr/bin/env python3
import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-crypto-evidence.py"


def target_record(ref: str, digest: str, controls=None) -> dict:
    return {
        "kind": "forge-crypto-device-evidence",
        "level": "Target-tested",
        "target": {
            "product": "approved-crypto-device",
            "version": "1.0.0",
            "firmware": "fw-1",
            "architecture": "arm64",
            "os": "xinchuang-os",
            "runtime": "containerd-1",
            "tested_at": "2026-08-21T00:00:00Z",
        },
        "provider": "approved-hsm-adapter",
        "key_source": "adapter",
        "algorithms": ["SM2", "SM3", "SM4", "GMTLS"],
        "controls": controls
        or {key: "passed" for key in ("key_rotation", "backup_restore", "dual_control", "audit", "tls")},
        "evidence_refs": [ref],
        "evidence_digests": {ref: digest},
    }


class CryptoEvidenceCheckerTest(unittest.TestCase):
    def run_checker(self, report: Path, root: Path):
        return subprocess.run(
            [sys.executable, str(CHECKER), "--file", str(report), "--evidence-root", str(root), "--require-target-tested"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )

    def test_target_tested_requires_matching_digest(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / "hsm-report.txt"
            evidence.write_text("rotation and dual control verified\n", encoding="utf-8")
            report = root / "report.json"
            report.write_text(json.dumps(target_record("hsm-report.txt", hashlib.sha256(evidence.read_bytes()).hexdigest())), encoding="utf-8")

            result = self.run_checker(report, root)
            self.assertEqual(result.returncode, 0, result.stderr)

            evidence.write_text("tampered\n", encoding="utf-8")
            result = self.run_checker(report, root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("digest mismatch", result.stderr)

    def test_target_tested_rejects_incomplete_controls_and_secrets(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / "hsm-report.txt"
            evidence.write_text("report\n", encoding="utf-8")
            report = root / "report.json"
            controls = {key: "passed" for key in ("key_rotation", "backup_restore", "dual_control", "audit", "tls")}
            controls["backup_restore"] = "not_tested"
            invalid = target_record("hsm-report.txt", hashlib.sha256(evidence.read_bytes()).hexdigest(), controls)
            invalid["pin"] = "must-not-be-recorded"
            report.write_text(json.dumps(invalid), encoding="utf-8")

            result = self.run_checker(report, root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("secret-bearing field", result.stderr)


if __name__ == "__main__":
    unittest.main()
