export type MicroAppRuntime = 'wujie' | 'iframe'
export type MicroAppStatus = 'active' | 'maintenance' | 'disabled'
export type RolloutStrategy = 'stable' | 'canary'

export interface MicroAppArtifact {
  version: string
  digest: string
  integrity: string
  signature: string
  keyId: string
  rollbackVersion?: string
}

export interface MicroAppCompatibility {
  shell: string
  react: string
  designSystem: string
  apiContract: string
}

export interface MicroAppRollout {
  strategy: RolloutStrategy
  percentage: number
  cohortKey: string
}

export interface MicroAppHealth {
  timeoutMs: number
  maxFailures: number
  recoverySeconds: number
}

export interface MicroAppCSP {
  connectSrc: string[]
  imgSrc: string[]
  frameSrc: string[]
}

export interface MicroAppManifest {
  schemaVersion: '1.0'
  name: string
  displayName: string
  runtime: MicroAppRuntime
  status: MicroAppStatus
  entry: string
  routePrefix: string
  requiredPermissions: string[]
  requiredDataScopes: string[]
  allowedApiPrefixes: string[]
  featureFlag?: string
  owner: string
  artifact: MicroAppArtifact
  compatibility: MicroAppCompatibility
  rollout: MicroAppRollout
  health: MicroAppHealth
  csp: MicroAppCSP
  fallbackPath: string
}

export interface ManifestValidationOptions {
  shellOrigin: string
  production: boolean
  trustedWujieOrigins: readonly string[]
}

export class ManifestValidationError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ManifestValidationError'
  }
}

const namePattern = /^[a-z][a-z0-9-]{2,63}$/
const permissionPattern = /^[a-z][a-z0-9_.:-]{2,127}$/
const versionPattern = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/
const digestPattern = /^sha256:[a-f0-9]{64}$/
const integrityPattern = /^sha384-[A-Za-z0-9+/]+={0,2}$/
const reservedRoutes = ['/login', '/403', '/404', '/500', '/account']

function record(value: unknown, name: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new ManifestValidationError(name + ' must be an object')
  }
  return value as Record<string, unknown>
}

function text(value: unknown, name: string, maximum = 256): string {
  if (typeof value !== 'string' || value.trim() === '' || value.length > maximum) {
    throw new ManifestValidationError(name + ' must be a non-empty string')
  }
  return value.trim()
}

function stringList(value: unknown, name: string, maximum = 64): string[] {
  if (!Array.isArray(value) || value.length > maximum) {
    throw new ManifestValidationError(name + ' must be an array with at most ' + maximum + ' values')
  }
  const values = value.map((item, index) => text(item, name + '[' + index + ']', 256))
  if (new Set(values).size !== values.length) {
    throw new ManifestValidationError(name + ' cannot contain duplicates')
  }
  return values
}

function integer(value: unknown, name: string, minimum: number, maximum: number): number {
  if (!Number.isInteger(value) || Number(value) < minimum || Number(value) > maximum) {
    throw new ManifestValidationError(name + ' must be an integer between ' + minimum + ' and ' + maximum)
  }
  return Number(value)
}

function path(value: unknown, name: string): string {
  const result = text(value, name, 256)
  if (!result.startsWith('/') || result.includes('..') || result.includes('?') || result.includes('#')) {
    throw new ManifestValidationError(name + ' must be an absolute normalized path')
  }
  return result.replace(/\/+$/, '') || '/'
}

function validateEntry(value: unknown, runtime: MicroAppRuntime, options: ManifestValidationOptions): string {
  const entry = text(value, 'entry', 2048)
  let url: URL
  try {
    url = new URL(entry, options.shellOrigin)
  } catch {
    throw new ManifestValidationError('entry must be a valid URL')
  }
  if (url.username || url.password || url.hash) {
    throw new ManifestValidationError('entry cannot contain credentials or a fragment')
  }
  if (options.production && url.protocol !== 'https:') {
    throw new ManifestValidationError('production entry must use HTTPS')
  }
  const shellOrigin = new URL(options.shellOrigin).origin
  if (runtime === 'iframe' && url.origin === shellOrigin) {
    throw new ManifestValidationError('iframe applications must use an independent origin')
  }
  const trustedOrigins = new Set(options.trustedWujieOrigins.map((origin) => new URL(origin).origin))
  if (runtime === 'wujie' && !trustedOrigins.has(url.origin)) {
    throw new ManifestValidationError('Wujie entry origin is not trusted')
  }
  return url.toString()
}

function validateAPIPrefixes(value: unknown): string[] {
  const prefixes = stringList(value, 'allowedApiPrefixes', 32).map((item) => path(item, 'allowedApiPrefixes'))
  if (prefixes.some((item) => !item.startsWith('/api/') || item.includes('*'))) {
    throw new ManifestValidationError('allowedApiPrefixes must use explicit /api/ namespaces')
  }
  return prefixes
}

function validatePermissionList(value: unknown, name: string): string[] {
  const permissions = stringList(value, name)
  if (permissions.some((permission) => !permissionPattern.test(permission))) {
    throw new ManifestValidationError(name + ' contains an invalid identifier')
  }
  return permissions
}

export function validateMicroAppManifest(input: unknown, options: ManifestValidationOptions): MicroAppManifest {
  const source = record(input, 'manifest')
  if (source.schemaVersion !== '1.0') throw new ManifestValidationError('unsupported manifest schemaVersion')
  const name = text(source.name, 'name', 64)
  if (!namePattern.test(name)) throw new ManifestValidationError('name must use lowercase kebab-case')
  const runtime = source.runtime
  if (runtime !== 'wujie' && runtime !== 'iframe') throw new ManifestValidationError('runtime must be wujie or iframe')
  const status = source.status
  if (status !== 'active' && status !== 'maintenance' && status !== 'disabled') {
    throw new ManifestValidationError('status is invalid')
  }
  const routePrefix = path(source.routePrefix, 'routePrefix')
  if (reservedRoutes.some((reserved) => routePrefix === reserved || routePrefix.startsWith(reserved + '/'))) {
    throw new ManifestValidationError('routePrefix overlaps a reserved Shell route')
  }

  const artifactSource = record(source.artifact, 'artifact')
  const version = text(artifactSource.version, 'artifact.version', 64)
  const rollbackVersion = artifactSource.rollbackVersion === undefined
    ? undefined
    : text(artifactSource.rollbackVersion, 'artifact.rollbackVersion', 64)
  if (!versionPattern.test(version) || (rollbackVersion !== undefined && !versionPattern.test(rollbackVersion))) {
    throw new ManifestValidationError('artifact versions must use semantic versions')
  }
  const digest = text(artifactSource.digest, 'artifact.digest', 96)
  const integrity = text(artifactSource.integrity, 'artifact.integrity', 160)
  if (!digestPattern.test(digest) || !integrityPattern.test(integrity)) {
    throw new ManifestValidationError('artifact digest or integrity is invalid')
  }

  const compatibilitySource = record(source.compatibility, 'compatibility')
  const rolloutSource = record(source.rollout, 'rollout')
  const healthSource = record(source.health, 'health')
  const cspSource = record(source.csp, 'csp')
  const strategy = rolloutSource.strategy
  if (strategy !== 'stable' && strategy !== 'canary') throw new ManifestValidationError('rollout.strategy is invalid')

  return {
    schemaVersion: '1.0',
    name,
    displayName: text(source.displayName, 'displayName', 128),
    runtime,
    status,
    entry: validateEntry(source.entry, runtime, options),
    routePrefix,
    requiredPermissions: validatePermissionList(source.requiredPermissions, 'requiredPermissions'),
    requiredDataScopes: validatePermissionList(source.requiredDataScopes, 'requiredDataScopes'),
    allowedApiPrefixes: validateAPIPrefixes(source.allowedApiPrefixes),
    featureFlag: source.featureFlag === undefined ? undefined : text(source.featureFlag, 'featureFlag', 128),
    owner: text(source.owner, 'owner', 256),
    artifact: {
      version,
      digest,
      integrity,
      signature: text(artifactSource.signature, 'artifact.signature', 2048),
      keyId: text(artifactSource.keyId, 'artifact.keyId', 128),
      rollbackVersion,
    },
    compatibility: {
      shell: text(compatibilitySource.shell, 'compatibility.shell', 64),
      react: text(compatibilitySource.react, 'compatibility.react', 64),
      designSystem: text(compatibilitySource.designSystem, 'compatibility.designSystem', 64),
      apiContract: text(compatibilitySource.apiContract, 'compatibility.apiContract', 64),
    },
    rollout: {
      strategy,
      percentage: integer(rolloutSource.percentage, 'rollout.percentage', 0, 100),
      cohortKey: text(rolloutSource.cohortKey, 'rollout.cohortKey', 128),
    },
    health: {
      timeoutMs: integer(healthSource.timeoutMs, 'health.timeoutMs', 1000, 30000),
      maxFailures: integer(healthSource.maxFailures, 'health.maxFailures', 1, 20),
      recoverySeconds: integer(healthSource.recoverySeconds, 'health.recoverySeconds', 5, 3600),
    },
    csp: {
      connectSrc: stringList(cspSource.connectSrc, 'csp.connectSrc', 32),
      imgSrc: stringList(cspSource.imgSrc, 'csp.imgSrc', 32),
      frameSrc: stringList(cspSource.frameSrc, 'csp.frameSrc', 32),
    },
    fallbackPath: path(source.fallbackPath, 'fallbackPath'),
  }
}
