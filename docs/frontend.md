# Frontend foundation

Forge 的前端不是业务 Demo，而是公共产品基座。目标是让新项目通过配置和业务模块扩展，而不是重复实现 Layout、菜单、权限、列表、状态、错误页和登录页。

## App Shell

默认提供：

- `ProLayout` mix 模式：固定顶栏 + 侧边菜单
- 运行环境标识：DEV/SIT/UAT/PRE/PROD
- 全局菜单搜索
- 标准/紧凑密度
- 全屏
- 明暗主题
- 用户菜单
- 面包屑/页面容器
- 最近访问页签（可通过 runtime config 关闭）

路由与菜单由 `web/src/app/router/routes.tsx` 单一来源生成。禁止在 Layout 里再维护另一套菜单。

## Runtime config

`web/public/runtime-config.js` 在应用 JS 之前加载。镜像构建完成后仍可以覆盖：

```js
window.__FORGE_CONFIG__ = {
  appName: '研发管理平台',
  appShortName: 'RDP',
  environment: 'UAT',
  apiBaseUrl: '/api/v1',
  helpUrl: '/docs',
  primaryColor: '#1677FF',
  defaultTheme: 'light',
  compactMode: false,
  layoutMode: 'mix', // mix | side | top
  pageTabs: true,
  showEnvironmentBadge: true,
  componentPlayground: false
}
```

这样可以做到 **build once, deploy many**。Helm 会把 `webRuntime` 渲染成独立 ConfigMap 并挂载到 `/app/web/runtime-config.js`。

敏感配置不得放到 runtime config：它会被浏览器读取。

## Reusable components

基础组件目录：

```text
components/
├── access/
│   ├── Access
│   └── PermissionButton
├── data-display/
│   ├── BoolTag
│   ├── CopyText
│   ├── DateTimeText
│   ├── SecretText
│   ├── SensitiveText
│   └── StatusTag
├── feedback/
│   ├── AppErrorBoundary
│   ├── EmptyState
│   ├── ErrorState
│   └── PageLoading
├── form/
│   └── FormSection
├── layout/
│   ├── AppPageContainer
│   ├── DetailDrawer
│   ├── MetricCard
│   └── SearchToolbar
├── security/
│   └── ConfirmAction
├── table/
│   └── AppProTable
└── upload/
    └── AppUpload
```

业务页面优先复用这些组件。`AppUpload` 只提供客户端体验级大小/类型约束，服务端仍必须执行独立文件安全校验。若一个 UI 模式在两个以上业务模块重复，应优先沉淀到公共组件，而不是复制 CSS/JSX。

## Core pages

当前基座包含：

- 工作台
- 用户管理
  - 创建
  - 角色调整
  - 启用/停用
  - 解锁
  - 管理员重置密码
- 角色管理 / 权限组合
- 权限清单
- 当前组织信息
- 在线会话 / 强制下线
- 审计日志
- 安全基线
- 系统状态
- 账号安全
- API Token
- 403 / 404 / 500
- 开发环境组件示例页（production runtime config 默认关闭）

角色是**组织级**的；权限标识是代码/API 契约定义的全局能力。这样既避免租户间角色授权串扰，又避免在页面中动态制造后端无法识别的权限字符串。

## Auth and permission UX

后端永远是权限最终裁决者。前端只做：

- 菜单过滤
- 路由 `RequirePermission`
- 按钮/操作 `Access`
- 用户角色分配使用独立 `system.user.role.manage` 权限，避免将普通用户维护权限等同于提权权限
- 403 页面
- 首次登录强制改密跳转

禁止仅依赖“隐藏按钮”实现安全控制。后端还会保护最后一个启用状态的 `system_admin`，防止管理操作导致组织完全失去系统管理员。

## Theme

主题 Token 集中在 `web/src/theme/index.ts`。业务页面禁止大面积直接写品牌色。允许：

- Ant Design semantic token
- 统一状态组件
- 业务图表自身的语义颜色

品牌名、Logo、主色、帮助链接、环境标识、导航模式由 runtime config 控制。生产环境默认关闭组件示例入口。

## API

- 所有 HTTP 请求统一走 `api/client.ts`
- 统一 Envelope / ApiError
- Request ID / Trace ID 在错误对象中保留
- API 契约以 `api/gen/openapi/openapi.yaml` 为准，该文件由 Proto 确定性生成
- `npm run api:types` 生成 OpenAPI TypeScript 类型
- 新接口必须先更新 OpenAPI，再更新调用层

## Testing

已预留 Vitest + React Testing Library 基线：

```bash
npm test
npm run typecheck
npm run build
```

当前基础测试覆盖权限判断与公共状态组件。业务模块应继续增加关键表单、权限边界和错误处理测试。

E2E 建议由具体产品根据实际认证方式（Local/OIDC/统一认证）接入 Playwright，并运行在可控测试环境，不在公共脚手架里内置固定账号密码。

## Browser telemetry extension

`app/telemetry/browser.ts` 统一派发 `forge:frontend-error` DOM 事件，覆盖 React ErrorBoundary、`window.error` 与 `unhandledrejection`。公共基座不绑定具体浏览器 APM 厂商；具体产品可以在一个适配点接入内部前端监控、OpenTelemetry 或经审批的第三方 SDK。
