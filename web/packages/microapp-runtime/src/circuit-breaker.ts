import { runtimeError } from './errors';

export interface CircuitBreakerPolicy {
  readonly failureThreshold: number;
  readonly failureWindowMs: number;
  readonly cooldownMs: number;
}

export interface CircuitBreakerDecision {
  readonly allowed: boolean;
  readonly retryAt?: number;
  readonly failures: number;
}

interface CircuitRecord {
  failures: number[];
  openUntil?: number;
}

const DEFAULT_POLICY: CircuitBreakerPolicy = {
  failureThreshold: 3,
  failureWindowMs: 60_000,
  cooldownMs: 120_000,
};

export class MicroAppCircuitBreaker {
  readonly #policy: CircuitBreakerPolicy;
  readonly #records = new Map<string, CircuitRecord>();

  constructor(policy: CircuitBreakerPolicy = DEFAULT_POLICY) {
    if (
      !Number.isSafeInteger(policy.failureThreshold) ||
      policy.failureThreshold < 1 ||
      policy.failureThreshold > 20 ||
      !Number.isSafeInteger(policy.failureWindowMs) ||
      policy.failureWindowMs < 1_000 ||
      policy.failureWindowMs > 60 * 60 * 1000 ||
      !Number.isSafeInteger(policy.cooldownMs) ||
      policy.cooldownMs < 1_000 ||
      policy.cooldownMs > 24 * 60 * 60 * 1000
    ) {
      runtimeError('INVALID_LAUNCH_PLAN', 'Circuit-breaker policy is invalid');
    }
    this.#policy = Object.freeze({ ...policy });
  }

  canAttempt(releaseId: string, now = Date.now()): CircuitBreakerDecision {
    const record = this.#records.get(releaseId);
    if (!record) return { allowed: true, failures: 0 };
    this.#prune(record, now);
    if (record.openUntil && record.openUntil > now) {
      return { allowed: false, retryAt: record.openUntil, failures: record.failures.length };
    }
    if (record.openUntil) {
      record.openUntil = undefined;
      record.failures = [];
    }
    return { allowed: true, failures: record.failures.length };
  }

  recordFailure(releaseId: string, now = Date.now()): CircuitBreakerDecision {
    const record = this.#records.get(releaseId) ?? { failures: [] };
    this.#prune(record, now);
    record.failures.push(now);
    if (record.failures.length >= this.#policy.failureThreshold) {
      record.openUntil = now + this.#policy.cooldownMs;
    }
    this.#records.set(releaseId, record);
    return this.canAttempt(releaseId, now);
  }

  recordSuccess(releaseId: string): void {
    this.#records.delete(releaseId);
  }

  reset(releaseId: string): void {
    this.#records.delete(releaseId);
  }

  #prune(record: CircuitRecord, now: number): void {
    const cutoff = now - this.#policy.failureWindowMs;
    record.failures = record.failures.filter((failure) => failure >= cutoff);
  }
}
