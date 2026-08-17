import {
  validateMicroAppManifest,
  type ManifestValidationOptions,
  type MicroAppManifest,
  type MicroAppRelease,
  type MicroAppStatus,
} from '@forge/app-contract';
import {
  isVerifiedManifestDocument,
  type VerifiedManifestDocument,
} from '@forge/manifest-security';

import { MicroAppCircuitBreaker } from './circuit-breaker';
import { runtimeError } from './errors';
import {
  authorizeMicroAppLaunch,
  isAuthorizedMicroAppLaunchPlan,
  type AuthorizedMicroAppLaunchPlan,
  type MicroAppLaunchCandidate,
} from './policy';

export interface MicroAppReleaseSetMetadata {
  readonly name: string;
  readonly displayName: string;
  readonly status: MicroAppStatus;
  readonly featureFlag?: string;
  readonly owner: string;
  readonly fallbackPath: string;
  readonly rolloutCohortKey: 'user_id' | 'organization_id';
  readonly requiredDataScopes: readonly string[];
  readonly allowedApiPrefixes: readonly string[];
  readonly events: Readonly<{
    publish: readonly string[];
    subscribe: readonly string[];
  }>;
  readonly csp: Readonly<{
    connectSrc: readonly string[];
    imgSrc: readonly string[];
    frameSrc: readonly string[];
  }>;
  readonly circuitPolicy: Readonly<{
    failureThreshold: number;
    failureWindowMs: number;
    cooldownMs: number;
  }>;
}

const AUTHORIZED_RELEASE_SET = Symbol('forge.authorized-release-set');
const authorizedReleaseSets = new WeakSet<object>();

export interface AuthorizedMicroAppReleaseSet {
  readonly [AUTHORIZED_RELEASE_SET]: true;
  readonly primary: AuthorizedMicroAppLaunchPlan;
  readonly rollback?: AuthorizedMicroAppLaunchPlan;
  readonly metadata: MicroAppReleaseSetMetadata;
}

export interface AuthorizeManifestReleaseSetOptions<TManifest extends object> {
  readonly verifiedManifest: VerifiedManifestDocument<TManifest>;
  readonly validation: ManifestValidationOptions;
}

function launchCandidate(
  manifest: MicroAppManifest,
  release: MicroAppRelease,
  rollback: boolean,
): MicroAppLaunchCandidate {
  return {
    id: manifest.name,
    title: manifest.displayName,
    version: release.version,
    runtime: manifest.runtime,
    trust: manifest.trust,
    entryUrl: release.entry,
    routePrefix: manifest.routePrefix,
    requiredPermissions: manifest.requiredPermissions,
    rollout: {
      percentage: rollback ? 100 : manifest.rollout.percentage,
      salt: manifest.rollout.salt,
    },
    health: {
      url: release.healthUrl,
      timeoutMs: manifest.health.timeoutMs,
    },
    startupTimeoutMs: manifest.health.startupTimeoutMs,
    resources: release.resources,
  };
}

function frozenList(values: readonly string[]): readonly string[] {
  return Object.freeze([...values]);
}

export function authorizeManifestReleaseSet<TManifest extends object>(
  options: AuthorizeManifestReleaseSetOptions<TManifest>,
): AuthorizedMicroAppReleaseSet {
  if (!isVerifiedManifestDocument(options.verifiedManifest)) {
    return runtimeError('UNVERIFIED_MANIFEST', 'Release set requires authentic signature evidence');
  }
  const manifest = validateMicroAppManifest(
    options.verifiedManifest.manifest,
    options.validation,
  );
  const primaryCandidate = launchCandidate(manifest, manifest.releases.primary, false);
  const primary = authorizeMicroAppLaunch({
    verifiedManifest: options.verifiedManifest,
    shellOrigin: options.validation.shellOrigin,
    select: () => primaryCandidate,
  });
  const rollbackCandidate = manifest.releases.rollback
    ? launchCandidate(manifest, manifest.releases.rollback, true)
    : undefined;
  const rollback = rollbackCandidate
    ? authorizeMicroAppLaunch({
        verifiedManifest: options.verifiedManifest,
        shellOrigin: options.validation.shellOrigin,
        select: () => rollbackCandidate,
      })
    : undefined;

  const releaseSet: AuthorizedMicroAppReleaseSet = {
    [AUTHORIZED_RELEASE_SET]: true,
    primary,
    ...(rollback === undefined ? {} : { rollback }),
    metadata: Object.freeze({
      name: manifest.name,
      displayName: manifest.displayName,
      status: manifest.status,
      ...(manifest.featureFlag === undefined ? {} : { featureFlag: manifest.featureFlag }),
      owner: manifest.owner,
      fallbackPath: manifest.fallbackPath,
      rolloutCohortKey: manifest.rollout.cohortKey,
      requiredDataScopes: frozenList(manifest.requiredDataScopes),
      allowedApiPrefixes: frozenList(manifest.allowedApiPrefixes),
      events: Object.freeze({
        publish: frozenList(manifest.events.publish),
        subscribe: frozenList(manifest.events.subscribe),
      }),
      csp: Object.freeze({
        connectSrc: frozenList(manifest.csp.connectSrc),
        imgSrc: frozenList(manifest.csp.imgSrc),
        frameSrc: frozenList(manifest.csp.frameSrc),
      }),
      circuitPolicy: Object.freeze({
        failureThreshold: manifest.health.maxFailures,
        failureWindowMs: manifest.health.failureWindowSeconds * 1000,
        cooldownMs: manifest.health.recoverySeconds * 1000,
      }),
    }),
  };
  authorizedReleaseSets.add(releaseSet);
  return Object.freeze(releaseSet);
}

export function isAuthorizedMicroAppReleaseSet(
  value: unknown,
): value is AuthorizedMicroAppReleaseSet {
  return typeof value === 'object' && value !== null && authorizedReleaseSets.has(value);
}

export function isMicroAppReleaseEnabled(
  releaseSet: AuthorizedMicroAppReleaseSet,
  enabledFeatureFlags: ReadonlySet<string>,
): boolean {
  if (!isAuthorizedMicroAppReleaseSet(releaseSet) || releaseSet.metadata.status !== 'active') {
    return false;
  }
  const featureFlag = releaseSet.metadata.featureFlag;
  return featureFlag === undefined || enabledFeatureFlags.has(featureFlag);
}

export function selectMicroAppRollback(
  releaseSet: AuthorizedMicroAppReleaseSet,
  failedReleaseId: string,
): AuthorizedMicroAppLaunchPlan | null {
  if (
    !isAuthorizedMicroAppReleaseSet(releaseSet) ||
    !isAuthorizedMicroAppLaunchPlan(releaseSet.primary) ||
    failedReleaseId !== releaseSet.primary.releaseId
  ) {
    return null;
  }
  return releaseSet.rollback ?? null;
}

export function createReleaseSetCircuitBreaker(
  releaseSet: AuthorizedMicroAppReleaseSet,
): MicroAppCircuitBreaker {
  if (!isAuthorizedMicroAppReleaseSet(releaseSet)) {
    return runtimeError('UNVERIFIED_MANIFEST', 'Circuit breaker requires an authorized release set');
  }
  return new MicroAppCircuitBreaker(releaseSet.metadata.circuitPolicy);
}

export function selectMicroAppRolloutSubject(
  releaseSet: AuthorizedMicroAppReleaseSet,
  session: Readonly<{ userId: string; organizationId: string }>,
): string {
  if (!isAuthorizedMicroAppReleaseSet(releaseSet)) {
    return runtimeError('UNVERIFIED_MANIFEST', 'Rollout subject requires an authorized release set');
  }
  return releaseSet.metadata.rolloutCohortKey === 'organization_id'
    ? session.organizationId
    : session.userId;
}
