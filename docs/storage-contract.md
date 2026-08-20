# S3 存储能力契约

## 目标

脚手架通过 AWS SDK for Go v2 使用统一 S3 协议适配层，支持 Generic S3、AWS S3、MinIO、Ceph RGW、阿里 OSS、腾讯 COS、华为 OBS 等 `Profile`。产品名称只用于配置路由，不代表兼容性认证。

## Fail-closed 规则

- `New` 只启用基础对象读写；高级能力保持 `Unknown`。
- 分片恢复、校验和、SSE-S3、SSE-KMS、版本、Object Lock、Retention、Legal Hold、受限预签名和临时凭证，必须由目标环境的契约测试逐项证明。
- 统一 `GovernanceStore` 提供受控对象写入、版本读取、版本控制、Retention 和 Legal Hold；写入参数会按请求能力逐项校验并 fail-closed。
- STS/临时凭证通过 `TemporaryCredentialIssuer` 适配位接入，因为不同 S3 产品的 STS 地址、签名和最小权限模型不一致。
- 契约必须标为 `Target-tested`，包含目标标识、测试时间、不可变证据引用和证据文件 SHA-256 摘要。
- 未通过的能力不可通过厂商名称、配置别名或前端开关绕过。

## 启用方式

目标环境完成契约测试后，通过 `storage.NewWithCapabilityContract` 注入契约。契约测试应使用专用测试桶和最小权限账号，覆盖成功、权限拒绝、超时、重试、清理和审计记录；不能把单元测试当作厂商认证。

```go
store, err := storage.NewWithCapabilityContract(ctx, cfg.Storage, targetContract)
```

当前仓库提供协议能力边界和 COS 基础契约验证入口：`make storage-cos-contract`。腾讯 COS 的 `ap-shanghai` 目标已验证基础对象读写、SSE-S3 AES256 和版本读取/列表观察；这不覆盖分片、Checksum、STS、SSE-KMS、Object Lock、Retention、Legal Hold、灾备或任何认证结论。除明确记录的基础证据外，AWS、MinIO、Ceph、OSS、COS、OBS 以及国产对象存储的能力仍属于 `Not certified`，必须在实际交付环境留存证据后再升级标签。

契约执行只从环境变量读取凭据，不把密钥写入仓库：

```bash
FORGE_COS_ACCESS_KEY='测试访问密钥' \
FORGE_COS_SECRET_KEY='测试秘密密钥' \
FORGE_COS_REGION='ap-shanghai' \
FORGE_COS_BUCKET='专用测试桶' \
FORGE_COS_EVIDENCE_FILE='artifacts/storage/tencent-cos-foundation.json' \
FORGE_COS_CONTRACT_FILE='artifacts/storage/tencent-cos-foundation.contract.json' \
make storage-cos-contract
```

高级能力必须单独运行显式契约，默认不会把缺少 KMS、Object Lock 或临时凭据的环境当成通过：

```bash
FORGE_COS_ACCESS_KEY='轮换后的最小权限访问密钥' \
FORGE_COS_SECRET_KEY='轮换后的最小权限秘密密钥' \
FORGE_COS_REGION='ap-shanghai' \
FORGE_COS_ENDPOINT='https://cos.ap-shanghai.myqcloud.com' \
FORGE_COS_BUCKET='专用高级能力测试桶' \
FORGE_COS_ADVANCED_EVIDENCE_FILE='artifacts/storage/tencent-cos-advanced.json' \
make storage-cos-advanced-contract
```

该入口逐项观察分片恢复、Checksum、受限预签名、SSE-KMS、Object Lock、Retention、Legal Hold 和临时凭证；KMS、临时凭证和变更型 Object Lock 测试没有显式材料时记录为 `not-tested`。只有设置 `FORGE_COS_REQUIRE_ADVANCED=true` 才会因未通过或未验证能力返回失败；禁止把未验证能力写入 Target-tested 合同。`FORGE_COS_ALLOW_MUTATING_ADVANCED=true` 仅允许在专用测试桶上执行 Legal Hold 开关测试，Retention 保留对象仍需单独人工批准。

测试身份必须是最小权限专用身份；探针对象使用唯一前缀并在测试结束时删除。若删除失败，脚本返回失败。生成的证据文件不得包含凭据，并应随目标版本、配置和报告一同归档。

通过测试后，可将生成的 `.contract.json` 配置到 `storage.capability_evidence_file` 或 `FORGE_STORAGE_CAPABILITY_EVIDENCE_FILE`。启动时会重新校验目标 profile、证据摘要、测试时间和 `Target-tested` 状态；原始证据文件被替换或摘要不一致时，启动失败关闭。

## Upload quarantine

`QuarantineController` enforces the common regulated-upload path: server-side size and content-type checks, SHA-256 binding, quarantine storage, malware-scan evidence, atomic state transitions, governed promotion, and cleanup retry. The record store must provide an atomic compare-and-swap implementation backed by the application database; the in-memory fakes in tests are not a production store.

The malware scanner and quarantine object store are adapter slots. A clean local test is not evidence that a target S3 provider, scanner, KMS/HSM, or retention policy is certified. Promotion always calls `GovernanceStore.PutGoverned`, so untested S3 capabilities still fail closed.
