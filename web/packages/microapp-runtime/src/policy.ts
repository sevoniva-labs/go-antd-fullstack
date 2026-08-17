import {
  isVerifiedManifestDocument,
  type ManifestSignatureAlgorithm,
  type VerifiedManifestDocument,
} from '@forge/manifest-security';

import { runtimeError } from './errors';

export type MicroAppRuntime = 'iframe' | 'wujie';
export type MicroAppTrust = 'trusted-internal' | 'untrusted-external';

export interface MicroAppResourceCandidate {
  readonly url: string;
  readonly integrity: string;
  readonly maxBytes?: number;
}

export interface MicroAppLaunchCandidate {
  readonly id: string;
  readonly title: string;
  readonly version: string;
  readonly runtime: MicroAppRuntime;
  readonly trust: MicroAppTrust;
  readonly entryUrl: string;
  readonly routePrefix: string;
  readonly requiredPermissions: readonly string[];
  readonly rollout: Readonly<{
    percentage: number;
    salt: string;
  }>;
  readonly health: Readonly<{
    url: string;
    timeoutMs: number;
  }>;
  readonly startupTimeoutMs: number;
  readonly resources: readonly MicroAppResourceCandidate[];
}

export interface MicroAppResource {
  readonly url: string;
  readonly integrity: string;
  readonly maxBytes: number;
}

const AUTHORIZED_LAUNCH_PLAN = Symbol('forge.authorized-launch-plan');
const authorizedLaunchPlans = new WeakSet<object>();

export interface AuthorizedMicroAppLaunchPlan {
  readonly [AUTHORIZED_LAUNCH_PLAN]: true;
  readonly id: string;
  readonly title: string;
  readonly version: string;
  readonly releaseId: string;
  readonly runtime: MicroAppRuntime;
  readonly trust: MicroAppTrust;
  readonly entryUrl: string;
  readonly routePrefix: string;
  readonly requiredPermissions: readonly string[];
  readonly rollout: Readonly<{
    percentage: number;
    salt: string;
  }>;
  readonly health: Readonly<{
    url: string;
    timeoutMs: number;
  }>;
  readonly startupTimeoutMs: number;
  readonly resources: readonly MicroAppResource[];
  readonly verification: Readonly<{
    keyId: string;
    algorithm: ManifestSignatureAlgorithm;
    payloadSha256: string;
    verifiedAt: string;
  }>;
}

export interface AuthorizeLaunchPlanOptions<TManifest extends object> {
  readonly verifiedManifest: VerifiedManifestDocument<TManifest>;
  readonly shellOrigin: string;
  readonly select: (manifest: Readonly<TManifest>) => MicroAppLaunchCandidate;
}

const APP_ID_PATTERN = /^[a-z][a-z0-9-]{1,62}[a-z0-9]$/;
const SEMVER_PATTERN = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;
const PERMISSION_PATTERN = /^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+$/;
const INTEGRITY_PATTERN = /^sha256-[A-Za-z0-9+/]+={0,2}$/;
const DEFAULT_MAX_RESOURCE_BYTES = 8 * 1024 * 1024;
const MAX_RESOURCE_BYTES = 32 * 1024 * 1024;

function parseOrigin(value: string): string {
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return runtimeError('INVALID_LAUNCH_PLAN', 'Shell origin must be an absolute URL');
  }
  if (parsed.origin !== value || !['http:', 'https:'].includes(parsed.protocol)) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'Shell origin must contain only scheme and authority');
  }
  return parsed.origin;
}

function parseSecureUrl(value: string, field: string): URL {
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return runtimeError('INVALID_LAUNCH_PLAN', `${field} must be an absolute URL`);
  }
  const localDevelopment =
    parsed.protocol === 'http:' && ['127.0.0.1', 'localhost'].includes(parsed.hostname);
  if ((parsed.protocol !== 'https:' && !localDevelopment) || parsed.username || parsed.password) {
    return runtimeError('INVALID_LAUNCH_PLAN', `${field} must use a secure URL without credentials`);
  }
  if (parsed.search || parsed.hash) {
    return runtimeError('INVALID_LAUNCH_PLAN', `${field} cannot contain a query or fragment`);
  }
  return parsed;
}

function normalizeResources(
  candidates: readonly MicroAppResourceCandidate[],
  entry: URL,
): readonly MicroAppResource[] {
  if (candidates.length === 0 || candidates.length > 2048) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'Resource inventory must contain 1 to 2048 items');
  }
  const seen = new Set<string>();
  const resources = candidates.map((candidate) => {
    const resourceUrl = parseSecureUrl(candidate.url, 'Resource URL');
    if (resourceUrl.origin !== entry.origin) {
      return runtimeError('TRUST_BOUNDARY_VIOLATION', 'All app resources must use the entry origin');
    }
    if (seen.has(resourceUrl.href)) {
      return runtimeError('INVALID_LAUNCH_PLAN', 'Resource inventory contains duplicate URLs');
    }
    seen.add(resourceUrl.href);
    if (!INTEGRITY_PATTERN.test(candidate.integrity)) {
      return runtimeError('INVALID_LAUNCH_PLAN', 'Resources must declare SHA-256 integrity');
    }
    const maxBytes = candidate.maxBytes ?? DEFAULT_MAX_RESOURCE_BYTES;
    if (!Number.isSafeInteger(maxBytes) || maxBytes < 1 || maxBytes > MAX_RESOURCE_BYTES) {
      return runtimeError('INVALID_LAUNCH_PLAN', 'Resource byte limit is outside policy');
    }
    return Object.freeze({
      url: resourceUrl.href,
      integrity: candidate.integrity,
      maxBytes,
    });
  });
  if (!seen.has(entry.href)) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'Resource inventory must include the entry document');
  }
  return Object.freeze(resources);
}

export function authorizeMicroAppLaunch<TManifest extends object>(
  options: AuthorizeLaunchPlanOptions<TManifest>,
): AuthorizedMicroAppLaunchPlan {
  if (!isVerifiedManifestDocument(options.verifiedManifest)) {
    return runtimeError('UNVERIFIED_MANIFEST', 'Launch plans require authentic verification evidence');
  }
  const candidate = options.select(options.verifiedManifest.manifest);
  const shellOrigin = parseOrigin(options.shellOrigin);
  if (!APP_ID_PATTERN.test(candidate.id) || !SEMVER_PATTERN.test(candidate.version)) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'App ID or version is invalid');
  }
  if (!candidate.title.trim() || candidate.title.length > 128) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'App title is invalid');
  }
  if (candidate.routePrefix !== `/apps/${candidate.id}`) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'App route must use its reserved /apps/{id} prefix');
  }

  const entry = parseSecureUrl(candidate.entryUrl, 'Entry URL');
  if (
    candidate.runtime === 'wujie' &&
    (candidate.trust !== 'trusted-internal' || entry.origin !== shellOrigin)
  ) {
    return runtimeError(
      'TRUST_BOUNDARY_VIOLATION',
      'Wujie is restricted to trusted apps on the Shell origin',
    );
  }
  if (
    candidate.runtime === 'iframe' &&
    (candidate.trust !== 'untrusted-external' || entry.origin === shellOrigin)
  ) {
    return runtimeError(
      'TRUST_BOUNDARY_VIOLATION',
      'Untrusted iframe apps require an independent origin',
    );
  }

  const permissions = [...new Set(candidate.requiredPermissions)].sort();
  if (
    permissions.length === 0 ||
    permissions.some((permission) => !PERMISSION_PATTERN.test(permission) || permission.includes('*'))
  ) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'App permissions must be explicit and non-empty');
  }
  if (
    !Number.isInteger(candidate.rollout.percentage) ||
    candidate.rollout.percentage < 0 ||
    candidate.rollout.percentage > 100 ||
    candidate.rollout.salt.length < 16 ||
    candidate.rollout.salt.length > 128
  ) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'Rollout policy is invalid');
  }

  const healthUrl = parseSecureUrl(candidate.health.url, 'Health URL');
  if (healthUrl.origin !== entry.origin) {
    return runtimeError('TRUST_BOUNDARY_VIOLATION', 'Health endpoint must use the app origin');
  }
  if (
    !Number.isSafeInteger(candidate.health.timeoutMs) ||
    candidate.health.timeoutMs < 250 ||
    candidate.health.timeoutMs > 10_000 ||
    !Number.isSafeInteger(candidate.startupTimeoutMs) ||
    candidate.startupTimeoutMs < 1_000 ||
    candidate.startupTimeoutMs > 30_000
  ) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'Health or startup timeout is outside policy');
  }

  const plan: AuthorizedMicroAppLaunchPlan = {
    [AUTHORIZED_LAUNCH_PLAN]: true,
    id: candidate.id,
    title: candidate.title.trim(),
    version: candidate.version,
    releaseId: `${candidate.id}@${candidate.version}`,
    runtime: candidate.runtime,
    trust: candidate.trust,
    entryUrl: entry.href,
    routePrefix: candidate.routePrefix,
    requiredPermissions: Object.freeze(permissions),
    rollout: Object.freeze({ ...candidate.rollout }),
    health: Object.freeze({ url: healthUrl.href, timeoutMs: candidate.health.timeoutMs }),
    startupTimeoutMs: candidate.startupTimeoutMs,
    resources: normalizeResources(candidate.resources, entry),
    verification: Object.freeze({
      keyId: options.verifiedManifest.keyId,
      algorithm: options.verifiedManifest.algorithm,
      payloadSha256: options.verifiedManifest.payloadSha256,
      verifiedAt: options.verifiedManifest.verifiedAt,
    }),
  };
  authorizedLaunchPlans.add(plan);
  return Object.freeze(plan);
}

export function isAuthorizedMicroAppLaunchPlan(
  value: unknown,
): value is AuthorizedMicroAppLaunchPlan {
  return typeof value === 'object' && value !== null && authorizedLaunchPlans.has(value);
}

function webCryptoBytes(value: Uint8Array): Uint8Array<ArrayBuffer> {
  const copy = new Uint8Array(value.byteLength);
  copy.set(value);
  return copy;
}

export async function isMicroAppRolloutEligible(
  plan: AuthorizedMicroAppLaunchPlan,
  subjectId: string,
): Promise<boolean> {
  if (!isAuthorizedMicroAppLaunchPlan(plan) || !subjectId.trim()) {
    return runtimeError('INVALID_LAUNCH_PLAN', 'Rollout requires an authorized plan and subject');
  }
  if (plan.rollout.percentage === 0) return false;
  if (plan.rollout.percentage === 100) return true;
  const input = new TextEncoder().encode(
    `${plan.rollout.salt}\u0000${plan.id}\u0000${subjectId.trim()}`,
  );
  const digest = await crypto.subtle.digest('SHA-256', webCryptoBytes(input));
  const bucket = new DataView(digest).getUint32(0, false) % 10_000;
  return bucket < plan.rollout.percentage * 100;
}
