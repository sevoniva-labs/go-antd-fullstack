#!/usr/bin/env python3
import hashlib
import json
import os
import pathlib
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
CHECKER = ROOT / "scripts/check-middleware-evidence.py"
IMAGE = "docker.m.daocloud.io/apache/rocketmq@sha256:" + "a" * 64


def record(level="Experimental"):
    return {
        "kind": "forge-middleware-runtime-evidence",
        "status": "passed",
        "level": level,
        "provider": "rocketmq",
        "target": {
            "product": "RocketMQ",
            "version": "5",
            "architecture": "arm64",
            "os": "linux",
            "runtime": "docker",
            "driver": "rocketmq-go-v5",
            "image": IMAGE,
        },
        "checks": {"broker-ready": "passed", "sdk-produce-consume": "passed"},
    }


class MiddlewareEvidenceTest(unittest.TestCase):
    def run_checker(self, path, *args):
        return subprocess.run(["python3", str(CHECKER), "--file", str(path), *args], capture_output=True, text=True)

    def test_experimental_record(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "evidence.json"
            path.write_text(json.dumps(record()), encoding="utf-8")
            result = self.run_checker(path)
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_secret_and_non_digest_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "evidence.json"
            invalid = record()
            invalid["target"]["password"] = "must-not-appear"
            invalid["target"]["image"] = "registry/rocketmq:5"
            path.write_text(json.dumps(invalid), encoding="utf-8")
            self.assertNotEqual(self.run_checker(path).returncode, 0)

    def test_target_tested_digest_is_checked(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            proof = root / "runtime.log"
            proof.write_text("runtime passed\n", encoding="utf-8")
            target = record("Target-tested")
            target["evidence_refs"] = ["runtime.log"]
            target["evidence_digests"] = {"runtime.log": hashlib.sha256(proof.read_bytes()).hexdigest()}
            path = root / "evidence.json"
            path.write_text(json.dumps(target), encoding="utf-8")
            result = self.run_checker(path, "--evidence-root", str(root), "--require-target-tested")
            self.assertEqual(result.returncode, 0, result.stderr)
            proof.write_text("tampered\n", encoding="utf-8")
            self.assertNotEqual(self.run_checker(path, "--evidence-root", str(root)).returncode, 0)


if __name__ == "__main__":
    unittest.main()
