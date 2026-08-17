export type MicroAppRuntimeErrorCode =
  | 'INVALID_LAUNCH_PLAN'
  | 'UNVERIFIED_MANIFEST'
  | 'TRUST_BOUNDARY_VIOLATION'
  | 'PERMISSION_DENIED'
  | 'ROLLOUT_EXCLUDED'
  | 'CIRCUIT_OPEN'
  | 'HEALTH_CHECK_FAILED'
  | 'RESOURCE_NOT_DECLARED'
  | 'RESOURCE_TOO_LARGE'
  | 'RESOURCE_REQUEST_DENIED'
  | 'RESOURCE_FETCH_FAILED'
  | 'RESOURCE_INTEGRITY_FAILED';

export class MicroAppRuntimeError extends Error {
  readonly code: MicroAppRuntimeErrorCode;
  readonly cause?: unknown;

  constructor(code: MicroAppRuntimeErrorCode, message: string, cause?: unknown) {
    super(message);
    this.name = 'MicroAppRuntimeError';
    this.code = code;
    this.cause = cause;
  }
}

export function runtimeError(
  code: MicroAppRuntimeErrorCode,
  message: string,
  cause?: unknown,
): never {
  throw new MicroAppRuntimeError(code, message, cause);
}
