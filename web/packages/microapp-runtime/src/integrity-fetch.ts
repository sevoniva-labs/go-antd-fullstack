import { verifySha256Integrity } from '@forge/manifest-security';

import { MicroAppRuntimeError, runtimeError } from './errors';
import {
  isAuthorizedMicroAppLaunchPlan,
  type AuthorizedMicroAppLaunchPlan,
  type MicroAppResource,
} from './policy';

const SENSITIVE_HEADER_PATTERN = /^(?:authorization|cookie|proxy-authorization|x-api-key)$/i;

function requestUrl(input: RequestInfo | URL, base: string): URL {
  const value = input instanceof Request ? input.url : input.toString();
  try {
    return new URL(value, base);
  } catch {
    return runtimeError('RESOURCE_REQUEST_DENIED', 'Resource request URL is invalid');
  }
}

function requestHeaders(input: RequestInfo | URL, init?: RequestInit): Headers {
  const headers = new Headers(input instanceof Request ? input.headers : undefined);
  new Headers(init?.headers).forEach((value, name) => headers.set(name, value));
  for (const name of headers.keys()) {
    if (SENSITIVE_HEADER_PATTERN.test(name)) {
      return runtimeError('RESOURCE_REQUEST_DENIED', 'Static resource requests cannot set credentials');
    }
  }
  return headers;
}

async function verifyResponse(
  response: Response,
  resource: MicroAppResource,
): Promise<void> {
  if (!response.ok || response.type === 'opaque') {
    return runtimeError('RESOURCE_FETCH_FAILED', 'Static resource request failed');
  }
  const declaredLength = Number(response.headers.get('content-length'));
  if (Number.isFinite(declaredLength) && declaredLength > resource.maxBytes) {
    return runtimeError('RESOURCE_TOO_LARGE', 'Static resource exceeds its declared byte limit');
  }
  const bytes = new Uint8Array(await response.clone().arrayBuffer());
  if (bytes.byteLength > resource.maxBytes) {
    return runtimeError('RESOURCE_TOO_LARGE', 'Static resource exceeds its declared byte limit');
  }
  try {
    await verifySha256Integrity(bytes, resource.integrity);
  } catch (error) {
    return runtimeError(
      'RESOURCE_INTEGRITY_FAILED',
      'Static resource integrity verification failed',
      error,
    );
  }
}

export function createMicroAppIntegrityFetch(
  plan: AuthorizedMicroAppLaunchPlan,
  fetcher: typeof fetch = globalThis.fetch,
): typeof fetch {
  if (!isAuthorizedMicroAppLaunchPlan(plan)) {
    return runtimeError('UNVERIFIED_MANIFEST', 'Integrity fetch requires an authorized launch plan');
  }
  const resources = new Map(plan.resources.map((resource) => [resource.url, resource]));
  const appOrigin = new URL(plan.entryUrl).origin;

  return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = requestUrl(input, plan.entryUrl);
    if (url.origin !== appOrigin || url.search || url.hash) {
      return runtimeError('RESOURCE_REQUEST_DENIED', 'Resource request crossed its app boundary');
    }
    const resource = resources.get(url.href);
    if (!resource) {
      return runtimeError('RESOURCE_NOT_DECLARED', 'Resource is absent from the signed inventory');
    }
    const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase();
    if (method !== 'GET') {
      return runtimeError('RESOURCE_REQUEST_DENIED', 'Static resource requests must use GET');
    }

    let response: Response;
    try {
      response = await fetcher(url, {
        ...init,
        method: 'GET',
        headers: requestHeaders(input, init),
        credentials: url.origin === globalThis.location?.origin ? 'same-origin' : 'omit',
        redirect: 'error',
      });
    } catch (error) {
      if (error instanceof MicroAppRuntimeError) throw error;
      return runtimeError('RESOURCE_FETCH_FAILED', 'Static resource request failed', error);
    }
    await verifyResponse(response, resource);
    return response;
  };
}

export async function probeMicroAppHealth(
  plan: AuthorizedMicroAppLaunchPlan,
  fetcher: typeof fetch = globalThis.fetch,
): Promise<void> {
  if (!isAuthorizedMicroAppLaunchPlan(plan)) {
    return runtimeError('UNVERIFIED_MANIFEST', 'Health check requires an authorized launch plan');
  }
  const controller = new AbortController();
  const timeout = globalThis.setTimeout(() => controller.abort(), plan.health.timeoutMs);
  try {
    const response = await fetcher(plan.health.url, {
      method: 'HEAD',
      cache: 'no-store',
      credentials: 'omit',
      redirect: 'error',
      signal: controller.signal,
    });
    if (!response.ok || response.type === 'opaque') {
      return runtimeError('HEALTH_CHECK_FAILED', 'Micro-app health endpoint is unavailable');
    }
  } catch (error) {
    if (error instanceof MicroAppRuntimeError) throw error;
    return runtimeError('HEALTH_CHECK_FAILED', 'Micro-app health probe failed', error);
  } finally {
    globalThis.clearTimeout(timeout);
  }
}
