import { beforeAll, describe, expect, it, vi } from 'vitest';

import {
  canonicalizeManifest,
  encodeBase64Url,
  sha256Integrity,
  verifyManifestDocument,
  type VerifiedManifestDocument,
} from '@forge/manifest-security';

import { MicroAppCircuitBreaker } from './circuit-breaker';
import { createMicroAppIntegrityFetch } from './integrity-fetch';
import {
  authorizeMicroAppLaunch,
  isAuthorizedMicroAppLaunchPlan,
  isMicroAppRolloutEligible,
  type AuthorizedMicroAppLaunchPlan,
  type MicroAppLaunchCandidate,
} from './policy';

interface FixtureManifest {
  id: string;
  version: string;
}

let verifiedManifest: VerifiedManifestDocument<FixtureManifest>;

beforeAll(async () => {
  const manifest: FixtureManifest = { id: 'risk-console', version: '1.2.3' };
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
});

async function candidate(
  overrides: Partial<MicroAppLaunchCandidate> = {},
): Promise<MicroAppLaunchCandidate> {
  const entry = new TextEncoder().encode('<!doctype html><title>risk</title>');
  return {
    id: 'risk-console',
    title: '风险控制台',
    version: '1.2.3',
    runtime: 'wujie',
    trust: 'trusted-internal',
    entryUrl: 'https://portal.example.cn/microapps/risk/1.2.3/index.html',
    routePrefix: '/apps/risk-console',
    requiredPermissions: ['risk.case.read'],
    rollout: { percentage: 100, salt: 'risk-rollout-2026-01' },
    health: {
      url: 'https://portal.example.cn/microapps/risk/1.2.3/healthz',
      timeoutMs: 1_000,
    },
    startupTimeoutMs: 5_000,
    resources: [{
      url: 'https://portal.example.cn/microapps/risk/1.2.3/index.html',
      integrity: await sha256Integrity(entry),
    }],
    ...overrides,
  };
}

async function plan(
  overrides: Partial<MicroAppLaunchCandidate> = {},
): Promise<AuthorizedMicroAppLaunchPlan> {
  const launchCandidate = await candidate(overrides);
  return authorizeMicroAppLaunch({
    verifiedManifest,
    shellOrigin: 'https://portal.example.cn',
    select: () => launchCandidate,
  });
}

describe('micro-app launch policy', () => {
  it('authorizes a trusted same-origin Wujie release from real signature evidence', async () => {
    const authorized = await plan();

    expect(isAuthorizedMicroAppLaunchPlan(authorized)).toBe(true);
    expect(authorized.releaseId).toBe('risk-console@1.2.3');
    expect(authorized.verification.keyId).toBe('frontend-release-2026-01');
  });

  it('rejects Wujie cross-origin and iframe same-origin trust confusion', async () => {
    const crossOriginWujie = await candidate({
      entryUrl: 'https://remote.example.cn/risk/index.html',
      health: { url: 'https://remote.example.cn/risk/healthz', timeoutMs: 1_000 },
      resources: [{
        url: 'https://remote.example.cn/risk/index.html',
        integrity: 'sha256-AQ==',
      }],
    });
    expect(() =>
      authorizeMicroAppLaunch({
        verifiedManifest,
        shellOrigin: 'https://portal.example.cn',
        select: () => crossOriginWujie,
      }),
    ).toThrowError(expect.objectContaining({ code: 'TRUST_BOUNDARY_VIOLATION' }));

    const sameOriginIframe = await candidate({ runtime: 'iframe', trust: 'untrusted-external' });
    expect(() =>
      authorizeMicroAppLaunch({
        verifiedManifest,
        shellOrigin: 'https://portal.example.cn',
        select: () => sameOriginIframe,
      }),
    ).toThrowError(expect.objectContaining({ code: 'TRUST_BOUNDARY_VIOLATION' }));
  });

  it('uses deterministic rollout buckets', async () => {
    const partial = await plan({ rollout: { percentage: 37, salt: 'stable-rollout-2026' } });
    const first = await isMicroAppRolloutEligible(partial, 'user-123');
    const second = await isMicroAppRolloutEligible(partial, 'user-123');

    expect(second).toBe(first);
    await expect(
      isMicroAppRolloutEligible(await plan({ rollout: { percentage: 0, salt: 'zero-rollout-2026' } }), 'user-123'),
    ).resolves.toBe(false);
  });
});

describe('micro-app circuit breaker', () => {
  it('opens after the threshold and recovers after cooldown', () => {
    const breaker = new MicroAppCircuitBreaker({
      failureThreshold: 2,
      failureWindowMs: 10_000,
      cooldownMs: 5_000,
    });
    expect(breaker.recordFailure('risk@1', 1_000).allowed).toBe(true);
    expect(breaker.recordFailure('risk@1', 2_000)).toMatchObject({
      allowed: false,
      retryAt: 7_000,
    });
    expect(breaker.canAttempt('risk@1', 6_999).allowed).toBe(false);
    expect(breaker.canAttempt('risk@1', 7_000).allowed).toBe(true);
  });

  it('clears failures after a successful launch', () => {
    const breaker = new MicroAppCircuitBreaker();
    breaker.recordFailure('risk@1', 1_000);
    breaker.recordSuccess('risk@1');
    expect(breaker.canAttempt('risk@1', 2_000)).toEqual({ allowed: true, failures: 0 });
  });
});

describe('signed resource inventory fetch', () => {
  it('verifies declared resources before returning them', async () => {
    const authorized = await plan();
    const body = '<!doctype html><title>risk</title>';
    const fetcher = vi.fn(async () => new Response(body, { status: 200 }));
    const guardedFetch = createMicroAppIntegrityFetch(authorized, fetcher);

    await expect(guardedFetch(authorized.entryUrl)).resolves.toBeInstanceOf(Response);
    expect(fetcher).toHaveBeenCalledWith(
      new URL(authorized.entryUrl),
      expect.objectContaining({ method: 'GET', redirect: 'error' }),
    );
  });

  it('rejects undeclared and modified resources', async () => {
    const authorized = await plan();
    const guardedFetch = createMicroAppIntegrityFetch(
      authorized,
      vi.fn(async () => new Response('tampered', { status: 200 })),
    );

    await expect(
      guardedFetch('https://portal.example.cn/microapps/risk/1.2.3/unknown.js'),
    ).rejects.toMatchObject({ code: 'RESOURCE_NOT_DECLARED' });
    await expect(guardedFetch(authorized.entryUrl)).rejects.toMatchObject({
      code: 'RESOURCE_INTEGRITY_FAILED',
    });
  });

  it('rejects caller-supplied credential headers', async () => {
    const authorized = await plan();
    const guardedFetch = createMicroAppIntegrityFetch(authorized, vi.fn());

    await expect(
      guardedFetch(authorized.entryUrl, { headers: { Authorization: 'Bearer forbidden' } }),
    ).rejects.toMatchObject({ code: 'RESOURCE_REQUEST_DENIED' });
  });
});
