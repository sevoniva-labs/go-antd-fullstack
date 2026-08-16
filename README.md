# Sevoniva Forge

**Sevoniva Forge** 是面向中国企业、商业化产品和金融机构项目的 Go + React 工程基座。它不是“后台管理页面模板”，而是把企业项目反复出现的身份、权限、安全、审计、统一 API、中间件、可观测、可靠消息、信创适配和部署能力收敛成可替换 Provider。

设计原则：**minimal 能轻量启动，production 能逐步加固；业务层不绑定数据库和中间件厂商；没有完成真实适配验证的能力不冒充“已支持”。**

## 技术基线

- Backend：Go 1.26 + chi + `database/sql`
- Frontend：React 19 + TypeScript + Vite 8 + Ant Design 6 + Pro Components + TanStack Query
- API：OpenAPI 3.1 + 统一 Envelope + 6 位稳定返回码 + request_id/trace_id
- DB：PostgreSQL / MySQL / OceanBase MySQL profile
- Cache：Disabled / Memory / Redis standalone/Sentinel/Cluster
- Messaging：Disabled / Kafka；RocketMQ 5.x adapter slot
- Search：Disabled / Elasticsearch / OpenSearch
- Storage：Local / S3-compatible
- Config/Registry：Nacos 配置中心 + 服务注册发现
- Crypto：Standard（SHA-256/AES-GCM）/ GM baseline（SM3/SM4-GCM）
- Observability：slog + Prometheus + OpenTelemetry + guarded pprof
- Deploy：Binary / systemd / Docker / Compose / Kubernetes Helm

## 企业/金融基础能力

| 能力 | 状态 | 说明 |
|---|---:|---|
| 统一返回码 | ✅ | `000000/10xxxx/20xxxx/30xxxx/40xxxx/50xxxx/90xxxx` 分段治理 |
| API 契约 | ✅ | OpenAPI 3.1；CI lint + error-code contract check |
| Browser Auth | ✅ | Server-side Cookie Session + HttpOnly + CSRF |
| Machine Identity | ✅ | Bearer API Token，hash-at-rest、scope、撤销 |
| RBAC | ✅ | 组织级 Role + 全局 Permission；三员基线；用户角色分配单独权限；保护最后一个活动 system_admin |
| Password | ✅ | Argon2id、复杂度、历史、有效期、失败锁定、首次改密 |
| Audit | ✅ | 安全事件独立表、组织隔离、request ID/IP/actor |
| Security HTTP | ✅ | CSP/HSTS/frame/nosniff/referrer/CORS/body limit/recovery |
| Outbound TLS | ✅ | Redis/Kafka/Search/S3 支持企业 CA、mTLS client cert、ServerName；无 skip-verify |
| Secret | ✅/扩展 | env/`*_FILE` 内置；Vault/KMS/HSM/密码机预留 Provider |
| Standard crypto | ✅ | SHA-256 + AES-GCM |
| GM crypto baseline | ✅* | SM3 + SM4-GCM；SM2/HSM/证书体系属于后续适配/密评范围 |
| Multi DB | ✅/Profile | PostgreSQL/MySQL 内置；OceanBase MySQL profile |
| Domestic DB | 扩展位 | Kingbase/达梦/GaussDB 目录和验收清单已预留，不虚假宣称实测 |
| Redis | ✅ | standalone/Sentinel/Cluster，分布式限流/锁 |
| Kafka | ✅ | franz-go；可选 |
| RocketMQ 5.x | 扩展位 | 官方 SDK adapter slot，未安装时不会静默降级 |
| Search | ✅ | Elasticsearch/OpenSearch REST provider |
| Object storage | ✅ | Local / S3-compatible |
| Nacos | ✅ | Config Center + Registry/Discovery |
| Idempotency | ✅ | DB-backed request reservation/result state |
| Transactional Outbox | ✅ | 事务内事件 + Worker 租约恢复；at-least-once，消费者需幂等 |
| Scheduler lock | ✅ | Redis 分布式锁/Memory 单机锁 |
| Resilience | ✅ | timeout/retry/circuit breaker/bulkhead primitives |
| Feature flag | ✅ | DB-backed primitive |
| Commercial license | ✅ primitive | 离线 Ed25519 license claims/entitlement 扩展基础 |
| Structured logs | ✅ | JSON/Text slog + sensitive-key redaction |
| Prometheus | ✅ | 请求量/错误/延迟/大小/in-flight + Go/process |
| OpenTelemetry | ✅ | W3C propagation + OTLP HTTP trace |
| pprof | ✅ guarded | 默认关闭；合规 profile 可强制禁用 |
| Docker/Compose | ✅ | minimal/mysql/standard/full/Nacos/observability profiles |
| Kubernetes | ✅ | Helm、startup/readiness/liveness、HPA/PDB/NetworkPolicy/ServiceMonitor |
| Supply chain | ✅ CI | govulncheck/gosec/Trivy/Secret Scan/SBOM/Dependency Review |
| Xinchuang | Profile | OceanBase + Redis + Nacos + GM + ARM/国产 OS 适配边界 |

`✅*` 仅表示应用层算法基线，不等于完成商用密码应用方案或密评。

## 架构

```text
React / Ant Design
        │ OpenAPI
        ▼
 HTTP Adapter / Unified Envelope
        │
 Application Services
        │
      Domain
        │ ports
        ▼
┌──────────────────────────────────────────────┐
│ Platform / Providers                         │
│ DB  Cache  MQ  Search  Storage  Nacos        │
│ Crypto  Secrets  Lock  Idempotency  Outbox   │
│ Logs  Metrics  Tracing  Health  Resilience   │
└──────────────────────────────────────────────┘
```

目录：

```text
cmd/server            API 进程
cmd/worker            Outbox/后台任务进程
cmd/migrate           发布流水线一次性数据库迁移
internal/domain       领域模型
internal/app          应用服务
internal/adapters     HTTP/Repository
internal/platform     企业基础设施能力
web                   React + Ant Design
api/openapi.yaml      API 契约
configs               minimal/standard/full/xinchuang
deploy                Docker/Compose/Helm/systemd/observability
integrations          RocketMQ/国产 DB/HSM 等明确扩展位
docs                  架构、安全、合规、运维、兼容矩阵
```

## 快速启动

### Minimal：PostgreSQL + Forge

```bash
cp .env.example .env
# 至少设置：FORGE_BOOTSTRAP_PASSWORD、FORGE_CRYPTO_KEY
# FORGE_CRYPTO_KEY 必须 >= 32 字节或为 >=32 字节密钥的 base64。
docker compose -f deploy/compose/minimal.yaml up -d --build
```

访问 `http://localhost:8080`。源码不提供管理员默认密码；首次登录要求改密。

### 其他开发组合

```bash
# MySQL
docker compose -f deploy/compose/mysql.yaml up -d --build

# PostgreSQL + Redis + S3-compatible 对象存储（例如 MinIO/COS/OSS 等）
docker compose -f deploy/compose/standard.yaml up -d --build

# PostgreSQL + Redis + Kafka + Elasticsearch + S3-compatible 对象存储（例如 MinIO/COS/OSS 等）+ Worker
docker compose -f deploy/compose/full.yaml up -d --build

# 本地 Nacos 3 辅助环境
docker compose -f deploy/compose/nacos-dev.yaml up -d

# 本地 Prometheus + OTel Collector
docker compose -f deploy/compose/observability-dev.yaml up -d
```

## Kubernetes / 银行生产多副本

Helm 默认 `replicaCount=1`，这是为了避免 Local Storage/Memory Cache 被误用成“高可用”。多副本前应至少改为：

- 外部 HA PostgreSQL/MySQL/OceanBase
- Redis（跨 Pod 限流/锁）
- S3-compatible 对象存储
- Worker + Outbox（如启用消息）
- `FORGE_DATABASE_AUTO_MIGRATE=false`，发布前由 `/app/forge-migrate` 串行迁移
- PDB + TopologySpread/AntiAffinity
- 外部 Secret / KMS/HSM
- 收紧 NetworkPolicy egress
- Prometheus/OTel/集中日志平台

详见 `docs/deployment.md` 和 `docs/operations.md`。

## 配置优先级

**环境变量 / `*_FILE` > Nacos/本地 YAML > 内置默认值**。

敏感字段（DSN、密码、Token、Crypto Key、AccessKey/SecretKey）不会从普通 YAML 加载。Nacos 适合非敏感动态配置；高敏 Secret 应由 Secret/KMS/HSM 管理。

示例见 `.env.example`、`configs/*.yaml`、`deploy/helm/forge/values.yaml`。

## 统一 API 返回

成功：

```json
{
  "code": "000000",
  "message": "success",
  "data": {},
  "request_id": "...",
  "trace_id": "...",
  "timestamp": "..."
}
```

错误码分段：

```text
10xxxx  请求/参数
20xxxx  身份/权限/安全
30xxxx  冲突/状态
40xxxx  基础设施/依赖
50xxxx  业务域预留
90xxxx  平台内部
```

HTTP status 仍表达传输语义，业务不得只看 `200 + code`。CI 会检查 handler 使用的固定错误 symbol 是否已登记。

## 前端产品基座

前端已经包含可直接复用的企业管理端 Shell，而不是只有几张示例页：

- 固定顶栏 + 分组侧边菜单
- 环境标识、全局导航搜索、全屏、明暗主题、紧凑模式
- 路由/菜单/权限单一配置源
- 最近访问页签
- 双栏产品化登录页
- 用户/角色/权限/组织/在线会话/审计/系统状态/账号安全/API Token 页面
- 用户启停、解锁、重置密码和角色调整；提权操作使用独立权限并保护最后一个活动系统管理员
- 403/404/500 + ErrorBoundary
- `Access` / `PermissionButton` / `AppProTable` / `AppPageContainer` / `DetailDrawer` / `MetricCard` / `StatusTag` / `SensitiveText` / `SecretText` / `AppUpload` / `ConfirmAction` 等公共组件
- runtime config：同一前端镜像可在 DEV/SIT/UAT/PRE/PROD 覆盖品牌、Logo、主色、环境、API 地址、导航模式和主题
- 开发环境组件示例页，生产 runtime config 默认关闭
- Vitest + React Testing Library 测试基线

Helm 会把浏览器运行时配置作为独立 ConfigMap 挂载，不需要为了环境差异重新构建前端镜像。敏感信息不得写入 runtime config。

详细约定见 `docs/frontend.md`。

## 中国企业 / 金融基线

Forge 将等级保护、金融数据安全、商用密码等要求转化为**应用层工程控制和扩展点**，但不会声称“用了框架就通过等保/密评”。重点包括：三员分立、最小权限、审计、日志留存配置底线、密码策略、密钥版本、数据脱敏、机器身份、供应链、容灾/恢复和信创兼容矩阵。

详见：

- `docs/compliance-china-financial.md`
- `docs/security.md`
- `docs/operations.md`
- `docs/observability.md`
- `docs/compatibility.md`
- `docs/providers.md`
- `docs/frontend.md`
- `docs/supply-chain.md`

## 创建新项目

```bash
make init APP=my-product MODULE=github.com/your-org/my-product
```

之后只新增业务领域，不复制修改底层 Provider。AI/Codex/Cursor 规则见 `AGENTS.md`。

## 校验

联网环境：

```bash
go mod tidy
cd web && npm install && cd ..
make check
```

CI 还会做 OpenAPI lint、多架构镜像构建、依赖漏洞、安全静态检查、Secret 扫描、Trivy 和 CycloneDX SBOM；tag release 提供 BuildKit provenance/SBOM attestation 与 Cosign digest 签名基线。

### 本次生成包验证状态

当前生成环境无法访问外部 Go/npm 依赖仓库，因此不会伪造“依赖级完整编译通过”。已执行离线 Go 语法/格式、YAML/JSON、Shell、OpenAPI 基础结构、错误码契约、TypeScript/TSX 语法和目录一致性检查。首次联网后应生成并提交 `go.sum` 与前端 lockfile，再由 CI 固化依赖。

## License

Apache-2.0。
