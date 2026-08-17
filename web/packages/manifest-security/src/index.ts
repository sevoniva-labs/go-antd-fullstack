export type ManifestSignatureAlgorithm = 'ECDSA_P256_SHA256' | 'SM2_SM3';

export interface ManifestSignatureEnvelope {
  readonly algorithm: ManifestSignatureAlgorithm;
  readonly keyId: string;
  readonly issuedAt: string;
  readonly expiresAt: string;
  readonly value: string;
}

export interface TrustedManifestKey {
  readonly keyId: string;
  readonly algorithm: ManifestSignatureAlgorithm;
  readonly status: 'active' | 'revoked';
  readonly notBefore: string;
  readonly notAfter: string;
  readonly material: unknown;
}

export interface ManifestKeyStore {
  resolve(keyId: string): Promise<TrustedManifestKey | null>;
}

export interface ManifestSignatureProvider {
  readonly algorithm: ManifestSignatureAlgorithm;
  verify(input: Readonly<{
    payload: Uint8Array;
    signature: Uint8Array;
    keyMaterial: unknown;
  }>): Promise<boolean>;
}

export interface VerifyManifestOptions {
  readonly keyStore: ManifestKeyStore;
  readonly providers?: readonly ManifestSignatureProvider[];
  readonly now?: Date;
  readonly clockSkewMs?: number;
  readonly maxSignatureLifetimeMs?: number;
}

export type ManifestSecurityErrorCode =
  | 'INVALID_DOCUMENT'
  | 'NON_CANONICAL_DOCUMENT'
  | 'INVALID_SIGNATURE_ENVELOPE'
  | 'SIGNATURE_EXPIRED'
  | 'SIGNATURE_LIFETIME_EXCEEDED'
  | 'KEY_NOT_TRUSTED'
  | 'KEY_REVOKED'
  | 'KEY_NOT_VALID'
  | 'ALGORITHM_MISMATCH'
  | 'ALGORITHM_UNAVAILABLE'
  | 'SIGNATURE_INVALID'
  | 'INTEGRITY_INVALID';

export class ManifestSecurityError extends Error {
  readonly code: ManifestSecurityErrorCode;

  constructor(code: ManifestSecurityErrorCode, message: string) {
    super(message);
    this.name = 'ManifestSecurityError';
    this.code = code;
  }
}

const KEY_ID_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._:-]{2,127}$/;
const BASE64URL_PATTERN = /^[A-Za-z0-9_-]+$/;
const FORBIDDEN_OBJECT_KEYS = new Set(['__proto__', 'constructor', 'prototype']);
const DEFAULT_CLOCK_SKEW_MS = 30_000;
const DEFAULT_MAX_SIGNATURE_LIFETIME_MS = 30 * 24 * 60 * 60 * 1000;

function securityError(code: ManifestSecurityErrorCode, message: string): never {
  throw new ManifestSecurityError(code, message);
}

function parseTimestamp(
  value: string,
  field: string,
  errorCode: ManifestSecurityErrorCode,
): number {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{3})?Z$/.test(value)) {
    return securityError(errorCode, `${field} must be an RFC 3339 UTC timestamp`);
  }
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) {
    return securityError(errorCode, `${field} is invalid`);
  }
  return parsed;
}

function canonicalizeValue(value: unknown, seen: WeakSet<object>): string {
  if (value === null) return 'null';
  if (typeof value === 'string' || typeof value === 'boolean') return JSON.stringify(value);
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) {
      return securityError('INVALID_DOCUMENT', 'Manifest numbers must be finite');
    }
    return JSON.stringify(value);
  }
  if (typeof value !== 'object') {
    return securityError('INVALID_DOCUMENT', 'Manifest contains an unsupported JSON value');
  }
  if (seen.has(value)) {
    return securityError('INVALID_DOCUMENT', 'Manifest cannot contain cyclic data');
  }
  seen.add(value);

  if (Array.isArray(value)) {
    const result = `[${value.map((entry) => canonicalizeValue(entry, seen)).join(',')}]`;
    seen.delete(value);
    return result;
  }
  const prototype = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) {
    return securityError('INVALID_DOCUMENT', 'Manifest objects must use a plain prototype');
  }

  const entries = Object.entries(value).sort(([left], [right]) =>
    left < right ? -1 : left > right ? 1 : 0,
  );
  const result = `{${entries
    .map(([key, entry]) => {
      if (FORBIDDEN_OBJECT_KEYS.has(key)) {
        return securityError('INVALID_DOCUMENT', 'Manifest contains a forbidden object key');
      }
      return `${JSON.stringify(key)}:${canonicalizeValue(entry, seen)}`;
    })
    .join(',')}}`;
  seen.delete(value);
  return result;
}

export function canonicalizeManifest(value: unknown): string {
  if (value === null || Array.isArray(value) || typeof value !== 'object') {
    return securityError('INVALID_DOCUMENT', 'Manifest root must be a JSON object');
  }
  return canonicalizeValue(value, new WeakSet());
}

function decodeUtf8(payload: Uint8Array): string {
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(payload);
  } catch {
    return securityError('INVALID_DOCUMENT', 'Manifest payload must be valid UTF-8');
  }
}

export function decodeBase64Url(value: string): Uint8Array {
  if (!BASE64URL_PATTERN.test(value)) {
    return securityError('INVALID_SIGNATURE_ENVELOPE', 'Signature must use unpadded base64url');
  }
  const padding = '='.repeat((4 - (value.length % 4)) % 4);
  try {
    const binary = atob(value.replace(/-/g, '+').replace(/_/g, '/') + padding);
    return Uint8Array.from(binary, (character) => character.charCodeAt(0));
  } catch {
    return securityError('INVALID_SIGNATURE_ENVELOPE', 'Signature base64url is invalid');
  }
}

export function encodeBase64Url(value: Uint8Array): string {
  let binary = '';
  value.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function encodeBase64(value: Uint8Array): string {
  let binary = '';
  value.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

function constantTimeEqual(left: Uint8Array, right: Uint8Array): boolean {
  let difference = left.length ^ right.length;
  const length = Math.max(left.length, right.length);
  for (let index = 0; index < length; index += 1) {
    difference |= (left[index] ?? 0) ^ (right[index] ?? 0);
  }
  return difference === 0;
}

function asJsonWebKey(material: unknown): JsonWebKey {
  if (material === null || typeof material !== 'object' || Array.isArray(material)) {
    return securityError('KEY_NOT_VALID', 'ECDSA key material must be a JWK object');
  }
  return material as JsonWebKey;
}

function webCryptoBytes(value: Uint8Array): Uint8Array<ArrayBuffer> {
  const copy = new Uint8Array(value.byteLength);
  copy.set(value);
  return copy;
}

export function createEcdsaP256Provider(
  subtle: SubtleCrypto = globalThis.crypto.subtle,
): ManifestSignatureProvider {
  const provider: ManifestSignatureProvider = {
    algorithm: 'ECDSA_P256_SHA256' as const,
    async verify({ payload, signature, keyMaterial }): Promise<boolean> {
      const key = await subtle.importKey(
        'jwk',
        asJsonWebKey(keyMaterial),
        { name: 'ECDSA', namedCurve: 'P-256' },
        false,
        ['verify'],
      );
      return subtle.verify(
        { name: 'ECDSA', hash: 'SHA-256' },
        key,
        webCryptoBytes(signature),
        webCryptoBytes(payload),
      );
    },
  };
  return Object.freeze(provider);
}

function validateSignatureWindow(
  envelope: ManifestSignatureEnvelope,
  now: number,
  clockSkewMs: number,
  maxSignatureLifetimeMs: number,
): Readonly<{ issuedAt: number; expiresAt: number }> {
  if (!KEY_ID_PATTERN.test(envelope.keyId) || !BASE64URL_PATTERN.test(envelope.value)) {
    return securityError('INVALID_SIGNATURE_ENVELOPE', 'Signature envelope is malformed');
  }
  const issuedAt = parseTimestamp(
    envelope.issuedAt,
    'Signature issuedAt',
    'INVALID_SIGNATURE_ENVELOPE',
  );
  const expiresAt = parseTimestamp(
    envelope.expiresAt,
    'Signature expiresAt',
    'INVALID_SIGNATURE_ENVELOPE',
  );
  if (expiresAt <= issuedAt) {
    return securityError('INVALID_SIGNATURE_ENVELOPE', 'Signature expiry must follow issuance');
  }
  if (expiresAt - issuedAt > maxSignatureLifetimeMs) {
    return securityError('SIGNATURE_LIFETIME_EXCEEDED', 'Signature lifetime exceeds policy');
  }
  if (issuedAt > now + clockSkewMs || expiresAt < now - clockSkewMs) {
    return securityError('SIGNATURE_EXPIRED', 'Signature is outside its validity window');
  }
  return { issuedAt, expiresAt };
}

function parseCanonicalDocument<T>(payload: Uint8Array): T {
  const source = decodeUtf8(payload);
  let parsed: unknown;
  try {
    parsed = JSON.parse(source);
  } catch {
    return securityError('INVALID_DOCUMENT', 'Manifest payload is not valid JSON');
  }
  const canonical = canonicalizeManifest(parsed);
  if (source !== canonical) {
    return securityError('NON_CANONICAL_DOCUMENT', 'Manifest payload is not canonical JSON');
  }
  return parsed as T;
}

function deepFreeze<T>(value: T, seen = new WeakSet<object>()): T {
  if (value === null || typeof value !== 'object' || seen.has(value)) return value;
  seen.add(value);
  Object.values(value).forEach((entry) => deepFreeze(entry, seen));
  return Object.freeze(value);
}

const VERIFIED_MANIFEST_DOCUMENT = Symbol('forge.verified-manifest-document');
const verifiedManifestDocuments = new WeakSet<object>();

export interface VerifiedManifestDocument<T extends object> {
  readonly [VERIFIED_MANIFEST_DOCUMENT]: true;
  readonly manifest: Readonly<T>;
  readonly keyId: string;
  readonly algorithm: ManifestSignatureAlgorithm;
  readonly payloadSha256: string;
  readonly verifiedAt: string;
}

function createVerifiedManifestDocument<T extends object>(input: Readonly<{
  manifest: T;
  keyId: string;
  algorithm: ManifestSignatureAlgorithm;
  payloadSha256: string;
  verifiedAt: string;
}>): VerifiedManifestDocument<T> {
  const document: VerifiedManifestDocument<T> = {
    [VERIFIED_MANIFEST_DOCUMENT]: true,
    manifest: deepFreeze(input.manifest),
    keyId: input.keyId,
    algorithm: input.algorithm,
    payloadSha256: input.payloadSha256,
    verifiedAt: input.verifiedAt,
  };
  verifiedManifestDocuments.add(document);
  return Object.freeze(document);
}

export function isVerifiedManifestDocument(
  value: unknown,
): value is VerifiedManifestDocument<object> {
  return typeof value === 'object' && value !== null && verifiedManifestDocuments.has(value);
}

export async function sha256Integrity(payload: Uint8Array): Promise<string> {
  const digest = new Uint8Array(
    await globalThis.crypto.subtle.digest('SHA-256', webCryptoBytes(payload)),
  );
  return `sha256-${encodeBase64(digest)}`;
}

export async function verifySha256Integrity(
  payload: Uint8Array,
  expectedIntegrity: string,
): Promise<void> {
  if (!expectedIntegrity.startsWith('sha256-')) {
    return securityError('INTEGRITY_INVALID', 'Only SHA-256 integrity values are accepted');
  }
  let expected: Uint8Array;
  try {
    const value = expectedIntegrity.slice('sha256-'.length);
    if (!/^[A-Za-z0-9+/]+={0,2}$/.test(value)) throw new Error('invalid base64');
    const binary = atob(value);
    expected = Uint8Array.from(binary, (character) => character.charCodeAt(0));
  } catch {
    return securityError('INTEGRITY_INVALID', 'Integrity value is malformed');
  }
  const actual = new Uint8Array(
    await globalThis.crypto.subtle.digest('SHA-256', webCryptoBytes(payload)),
  );
  if (!constantTimeEqual(actual, expected)) {
    return securityError('INTEGRITY_INVALID', 'Artifact integrity verification failed');
  }
}

export async function verifyManifestDocument<T extends object>(
  payload: Uint8Array,
  envelope: ManifestSignatureEnvelope,
  options: VerifyManifestOptions,
): Promise<VerifiedManifestDocument<T>> {
  const now = options.now ?? new Date();
  if (!Number.isFinite(now.getTime())) {
    return securityError('INVALID_SIGNATURE_ENVELOPE', 'Verification time is invalid');
  }
  const clockSkewMs = options.clockSkewMs ?? DEFAULT_CLOCK_SKEW_MS;
  const maxSignatureLifetimeMs =
    options.maxSignatureLifetimeMs ?? DEFAULT_MAX_SIGNATURE_LIFETIME_MS;
  if (clockSkewMs < 0 || maxSignatureLifetimeMs <= 0) {
    return securityError('INVALID_SIGNATURE_ENVELOPE', 'Signature policy is invalid');
  }
  const validity = validateSignatureWindow(
    envelope,
    now.getTime(),
    clockSkewMs,
    maxSignatureLifetimeMs,
  );
  const manifest = parseCanonicalDocument<T>(payload);
  const key = await options.keyStore.resolve(envelope.keyId);
  if (!key) {
    return securityError('KEY_NOT_TRUSTED', 'Manifest signing key is not trusted');
  }
  if (key.status === 'revoked') {
    return securityError('KEY_REVOKED', 'Manifest signing key is revoked');
  }
  if (key.algorithm !== envelope.algorithm) {
    return securityError('ALGORITHM_MISMATCH', 'Signature and key algorithms do not match');
  }
  const keyNotBefore = parseTimestamp(key.notBefore, 'Key notBefore', 'KEY_NOT_VALID');
  const keyNotAfter = parseTimestamp(key.notAfter, 'Key notAfter', 'KEY_NOT_VALID');
  if (validity.issuedAt < keyNotBefore || validity.expiresAt > keyNotAfter) {
    return securityError('KEY_NOT_VALID', 'Signature validity exceeds the key validity window');
  }

  const providers = options.providers ?? [createEcdsaP256Provider()];
  const provider = providers.find(({ algorithm }) => algorithm === envelope.algorithm);
  if (!provider) {
    return securityError('ALGORITHM_UNAVAILABLE', 'Required signature provider is unavailable');
  }
  let verified = false;
  try {
    verified = await provider.verify({
      payload,
      signature: decodeBase64Url(envelope.value),
      keyMaterial: key.material,
    });
  } catch (error) {
    if (error instanceof ManifestSecurityError) throw error;
    return securityError('SIGNATURE_INVALID', 'Manifest signature verification failed');
  }
  if (!verified) {
    return securityError('SIGNATURE_INVALID', 'Manifest signature verification failed');
  }

  return createVerifiedManifestDocument({
    manifest,
    keyId: key.keyId,
    algorithm: envelope.algorithm,
    payloadSha256: await sha256Integrity(payload),
    verifiedAt: now.toISOString(),
  });
}
