import { describe, expect, it, vi } from 'vitest';

import {
  HostSdkError,
  createHostSdk,
  type CreateHostSdkOptions,
  type HostEventHub,
} from './index';

function createEventHub(): HostEventHub {
  const listeners = new Map<string, Set<(payload: unknown) => void>>();
  return {
    publish(topic, payload) {
      listeners.get(topic)?.forEach((listener) => listener(payload));
    },
    subscribe(topic, listener) {
      const topicListeners = listeners.get(topic) ?? new Set();
      topicListeners.add(listener);
      listeners.set(topic, topicListeners);
      return () => topicListeners.delete(listener);
    },
  };
}

function createOptions(overrides: Partial<CreateHostSdkOptions> = {}): CreateHostSdkOptions {
  return {
    appId: 'risk-console',
    appVersion: '1.2.3',
    origin: 'https://portal.example.cn',
    apiNamespaces: ['/api/risk'],
    routePrefixes: ['/apps/risk'],
    publishTopics: ['risk.case-updated'],
    subscribeTopics: ['shell.theme-changed'],
    getSession: () => ({
      userId: 'user-1',
      displayName: '审计员',
      organizationId: 'org-1',
      permissions: ['risk.case.read'],
      dataScopes: ['organization:org-1'],
    }),
    getContext: () => ({
      locale: 'zh-CN',
      theme: { mode: 'light', primaryColor: '#0052d9' },
    }),
    transport: vi.fn(async () => ({ ok: true })),
    navigate: vi.fn(),
    eventHub: createEventHub(),
    ...overrides,
  };
}

describe('createHostSdk', () => {
  it('allows only the exact API namespace and never accepts credential headers', async () => {
    const transport = vi.fn(async () => ({ id: 'case-1' }));
    const sdk = createHostSdk(createOptions({ transport }));

    await expect(
      sdk.request({ method: 'POST', path: '/api/risk/cases?state=open', body: { level: 2 } }),
    ).resolves.toEqual({ id: 'case-1' });
    expect(transport).toHaveBeenCalledWith({
      method: 'POST',
      path: '/api/risk/cases?state=open',
      body: { level: 2 },
    });
    await expect(sdk.request({ path: '/api/risk-admin' })).rejects.toMatchObject({
      code: 'API_NAMESPACE_DENIED',
    });
    await expect(sdk.request({ path: 'https://evil.example/api/risk' })).rejects.toBeInstanceOf(
      HostSdkError,
    );
  });

  it('returns an immutable credential-free session snapshot', async () => {
    const sdk = createHostSdk(
      createOptions({
        getSession: () => ({
          userId: 'user-1',
          displayName: '管理员',
          organizationId: 'org-1',
          permissions: ['system.user.read'],
          dataScopes: ['organization:org-1'],
          token: 'must-not-escape',
        } as ReturnType<CreateHostSdkOptions['getSession']> & { token: string }),
      }),
    );

    const session = await sdk.getSession();
    expect(session).toEqual({
      userId: 'user-1',
      displayName: '管理员',
      organizationId: 'org-1',
      permissions: ['system.user.read'],
      dataScopes: ['organization:org-1'],
    });
    expect('token' in session).toBe(false);
    expect(Object.isFrozen(session)).toBe(true);
    expect(Object.isFrozen(session.permissions)).toBe(true);
  });

  it('confines navigation to granted routes', async () => {
    const navigate = vi.fn();
    const sdk = createHostSdk(createOptions({ navigate }));

    await sdk.navigate('/apps/risk/cases/1?tab=events#latest', { replace: true });
    expect(navigate).toHaveBeenCalledWith('/apps/risk/cases/1?tab=events#latest', {
      replace: true,
    });
    await expect(sdk.navigate('/platform/users')).rejects.toMatchObject({
      code: 'INVALID_NAVIGATION',
    });
  });

  it('enforces topic grants and rejects sensitive event fields', () => {
    const sdk = createHostSdk(createOptions());
    const listener = vi.fn();
    const unsubscribe = sdk.subscribe('shell.theme-changed', listener);

    expect(() => sdk.publish('risk.case-updated', { caseId: 'case-1' })).not.toThrow();
    expect(() => sdk.publish('risk.case-updated', { accessToken: 'secret' })).toThrowError(
      HostSdkError,
    );
    expect(() => sdk.publish('shell.theme-changed', {})).toThrowError(HostSdkError);
    expect(unsubscribe()).toBe(true);
  });

  it('reports only bounded, non-sensitive telemetry attributes', () => {
    const telemetry = vi.fn();
    const sdk = createHostSdk(createOptions({ telemetry }));

    sdk.report('microapp.ready', { path: '/apps/risk?case=secret', durationMs: 25 });
    expect(telemetry).toHaveBeenCalledWith({
      appId: 'risk-console',
      name: 'microapp.ready',
      attributes: { path: '/apps/risk', durationMs: 25 },
    });
    expect(() => sdk.report('microapp.error', { authorization: 'Bearer secret' })).toThrowError(
      HostSdkError,
    );
  });
});
