# Disaster-Recovery Evidence Contract

The scaffold provides a machine-checkable evidence format. It does not claim that any database, message queue, S3-compatible provider, Kubernetes cluster, HSM, or site topology has passed a disaster drill.

The local database backup/restore contract is intentionally narrower than this report: `make postgres-backup-restore-contract` exercises a disposable PostgreSQL dump, database recreation, restore, and row verification. Its result must not be copied into a certified disaster report.

## Required scenarios

- `node_failure`
- `network_partition`
- `database_failover`
- `mq_failure`
- `s3_failure`
- `site_failure`
- `backup_restore`

Each scenario must record `passed`, `failed`, or `not_tested`. A passed scenario must include dated evidence references and measured RPO/RTO values that do not exceed the target. The target metadata must identify the exact release, CPU, OS/kernel, runtime, database, MQ, and object-storage versions.

## Verification

```bash
DR_EVIDENCE_FILE=deploy/disaster/evidence.example.json make disaster-check
DR_EVIDENCE_FILE=/path/to/report.json make disaster-check
DR_EVIDENCE_FILE=/path/to/report.json make disaster-check-certified
```

`disaster-check` validates the format and preserves `Not certified` for incomplete scenarios. `disaster-check-certified` is the release gate for a real target report and fails until every scenario is passed. The report and referenced logs must be retained in the institution's approved evidence system.
