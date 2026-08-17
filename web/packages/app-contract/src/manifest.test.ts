import { describe, expect, it } from 'vitest'

import { validateMicroAppManifest, type MicroAppManifest } from './manifest'

const options = { shellOrigin: 'https://portal.bank.example', production: true }

function validManifest(): MicroAppManifest {
  return {
    schemaVersion: '2.0',
    name: 'risk-console',
    displayName: '风险控制台',
    runtime: 'wujie',
    trust: 'trusted-internal',
    status: 'active',
    routePrefix: '/apps/risk-console',
    requiredPermissions: ['risk.case.read'],
    requiredDataScopes: ['organization.current'],
    allowedApiPrefixes: ['/api/v1/risk'],
    events: { publish: ['risk.case-updated'], subscribe: ['shell.theme-changed'] },
    featureFlag: 'micro_frontend.risk_console',
    owner: 'risk-platform-team',
    releases: {
      primary: {
        version: '1.2.3',
        entry: 'https://portal.bank.example/microapps/risk/1.2.3/index.html',
        healthUrl: 'https://portal.bank.example/microapps/risk/1.2.3/healthz',
        digest: 'sha256:' + 'a'.repeat(64),
        resources: [{
          url: 'https://portal.bank.example/microapps/risk/1.2.3/index.html',
          integrity: 'sha256-' + 'A'.repeat(43) + '=',
          maxBytes: 1024 * 1024,
        }],
      },
      rollback: {
        version: '1.2.2',
        entry: 'https://portal.bank.example/microapps/risk/1.2.2/index.html',
        healthUrl: 'https://portal.bank.example/microapps/risk/1.2.2/healthz',
        digest: 'sha256:' + 'b'.repeat(64),
        resources: [{
          url: 'https://portal.bank.example/microapps/risk/1.2.2/index.html',
          integrity: 'sha256-' + 'B'.repeat(43) + '=',
          maxBytes: 1024 * 1024,
        }],
      },
    },
    compatibility: {
      shellApi: 1,
      hostSdkApi: 1,
      designSystemApi: 1,
      apiContract: 'v1',
      reactMajor: 19,
    },
    rollout: {
      strategy: 'stable',
      percentage: 100,
      cohortKey: 'user_id',
      salt: 'risk-console-rollout-2026',
    },
    health: {
      timeoutMs: 3000,
      startupTimeoutMs: 8000,
      maxFailures: 3,
      failureWindowSeconds: 60,
      recoverySeconds: 120,
    },
    csp: {
      connectSrc: ["'self'"],
      imgSrc: ["'self'", 'data:'],
      frameSrc: ["'self'"],
    },
    fallbackPath: '/micro-app-unavailable',
  }
}

describe('validateMicroAppManifest', () => {
  it('accepts complete primary and rollback release inventories', () => {
    const manifest = validateMicroAppManifest(validManifest(), options)
    expect(manifest.schemaVersion).toBe('2.0')
    expect(manifest.releases.rollback?.version).toBe('1.2.2')
    expect(manifest.releases.primary.resources).toHaveLength(1)
  })

  it('rejects runtime and origin trust confusion', () => {
    const crossOrigin = validManifest()
    crossOrigin.releases = {
      primary: {
        ...crossOrigin.releases.primary,
        entry: 'https://external.example/risk/index.html',
        healthUrl: 'https://external.example/risk/healthz',
        resources: [{
          ...crossOrigin.releases.primary.resources[0]!,
          url: 'https://external.example/risk/index.html',
        }],
      },
    }
    expect(() => validateMicroAppManifest(crossOrigin, options)).toThrow(/Wujie releases must be trusted/)

    const sameOriginIframe = validManifest()
    sameOriginIframe.runtime = 'iframe'
    sameOriginIframe.trust = 'untrusted-external'
    expect(() => validateMicroAppManifest(sameOriginIframe, options)).toThrow(/iframe releases must be untrusted/)
  })

  it('rejects wildcard capabilities and unsafe CSP expressions', () => {
    expect(() => validateMicroAppManifest({
      ...validManifest(),
      allowedApiPrefixes: ['/api/*'],
    }, options)).toThrow(/explicit \/api\//)
    expect(() => validateMicroAppManifest({
      ...validManifest(),
      csp: { ...validManifest().csp, connectSrc: ["'unsafe-eval'"] },
    }, options)).toThrow(/forbidden CSP/)
  })

  it('rejects incomplete rollback evidence and duplicate release versions', () => {
    const missingEntry = validManifest()
    missingEntry.releases.primary.resources = []
    expect(() => validateMicroAppManifest(missingEntry, options)).toThrow(/resources must contain/)

    const duplicate = validManifest()
    duplicate.releases.rollback = {
      ...duplicate.releases.rollback!,
      version: duplicate.releases.primary.version,
    }
    expect(() => validateMicroAppManifest(duplicate, options)).toThrow(/different version/)
  })

  it('rejects unknown fields instead of silently ignoring configuration mistakes', () => {
    expect(() => validateMicroAppManifest({
      ...validManifest(),
      legacyEntry: 'https://legacy.example',
    }, options)).toThrow(/unknown fields: legacyEntry/)
  })
})
