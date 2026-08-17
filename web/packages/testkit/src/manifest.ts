import {
  validateMicroAppManifest,
  type ManifestValidationOptions,
  type MicroAppManifest,
} from '@forge/app-contract'

export type ManifestOverrides = Partial<Omit<
  MicroAppManifest,
  'artifact' | 'compatibility' | 'rollout' | 'health' | 'csp'
>> & {
  artifact?: Partial<MicroAppManifest['artifact']>
  compatibility?: Partial<MicroAppManifest['compatibility']>
  rollout?: Partial<MicroAppManifest['rollout']>
  health?: Partial<MicroAppManifest['health']>
  csp?: Partial<MicroAppManifest['csp']>
}

export function createMicroAppManifest(overrides: ManifestOverrides = {}): MicroAppManifest {
  const base: MicroAppManifest = {
    schemaVersion: '1.0',
    name: 'example-remote',
    displayName: '示例远程应用',
    runtime: 'wujie',
    status: 'active',
    entry: 'https://apps.bank.example/example-remote/1.0.0/',
    routePrefix: '/example-remote',
    requiredPermissions: ['example.remote.read'],
    requiredDataScopes: ['organization'],
    allowedApiPrefixes: ['/api/v1/example-remote'],
    featureFlag: 'micro_frontend.example_remote',
    owner: 'platform-team',
    artifact: {
      version: '1.0.0',
      digest: 'sha256:' + 'a'.repeat(64),
      integrity: 'sha384-' + 'A'.repeat(64),
      signature: 'test-signature',
      keyId: 'test-release-key',
      rollbackVersion: '0.9.0',
    },
    compatibility: {
      shell: '1.x',
      react: '19.x',
      designSystem: '0.2.x',
      apiContract: '1.x',
    },
    rollout: {
      strategy: 'stable',
      percentage: 100,
      cohortKey: 'user_id',
    },
    health: {
      timeoutMs: 8000,
      maxFailures: 3,
      recoverySeconds: 60,
    },
    csp: {
      connectSrc: ["'self'"],
      imgSrc: ["'self'", 'data:'],
      frameSrc: [],
    },
    fallbackPath: '/micro-app-unavailable',
  }
  return {
    ...base,
    ...overrides,
    artifact: { ...base.artifact, ...overrides.artifact },
    compatibility: { ...base.compatibility, ...overrides.compatibility },
    rollout: { ...base.rollout, ...overrides.rollout },
    health: { ...base.health, ...overrides.health },
    csp: { ...base.csp, ...overrides.csp },
  }
}

export function createValidatedMicroAppManifest(
  overrides: ManifestOverrides,
  options: ManifestValidationOptions,
): MicroAppManifest {
  return validateMicroAppManifest(createMicroAppManifest(overrides), options)
}
