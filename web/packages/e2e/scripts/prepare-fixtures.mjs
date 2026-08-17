import { createHash, webcrypto } from 'node:crypto'
import { cp, mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const packageRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repositoryRoot = resolve(packageRoot, '../../..')
const remoteDist = join(repositoryRoot, 'web/apps/example-remote/dist')
const generatedRoot = join(repositoryRoot, 'web/apps/shell/public/microapps/example-remote')
const shellOrigin = 'http://127.0.0.1:4173'
const iframeOrigin = 'http://127.0.0.1:4181'
const keyId = 'e2e-release-key-01'
const privateKey = {
  key_ops: ['sign'],
  ext: true,
  kty: 'EC',
  x: 'k2_WeVc1iRvpiz4sx0Cuv9TDbG7LNOq9CKCY_zBkDqU',
  y: 'hiS6ooDx-9jDLMlCId3JAuc8Gkn00jc5USNxOZ-j8CY',
  crv: 'P-256',
  d: 'r3ii9ry3JpGkwnqlaDo_NDv0h19azs25e5w7zWSy2wA',
}

function canonicalize(value) {
  if (value === null || typeof value === 'boolean' || typeof value === 'string') {
    return JSON.stringify(value)
  }
  if (typeof value === 'number' && Number.isFinite(value)) return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map(canonicalize).join(',')}]`
  if (value && typeof value === 'object') {
    return `{${Object.entries(value)
      .sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0))
      .map(([key, entry]) => `${JSON.stringify(key)}:${canonicalize(entry)}`)
      .join(',')}}`
  }
  throw new Error('Fixture manifest contains an unsupported value')
}

function base64Url(bytes) {
  return Buffer.from(bytes).toString('base64url')
}

async function files(root) {
  const output = []
  const visit = async (directory) => {
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      const absolute = join(directory, entry.name)
      if (entry.isDirectory()) await visit(absolute)
      else output.push(absolute)
    }
  }
  await visit(root)
  return output.sort()
}

async function inventory(root, origin, basePath) {
  const resources = []
  for (const file of await files(root)) {
    if (relative(root, file) === 'healthz') continue
    const bytes = await readFile(file)
    const path = relative(root, file).split('\\').join('/')
    resources.push({
      url: `${origin}${basePath}/${path}`,
      integrity: `sha256-${createHash('sha256').update(bytes).digest('base64')}`,
      maxBytes: Math.max(bytes.byteLength, 1),
    })
  }
  return resources
}

function releaseDigest(resources) {
  const evidence = resources.map(({ url, integrity }) => `${url}\u0000${integrity}`).join('\n')
  return `sha256:${createHash('sha256').update(evidence).digest('hex')}`
}

async function signedBundle(manifest) {
  const payload = new TextEncoder().encode(canonicalize(manifest))
  const key = await webcrypto.subtle.importKey(
    'jwk',
    privateKey,
    { name: 'ECDSA', namedCurve: 'P-256' },
    false,
    ['sign'],
  )
  const signature = await webcrypto.subtle.sign(
    { name: 'ECDSA', hash: 'SHA-256' },
    key,
    payload,
  )
  const now = Date.now()
  return JSON.stringify({
    payload: base64Url(payload),
    signature: {
      algorithm: 'ECDSA_P256_SHA256',
      keyId,
      issuedAt: new Date(now - 60_000).toISOString(),
      expiresAt: new Date(now + 7 * 24 * 60 * 60 * 1000).toISOString(),
      value: base64Url(new Uint8Array(signature)),
    },
  })
}

function manifestBase(runtime, trust, primary, rollback, frameSrc) {
  return {
    schemaVersion: '2.0',
    name: 'example-remote',
    displayName: '示例远程应用',
    runtime,
    trust,
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
    owner: 'e2e-platform-team',
    releases: { primary, ...(rollback ? { rollback } : {}) },
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
      salt: 'example-remote-e2e-rollout',
    },
    health: {
      timeoutMs: 1000,
      startupTimeoutMs: 8000,
      maxFailures: 2,
      failureWindowSeconds: 30,
      recoverySeconds: 60,
    },
    csp: {
      connectSrc: ["'self'"],
      imgSrc: ["'self'", 'data:'],
      frameSrc: [frameSrc],
    },
    fallbackPath: '/micro-app-unavailable',
  }
}

await writeFile(join(remoteDist, 'healthz'), 'OK')
await rm(generatedRoot, { recursive: true, force: true })
await mkdir(generatedRoot, { recursive: true })

const releases = {}
for (const version of ['1.0.0', '0.9.0']) {
  const target = join(generatedRoot, version)
  await cp(remoteDist, target, { recursive: true })
  const basePath = `/microapps/example-remote/${version}`
  const resources = await inventory(target, shellOrigin, basePath)
  releases[version] = {
    version,
    entry: `${shellOrigin}${basePath}/index.html`,
    healthUrl: `${shellOrigin}${basePath}/healthz`,
    digest: releaseDigest(resources),
    resources,
  }
}

const wujieManifest = manifestBase(
  'wujie',
  'trusted-internal',
  releases['1.0.0'],
  releases['0.9.0'],
  "'self'",
)
const rollbackManifest = structuredClone(wujieManifest)
const tamperedEntry = rollbackManifest.releases.primary.resources.find(
  ({ url }) => url.endsWith('/index.html'),
)
if (!tamperedEntry) throw new Error('Primary entry resource is missing')
tamperedEntry.integrity = `sha256-${'A'.repeat(43)}=`

const iframeResources = await inventory(remoteDist, iframeOrigin, '')
const iframeRelease = {
  version: '1.0.0',
  entry: `${iframeOrigin}/index.html`,
  healthUrl: `${iframeOrigin}/healthz`,
  digest: releaseDigest(iframeResources),
  resources: iframeResources,
}
const iframeManifest = manifestBase(
  'iframe',
  'untrusted-external',
  iframeRelease,
  undefined,
  iframeOrigin,
)

await Promise.all([
  writeFile(join(generatedRoot, 'manifest.bundle.json'), await signedBundle(wujieManifest)),
  writeFile(join(generatedRoot, 'manifest-rollback.bundle.json'), await signedBundle(rollbackManifest)),
  writeFile(join(generatedRoot, 'manifest-iframe.bundle.json'), await signedBundle(iframeManifest)),
])
