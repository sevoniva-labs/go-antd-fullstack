import { expect, test, type Page } from '@playwright/test'

interface ScenarioOptions {
  manifest?: string
  featureFlags?: string[]
  permissions?: string[]
  scopes?: string[]
}

interface CSPWindow extends Window {
  __forgeCSPViolations?: string[]
}

async function captureCSPViolations(page: Page) {
  await page.addInitScript(() => {
    const target = window as CSPWindow
    target.__forgeCSPViolations = []
    document.addEventListener('securitypolicyviolation', (event) => {
      target.__forgeCSPViolations?.push(`${event.effectiveDirective}:${event.blockedURI}`)
    })
  })
}

async function configureScenario(page: Page, options: ScenarioOptions = {}) {
  const manifest = options.manifest ?? 'manifest.bundle.json'
  const featureFlags = options.featureFlags ?? ['micro_frontend.example_remote']
  const permissions = options.permissions ?? ['example.remote.read']
  const scopes = options.scopes ?? ['organization.current']
  const runtimeConfig = {
    environment: 'E2E',
    apiBaseUrl: '/api/v1',
    componentPlayground: false,
    microFrontendsEnabled: true,
    microAppManifestUrl: `/microapps/example-remote/${manifest}`,
    microAppFeatureFlags: featureFlags,
  }
  await page.route('**/runtime-config.js', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/javascript',
      body: `window.__FORGE_CONFIG__ = ${JSON.stringify(runtimeConfig)};`,
    })
  })
  await page.route('**/api/v1/me', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: '000000',
        request_id: 'e2e-request-id',
        data: {
          principal_type: 'USER',
          user_id: 'e2e-user-1',
          organization_id: 'e2e-org-1',
          login_name: 'e2e.operator',
          display_name: '端到端操作员',
          roles: ['risk_operator'],
          permissions,
          scopes,
          must_change_password: false,
        },
      }),
    })
  })
  await page.route('**/api/v1/system/info', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: '000000',
        request_id: 'e2e-request-id',
        data: {
          application: 'forge',
          environment: 'E2E',
          version: 'e2e',
          providers: { database: 'PostgreSQL', cache: 'Redis' },
          compliance_profile: 'E2E',
        },
      }),
    })
  })
  await page.route('**/api/v1/system/ready', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: '000000',
        request_id: 'e2e-request-id',
        data: {
          status: 'UP',
          checks: [{
            name: 'database',
            provider: 'PostgreSQL',
            status: 'UP',
            duration_ms: 2,
          }],
        },
      }),
    })
  })
}

test('keeps the regular Shell SPA route operational', async ({ page }) => {
  await configureScenario(page)
  await page.goto('/dashboard')

  await expect(page.getByText('基础设施健康', { exact: true })).toBeVisible()
  await expect(page.getByText('当前运行 Profile', { exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/dashboard$/)
})

test('starts a trusted same-origin app through Wujie and Host SDK', async ({ page }) => {
  await captureCSPViolations(page)
  await configureScenario(page)
  const response = await page.goto('/apps/example-remote')

  await expect(page.getByRole('heading', { name: '示例远程应用' })).toBeVisible()
  await expect(page.getByText('风险工作台示例')).toBeVisible()
  await expect(page.getByText('Host SDK 已连接')).toBeVisible()
  await expect(page.getByText('主发布 1.0.0')).toBeVisible()
  const csp = response?.headers()['content-security-policy'] ?? ''
  expect(csp).toContain("script-src 'self' 'unsafe-inline'")
  expect(csp).not.toContain('unsafe-eval')
  expect(await page.evaluate(() => (window as CSPWindow).__forgeCSPViolations ?? [])).toEqual([])
})

test('runs an untrusted app only in an independent-origin iframe', async ({ page }) => {
  await configureScenario(page, { manifest: 'manifest-iframe.bundle.json' })
  await page.goto('/apps/example-remote')

  await expect(page.getByText('独立域 iframe')).toBeVisible()
  const remote = page.frameLocator('iframe[data-microapp-runtime="iframe"]')
  await expect(remote.getByText('远程应用未连接宿主')).toBeVisible()
  await expect(page.locator('iframe[data-microapp-runtime="iframe"]')).toHaveAttribute(
    'sandbox',
    'allow-forms allow-same-origin allow-scripts',
  )
})

test('filters the route before loading a manifest when permission is missing', async ({ page }) => {
  await configureScenario(page, { permissions: [] })
  await page.goto('/apps/example-remote')

  await expect(page.getByText('403')).toBeVisible()
  await expect(page.getByText('风险工作台示例')).toHaveCount(0)
})

test('fails closed when both primary and rollback health checks are unavailable', async ({ page }) => {
  await configureScenario(page)
  await page.route('**/healthz', async (route) => {
    await route.fulfill({ status: 503, body: 'unavailable' })
  })
  await page.goto('/apps/example-remote')

  await expect(page.getByText('微应用当前不可用')).toBeVisible()
  await expect(page.getByText(/运行状态：error/)).toBeVisible()
})

test('switches only to the rollback release declared in the signed manifest', async ({ page }) => {
  await configureScenario(page, { manifest: 'manifest-rollback.bundle.json' })
  await page.goto('/apps/example-remote')

  await expect(page.getByText('已回滚 0.9.0')).toBeVisible()
  await expect(page.getByText('风险工作台示例')).toBeVisible()
  await expect(page.getByText(/已切换至签名清单指定的回滚版本/)).toBeVisible()
})

test('renders Ant Design under script-strict CSP without browser violations', async ({ page }) => {
  await expect.poll(async () => {
    try {
      return (await page.request.get('http://127.0.0.1:4190/runtime-config.js')).status()
    } catch {
      return 0
    }
  }, { timeout: 30_000 }).toBe(200)
  await captureCSPViolations(page)
  await configureScenario(page)
  const response = await page.goto('http://127.0.0.1:4190/dashboard')

  await expect(page.getByText('基础设施健康', { exact: true })).toBeVisible()
  const csp = response?.headers()['content-security-policy'] ?? ''
  const nonce = await page.locator('meta[name="forge-csp-nonce"]').getAttribute('content')
  expect(nonce).toMatch(/^[A-Za-z0-9+/]{32}$/)
  expect(csp).toContain("script-src 'self'")
  expect(csp).not.toContain("script-src 'self' 'unsafe-inline'")
  expect(csp).not.toContain('unsafe-eval')
  expect(csp).toContain("style-src-elem 'self' 'unsafe-inline'")
  await expect.poll(() => page.locator('style').count()).toBeGreaterThan(0)
  expect(await page.evaluate(() => (window as CSPWindow).__forgeCSPViolations ?? [])).toEqual([])
})
