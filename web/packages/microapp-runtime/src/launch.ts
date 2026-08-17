import { MicroAppCircuitBreaker } from './circuit-breaker';
import { runtimeError } from './errors';
import { createMicroAppIntegrityFetch, probeMicroAppHealth } from './integrity-fetch';
import {
  isAuthorizedMicroAppLaunchPlan,
  isMicroAppRolloutEligible,
  type AuthorizedMicroAppLaunchPlan,
} from './policy';

export type MicroAppLaunchBlockReason =
  | 'permission-denied'
  | 'rollout-excluded'
  | 'circuit-open';

export type MicroAppLaunchDecision =
  | Readonly<{ allowed: true }>
  | Readonly<{
      allowed: false;
      reason: MicroAppLaunchBlockReason;
      retryAt?: number;
    }>;

export interface EvaluateMicroAppLaunchOptions {
  readonly plan: AuthorizedMicroAppLaunchPlan;
  readonly subjectId: string;
  readonly permissions: ReadonlySet<string>;
  readonly circuitBreaker: MicroAppCircuitBreaker;
  readonly fetcher?: typeof fetch;
}

export async function evaluateMicroAppLaunch(
  options: EvaluateMicroAppLaunchOptions,
): Promise<MicroAppLaunchDecision> {
  if (!isAuthorizedMicroAppLaunchPlan(options.plan)) {
    return runtimeError('UNVERIFIED_MANIFEST', 'Launch evaluation requires an authorized plan');
  }
  if (!options.plan.requiredPermissions.every((permission) => options.permissions.has(permission))) {
    return { allowed: false, reason: 'permission-denied' };
  }
  if (!(await isMicroAppRolloutEligible(options.plan, options.subjectId))) {
    return { allowed: false, reason: 'rollout-excluded' };
  }
  const circuit = options.circuitBreaker.canAttempt(options.plan.releaseId);
  if (!circuit.allowed) {
    return {
      allowed: false,
      reason: 'circuit-open',
      ...(circuit.retryAt === undefined ? {} : { retryAt: circuit.retryAt }),
    };
  }

  const fetcher = options.fetcher ?? globalThis.fetch;
  await probeMicroAppHealth(options.plan, fetcher);
  if (options.plan.runtime === 'iframe') {
    await createMicroAppIntegrityFetch(options.plan, fetcher)(options.plan.entryUrl);
  }
  return { allowed: true };
}
