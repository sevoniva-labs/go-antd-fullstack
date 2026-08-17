import type { HostContextSnapshot, HostSdk, HostSessionSnapshot } from '@forge/host-sdk';

export interface RemoteViewModel {
  readonly session: HostSessionSnapshot;
  readonly context: HostContextSnapshot;
  readonly canReadUsers: boolean;
}

export function resolveHostSdk(target: Window): HostSdk | null {
  return target.$wujie?.props?.hostSdk ?? null;
}

export async function loadRemoteViewModel(hostSdk: HostSdk): Promise<RemoteViewModel> {
  const [session, canReadUsers] = await Promise.all([
    hostSdk.getSession(),
    hostSdk.hasPermission('system.user.read'),
  ]);
  return Object.freeze({
    session,
    context: hostSdk.getContext(),
    canReadUsers,
  });
}
