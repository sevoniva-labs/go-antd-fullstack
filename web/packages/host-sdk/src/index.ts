export type HostThemeMode = 'light' | 'dark' | 'system';

export type HostRequestMethod = 'DELETE' | 'GET' | 'PATCH' | 'POST' | 'PUT';

export interface HostSessionSource {
  userId: string;
  displayName: string;
  organizationId: string;
  permissions: readonly string[];
  dataScopes: readonly string[];
}

export interface HostSessionSnapshot {
  readonly userId: string;
  readonly displayName: string;
  readonly organizationId: string;
  readonly permissions: readonly string[];
  readonly dataScopes: readonly string[];
}

export interface HostContextSnapshot {
  readonly locale: string;
  readonly theme: Readonly<{
    mode: HostThemeMode;
    primaryColor: string;
  }>;
}

export interface HostApiRequest {
  readonly method?: HostRequestMethod;
  readonly path: string;
  readonly body?: unknown;
  readonly signal?: AbortSignal;
}

export interface HostTransportRequest extends HostApiRequest {
  readonly method: HostRequestMethod;
}

export type HostTransport = (request: HostTransportRequest) => Promise<unknown>;

export type HostNavigate = (
  target: string,
  options: Readonly<{ replace: boolean }>,
) => void | Promise<void>;

export type HostTelemetry = (
  event: Readonly<{
    appId: string;
    name: string;
    attributes: Readonly<Record<string, string | number | boolean>>;
  }>,
) => void;

export interface HostEventHub {
  publish(topic: string, payload: unknown): void;
  subscribe(topic: string, listener: (payload: unknown) => void): () => void;
}

export interface CreateHostSdkOptions {
  readonly appId: string;
  readonly appVersion: string;
  readonly origin: string;
  readonly apiNamespaces: readonly string[];
  readonly routePrefixes: readonly string[];
  readonly publishTopics: readonly string[];
  readonly subscribeTopics: readonly string[];
  readonly getSession: () => HostSessionSource | Promise<HostSessionSource>;
  readonly getContext: () => HostContextSnapshot;
  readonly transport: HostTransport;
  readonly navigate: HostNavigate;
  readonly eventHub: HostEventHub;
  readonly telemetry?: HostTelemetry;
  readonly maxEventBytes?: number;
}

export interface HostSdk {
  readonly appId: string;
  readonly appVersion: string;
  getSession(): Promise<HostSessionSnapshot>;
  getContext(): HostContextSnapshot;
  hasPermission(permission: string): Promise<boolean>;
  request<T>(request: HostApiRequest): Promise<T>;
  navigate(target: string, options?: Readonly<{ replace?: boolean }>): Promise<void>;
  publish(topic: string, payload: unknown): void;
  subscribe(topic: string, listener: (payload: unknown) => void): () => void;
  report(
    name: string,
    attributes?: Readonly<Record<string, string | number | boolean>>,
  ): void;
}

export type HostSdkErrorCode =
  | 'INVALID_CONFIGURATION'
  | 'INVALID_NAVIGATION'
  | 'API_NAMESPACE_DENIED'
  | 'EVENT_TOPIC_DENIED'
  | 'EVENT_PAYLOAD_DENIED'
  | 'INVALID_TELEMETRY';

export class HostSdkError extends Error {
  readonly code: HostSdkErrorCode;

  constructor(code: HostSdkErrorCode, message: string) {
    super(message);
    this.name = 'HostSdkError';
    this.code = code;
  }
}

const APP_ID_PATTERN = /^[a-z][a-z0-9-]{1,62}[a-z0-9]$/;
const TOPIC_PATTERN = /^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$/;
const TELEMETRY_NAME_PATTERN = /^[a-z][a-z0-9_.-]{2,127}$/;
const SENSITIVE_KEY_PATTERN = /(?:authorization|cookie|credential|password|secret|session|token)/i;
const DEFAULT_MAX_EVENT_BYTES = 32 * 1024;

function configurationError(message: string): never {
  throw new HostSdkError('INVALID_CONFIGURATION', message);
}

function normalizeOrigin(value: string): string {
  let origin: URL;
  try {
    origin = new URL(value);
  } catch {
    return configurationError('Host origin must be an absolute URL');
  }
  if (origin.origin !== value || !['http:', 'https:'].includes(origin.protocol)) {
    return configurationError('Host origin must contain only scheme and authority');
  }
  return origin.origin;
}

function normalizePathPrefix(value: string, field: string): string {
  if (!value.startsWith('/') || value.startsWith('//') || value.includes('\\')) {
    return configurationError(`${field} must be an absolute same-origin path`);
  }
  const normalized = value.length > 1 ? value.replace(/\/+$/, '') : value;
  if (normalized.includes('?') || normalized.includes('#')) {
    return configurationError(`${field} cannot contain a query or fragment`);
  }
  return normalized;
}

function normalizePrefixes(values: readonly string[], field: string): readonly string[] {
  if (values.length === 0) {
    return configurationError(`${field} cannot be empty`);
  }
  return Object.freeze([...new Set(values.map((value) => normalizePathPrefix(value, field)))]);
}

function normalizeTopics(values: readonly string[], field: string): ReadonlySet<string> {
  for (const value of values) {
    if (!TOPIC_PATTERN.test(value)) {
      return configurationError(`${field} contains an invalid topic`);
    }
  }
  return new Set(values);
}

function isWithinPrefix(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

function parseSameOriginTarget(
  target: string,
  origin: string,
  errorCode: 'API_NAMESPACE_DENIED' | 'INVALID_NAVIGATION',
): URL {
  if (!target.startsWith('/') || target.startsWith('//') || target.includes('\\')) {
    throw new HostSdkError(errorCode, 'Only absolute same-origin paths are allowed');
  }
  const parsed = new URL(target, origin);
  if (parsed.origin !== origin || parsed.username || parsed.password) {
    throw new HostSdkError(errorCode, 'Cross-origin targets are not allowed');
  }
  return parsed;
}

function cloneSession(source: HostSessionSource): HostSessionSnapshot {
  const snapshot: HostSessionSnapshot = {
    userId: source.userId,
    displayName: source.displayName,
    organizationId: source.organizationId,
    permissions: Object.freeze([...source.permissions]),
    dataScopes: Object.freeze([...source.dataScopes]),
  };
  return Object.freeze(snapshot);
}

function cloneContext(source: HostContextSnapshot): HostContextSnapshot {
  return Object.freeze({
    locale: source.locale,
    theme: Object.freeze({ ...source.theme }),
  });
}

function assertSafeEventPayload(payload: unknown, maxBytes: number): void {
  const seen = new WeakSet<object>();
  const inspect = (value: unknown): void => {
    if (value === null || typeof value !== 'object') {
      return;
    }
    if (seen.has(value)) {
      throw new HostSdkError('EVENT_PAYLOAD_DENIED', 'Cyclic event payloads are not allowed');
    }
    seen.add(value);
    if (Array.isArray(value)) {
      value.forEach(inspect);
      return;
    }
    for (const [key, nested] of Object.entries(value)) {
      if (SENSITIVE_KEY_PATTERN.test(key)) {
        throw new HostSdkError('EVENT_PAYLOAD_DENIED', 'Sensitive event fields are not allowed');
      }
      inspect(nested);
    }
  };

  inspect(payload);
  let serialized: string | undefined;
  try {
    serialized = JSON.stringify(payload);
  } catch {
    throw new HostSdkError('EVENT_PAYLOAD_DENIED', 'Event payload must be JSON serializable');
  }
  if (serialized === undefined || new TextEncoder().encode(serialized).byteLength > maxBytes) {
    throw new HostSdkError('EVENT_PAYLOAD_DENIED', 'Event payload exceeds the allowed size');
  }
}

function sanitizeTelemetryAttributes(
  attributes: Readonly<Record<string, string | number | boolean>>,
): Readonly<Record<string, string | number | boolean>> {
  const sanitized: Record<string, string | number | boolean> = {};
  for (const [key, value] of Object.entries(attributes)) {
    if (SENSITIVE_KEY_PATTERN.test(key)) {
      throw new HostSdkError('INVALID_TELEMETRY', 'Sensitive telemetry fields are not allowed');
    }
    if (typeof value === 'string') {
      sanitized[key] = value.split(/[?#]/, 1)[0]?.slice(0, 256) ?? '';
    } else {
      sanitized[key] = value;
    }
  }
  return Object.freeze(sanitized);
}

export function createHostSdk(options: CreateHostSdkOptions): HostSdk {
  if (!APP_ID_PATTERN.test(options.appId)) {
    return configurationError('App ID must use lowercase kebab-case');
  }
  if (!options.appVersion.trim()) {
    return configurationError('App version cannot be empty');
  }

  const origin = normalizeOrigin(options.origin);
  const apiNamespaces = normalizePrefixes(options.apiNamespaces, 'API namespaces');
  const routePrefixes = normalizePrefixes(options.routePrefixes, 'Route prefixes');
  const publishTopics = normalizeTopics(options.publishTopics, 'Publish topics');
  const subscribeTopics = normalizeTopics(options.subscribeTopics, 'Subscribe topics');
  const maxEventBytes = options.maxEventBytes ?? DEFAULT_MAX_EVENT_BYTES;
  if (!Number.isSafeInteger(maxEventBytes) || maxEventBytes < 1 || maxEventBytes > 1024 * 1024) {
    return configurationError('Event payload limit must be between 1 byte and 1 MiB');
  }

  const ensureTopic = (topic: string, allowed: ReadonlySet<string>): void => {
    if (!allowed.has(topic)) {
      throw new HostSdkError('EVENT_TOPIC_DENIED', 'Event topic is not granted to this app');
    }
  };

  return Object.freeze({
    appId: options.appId,
    appVersion: options.appVersion,
    async getSession(): Promise<HostSessionSnapshot> {
      return cloneSession(await options.getSession());
    },
    getContext(): HostContextSnapshot {
      return cloneContext(options.getContext());
    },
    async hasPermission(permission: string): Promise<boolean> {
      const session = await options.getSession();
      return session.permissions.includes(permission);
    },
    async request<T>(request: HostApiRequest): Promise<T> {
      const target = parseSameOriginTarget(request.path, origin, 'API_NAMESPACE_DENIED');
      if (!apiNamespaces.some((prefix) => isWithinPrefix(target.pathname, prefix))) {
        throw new HostSdkError('API_NAMESPACE_DENIED', 'API path is outside the app grant');
      }
      return (await options.transport({
        method: request.method ?? 'GET',
        path: `${target.pathname}${target.search}`,
        ...(request.body === undefined ? {} : { body: request.body }),
        ...(request.signal === undefined ? {} : { signal: request.signal }),
      })) as T;
    },
    async navigate(
      target: string,
      navigationOptions: Readonly<{ replace?: boolean }> = {},
    ): Promise<void> {
      const parsed = parseSameOriginTarget(target, origin, 'INVALID_NAVIGATION');
      if (!routePrefixes.some((prefix) => isWithinPrefix(parsed.pathname, prefix))) {
        throw new HostSdkError('INVALID_NAVIGATION', 'Route is outside the app grant');
      }
      await options.navigate(`${parsed.pathname}${parsed.search}${parsed.hash}`, {
        replace: navigationOptions.replace ?? false,
      });
    },
    publish(topic: string, payload: unknown): void {
      ensureTopic(topic, publishTopics);
      assertSafeEventPayload(payload, maxEventBytes);
      options.eventHub.publish(topic, payload);
    },
    subscribe(topic: string, listener: (payload: unknown) => void): () => void {
      ensureTopic(topic, subscribeTopics);
      return options.eventHub.subscribe(topic, listener);
    },
    report(
      name: string,
      attributes: Readonly<Record<string, string | number | boolean>> = {},
    ): void {
      if (!TELEMETRY_NAME_PATTERN.test(name)) {
        throw new HostSdkError('INVALID_TELEMETRY', 'Telemetry name is invalid');
      }
      options.telemetry?.({
        appId: options.appId,
        name,
        attributes: sanitizeTelemetryAttributes(attributes),
      });
    },
  });
}
