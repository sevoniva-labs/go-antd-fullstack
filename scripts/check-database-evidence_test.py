#!/usr/bin/env python3
import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-database-evidence.py"
CONTROL_KEYS = ("migration", "transactions", "sql_types", "locking", "backup_restore", "failover", "performance", "tls")


def target_record(ref: str, digest: str) -> dict:
    return {
        "kind": "forge-database-compatibility-evidence",
        "level": "Target-tested",
        "provider": "oceanbase",
        "mode": "mysql",
        "target": {
            "product": "approved-database",
            "version": "4.3",
            "patch": "p1",
            "architecture": "arm64",
            "os": "xinchuang-os",
            "runtime": "containerd-1",
            "driver": "go-sql-driver/mysql@approved",
            "tested_at": "2026-08-21T00:00:00Z",
        },
        "controls": {key: "passed" for key in CONTROL_KEYS},
        "evidence_refs": [ref],
        "evidence_digests": {ref: digest},
    }


class DatabaseEvidenceCheckerTest(unittest.TestCase):
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
            evidence = root / "database-report.txt"
            evidence.write_text("migration and failover verified\n", encoding="utf-8")
            report = root / "report.json"
            report.write_text(json.dumps(target_record("database-report.txt", hashlib.sha256(evidence.read_bytes()).hexdigest())), encoding="utf-8")

            result = self.run_checker(report, root)
            self.assertEqual(result.returncode, 0, result.stderr)

            evidence.write_text("tampered\n", encoding="utf-8")
            result = self.run_checker(report, root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("digest mismatch", result.stderr)

    def test_target_tested_rejects_unverified_control_and_secret(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / "database-report.txt"
            evidence.write_text("report\n", encoding="utf-8")
            report = root / "report.json"
            invalid = target_record("database-report.txt", hashlib.sha256(evidence.read_bytes()).hexdigest())
            invalid["controls"]["failover"] = "not_tested"
            invalid["password"] = "must-not-be-recorded"
            report.write_text(json.dumps(invalid), encoding="utf-8")

            result = self.run_checker(report, root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("secret-bearing field", result.stderr)


if __name__ == "__main__":
    unittest.main()
