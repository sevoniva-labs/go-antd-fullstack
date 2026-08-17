export type MicroAppRuntime = 'wujie' | 'iframe'
export type MicroAppTrust = 'trusted-internal' | 'untrusted-external'
export type MicroAppStatus = 'active' | 'maintenance' | 'disabled'
export type RolloutStrategy = 'stable' | 'canary'
export type RolloutCohortKey = 'user_id' | 'organization_id'

export interface MicroAppResource {
  url: string
  integrity: string
  maxBytes: number
}

export interface MicroAppRelease {
  version: string
  entry: string
  healthUrl: string
  digest: string
  resources: MicroAppResource[]
}

export interface MicroAppReleaseSet {
  primary: MicroAppRelease
  rollback?: MicroAppRelease
}

export interface MicroAppCompatibility {
  shellApi: 1
  hostSdkApi: 1
  designSystemApi: 1
  apiContract: 'v1'
  reactMajor: 19
}

export interface MicroAppRollout {
  strategy: RolloutStrategy
  percentage: number
  cohortKey: RolloutCohortKey
  salt: string
}

export interface MicroAppHealth {
  timeoutMs: number
  startupTimeoutMs: number
  maxFailures: number
  failureWindowSeconds: number
  recoverySeconds: number
}

export interface MicroAppCSP {
  connectSrc: string[]
  imgSrc: string[]
  frameSrc: string[]
}

export interface MicroAppEvents {
  publish: string[]
  subscribe: string[]
}

export interface MicroAppManifest {
  schemaVersion: '2.0'
  name: string
  displayName: string
  runtime: MicroAppRuntime
  trust: MicroAppTrust
  status: MicroAppStatus
  routePrefix: string
  requiredPermissions: string[]
  requiredDataScopes: string[]
  allowedApiPrefixes: string[]
  events: MicroAppEvents
  featureFlag?: string
  owner: string
  releases: MicroAppReleaseSet
  compatibility: MicroAppCompatibility
  rollout: MicroAppRollout
  health: MicroAppHealth
  csp: MicroAppCSP
  fallbackPath: string
}

export interface ManifestValidationOptions {
  shellOrigin: string
  production: boolean
}

export class ManifestValidationError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ManifestValidationError'
  }
}

const namePattern = /^[a-z][a-z0-9-]{2,63}$/
const identifierPattern = /^[a-z][a-z0-9_-]*(?:[.:][a-z0-9_-]+)+$/
const topicPattern = /^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$/
const versionPattern = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/
const digestPattern = /^sha256:[a-f0-9]{64}$/
const integrityPattern = /^sha256-[A-Za-z0-9+/]+={0,2}$/
const allowedRootKeys = [
  'schemaVersion', 'name', 'displayName', 'runtime', 'trust', 'status', 'routePrefix',
  'requiredPermissions', 'requiredDataScopes', 'allowedApiPrefixes', 'events', 'featureFlag',
  'owner', 'releases', 'compatibility', 'rollout', 'health', 'csp', 'fallbackPath',
] as const

function record(value: unknown, name: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new ManifestValidationError(name + ' must be an object')
  }
  return value as Record<string, unknown>
}

function exactKeys(source: Record<string, unknown>, allowed: readonly string[], name: string): void {
  const allowedSet = new Set(allowed)
  const unknown = Object.keys(source).filter((key) => !allowedSet.has(key))
  if (unknown.length > 0) {
    throw new ManifestValidationError(name + ' contains unknown fields: ' + unknown.sort().join(', '))
  }
}

function text(value: unknown, name: string, maximum = 256): string {
  if (typeof value !== 'string' || value.trim() === '' || value.length > maximum) {
    throw new ManifestValidationError(name + ' must be a non-empty string')
  }
  return value.trim()
}

function stringList(
  value: unknown,
  name: string,
  options: { maximum?: number; minimum?: number } = {},
): string[] {
  const maximum = options.maximum ?? 64
  const minimum = options.minimum ?? 0
  if (!Array.isArray(value) || value.length < minimum || value.length > maximum) {
    throw new ManifestValidationError(
      name + ' must contain between ' + minimum + ' and ' + maximum + ' values',
    )
  }
  const values = value.map((item, index) => text(item, name + '[' + index + ']', 512))
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

function normalizedPath(value: unknown, name: string): string {
  const result = text(value, name, 256)
  if (
    !result.startsWith('/') || result.startsWith('//') || result.includes('..') ||
    result.includes('?') || result.includes('#') || result.includes('\\')
  ) {
    throw new ManifestValidationError(name + ' must be an absolute normalized path')
  }
  return result.replace(/\/+$/, '') || '/'
}

function expectedShellOrigin(options: ManifestValidationOptions): string {
  let parsed: URL
  try {
    parsed = new URL(options.shellOrigin)
  } catch {
    throw new ManifestValidationError('shellOrigin must be an absolute URL origin')
  }
  if (parsed.origin !== options.shellOrigin || !['http:', 'https:'].includes(parsed.protocol)) {
    throw new ManifestValidationError('shellOrigin must contain only scheme and authority')
  }
  return parsed.origin
}

function secureUrl(value: unknown, name: string, options: ManifestValidationOptions): URL {
  const input = text(value, name, 2048)
  let url: URL
  try {
    url = new URL(input)
  } catch {
    throw new ManifestValidationError(name + ' must be an absolute URL')
  }
  const localDevelopment =
    url.protocol === 'http:' && ['127.0.0.1', 'localhost'].includes(url.hostname)
  if (
    (options.production && url.protocol !== 'https:') ||
    (!options.production && url.protocol !== 'https:' && !localDevelopment)
  ) {
    throw new ManifestValidationError(name + ' must use HTTPS outside local development')
  }
  if (url.username || url.password || url.search || url.hash) {
    throw new ManifestValidationError(name + ' cannot contain credentials, query, or fragment')
  }
  return url
}

function validateIdentifiers(value: unknown, name: string, minimum = 0): string[] {
  const identifiers = stringList(value, name, { minimum })
  if (identifiers.some((identifier) => !identifierPattern.test(identifier) || identifier.includes('*'))) {
    throw new ManifestValidationError(name + ' contains an invalid or wildcard identifier')
  }
  return identifiers
}

function validateTopics(value: unknown, name: string): string[] {
  const topics = stringList(value, name, { maximum: 64 })
  if (topics.some((topic) => !topicPattern.test(topic))) {
    throw new ManifestValidationError(name + ' contains an invalid event topic')
  }
  return topics
}

function validateApiPrefixes(value: unknown): string[] {
  const prefixes = stringList(value, 'allowedApiPrefixes', { maximum: 32, minimum: 1 })
    .map((item) => normalizedPath(item, 'allowedApiPrefixes'))
  if (prefixes.some((item) => !item.startsWith('/api/') || item.includes('*'))) {
    throw new ManifestValidationError('allowedApiPrefixes must use explicit /api/ namespaces')
  }
  return prefixes
}

function validateResource(
  input: unknown,
  releaseName: string,
  releaseOrigin: string,
  options: ManifestValidationOptions,
): MicroAppResource {
  const source = record(input, releaseName + '.resource')
  exactKeys(source, ['url', 'integrity', 'maxBytes'], releaseName + '.resource')
  const url = secureUrl(source.url, releaseName + '.resource.url', options)
  if (url.origin !== releaseOrigin) {
    throw new ManifestValidationError(releaseName + ' resources must use the release origin')
  }
  const integrity = text(source.integrity, releaseName + '.resource.integrity', 128)
  if (!integrityPattern.test(integrity)) {
    throw new ManifestValidationError(releaseName + ' resources must use SHA-256 integrity')
  }
  return {
    url: url.href,
    integrity,
    maxBytes: integer(source.maxBytes, releaseName + '.resource.maxBytes', 1, 32 * 1024 * 1024),
  }
}

function validateRelease(
  input: unknown,
  name: string,
  runtime: MicroAppRuntime,
  trust: MicroAppTrust,
  options: ManifestValidationOptions,
): MicroAppRelease {
  const source = record(input, name)
  exactKeys(source, ['version', 'entry', 'healthUrl', 'digest', 'resources'], name)
  const version = text(source.version, name + '.version', 64)
  if (!versionPattern.test(version)) {
    throw new ManifestValidationError(name + '.version must use semantic versioning')
  }
  const entry = secureUrl(source.entry, name + '.entry', options)
  const shellOrigin = expectedShellOrigin(options)
  if (runtime === 'wujie' && (trust !== 'trusted-internal' || entry.origin !== shellOrigin)) {
    throw new ManifestValidationError('Wujie releases must be trusted and use the Shell origin')
  }
  if (runtime === 'iframe' && (trust !== 'untrusted-external' || entry.origin === shellOrigin)) {
    throw new ManifestValidationError('iframe releases must be untrusted and use an independent origin')
  }
  const healthUrl = secureUrl(source.healthUrl, name + '.healthUrl', options)
  if (healthUrl.origin !== entry.origin) {
    throw new ManifestValidationError(name + '.healthUrl must use the release origin')
  }
  const digest = text(source.digest, name + '.digest', 96)
  if (!digestPattern.test(digest)) {
    throw new ManifestValidationError(name + '.digest must use a SHA-256 digest')
  }
  if (!Array.isArray(source.resources) || source.resources.length < 1 || source.resources.length > 2048) {
    throw new ManifestValidationError(name + '.resources must contain between 1 and 2048 values')
  }
  const resources = source.resources.map((resource) =>
    validateResource(resource, name, entry.origin, options),
  )
  const resourceUrls = resources.map((resource) => resource.url)
  if (new Set(resourceUrls).size !== resourceUrls.length || !resourceUrls.includes(entry.href)) {
    throw new ManifestValidationError(name + '.resources must be unique and include the entry document')
  }
  return { version, entry: entry.href, healthUrl: healthUrl.href, digest, resources }
}

function validateCspSource(
  value: string,
  name: string,
  allowData: boolean,
  options: ManifestValidationOptions,
): string {
  if (value === "'self'" || value === "'none'" || (allowData && value === 'data:')) return value
  if (value.includes('*') || value.includes('unsafe-') || value.includes('nonce-') || value.includes('sha')) {
    throw new ManifestValidationError(name + ' contains a forbidden CSP expression')
  }
  let url: URL
  try {
    url = new URL(value)
  } catch {
    throw new ManifestValidationError(name + ' must contain exact HTTPS origins')
  }
  const localDevelopment =
    !options.production &&
    url.protocol === 'http:' &&
    ['127.0.0.1', 'localhost'].includes(url.hostname)
  if (
    (url.protocol !== 'https:' && !localDevelopment) ||
    url.origin !== value ||
    url.username ||
    url.password
  ) {
    throw new ManifestValidationError(name + ' must contain exact HTTPS origins')
  }
  return url.origin
}

function validateCspList(
  value: unknown,
  name: string,
  options: ManifestValidationOptions,
  allowData = false,
): string[] {
  const values = stringList(value, name, { maximum: 32, minimum: 1 })
    .map((source) => validateCspSource(source, name, allowData, options))
  if (values.includes("'none'") && values.length !== 1) {
    throw new ManifestValidationError(name + " cannot combine 'none' with other sources")
  }
  return values
}

export function validateMicroAppManifest(
  input: unknown,
  options: ManifestValidationOptions,
): MicroAppManifest {
  const source = record(input, 'manifest')
  exactKeys(source, allowedRootKeys, 'manifest')
  if (source.schemaVersion !== '2.0') {
    throw new ManifestValidationError('unsupported manifest schemaVersion')
  }
  const name = text(source.name, 'name', 64)
  if (!namePattern.test(name)) throw new ManifestValidationError('name must use lowercase kebab-case')
  const runtime = source.runtime
  if (runtime !== 'wujie' && runtime !== 'iframe') {
    throw new ManifestValidationError('runtime must be wujie or iframe')
  }
  const trust = source.trust
  if (trust !== 'trusted-internal' && trust !== 'untrusted-external') {
    throw new ManifestValidationError('trust is invalid')
  }
  const status = source.status
  if (status !== 'active' && status !== 'maintenance' && status !== 'disabled') {
    throw new ManifestValidationError('status is invalid')
  }
  const routePrefix = normalizedPath(source.routePrefix, 'routePrefix')
  if (routePrefix !== '/apps/' + name) {
    throw new ManifestValidationError('routePrefix must use the reserved /apps/{name} route')
  }

  const releasesSource = record(source.releases, 'releases')
  exactKeys(releasesSource, ['primary', 'rollback'], 'releases')
  const primary = validateRelease(
    releasesSource.primary,
    'releases.primary',
    runtime,
    trust,
    options,
  )
  const rollback = releasesSource.rollback === undefined
    ? undefined
    : validateRelease(releasesSource.rollback, 'releases.rollback', runtime, trust, options)
  if (rollback?.version === primary.version) {
    throw new ManifestValidationError('rollback release must use a different version')
  }

  const compatibilitySource = record(source.compatibility, 'compatibility')
  exactKeys(
    compatibilitySource,
    ['shellApi', 'hostSdkApi', 'designSystemApi', 'apiContract', 'reactMajor'],
    'compatibility',
  )
  if (
    compatibilitySource.shellApi !== 1 || compatibilitySource.hostSdkApi !== 1 ||
    compatibilitySource.designSystemApi !== 1 || compatibilitySource.apiContract !== 'v1' ||
    compatibilitySource.reactMajor !== 19
  ) {
    throw new ManifestValidationError('manifest compatibility is unsupported by this Shell')
  }

  const rolloutSource = record(source.rollout, 'rollout')
  exactKeys(rolloutSource, ['strategy', 'percentage', 'cohortKey', 'salt'], 'rollout')
  const strategy = rolloutSource.strategy
  if (strategy !== 'stable' && strategy !== 'canary') {
    throw new ManifestValidationError('rollout.strategy is invalid')
  }
  const percentage = integer(rolloutSource.percentage, 'rollout.percentage', 1, 100)
  if ((strategy === 'stable' && percentage !== 100) || (strategy === 'canary' && percentage === 100)) {
    throw new ManifestValidationError('rollout strategy and percentage are inconsistent')
  }
  if (rolloutSource.cohortKey !== 'user_id' && rolloutSource.cohortKey !== 'organization_id') {
    throw new ManifestValidationError('rollout.cohortKey is unsupported')
  }
  const salt = text(rolloutSource.salt, 'rollout.salt', 128)
  if (salt.length < 16) throw new ManifestValidationError('rollout.salt must contain at least 16 characters')

  const healthSource = record(source.health, 'health')
  exactKeys(
    healthSource,
    ['timeoutMs', 'startupTimeoutMs', 'maxFailures', 'failureWindowSeconds', 'recoverySeconds'],
    'health',
  )
  const cspSource = record(source.csp, 'csp')
  exactKeys(cspSource, ['connectSrc', 'imgSrc', 'frameSrc'], 'csp')
  const csp: MicroAppCSP = {
    connectSrc: validateCspList(cspSource.connectSrc, 'csp.connectSrc', options),
    imgSrc: validateCspList(cspSource.imgSrc, 'csp.imgSrc', options, true),
    frameSrc: validateCspList(cspSource.frameSrc, 'csp.frameSrc', options),
  }
  const requiredFrameSource = runtime === 'wujie' ? "'self'" : new URL(primary.entry).origin
  if (!csp.frameSrc.includes(requiredFrameSource)) {
    throw new ManifestValidationError('csp.frameSrc must allow the selected runtime origin')
  }

  const eventsSource = record(source.events, 'events')
  exactKeys(eventsSource, ['publish', 'subscribe'], 'events')
  const fallbackPath = normalizedPath(source.fallbackPath, 'fallbackPath')
  if (fallbackPath.startsWith(routePrefix + '/') || fallbackPath === routePrefix) {
    throw new ManifestValidationError('fallbackPath must be owned by the Shell')
  }

  return {
    schemaVersion: '2.0',
    name,
    displayName: text(source.displayName, 'displayName', 128),
    runtime,
    trust,
    status,
    routePrefix,
    requiredPermissions: validateIdentifiers(source.requiredPermissions, 'requiredPermissions', 1),
    requiredDataScopes: validateIdentifiers(source.requiredDataScopes, 'requiredDataScopes', 1),
    allowedApiPrefixes: validateApiPrefixes(source.allowedApiPrefixes),
    events: {
      publish: validateTopics(eventsSource.publish, 'events.publish'),
      subscribe: validateTopics(eventsSource.subscribe, 'events.subscribe'),
    },
    featureFlag: source.featureFlag === undefined
      ? undefined
      : text(source.featureFlag, 'featureFlag', 128),
    owner: text(source.owner, 'owner', 256),
    releases: { primary, ...(rollback === undefined ? {} : { rollback }) },
    compatibility: {
      shellApi: 1, hostSdkApi: 1, designSystemApi: 1, apiContract: 'v1', reactMajor: 19,
    },
    rollout: {
      strategy,
      percentage,
      cohortKey: rolloutSource.cohortKey,
      salt,
    },
    health: {
      timeoutMs: integer(healthSource.timeoutMs, 'health.timeoutMs', 250, 10000),
      startupTimeoutMs: integer(healthSource.startupTimeoutMs, 'health.startupTimeoutMs', 1000, 30000),
      maxFailures: integer(healthSource.maxFailures, 'health.maxFailures', 1, 20),
      failureWindowSeconds: integer(
        healthSource.failureWindowSeconds,
        'health.failureWindowSeconds',
        1,
        3600,
      ),
      recoverySeconds: integer(healthSource.recoverySeconds, 'health.recoverySeconds', 1, 86400),
    },
    csp,
    fallbackPath,
  }
}
