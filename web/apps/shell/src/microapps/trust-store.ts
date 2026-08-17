import type { ManifestKeyStore, TrustedManifestKey } from '@forge/manifest-security'

interface SerializedTrustedKey {
  keyId: string
  algorithm: 'ECDSA_P256_SHA256' | 'SM2_SM3'
  status: 'active' | 'revoked'
  notBefore: string
  notAfter: string
  publicKey: unknown
}

const keyIdPattern = /^[a-zA-Z0-9][a-zA-Z0-9._:-]{2,127}$/

function object(value: unknown, name: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(name + ' must be an object')
  }
  return value as Record<string, unknown>
}

function parseKey(input: unknown): TrustedManifestKey {
  const source = object(input, 'Manifest trust key')
  const fields = ['algorithm', 'keyId', 'notAfter', 'notBefore', 'publicKey', 'status']
  if (Object.keys(source).sort().join('\u0000') !== fields.sort().join('\u0000')) {
    throw new Error('Manifest trust key contains unknown or missing fields')
  }
  if (
    typeof source.keyId !== 'string' ||
    !keyIdPattern.test(source.keyId) ||
    (source.algorithm !== 'ECDSA_P256_SHA256' && source.algorithm !== 'SM2_SM3') ||
    (source.status !== 'active' && source.status !== 'revoked') ||
    typeof source.notBefore !== 'string' ||
    typeof source.notAfter !== 'string'
  ) {
    throw new Error('Manifest trust key metadata is invalid')
  }
  const publicKey = object(source.publicKey, 'Manifest public key')
  const sensitiveFields = Object.keys(publicKey).filter((field) =>
    /^(?:d|private|secret|seed)$/i.test(field),
  )
  if (sensitiveFields.length > 0) {
    throw new Error('Manifest trust store cannot contain private key material')
  }
  if (
    source.algorithm === 'ECDSA_P256_SHA256' &&
    (publicKey.kty !== 'EC' || publicKey.crv !== 'P-256' ||
      typeof publicKey.x !== 'string' || typeof publicKey.y !== 'string')
  ) {
    throw new Error('ECDSA manifest trust key must be a public P-256 JWK')
  }
  const serialized: SerializedTrustedKey = {
    keyId: source.keyId,
    algorithm: source.algorithm,
    status: source.status,
    notBefore: source.notBefore,
    notAfter: source.notAfter,
    publicKey,
  }
  return Object.freeze({
    keyId: serialized.keyId,
    algorithm: serialized.algorithm,
    status: serialized.status,
    notBefore: serialized.notBefore,
    notAfter: serialized.notAfter,
    material: Object.freeze({ ...publicKey }),
  })
}

export function parseBuildTimeManifestKeys(raw: string | undefined): readonly TrustedManifestKey[] {
  if (raw === undefined || raw.trim() === '') return Object.freeze([])
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    throw new Error('VITE_MICROAPP_TRUSTED_KEYS must be valid JSON')
  }
  if (!Array.isArray(parsed) || parsed.length < 1 || parsed.length > 16) {
    throw new Error('Manifest trust store must contain between 1 and 16 keys')
  }
  const keys = parsed.map(parseKey)
  if (new Set(keys.map((key) => key.keyId)).size !== keys.length) {
    throw new Error('Manifest trust store contains duplicate key IDs')
  }
  return Object.freeze(keys)
}

export function createManifestKeyStore(keys: readonly TrustedManifestKey[]): ManifestKeyStore {
  const byId = new Map(keys.map((key) => [key.keyId, key]))
  return Object.freeze({
    async resolve(keyId: string): Promise<TrustedManifestKey | null> {
      return byId.get(keyId) ?? null
    },
  })
}

export const manifestKeyStore = createManifestKeyStore(
  parseBuildTimeManifestKeys(import.meta.env.VITE_MICROAPP_TRUSTED_KEYS),
)
