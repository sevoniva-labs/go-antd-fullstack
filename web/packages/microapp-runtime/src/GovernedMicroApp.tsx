import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';

import type { HostSdk } from '@forge/host-sdk';

import { MicroAppCircuitBreaker } from './circuit-breaker';
import { MicroAppRuntimeError } from './errors';
import { createMicroAppIntegrityFetch } from './integrity-fetch';
import { IsolatedIframeRuntime } from './IsolatedIframe';
import { evaluateMicroAppLaunch, type MicroAppLaunchBlockReason } from './launch';
import {
  isAuthorizedMicroAppLaunchPlan,
  type AuthorizedMicroAppLaunchPlan,
} from './policy';

export type GovernedMicroAppStatus =
  | Readonly<{ state: 'checking' }>
  | Readonly<{ state: 'disabled' }>
  | Readonly<{ state: 'blocked'; reason: MicroAppLaunchBlockReason; retryAt?: number }>
  | Readonly<{ state: 'loading' }>
  | Readonly<{ state: 'ready' }>
  | Readonly<{ state: 'error'; code: string }>;

export interface GovernedMicroAppProps {
  readonly plan: AuthorizedMicroAppLaunchPlan;
  readonly enabled?: boolean;
  readonly subjectId: string;
  readonly permissions: readonly string[];
  readonly hostSdk?: HostSdk;
  readonly circuitBreaker?: MicroAppCircuitBreaker;
  readonly fetcher?: typeof fetch;
  readonly className?: string;
  readonly loading?: ReactNode;
  readonly renderFallback?: (status: GovernedMicroAppStatus) => ReactNode;
  readonly onStatusChange?: (status: GovernedMicroAppStatus) => void;
}

interface RuntimeProviderProps {
  readonly plan: AuthorizedMicroAppLaunchPlan;
  readonly hostSdk?: HostSdk;
  readonly fetcher: typeof fetch;
  readonly className?: string;
  readonly onReady: () => void;
  readonly onFailure: (error: unknown) => void;
}

const defaultCircuitBreaker = new MicroAppCircuitBreaker();
const wujieLifecycleQueues = new Map<string, Promise<void>>();

function enqueueWujieLifecycle(name: string, operation: () => Promise<void>): Promise<void> {
  const previous = wujieLifecycleQueues.get(name) ?? Promise.resolve();
  const current = previous.catch(() => undefined).then(operation);
  wujieLifecycleQueues.set(name, current);
  void current.then(
    () => {
      if (wujieLifecycleQueues.get(name) === current) wujieLifecycleQueues.delete(name);
    },
    () => {
      if (wujieLifecycleQueues.get(name) === current) wujieLifecycleQueues.delete(name);
    },
  );
  return current;
}

function WujieRuntimeProvider({
  plan,
  hostSdk,
  fetcher,
  className,
  onReady,
  onFailure,
}: RuntimeProviderProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!hostSdk || hostSdk.appId !== plan.id || hostSdk.appVersion !== plan.version) {
      onFailure(new MicroAppRuntimeError('INVALID_LAUNCH_PLAN', 'Host SDK release does not match'));
      return undefined;
    }
    const container = containerRef.current;
    if (!container) {
      onFailure(new MicroAppRuntimeError('INVALID_LAUNCH_PLAN', 'Wujie container is unavailable'));
      return undefined;
    }

    let disposed = false;
    const appName = `forge-${plan.releaseId.replace(/[^a-zA-Z0-9-]/g, '-')}`;
    const timeout = globalThis.setTimeout(() => {
      if (!disposed) onFailure(new Error('Micro-app startup timed out'));
    }, plan.startupTimeoutMs);

    void enqueueWujieLifecycle(appName, async () => {
      const { startApp } = await import('wujie');
      if (disposed) return;
      await startApp({
        name: appName,
        url: plan.entryUrl,
        el: container,
        alive: false,
        degrade: false,
        fiber: false,
        sync: false,
        fetch: createMicroAppIntegrityFetch(plan, fetcher),
        props: { hostSdk },
        afterMount: () => {
          if (disposed) return;
          globalThis.clearTimeout(timeout);
          onReady();
        },
        loadError: (_url, error) => {
          if (disposed) return;
          globalThis.clearTimeout(timeout);
          onFailure(error);
        },
      });
    }).catch((error: unknown) => {
      if (disposed) return;
      globalThis.clearTimeout(timeout);
      onFailure(error);
    });

    return () => {
      disposed = true;
      globalThis.clearTimeout(timeout);
      void enqueueWujieLifecycle(appName, async () => {
        const { destroyApp } = await import('wujie');
        await destroyApp(appName);
      }).catch(() => undefined);
    };
  }, [fetcher, hostSdk, onFailure, onReady, plan]);

  return <div ref={containerRef} className={className} data-microapp-runtime="wujie" />;
}

function IframeRuntimeProvider({
  plan,
  className,
  onReady,
  onFailure,
}: RuntimeProviderProps) {
  return (
    <IsolatedIframeRuntime
      className={className}
      entryUrl={plan.entryUrl}
      startupTimeoutMs={plan.startupTimeoutMs}
      title={plan.title}
      onFailure={onFailure}
      onReady={onReady}
    />
  );
}

function defaultFallback(status: GovernedMicroAppStatus): ReactNode {
  if (status.state === 'checking' || status.state === 'loading') return null;
  const code = status.state === 'blocked' ? status.reason : status.state;
  return (
    <section role="alert" data-microapp-fallback={code}>
      微应用当前不可用，请联系平台管理员或稍后重试。
    </section>
  );
}

export function GovernedMicroApp({
  plan,
  enabled = false,
  subjectId,
  permissions,
  hostSdk,
  circuitBreaker = defaultCircuitBreaker,
  fetcher = globalThis.fetch,
  className,
  loading = null,
  renderFallback = defaultFallback,
  onStatusChange,
}: GovernedMicroAppProps) {
  const [status, setStatus] = useState<GovernedMicroAppStatus>({ state: 'checking' });
  const permissionsKey = [...permissions].sort().join('\u0000');
  const permissionSet = useMemo(() => new Set(permissions), [permissionsKey]);
  const statusCallback = useRef(onStatusChange);
  statusCallback.current = onStatusChange;

  const updateStatus = useCallback((next: GovernedMicroAppStatus) => {
    setStatus(next);
    statusCallback.current?.(next);
  }, []);

  useEffect(() => {
    let active = true;
    if (!enabled) {
      updateStatus({ state: 'disabled' });
      return () => {
        active = false;
      };
    }
    if (!isAuthorizedMicroAppLaunchPlan(plan)) {
      updateStatus({ state: 'error', code: 'UNVERIFIED_MANIFEST' });
      return () => {
        active = false;
      };
    }

    updateStatus({ state: 'checking' });
    void evaluateMicroAppLaunch({
      plan,
      subjectId,
      permissions: permissionSet,
      circuitBreaker,
      fetcher,
    })
      .then((decision) => {
        if (!active) return;
        if (!decision.allowed) {
          updateStatus({
            state: 'blocked',
            reason: decision.reason,
            ...(decision.retryAt === undefined ? {} : { retryAt: decision.retryAt }),
          });
          return;
        }
        updateStatus({ state: 'loading' });
      })
      .catch((error: unknown) => {
        if (!active) return;
        circuitBreaker.recordFailure(plan.releaseId);
        updateStatus({
          state: 'error',
          code: error instanceof MicroAppRuntimeError ? error.code : 'PREPARATION_FAILED',
        });
      });

    return () => {
      active = false;
    };
  }, [circuitBreaker, enabled, fetcher, permissionSet, plan, subjectId, updateStatus]);

  const handleReady = useCallback(() => {
    circuitBreaker.recordSuccess(plan.releaseId);
    updateStatus({ state: 'ready' });
  }, [circuitBreaker, plan.releaseId, updateStatus]);

  const handleFailure = useCallback(
    (error: unknown) => {
      circuitBreaker.recordFailure(plan.releaseId);
      updateStatus({
        state: 'error',
        code: error instanceof MicroAppRuntimeError ? error.code : 'STARTUP_FAILED',
      });
    },
    [circuitBreaker, plan.releaseId, updateStatus],
  );

  if (status.state !== 'loading' && status.state !== 'ready') {
    return <>{status.state === 'checking' ? loading : renderFallback(status)}</>;
  }

  const providerProps: RuntimeProviderProps = {
    plan,
    hostSdk,
    fetcher,
    className,
    onReady: handleReady,
    onFailure: handleFailure,
  };
  return plan.runtime === 'wujie' ? (
    <WujieRuntimeProvider {...providerProps} />
  ) : (
    <IframeRuntimeProvider {...providerProps} />
  );
}
