# AGENTS.md — Sevoniva Forge Engineering Rules

## Architecture
- Keep `cmd/*` thin. Composition belongs in `internal/bootstrap`.
- `internal/domain` MUST NOT import HTTP, DB, Redis, Kafka, Nacos, search, storage or vendor SDKs.
- Optional infrastructure is accessed through Provider interfaces and must not become a minimal-mode hard dependency.
- Do not add microservices merely for layering; modular monolith is the default.

## Backend
- Go + chi. Use `context.Context` and bounded timeouts on all external I/O.
- New APIs MUST use `httpx.Success/Error`; do not invent response envelopes.
- New literal error symbols MUST be added to `internal/platform/httpx/codes.go`; CI runs `scripts/check-error-codes.py`.
- Stable code ranges: 10 request, 20 security, 30 conflict/state, 40 dependency, 50 domain, 90 platform.
- Never log authorization/cookie/password/token/secret/private keys or unmasked sensitive data.
- Browser writes require CSRF. Machine API uses Bearer API Token + scopes/permissions.
- Use Permission-based authorization; avoid handler-local role checks.
- For DB + message changes, prefer Transactional Outbox. For repeatable external writes, add idempotency.
- Retry only idempotent/safe operations and always bound retries/timeouts.

## Database
- Business code must not depend on driver-specific types.
- SQL/DDL differences stay inside database migrations/repository adapters.
- PostgreSQL/MySQL are built-in; OceanBase is a validated-by-project MySQL-compatible profile.
- Do not claim Kingbase/DM/GaussDB support until integration tests pass on the target versions.

## Frontend
- React 19 + TypeScript + Ant Design 6 + Pro Components.
- Server state uses TanStack Query; avoid copying server data into a global store.
- API calls stay in `web/src/api`; UI components do not call `fetch` directly.
- Permission UI is convenience only; backend authorization remains authoritative.

## Security / China financial profile
- No source-controlled default passwords or long-lived secrets.
- Production profile uses secure cookies, centralized logs, Redis for multi-instance security state, S3-compatible storage for multi-replica files, and hardened NetworkPolicy.
- GM provider is SM3/SM4 application crypto baseline, not a substitute for commercial cryptography assessment, SM2/cert/HSM/KMS design.
- Keep compatibility claims factual: Built-in / Profile / Adapter slot / Target-tested.

## Delivery
- Update OpenAPI when endpoints or response contracts change.
- Add/adjust tests for security-sensitive behavior.
- Keep Docker multi-arch and Helm non-root/read-only compatible.
- Keep SBOM, vulnerability scan, secret scan and dependency review gates green.

## Frontend foundation rules

- 路由、菜单、页面标题和权限只在 `web/src/app/router/routes.tsx` 定义一次；禁止在 Layout 维护第二份菜单。
- 页面容器优先 `AppPageContainer`，列表优先 `AppProTable`，状态优先 `StatusTag/BoolTag`。
- 权限按钮使用 `Access`/`PermissionButton`；后端必须再次校验权限。用户角色分配使用独立 `system.user.role.manage`，不得复用普通用户编辑权限。
- 业务页面不得直接读取 Cookie/拼接 `/api/v1`，统一通过 `api/client.ts`。
- 环境、品牌、帮助链接等浏览器公开配置使用 `runtime-config.js`；任何 Secret/Token/DSN 禁止进入前端配置。
- 角色是组织级资源；权限标识是后端/API 契约定义的稳定能力，不允许前端任意创建。
- 新增页面必须考虑 loading / empty / 403 / API error / responsive 状态。
- 文件上传可复用 `AppUpload` 做客户端大小/类型提示，但服务端必须独立验证文件名、大小、类型、内容和恶意文件。
- 两个以上模块重复出现的 JSX/CSS 模式应提炼公共组件。
- 主题和品牌色优先使用 Ant Design Token；禁止散落大量硬编码品牌色。
- 新增关键组件或权限逻辑必须补 Vitest/RTL 测试。
