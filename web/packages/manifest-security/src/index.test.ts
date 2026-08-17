import { describe, expect, it, vi } from 'vitest';

import {
  ManifestSecurityError,
  canonicalizeManifest,
  encodeBase64Url,
  sha256Integrity,
  verifyManifestDocument,
  verifySha256Integrity,
  type ManifestKeyStore,
  type ManifestSignatureEnvelope,
  type TrustedManifestKey,
} from './index';

const now = new Date('2026-08-17T06:00:00.000Z');

async function signedFixture(status: TrustedManifestKey['status'] = 'active') {
  const pair = await crypto.subtle.generateKey(
    { name: 'ECDSA', namedCurve: 'P-256' },
    true,
    ['sign', 'verify'],
  );
  const manifest = {
    id: 'risk-console',
    entry: 'https://portal.example.cn/microapps/risk/1.2.3/',
    version: '1.2.3',
  };
  const payload = new TextEncoder().encode(canonicalizeManifest(manifest));
  const signature = new Uint8Array(
    await crypto.subtle.sign({ name: 'ECDSA', hash: 'SHA-256' }, pair.privateKey, payload),
  );
  const key: TrustedManifestKey = {
    keyId: 'frontend-release-2026-01',
    algorithm: 'ECDSA_P256_SHA256',
    status,
    notBefore: '2026-01-01T00:00:00.000Z',
    notAfter: '2027-01-01T00:00:00.000Z',
    material: await crypto.subtle.exportKey('jwk', pair.publicKey),
  };
  const keyStore: ManifestKeyStore = {
    resolve: vi.fn(async (keyId) => (keyId === key.keyId ? key : null)),
  };
  const envelope: ManifestSignatureEnvelope = {
    algorithm: 'ECDSA_P256_SHA256',
    keyId: key.keyId,
    issuedAt: '2026-08-17T05:00:00.000Z',
    expiresAt: '2026-08-24T05:00:00.000Z',
    value: encodeBase64Url(signature),
  };
  return { envelope, keyStore, manifest, payload };
}

describe('manifest signature verification', () => {
  it('verifies a canonical manifest and returns immutable evidence', async () => {
    const fixture = await signedFixture();

    const verified = await verifyManifestDocument<typeof fixture.manifest>(
      fixture.payload,
      fixture.envelope,
      { keyStore: fixture.keyStore, now },
    );

    expect(verified.manifest).toEqual(fixture.manifest);
    expect(verified.keyId).toBe('frontend-release-2026-01');
    expect(verified.payloadSha256).toMatch(/^sha256-/);
    expect(Object.isFrozen(verified.manifest)).toBe(true);
  });

  it('rejects non-canonical and tampered manifests', async () => {
    const fixture = await signedFixture();
    const nonCanonical = new TextEncoder().encode('{ "id": "risk-console" }');

    await expect(
      verifyManifestDocument(nonCanonical, fixture.envelope, {
        keyStore: fixture.keyStore,
        now,
      }),
    ).rejects.toMatchObject({ code: 'NON_CANONICAL_DOCUMENT' });

    const tampered = new TextEncoder().encode(
      canonicalizeManifest({ ...fixture.manifest, version: '9.9.9' }),
    );
    await expect(
      verifyManifestDocument(tampered, fixture.envelope, {
        keyStore: fixture.keyStore,
        now,
      }),
    ).rejects.toMatchObject({ code: 'SIGNATURE_INVALID' });
  });

  it('rejects revoked keys and expired signatures', async () => {
    const revoked = await signedFixture('revoked');
    await expect(
      verifyManifestDocument(revoked.payload, revoked.envelope, {
        keyStore: revoked.keyStore,
        now,
      }),
    ).rejects.toMatchObject({ code: 'KEY_REVOKED' });

    const active = await signedFixture();
    await expect(
      verifyManifestDocument(active.payload, active.envelope, {
        keyStore: active.keyStore,
        now: new Date('2026-09-01T00:00:00.000Z'),
      }),
    ).rejects.toMatchObject({ code: 'SIGNATURE_EXPIRED' });
  });

  it('supports an explicit SM2/SM3 provider without weakening fail-closed behavior', async () => {
    const payload = new TextEncoder().encode(canonicalizeManifest({ id: 'domestic-app' }));
    const provider = {
      algorithm: 'SM2_SM3' as const,
      verify: vi.fn(async () => true),
    };
    const keyStore: ManifestKeyStore = {
      resolve: vi.fn(async () => ({
        keyId: 'sm2-release-key-01',
        algorithm: 'SM2_SM3' as const,
        status: 'active' as const,
        notBefore: '2026-01-01T00:00:00.000Z',
        notAfter: '2027-01-01T00:00:00.000Z',
        material: { provider: 'SJJ1962' },
      })),
    };

    await expect(
      verifyManifestDocument(payload, {
        algorithm: 'SM2_SM3',
        keyId: 'sm2-release-key-01',
        issuedAt: '2026-08-17T05:00:00.000Z',
        expiresAt: '2026-08-24T05:00:00.000Z',
        value: 'AQ',
      }, { keyStore, providers: [provider], now }),
    ).resolves.toMatchObject({ algorithm: 'SM2_SM3' });
    expect(provider.verify).toHaveBeenCalledOnce();

    await expect(
      verifyManifestDocument(payload, {
        algorithm: 'SM2_SM3',
        keyId: 'sm2-release-key-01',
        issuedAt: '2026-08-17T05:00:00.000Z',
        expiresAt: '2026-08-24T05:00:00.000Z',
        value: 'AQ',
      }, { keyStore, providers: [], now }),
    ).rejects.toMatchObject({ code: 'ALGORITHM_UNAVAILABLE' });
  });
});

describe('artifact integrity verification', () => {
  it('accepts the matching digest and rejects a changed artifact', async () => {
    const artifact = new TextEncoder().encode('immutable frontend artifact');
    const integrity = await sha256Integrity(artifact);

    await expect(verifySha256Integrity(artifact, integrity)).resolves.toBeUndefined();
    await expect(
      verifySha256Integrity(new TextEncoder().encode('changed artifact'), integrity),
    ).rejects.toBeInstanceOf(ManifestSecurityError);
  });
});
