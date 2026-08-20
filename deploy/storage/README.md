# S3 capability evidence

The scaffold uses one capability model for generic S3, AWS S3, MinIO, Ceph RGW, Alibaba OSS, Tencent COS, and Huawei OBS profiles. A provider name never implies certification.

Use `deploy/storage/evidence.example.json` as the `Not certified` starting point. A `Target-tested` document must include:

- The exact provider product, version, architecture, OS, runtime, driver, endpoint, region, and disposable bucket.
- A `tested_capabilities` list containing only capabilities that passed the target contract.
- Relative evidence references and lowercase SHA-256 digests.
- An evidence root containing the referenced, non-secret reports.

Validate the structure with `make s3-evidence-check`. Validate a real target with `make s3-evidence-check-certified` and provide `FORGE_S3_EVIDENCE_FILE` plus `FORGE_S3_EVIDENCE_ROOT`. Credentials, access keys, session tokens, private keys, and passwords must never be placed in the document or evidence files.
