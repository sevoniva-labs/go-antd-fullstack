# Provider / Capability Matrix

状态定义：**Built-in**=根模块已有可执行实现；**Profile**=复用兼容协议但必须在目标产品版本验证；**Adapter slot**=接口/配置已预留，未完成厂商实现，不宣称支持。

| Category | Provider / Capability | Status | Notes |
|---|---|---:|---|
| Database | PostgreSQL | Built-in | pgx stdlib + `database/sql` |
| Database | MySQL | Built-in | go-sql-driver/mysql |
| Database | OceanBase MySQL mode | Profile | MySQL protocol + 独立 migration profile，需目标集群验证 |
| Database | Kingbase / 达梦 / GaussDB | Adapter slot | `integrations/database/*` |
| Cache | Memory | Built-in | 单实例/开发 |
| Cache | Redis standalone | Built-in | go-redis |
| Cache | Redis Sentinel | Built-in | UniversalClient |
| Cache | Redis Cluster | Built-in | UniversalClient |
| Distributed | Rate limit | Built-in | Redis 原子计数；Memory 为本机语义 |
| Distributed | Lock / Scheduler lock | Built-in | Redis SET NX + compare-delete |
| Messaging | Kafka | Built-in | franz-go |
| Messaging | RocketMQ 5.x | Built-in default | Apache 官方 gRPC SDK；TLS/mTLS、ACL、Topic 白名单 |
| Reliability | 本地可靠消息表 | Built-in | 与业务事务同库写入，Worker 租约恢复；at-least-once |
| Reliability | Idempotency | Built-in | DB 记录 request hash/result state |
| Search | Elasticsearch | Built-in | REST adapter |
| Search | OpenSearch | Built-in | REST-compatible adapter，目标版本仍需兼容测试 |
| Storage | Local | Built-in | 单实例/开发 |
| Storage | S3-compatible | Built-in | AWS S3 SDK；兼容 MinIO、S3 兼容对象存储协议（COS/OSS/R2/Ceph/CloudStack 等）按实际版本验证 |

`FORGE_STORAGE_PROVIDER` 支持别名：`s3`, `s3-compatible`, `minio`, `oss`, `cos`, `ceph`, `ceph-rgw`，启动时会统一归一化为 `s3`（大小写不敏感，带下划线/短横线兼容）。

| Config Center | Nacos | Built-in | 启动拉取 + Watch API |
| Service Registry | Nacos | Built-in | register/deregister/health |
| Crypto | Standard | Built-in | SHA-256 + AES-GCM |
| Crypto | GM | Built-in baseline | SM3 + SM4-GCM；不等同于完整密评方案 |
| Crypto | SM2 / HSM / KMS / 密码机 | Adapter slot | 见 `integrations/secrets/hsm` |
| Secrets | Env / `*_FILE` | Built-in | Secret file 优先 |
| Secrets | Vault/Cloud KMS/HSM | Adapter slot | 实现 `secrets.Provider` |
| Observability | Structured log | Built-in | slog JSON/Text + sensitive-key redaction |
| Observability | Prometheus | Built-in | low-cardinality route metrics |
| Observability | OpenTelemetry | Built-in | W3C propagation + OTLP HTTP tracing |
| Diagnostics | pprof | Built-in guarded | 默认关闭，合规 profile 可强制禁用 |
| Identity | Browser Session | Built-in | Server-side Session + CSRF |
| Identity | API Token | Built-in | machine identity, hash-at-rest, scopes |
| Authorization | Permission RBAC | Built-in | 组织隔离角色 + 三员基线 + 可扩展 permission |
| MFA | TOTP utility | Built-in primitive | 可供业务接入；默认登录流未强制 MFA |
| Feature | DB feature flags | Built-in | 环境/组织扩展可继续加 |
| Commercial | Offline license verifier | Built-in extension | Ed25519 claims primitive；业务 entitlement 自定义 |

## Provider 设计规则

1. 业务层禁止 import Redis/Kafka/S3/Nacos/ES 等厂商 SDK。
2. 可选 Provider 不能成为 minimal 模式的强启动依赖。
3. “协议兼容”不等于“生产认证”；兼容矩阵必须记录实际版本/CPU/OS/数据库组合。
4. 国产替换优先落在 adapter/provider，不为某个数据库把业务 SQL/类型扩散到 domain。
5. Redis/Kafka/Search/S3 的 TLS 使用统一 `tlsx` 策略，支持企业 CA、客户端证书和 ServerName；框架不提供跳过证书校验开关。数据库 TLS 由对应驱动 DSN 配置。
