# 灾备证据使用方式

`test-postgres-backup-restore-contract.sh` 和
`test-mysql-backup-restore-contract.sh` 支持通过
`FORGE_DISASTER_EVIDENCE_FILE` 生成统一灾备报告。报告只会把真实执行的
`backup_restore` 标记为 `passed`，节点故障、网络分区、数据库故障转移、消息队列故障、对象存储故障和站点故障保持 `not_tested`。

生成报告时必须显式提供目标元数据、证据根目录和 RPO/RTO 目标，例如：

```sh
FORGE_DISASTER_EVIDENCE_FILE=.evidence/postgres-disaster.json \
FORGE_DISASTER_EVIDENCE_ROOT=.evidence \
FORGE_DR_TARGET_PRODUCT=PostgreSQL \
FORGE_DR_TARGET_VERSION=16.15 \
FORGE_DR_TARGET_TESTED_AT=2026-08-21T00:00:00Z \
FORGE_DR_TARGET_CPU=arm64 \
FORGE_DR_TARGET_OS=linux \
FORGE_DR_TARGET_RUNTIME=docker \
FORGE_DR_TARGET_DATABASE='PostgreSQL 16.15 single instance' \
FORGE_DR_TARGET_MESSAGE_QUEUE=not-tested \
FORGE_DR_TARGET_OBJECT_STORAGE=not-tested \
FORGE_DR_RPO_TARGET_SECONDS=300 \
FORGE_DR_RTO_TARGET_SECONDS=1800 \
make postgres-backup-restore-contract

DR_EVIDENCE_FILE=.evidence/postgres-disaster.json \
DR_EVIDENCE_ROOT=.evidence \
make disaster-check
```

该报告是本地单实例恢复证据，不是 PITR、HA、跨站切换、断网恢复、信创环境或监管认证。只有所有必需场景在精确目标环境真实通过，并附带完整证据引用，才允许使用 `disaster-check-certified`。
