import {
  ManifestSecurityError,
  decodeBase64Url,
  verifyManifestDocument,
  type ManifestKeyStore,
  type ManifestSignatureEnvelope,
  type ManifestSignatureProvider,
  type VerifiedManifestDocument,
} from './index';

export interface LoadSignedManifestOptions {
  readonly shellOrigin: string;
  readonly keyStore: ManifestKeyStore;
  readonly providers?: readonly ManifestSignatureProvider[];
  readonly fetcher?: typeof fetch;
  readonly timeoutMs?: number;
  readonly maxBundleBytes?: number;
  readonly maxPayloadBytes?: number;
  readonly now?: Date;
}

const DEFAULT_TIMEOUT_MS = 3_000;
const DEFAULT_MAX_BUNDLE_BYTES = 512 * 1024;
const DEFAULT_MAX_PAYLOAD_BYTES = 256 * 1024;
const BUNDLE_CONTENT_TYPES = new Set([
  'application/json',
  'application/vnd.forge.microapp-manifest+json',
]);

function loaderError(code: 'INVALID_DOCUMENT' | 'KEY_NOT_TRUSTED', message: string): never {
  throw new ManifestSecurityError(code, message);
}

function positiveLimit(value: number, maximum: number, name: string): number {
  if (!Number.isSafeInteger(value) || value < 1 || value > maximum) {
    return loaderError('INVALID_DOCUMENT', name + ' is outside policy');
  }
  return value;
}

function manifestUrl(value: string, shellOrigin: string): URL {
  let origin: URL;
  let url: URL;
  try {
    origin = new URL(shellOrigin);
    url = new URL(value, origin);
  } catch {
    return loaderError('INVALID_DOCUMENT', 'Manifest URL or Shell origin is invalid');
  }
  const localDevelopment =
    origin.protocol === 'http:' && ['127.0.0.1', 'localhost'].includes(origin.hostname);
  if (
    origin.origin !== shellOrigin ||
    (origin.protocol !== 'https:' && !localDevelopment) ||
    url.origin !== origin.origin ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    return loaderError('INVALID_DOCUMENT', 'Manifest must use a clean same-origin URL');
  }
  return url;
}

function parseEnvelope(value: unknown): ManifestSignatureEnvelope {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return loaderError('INVALID_DOCUMENT', 'Manifest signature must be an object');
  }
  const source = value as Record<string, unknown>;
  const expected = ['algorithm', 'expiresAt', 'issuedAt', 'keyId', 'value'];
  if (
    Object.keys(source).sort().join('\u0000') !== expected.sort().join('\u0000') ||
    (source.algorithm !== 'ECDSA_P256_SHA256' && source.algorithm !== 'SM2_SM3') ||
    typeof source.keyId !== 'string' ||
    typeof source.issuedAt !== 'string' ||
    typeof source.expiresAt !== 'string' ||
    typeof source.value !== 'string'
  ) {
    return loaderError('INVALID_DOCUMENT', 'Manifest signature envelope is malformed');
  }
  return {
    algorithm: source.algorithm,
    keyId: source.keyId,
    issuedAt: source.issuedAt,
    expiresAt: source.expiresAt,
    value: source.value,
  };
}

function parseBundle(source: string, maxPayloadBytes: number): Readonly<{
  payload: Uint8Array;
  signature: ManifestSignatureEnvelope;
}> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(source);
  } catch {
    return loaderError('INVALID_DOCUMENT', 'Manifest bundle is not valid JSON');
  }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return loaderError('INVALID_DOCUMENT', 'Manifest bundle must be an object');
  }
  const bundle = parsed as Record<string, unknown>;
  if (
    Object.keys(bundle).sort().join('\u0000') !== 'payload\u0000signature' ||
    typeof bundle.payload !== 'string'
  ) {
    return loaderError('INVALID_DOCUMENT', 'Manifest bundle fields are invalid');
  }
  let payload: Uint8Array;
  try {
    payload = decodeBase64Url(bundle.payload);
  } catch {
    return loaderError('INVALID_DOCUMENT', 'Manifest payload encoding is invalid');
  }
  if (payload.byteLength > maxPayloadBytes) {
    return loaderError('INVALID_DOCUMENT', 'Manifest payload exceeds its byte limit');
  }
  return { payload, signature: parseEnvelope(bundle.signature) };
}

export async function loadSignedManifest<T extends object>(
  url: string,
  options: LoadSignedManifestOptions,
): Promise<VerifiedManifestDocument<T>> {
  const target = manifestUrl(url, options.shellOrigin);
  const timeoutMs = positiveLimit(options.timeoutMs ?? DEFAULT_TIMEOUT_MS, 10_000, 'Timeout');
  const maxBundleBytes = positiveLimit(
    options.maxBundleBytes ?? DEFAULT_MAX_BUNDLE_BYTES,
    2 * 1024 * 1024,
    'Bundle byte limit',
  );
  const maxPayloadBytes = positiveLimit(
    options.maxPayloadBytes ?? DEFAULT_MAX_PAYLOAD_BYTES,
    1024 * 1024,
    'Payload byte limit',
  );
  const controller = new AbortController();
  const timeout = globalThis.setTimeout(() => controller.abort(), timeoutMs);
  let response: Response;
  try {
    response = await (options.fetcher ?? globalThis.fetch)(target, {
      method: 'GET',
      headers: { Accept: 'application/vnd.forge.microapp-manifest+json, application/json' },
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      signal: controller.signal,
    });
  } catch {
    return loaderError('KEY_NOT_TRUSTED', 'Signed manifest could not be loaded');
  } finally {
    globalThis.clearTimeout(timeout);
  }
  if (!response.ok || response.type === 'opaque') {
    return loaderError('KEY_NOT_TRUSTED', 'Signed manifest endpoint is unavailable');
  }
  const contentType = response.headers.get('content-type')?.split(';', 1)[0]?.trim() ?? '';
  if (!BUNDLE_CONTENT_TYPES.has(contentType)) {
    return loaderError('INVALID_DOCUMENT', 'Signed manifest Content-Type is not allowed');
  }
  const declaredLength = Number(response.headers.get('content-length'));
  if (Number.isFinite(declaredLength) && declaredLength > maxBundleBytes) {
    return loaderError('INVALID_DOCUMENT', 'Manifest bundle exceeds its byte limit');
  }
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (bytes.byteLength > maxBundleBytes) {
    return loaderError('INVALID_DOCUMENT', 'Manifest bundle exceeds its byte limit');
  }
  let source: string;
  try {
    source = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  } catch {
    return loaderError('INVALID_DOCUMENT', 'Manifest bundle must use valid UTF-8');
  }
  const bundle = parseBundle(source, maxPayloadBytes);
  return verifyManifestDocument<T>(bundle.payload, bundle.signature, {
    keyStore: options.keyStore,
    ...(options.providers === undefined ? {} : { providers: options.providers }),
    ...(options.now === undefined ? {} : { now: options.now }),
  });
}
