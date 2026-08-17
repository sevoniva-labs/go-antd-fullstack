import { describe, expect, it } from 'vitest'
import { ManifestValidationError, validateMicroAppManifest } from './manifest'

const options = {
  shellOrigin: 'https://forge.bank.example',
  production: true,
  trustedWujieOrigins: ['https://forge.bank.example', 'https://apps.bank.example'],
} as const

function manifest(overrides: Record<string, unknown> = {}) {
  return {
    schemaVersion: '1.0',
    name: 'credit-review',
    displayName: '授信复核',
    runtime: 'wujie',
    status: 'active',
    entry: 'https://apps.bank.example/credit-review/1.2.3/',
    routePrefix: '/credit-review',
    requiredPermissions: ['credit.review.read'],
    requiredDataScopes: ['organization'],
    allowedApiPrefixes: ['/api/v1/credit-review'],
    owner: 'credit-platform',
    artifact: {
      version: '1.2.3',
      rollbackVersion: '1.2.2',
      digest: 'sha256:' + 'a'.repeat(64),
      integrity: 'sha384-' + 'A'.repeat(64),
      signature: 'signed-manifest-value',
      keyId: 'frontend-release-2026',
    },
    compatibility: { shell: '1.x', react: '19.x', designSystem: '0.2.x', apiContract: '1.x' },
    rollout: { strategy: 'canary', percentage: 10, cohortKey: 'user_id' },
    health: { timeoutMs: 8000, maxFailures: 3, recoverySeconds: 60 },
    csp: { connectSrc: ["'self'"], imgSrc: ["'self'"], frameSrc: [] },
    fallbackPath: '/micro-app-unavailable',
    ...overrides,
  }
}

describe('validateMicroAppManifest', () => {
  it('accepts a signed Wujie manifest from a trusted origin', () => {
    expect(validateMicroAppManifest(manifest(), options).name).toBe('credit-review')
  })

  it('rejects an untrusted Wujie origin and a same-origin iframe', () => {
    expect(() => validateMicroAppManifest(manifest({ entry: 'https://evil.example/app/' }), options))
      .toThrow(ManifestValidationError)
    expect(() => validateMicroAppManifest(manifest({
      runtime: 'iframe', entry: 'https://forge.bank.example/vendor/',
    }), options)).toThrow(/independent origin/)
  })

  it('rejects wildcard API access and reserved Shell routes', () => {
    expect(() => validateMicroAppManifest(manifest({ allowedApiPrefixes: ['/api/*'] }), options)).toThrow(/namespace/)
    expect(() => validateMicroAppManifest(manifest({ routePrefix: '/account/remote' }), options)).toThrow(/reserved/)
  })

  it('rejects missing artifact evidence', () => {
    const unsigned = manifest({ artifact: { version: '1.2.3' } })
    expect(() => validateMicroAppManifest(unsigned, options)).toThrow(ManifestValidationError)
  })
})
