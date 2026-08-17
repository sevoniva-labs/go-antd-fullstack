import { beforeAll, describe, expect, it, vi } from 'vitest';

import {
  canonicalizeManifest,
  encodeBase64Url,
  type ManifestKeyStore,
  type ManifestSignatureEnvelope,
} from './index';
import { loadSignedManifest } from './loader';

let body: string;
let keyStore: ManifestKeyStore;
const now = new Date('2026-08-17T06:00:00.000Z');

beforeAll(async () => {
  const payload = new TextEncoder().encode(canonicalizeManifest({ id: 'risk-console' }));
  const pair = await crypto.subtle.generateKey(
    { name: 'ECDSA', namedCurve: 'P-256' },
    true,
    ['sign', 'verify'],
  );
  const signature = new Uint8Array(
    await crypto.subtle.sign({ name: 'ECDSA', hash: 'SHA-256' }, pair.privateKey, payload),
  );
  const envelope: ManifestSignatureEnvelope = {
    algorithm: 'ECDSA_P256_SHA256',
    keyId: 'frontend-release-2026-01',
    issuedAt: '2026-08-17T05:00:00.000Z',
    expiresAt: '2026-08-24T05:00:00.000Z',
    value: encodeBase64Url(signature),
  };
  body = JSON.stringify({ payload: encodeBase64Url(payload), signature: envelope });
  keyStore = {
    resolve: async () => ({
      keyId: envelope.keyId,
      algorithm: envelope.algorithm,
      status: 'active',
      notBefore: '2026-01-01T00:00:00.000Z',
      notAfter: '2027-01-01T00:00:00.000Z',
      material: await crypto.subtle.exportKey('jwk', pair.publicKey),
    }),
  };
});

function response(value = body): Response {
  return new Response(value, {
    status: 200,
    headers: { 'Content-Type': 'application/vnd.forge.microapp-manifest+json' },
  });
}

describe('loadSignedManifest', () => {
  it('loads only a same-origin bounded bundle before verifying it', async () => {
    const fetcher = vi.fn(async () => response());
    const verified = await loadSignedManifest<{ id: string }>('/manifests/risk.json', {
      shellOrigin: 'https://portal.example.cn',
      keyStore,
      fetcher,
      now,
    });

    expect(verified.manifest).toEqual({ id: 'risk-console' });
    expect(fetcher).toHaveBeenCalledWith(
      new URL('https://portal.example.cn/manifests/risk.json'),
      expect.objectContaining({ credentials: 'same-origin', redirect: 'error' }),
    );
  });

  it('rejects cross-origin URLs before making a request', async () => {
    const fetcher = vi.fn();
    await expect(
      loadSignedManifest('https://evil.example/manifest.json', {
        shellOrigin: 'https://portal.example.cn',
        keyStore,
        fetcher,
        now,
      }),
    ).rejects.toMatchObject({ code: 'INVALID_DOCUMENT' });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('rejects oversized payloads and unexpected content types', async () => {
    await expect(
      loadSignedManifest('/manifest.json', {
        shellOrigin: 'https://portal.example.cn',
        keyStore,
        fetcher: vi.fn(async () => response()),
        maxPayloadBytes: 1,
        now,
      }),
    ).rejects.toMatchObject({ code: 'INVALID_DOCUMENT' });
    await expect(
      loadSignedManifest('/manifest.json', {
        shellOrigin: 'https://portal.example.cn',
        keyStore,
        fetcher: vi.fn(async () => new Response(body, {
          headers: { 'Content-Type': 'text/plain' },
        })),
        now,
      }),
    ).rejects.toMatchObject({ code: 'INVALID_DOCUMENT' });
  });
});
