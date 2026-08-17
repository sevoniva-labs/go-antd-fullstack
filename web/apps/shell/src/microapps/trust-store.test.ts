import { describe, expect, it } from 'vitest'

import { createManifestKeyStore, parseBuildTimeManifestKeys } from './trust-store'

const validKey = {
  keyId: 'frontend-release-2026-01',
  algorithm: 'ECDSA_P256_SHA256',
  status: 'active',
  notBefore: '2026-01-01T00:00:00.000Z',
  notAfter: '2027-01-01T00:00:00.000Z',
  publicKey: { kty: 'EC', crv: 'P-256', x: 'public-x', y: 'public-y' },
}

describe('build-time manifest trust store', () => {
  it('loads explicit public keys and resolves only known IDs', async () => {
    const keys = parseBuildTimeManifestKeys(JSON.stringify([validKey]))
    const store = createManifestKeyStore(keys)

    await expect(store.resolve(validKey.keyId)).resolves.toMatchObject({
      keyId: validKey.keyId,
      algorithm: 'ECDSA_P256_SHA256',
    })
    await expect(store.resolve('unknown-key')).resolves.toBeNull()
  })

  it('keeps an absent build-time trust store empty instead of falling back', () => {
    expect(parseBuildTimeManifestKeys(undefined)).toEqual([])
    expect(parseBuildTimeManifestKeys('')).toEqual([])
  })

  it('rejects duplicate IDs, private material, and unknown fields', () => {
    expect(() => parseBuildTimeManifestKeys(JSON.stringify([validKey, validKey]))).toThrow(/duplicate/)
    expect(() => parseBuildTimeManifestKeys(JSON.stringify([{
      ...validKey,
      publicKey: { ...validKey.publicKey, d: 'private-key' },
    }]))).toThrow(/private key/)
    expect(() => parseBuildTimeManifestKeys(JSON.stringify([{
      ...validKey,
      runtimeOverride: true,
    }]))).toThrow(/unknown or missing/)
  })
})
