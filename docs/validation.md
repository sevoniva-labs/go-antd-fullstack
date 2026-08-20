# 验证证据与边界

本文只记录在当前工作区实际执行并得到成功退出码的验证，不把配置存在、接口预留或文档声明等同于兼容认证。

## 当前已验证

### Latest complete gate

- 2026-08-20: `make verify` passed on the scaffold branch, including Go contract/module/race checks, frontend generated API/lint/typecheck/test/build/budget checks, APISIX and Helm policy checks, Gosec, govulncheck, Staticcheck, golangci-lint, SBOM, Gitleaks history/worktree scans, and license evidence generation.
- 2026-08-21: after isolating generated-code checks and confining capability-evidence reads with `os.Root`, `make verify` passed again; Gosec reported 0 issues, govulncheck reported no code-reachable vulnerabilities, and staticcheck/golangci-lint both reported 0 issues.
- The complete gate does not certify a target vendor, hardware, operating system, HSM/KMS, cluster, air-gapped bundle, or disaster-recovery topology; those require the target evidence workflows below.

### Go 与安全边界

- Go 依赖通过 `https://goproxy.cn` 获取，`go.sum` 已固定。
- Kratos v2 后端单元测试、静态检查和 Phase 1 后端门禁已执行通过。
- HTTP SPA 测试覆盖随机 CSP nonce 注入、脚本严格策略、Wujie 审批策略、独立 `connect-src`/`frame-src` 和签名静态资源 URL 不重定向。
- 配置测试覆盖生产来源必须 HTTPS、拒绝通配符/路径/用户信息/查询参数以及重复来源。

### Tencent COS S3 foundation contract

- 2026-08-21: 已使用腾讯 COS `ap-shanghai` 私有测试桶完成 `HeadBucket`、对象列表、SSE-S3 `AES256` 上传、对象元数据、下载内容校验、删除、版本状态读取和版本列表权限验证。
- 仓库入口为 `make storage-cos-contract`，凭据只从环境变量读取并自动清理探针对象；可通过 `FORGE_COS_EVIDENCE_FILE` 生成不含密钥的 JSON 证据，并通过 `FORGE_COS_CONTRACT_FILE` 生成启动可加载的 Target-tested 合同清单。
- 当前证据只支持基础对象读写、SSE-S3 AES256 和版本读取/列表观察；分片、Checksum、STS、SSE-KMS、Object Lock、Retention、Legal Hold、预签名和灾备仍未验证。

### 前端

- `pnpm install --frozen-lockfile`、workspace typecheck、单元测试和 Vite 8 生产构建已执行通过。
- 构建预算检查已执行通过，覆盖初始/全量 JS raw 与 gzip、chunk 数、最大 chunk、CSS、source map 和 hash 文件名。
- 生产 Wujie 构建审批门禁已执行通过；未审批路径按预期拒绝。
- 首次执行 `make ci-web-e2e` 因本机没有 Playwright 缓存的 Chromium，在浏览器启动前失败；随后使用调用方显式指定的本机 Chrome 执行同一套 7 条生产构建 E2E，结果为 `7 passed`。这证明测试通过，不证明 Playwright 浏览器制品可从海外源下载。
- 浏览器场景覆盖普通 SPA、Wujie + Host SDK、独立域 iframe、缺权拦截、双版本不可用故障关闭、签名清单回滚和默认脚本严格 CSP。

### 部署模板

- Helm 基础与信创 values 均通过 `helm lint`。
- 基础开发配置与信创生产配置均完成 `helm template` 渲染。
- 生产模板会拒绝关闭 NetworkPolicy、缺少明确 ingress/egress 或使用无 digest 镜像；基础默认 values 不能冒充生产可部署配置。
- `make offline-check` 已具备锁文件、信创配置、公共 OCI 源、离线包 provenance、镜像 digest 和 SHA-256 manifest 的静态门禁；未提供 `OFFLINE_BUNDLE_DIR` 时不会宣称离线包验证完成。
- `make disaster-check` 已具备目标版本元数据、RPO/RTO、节点/网络/数据库/MQ/S3/机房/备份恢复七类场景的证据格式校验；当前示例仍是 `Not certified`，必须由真实目标环境报告通过 `disaster-check-certified`。

## 可复现命令

```bash
GOPROXY=https://goproxy.cn \
GOSUMDB='sum.golang.org https://goproxy.cn/sumdb/sum.golang.org' \
go test ./internal/platform/config ./internal/platform/httpserver ./internal/bootstrap

make ci-web

PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 \
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/path/to/chrome \
make ci-web-e2e

helm lint deploy/helm/forge
helm lint deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml
helm template forge deploy/helm/forge --set config.environment=development >/tmp/forge-base.yaml
helm template forge deploy/helm/forge -f deploy/helm/forge/values-xinchuang.yaml >/tmp/forge-xinchuang.yaml

make offline-check
OFFLINE_BUNDLE_DIR=/path/to/approved-bundle make offline-check
```

所有自动下载入口必须显式使用国内源，并在国内源失败时立即失败。禁止 `direct` 或其他参数造成不受控的海外静默回退；生产 CI 应优先使用组织内 Go Proxy/制品代理。

## 国内浏览器制品边界

受控安装脚本只在已验证平台启用公开国内镜像。当前证据仅包括 Linux ARM64 Playwright Chromium 制品地址的可达性检查；macOS 对应公开镜像返回不可用，因此本次 macOS E2E 使用本机已安装 Chrome。

这证明浏览器场景可运行，不证明 macOS 浏览器二进制能够从国内镜像重复安装。银行内网 CI 应将审核后的 Chromium 制品同步到 Harbor/制品库，固定版本和摘要，再通过显式路径执行。

## 尚未验证，不得宣称

- OTel Collector 真实进程联调：当前环境没有提供 `OTELCOL_BIN`。
- 真实 Nacos 3、RocketMQ 5、Kafka、Redis 集群和除腾讯 COS 基础契约外的各云厂商 S3 端点兼容认证。
- OceanBase、达梦、人大金仓、GaussDB 等国产数据库的驱动、SQL 方言、迁移、故障切换和性能认证。
- 国密 SSL、SM2 证书链、HSM/密码机、KMS 和密钥轮换演练。
- 组织内 Harbor、多架构镜像、离线包、Kubernetes 集群、备份恢复和两地三中心容灾演练。
- 等保测评、密评、金融监管验收或任何厂商认证。

这些项目必须在目标机构的网络、硬件、数据库、中间件和安全设备上形成独立测试报告，不能由脚手架单元测试替代。
## Identity runtime contract evidence (2026-08-21)

- Using Lima rootful nerdctl and the domestic ARM64 image digests `docker.m.daocloud.io/bitnamilegacy/openldap@sha256:687f14a22b5c74fb057a57221acdbe7b8c82e2d3619fc380db3af48ec4aa04ed` and `quay.m.daocloud.io/keycloak/keycloak@sha256:98fab020a3a490aba0978f237e2a06cd0ea42bf149c6cf10f11c0aaf27728ff2`, `scripts/test-identity-contract.sh` passed LDAP test-user bind/search, Keycloak management health, and master-realm OIDC discovery.
- Test secrets were temporary environment values and were not recorded. This is exact local development-profile evidence only, not production or regulatory certification.

## PostgreSQL runtime evidence (2026-08-21)

- A local Linux ARM64 runtime contract used the domestic PostgreSQL image digest `docker.m.daocloud.io/library/postgres@sha256:cf78e76683b9ca8c5733cbbdce6c9262b45b6767934dd0a95e671f9a0fc20685` through Lima/nerdctl.
- `cmd/migrate` applied PostgreSQL migrations `00001` through `00022` successfully. The runtime check exposed an existing `organizations.updated_at` schema drift during bootstrap; migration `00022_organization_updated_at.sql` closes that drift for PostgreSQL and MySQL.
- `cmd/server` then started successfully against the migrated database. `GET /api/v1/system/health` and `GET /api/v1/system/ready` both returned `status: UP`, including database, cache, messaging, search, and storage readiness checks in the minimal local profile.
- This is local PostgreSQL runtime evidence only. It does not certify MySQL runtime behavior, distributed dependencies, Xinchuang target systems, or any regulatory compliance outcome.

## Nacos runtime contract evidence (2026-08-21)

- `scripts/test-nacos-contract.sh` provides a disposable Nacos 3 runtime contract using an approved immutable image digest and local-only environment secrets. It validates the console readiness endpoint, server readiness endpoint, anonymous configuration access rejection, and the Nacos Base64 authentication-token minimum length before startup.
- 2026-08-21: the exact local Nacos image was started through Lima rootful nerdctl from the domestic mirror digest `docker.m.daocloud.io/nacos/nacos-server@sha256:a223937902d4292e49ce6bcca8c9d47d29d508075b7f7c6ba98e1a34ff9c3f3b`. The contract passed the console readiness endpoint on the compose-mapped console port, the server readiness endpoint returning `data: "ok"`, anonymous configuration access rejection, and the token length policy.
- Run `make nacos-runtime-contract` with a rotated, temporary Base64 token that decodes to at least 32 bytes and set `FORGE_NACOS_EVIDENCE_FILE` for a non-secret JSON record. This is local development evidence only, not Nacos cluster, TLS, HA, Xinchuang, production, or regulatory certification.

## Redis runtime contract evidence (2026-08-21)

- `scripts/test-redis-contract.sh` provides a disposable Redis runtime contract using an approved immutable image digest and a local-only password. It validates authenticated `PING`, rejection of unauthenticated `PING`, authenticated `SET`/`GET`, and a positive key TTL.
- 2026-08-21: the exact local Redis image `docker.m.daocloud.io/library/redis@sha256:fbdbaea47b9ae4ecc2082ecdb4e1cea81e32176ffb1dcf643d422ad07427e5d9` was pulled from the domestic mirror and the standalone runtime contract passed with a temporary local password. Sentinel, Cluster, TLS, ACL rotation, persistence recovery, and production failover remain unverified until their target evidence is executed.
- Run `make redis-runtime-contract` with a temporary local password and set `FORGE_REDIS_EVIDENCE_FILE` for a non-secret JSON record. This is standalone development evidence only, not Redis topology, production, Xinchuang, or regulatory certification.

## RocketMQ runtime contract evidence (2026-08-21)

- `scripts/test-rocketmq-contract.sh` provides a disposable RocketMQ 5 development contract with nameserver, broker and proxy. It waits for broker readiness, creates a unique topic, waits for nameserver route visibility, and runs the same RocketMQ Go SDK path through a Linux ARM64 helper inside the broker network to produce, receive and acknowledge a message.
- 2026-08-21: the exact domestic mirror image `docker.m.daocloud.io/apache/rocketmq@sha256:455287638deddbbcd6cf48cdf988ed4d05bbac7265acc0c399d9d8228862a7d7` passed the standalone contract. The local overlay uses plaintext gRPC and an empty namespace only for disposable development; production TLS, ACL2, namespace isolation, FIFO/delay/transaction semantics, retry/DLQ, cluster HA, persistence recovery and failover remain unverified.
- Run `make rocketmq-runtime-contract` with temporary local access/secret values and set `FORGE_ROCKETMQ_EVIDENCE_FILE` for a non-secret JSON record. This is standalone development evidence only, not RocketMQ production, Xinchuang, or regulatory certification.
