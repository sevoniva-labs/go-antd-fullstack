# Database Compatibility Evidence Contract

PostgreSQL and MySQL are the scaffold baselines. OceanBase MySQL mode is a `Profile`; KingbaseES, 达梦, and GaussDB remain `Adapter slot` entries until the exact target driver and environment pass this contract. The checked-in example is `Not certified` and contains no credentials.

Run format and regression checks with:

```bash
FORGE_DATABASE_EVIDENCE_FILE=deploy/database/evidence.example.json make database-evidence-check
```

`database-evidence-check-certified` requires exact product/version/patch, CPU, OS, runtime, driver, database mode, migration, transaction, SQL/type, locking, backup/restore, failover, performance, and TLS results. It also requires evidence files below `FORGE_DATABASE_EVIDENCE_ROOT` with matching lowercase SHA-256 digests. This contract does not manufacture a vendor certification from MySQL protocol compatibility.
