import {
  validateMicroAppManifest,
  type ManifestValidationOptions,
  type MicroAppManifest,
  type MicroAppRelease,
} from '@forge/app-contract'

type ReleaseOverride = Partial<Omit<MicroAppRelease, 'resources'>> & {
  resources?: MicroAppRelease['resources']
}

export type ManifestOverrides = Partial<Omit<
  MicroAppManifest,
  'releases' | 'compatibility' | 'rollout' | 'health' | 'csp' | 'events'
>> & {
  releases?: {
    primary?: ReleaseOverride
    rollback?: ReleaseOverride | null
  }
  compatibility?: Partial<MicroAppManifest['compatibility']>
  rollout?: Partial<MicroAppManifest['rollout']>
  health?: Partial<MicroAppManifest['health']>
  csp?: Partial<MicroAppManifest['csp']>
  events?: Partial<MicroAppManifest['events']>
}

const integrityA = 'sha256-' + 'A'.repeat(43) + '='
const integrityB = 'sha256-' + 'B'.repeat(43) + '='

export function createMicroAppManifest(overrides: ManifestOverrides = {}): MicroAppManifest {
  const primary: MicroAppRelease = {
    version: '1.0.0',
    entry: 'https://portal.bank.example/microapps/example-remote/1.0.0/index.html',
    healthUrl: 'https://portal.bank.example/microapps/example-remote/1.0.0/healthz',
    digest: 'sha256:' + 'a'.repeat(64),
    resources: [{
      url: 'https://portal.bank.example/microapps/example-remote/1.0.0/index.html',
      integrity: integrityA,
      maxBytes: 2 * 1024 * 1024,
    }],
  }
  const rollback: MicroAppRelease = {
    version: '0.9.0',
    entry: 'https://portal.bank.example/microapps/example-remote/0.9.0/index.html',
    healthUrl: 'https://portal.bank.example/microapps/example-remote/0.9.0/healthz',
    digest: 'sha256:' + 'b'.repeat(64),
    resources: [{
      url: 'https://portal.bank.example/microapps/example-remote/0.9.0/index.html',
      integrity: integrityB,
      maxBytes: 2 * 1024 * 1024,
    }],
  }
  const base: MicroAppManifest = {
    schemaVersion: '2.0',
    name: 'example-remote',
    displayName: '示例远程应用',
    runtime: 'wujie',
    trust: 'trusted-internal',
    status: 'active',
    routePrefix: '/apps/example-remote',
    requiredPermissions: ['example.remote.read'],
    requiredDataScopes: ['organization.current'],
    allowedApiPrefixes: ['/api/v1/example-remote'],
    events: {
      publish: ['example.record-updated'],
      subscribe: ['shell.theme-changed'],
    },
    featureFlag: 'micro_frontend.example_remote',
    owner: 'platform-team',
    releases: { primary, rollback },
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
      salt: 'example-remote-2026-01',
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

  const rollbackOverride = overrides.releases?.rollback
  return {
    ...base,
    ...overrides,
    releases: {
      primary: { ...primary, ...overrides.releases?.primary },
      ...(rollbackOverride === null
        ? {}
        : { rollback: { ...rollback, ...rollbackOverride } }),
    },
    compatibility: { ...base.compatibility, ...overrides.compatibility },
    rollout: { ...base.rollout, ...overrides.rollout },
    health: { ...base.health, ...overrides.health },
    csp: { ...base.csp, ...overrides.csp },
    events: { ...base.events, ...overrides.events },
  }
}

export function createValidatedMicroAppManifest(
  overrides: ManifestOverrides,
  options: ManifestValidationOptions,
): MicroAppManifest {
  return validateMicroAppManifest(createMicroAppManifest(overrides), options)
}
