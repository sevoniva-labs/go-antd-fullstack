#!/usr/bin/env python3
import copy
import hashlib
import json
import runpy
import tempfile
import unittest
from pathlib import Path


CHECKER = Path(__file__).with_name("check-s3-evidence.py")
checker = runpy.run_path(str(CHECKER))
EvidenceError = checker["EvidenceError"]
validate = checker["validate"]


def document(evidence_ref="reports/cos-foundation.json"):
    return {
        "kind": "forge-s3-compatibility-evidence",
        "level": "Target-tested",
        "provider": "tencent-cos",
        "target": {
            "product": "Tencent COS",
            "version": "2026-test-profile",
            "architecture": "amd64",
            "os": "linux",
            "runtime": "aws-cli-v2",
            "driver": "aws-sdk-go-v2-s3",
            "endpoint": "https://cos.ap-shanghai.myqcloud.com",
            "region": "ap-shanghai",
            "bucket": "replace-with-disposable-bucket",
        },
        "tested_at": "2026-08-21T00:00:00Z",
        "tested_capabilities": ["basic_object_io", "sse_s3"],
        "capabilities": {
            "basic_object_io": {"state": "passed", "evidence_ref": evidence_ref},
            "sse_s3": {"state": "passed", "evidence_ref": evidence_ref},
            "object_lock": {"state": "not_tested"},
        },
        "evidence": {},
    }


class S3EvidenceTest(unittest.TestCase):
    def test_target_digest_and_tamper(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            report = root / "reports" / "cos-foundation.json"
            report.parent.mkdir()
            report.write_text('{"status":"passed"}\n', encoding="utf-8")
            data = document()
            data["evidence"] = {
                "reports/cos-foundation.json": hashlib.sha256(report.read_bytes()).hexdigest()
            }
            validate(data, root, True)
            report.write_text('{"status":"tampered"}\n', encoding="utf-8")
            with self.assertRaises(EvidenceError):
                validate(data, root, True)

    def test_rejects_secrets_and_unverified_claims(self):
        secret = document()
        secret["target"]["secret_key"] = "must-not-be-recorded"
        with self.assertRaises(EvidenceError):
            validate(secret)

        incomplete = copy.deepcopy(document())
        incomplete["tested_capabilities"] = ["object_lock"]
        with self.assertRaises(EvidenceError):
            validate(incomplete)

    def test_not_certified_example_is_allowed(self):
        data = document()
        data["level"] = "Not certified"
        data["tested_capabilities"] = []
        for result in data["capabilities"].values():
            result["state"] = "not_tested"
            result.pop("evidence_ref", None)
        validate(data)


if __name__ == "__main__":
    unittest.main()
