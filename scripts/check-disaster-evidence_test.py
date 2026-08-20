#!/usr/bin/env python3
import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-disaster-evidence.py"
SCENARIOS = (
    "node_failure",
    "network_partition",
    "database_failover",
    "mq_failure",
    "s3_failure",
    "site_failure",
    "backup_restore",
)


def certified_report(ref: str, digest: str) -> dict:
    return {
        "target": {
            "product": "forge",
            "version": "test",
            "tested_at": "2026-08-21T00:00:00Z",
            "cpu": "amd64",
            "os": "test-os",
            "runtime": "test-runtime",
            "database": "test-db",
            "message_queue": "test-mq",
            "object_storage": "test-s3",
        },
        "rpo_target_seconds": 10,
        "rto_target_seconds": 10,
        "evidence_digests": {ref: digest},
        "scenarios": [
            {
                "name": name,
                "status": "passed",
                "evidence_refs": [ref],
                "observed_rpo_seconds": 1,
                "observed_rto_seconds": 1,
            }
            for name in SCENARIOS
        ],
    }


class DisasterEvidenceCheckerTest(unittest.TestCase):
    def run_checker(self, report: Path, root: Path):
        return subprocess.run(
            [
                sys.executable,
                str(CHECKER),
                "--file",
                str(report),
                "--evidence-root",
                str(root),
                "--require-certified",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )

    def test_certified_report_requires_matching_evidence_digest(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / "restore.log"
            evidence.write_text("restore verified\n", encoding="utf-8")
            digest = hashlib.sha256(evidence.read_bytes()).hexdigest()
            report = root / "report.json"
            report.write_text(json.dumps(certified_report("restore.log", digest)), encoding="utf-8")

            result = self.run_checker(report, root)

            self.assertEqual(result.returncode, 0, result.stderr)

            evidence.write_text("tampered\n", encoding="utf-8")
            result = self.run_checker(report, root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("digest mismatch", result.stderr)

    def test_certified_report_rejects_path_traversal(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            outside = root.parent / "outside.log"
            outside.write_text("outside\n", encoding="utf-8")
            report = root / "report.json"
            report.write_text(
                json.dumps(certified_report("../outside.log", hashlib.sha256(outside.read_bytes()).hexdigest())),
                encoding="utf-8",
            )

            result = self.run_checker(report, root)

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("relative safe path", result.stderr)


if __name__ == "__main__":
    unittest.main()
