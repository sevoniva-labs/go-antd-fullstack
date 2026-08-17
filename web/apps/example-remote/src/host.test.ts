import { describe, expect, it, vi } from 'vitest';

import type { HostSdk } from '@forge/host-sdk';

import { loadRemoteViewModel, resolveHostSdk } from './host';

function hostSdk(): HostSdk {
  return {
    appId: 'example-remote',
    appVersion: '1.0.0',
    getSession: vi.fn(async () => ({
      userId: 'user-1',
      displayName: '风险管理员',
      organizationId: 'org-1',
      permissions: ['system.user.read'],
      dataScopes: ['organization:org-1'],
    })),
    getContext: vi.fn(() => ({
      locale: 'zh-CN',
      theme: { mode: 'light' as const, primaryColor: '#0052d9' },
    })),
    hasPermission: vi.fn(async () => true),
    request: vi.fn(),
    navigate: vi.fn(),
    publish: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    report: vi.fn(),
  };
}

describe('remote host boundary', () => {
  it('fails closed when the app is opened without a host', () => {
    expect(resolveHostSdk({ $wujie: undefined } as Window)).toBeNull();
  });

  it('loads only the credential-free host view model', async () => {
    const sdk = hostSdk();

    await expect(loadRemoteViewModel(sdk)).resolves.toMatchObject({
      session: { userId: 'user-1', organizationId: 'org-1' },
      context: { locale: 'zh-CN' },
      canReadUsers: true,
    });
    expect(sdk.hasPermission).toHaveBeenCalledWith('system.user.read');
  });
});
