import { beforeAll, describe, expect, it } from 'vitest';

import { createMicroAppManifest } from '@forge/testkit';
import {
  canonicalizeManifest,
  encodeBase64Url,
  verifyManifestDocument,
  type VerifiedManifestDocument,
} from '@forge/manifest-security';
import type { MicroAppManifest } from '@forge/app-contract';

import {
  authorizeManifestReleaseSet,
  createReleaseSetCircuitBreaker,
  isMicroAppReleaseEnabled,
  selectMicroAppRollback,
  selectMicroAppRolloutSubject,
  type AuthorizedMicroAppReleaseSet,
} from './manifest-adapter';

let verifiedManifest: VerifiedManifestDocument<MicroAppManifest>;
let releaseSet: AuthorizedMicroAppReleaseSet;

beforeAll(async () => {
  const manifest = createMicroAppManifest();
  const payload = new TextEncoder().encode(canonicalizeManifest(manifest));
  const keyPair = await crypto.subtle.generateKey(
    { name: 'ECDSA', namedCurve: 'P-256' },
    true,
    ['sign', 'verify'],
  );
  const signature = new Uint8Array(
    await crypto.subtle.sign({ name: 'ECDSA', hash: 'SHA-256' }, keyPair.privateKey, payload),
  );
  verifiedManifest = await verifyManifestDocument(payload, {
    algorithm: 'ECDSA_P256_SHA256',
    keyId: 'frontend-release-2026-01',
    issuedAt: '2026-08-17T05:00:00.000Z',
    expiresAt: '2026-08-24T05:00:00.000Z',
    value: encodeBase64Url(signature),
  }, {
    keyStore: {
      resolve: async () => ({
        keyId: 'frontend-release-2026-01',
        algorithm: 'ECDSA_P256_SHA256',
        status: 'active',
        notBefore: '2026-01-01T00:00:00.000Z',
        notAfter: '2027-01-01T00:00:00.000Z',
        material: await crypto.subtle.exportKey('jwk', keyPair.publicKey),
      }),
    },
    now: new Date('2026-08-17T06:00:00.000Z'),
  });
  releaseSet = authorizeManifestReleaseSet({
    verifiedManifest,
    validation: {
      shellOrigin: 'https://portal.bank.example',
      production: true,
    },
  });
});

describe('signed manifest release adapter', () => {
  it('derives primary and explicit rollback plans from one signed document', () => {
    expect(releaseSet.primary.releaseId).toBe('example-remote@1.0.0');
    expect(releaseSet.rollback?.releaseId).toBe('example-remote@0.9.0');
    expect(releaseSet.primary.verification.payloadSha256).toBe(
      releaseSet.rollback?.verification.payloadSha256,
    );
    expect(releaseSet.metadata.allowedApiPrefixes).toEqual(['/api/v1/example-remote']);
  });

  it('keeps feature-flag activation fail-closed', () => {
    expect(isMicroAppReleaseEnabled(releaseSet, new Set())).toBe(false);
    expect(
      isMicroAppReleaseEnabled(releaseSet, new Set(['micro_frontend.example_remote'])),
    ).toBe(true);
  });

  it('allows rollback only for failure of the signed primary release', () => {
    expect(selectMicroAppRollback(releaseSet, releaseSet.primary.releaseId)).toBe(
      releaseSet.rollback,
    );
    expect(selectMicroAppRollback(releaseSet, 'example-remote@unknown')).toBeNull();
    expect(selectMicroAppRollback(releaseSet, releaseSet.rollback!.releaseId)).toBeNull();
  });

  it('derives cohort and circuit policy without caller overrides', () => {
    expect(
      selectMicroAppRolloutSubject(releaseSet, {
        userId: 'user-1',
        organizationId: 'org-1',
      }),
    ).toBe('user-1');

    const breaker = createReleaseSetCircuitBreaker(releaseSet);
    breaker.recordFailure(releaseSet.primary.releaseId, 1_000);
    breaker.recordFailure(releaseSet.primary.releaseId, 2_000);
    expect(breaker.recordFailure(releaseSet.primary.releaseId, 3_000)).toMatchObject({
      allowed: false,
      retryAt: 123_000,
    });
  });
});
