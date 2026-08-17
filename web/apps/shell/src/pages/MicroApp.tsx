import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Result, Space, Spin, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'

import type { MicroAppManifest } from '@forge/app-contract'
import { apiFetch } from '@forge/api-client'
import { useMe } from '@forge/auth-sdk'
import { createHostSdk, type HostApiRequest } from '@forge/host-sdk'
import { loadSignedManifest } from '@forge/manifest-security/loader'
import {
  GovernedMicroApp,
  authorizeManifestReleaseSet,
  createReleaseSetCircuitBreaker,
  isMicroAppReleaseEnabled,
  selectMicroAppRollback,
  selectMicroAppRolloutSubject,
  type GovernedMicroAppStatus,
} from '@forge/microapp-runtime'

import { isProductionEnvironment, runtimeConfig } from '../app/config/runtime'
import { useThemeMode } from '../app/providers/ThemeModeProvider'
import { shellEventHub } from '../microapps/event-hub'
import { manifestKeyStore } from '../microapps/trust-store'

function enabledFeatureFlags(): ReadonlySet<string> {
  if (!Array.isArray(runtimeConfig.microAppFeatureFlags)) return new Set()
  return new Set(runtimeConfig.microAppFeatureFlags.filter((flag) =>
    typeof flag === 'string' && /^[a-z][a-z0-9_.-]{2,127}$/.test(flag),
  ))
}

async function hostTransport(request: HostApiRequest): Promise<unknown> {
  if ((request.method === undefined || request.method === 'GET') && request.body !== undefined) {
    throw new Error('GET requests cannot carry a body')
  }
  return apiFetch<unknown>(request.path, {
    method: request.method ?? 'GET',
    ...(request.body === undefined
      ? {}
      : {
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(request.body),
        }),
    ...(request.signal === undefined ? {} : { signal: request.signal }),
  })
}

export function MicroAppPage() {
  const meQuery = useMe()
  const navigate = useNavigate()
  const theme = useThemeMode()
  const [selectedReleaseId, setSelectedReleaseId] = useState<string>()
  const [runtimeStatus, setRuntimeStatus] = useState<GovernedMicroAppStatus>({ state: 'checking' })

  const releaseQuery = useQuery({
    queryKey: ['micro-app-manifest', runtimeConfig.microAppManifestUrl],
    enabled: runtimeConfig.microFrontendsEnabled === true,
    retry: false,
    refetchOnWindowFocus: false,
    queryFn: async () => {
      const verified = await loadSignedManifest<MicroAppManifest>(runtimeConfig.microAppManifestUrl, {
        shellOrigin: window.location.origin,
        keyStore: manifestKeyStore,
      })
      return authorizeManifestReleaseSet({
        verifiedManifest: verified,
        validation: {
          shellOrigin: window.location.origin,
          production: isProductionEnvironment(),
        },
      })
    },
  })

  const releaseSet = releaseQuery.data
  useEffect(() => {
    setSelectedReleaseId(releaseSet?.primary.releaseId)
  }, [releaseSet])

  const activePlan = releaseSet
    ? selectedReleaseId === releaseSet.rollback?.releaseId
      ? releaseSet.rollback
      : releaseSet.primary
    : undefined
  const breaker = useMemo(
    () => (releaseSet ? createReleaseSetCircuitBreaker(releaseSet) : undefined),
    [releaseSet],
  )
  const featureFlags = useMemo(enabledFeatureFlags, [])
  const me = meQuery.data
  const dataScopes = useMemo(() => me?.scopes ?? [], [me?.scopes])
  const dataScopesAllowed = releaseSet
    ? releaseSet.metadata.requiredDataScopes.every((scope) => dataScopes.includes(scope))
    : false

  const hostSdk = useMemo(() => {
    if (!releaseSet || !activePlan || !me) return undefined
    return createHostSdk({
      appId: activePlan.id,
      appVersion: activePlan.version,
      origin: window.location.origin,
      apiNamespaces: releaseSet.metadata.allowedApiPrefixes,
      routePrefixes: [activePlan.routePrefix],
      publishTopics: releaseSet.metadata.events.publish,
      subscribeTopics: releaseSet.metadata.events.subscribe,
      getSession: () => ({
        userId: me.user_id,
        displayName: me.display_name || me.login_name,
        organizationId: me.organization_id,
        permissions: me.permissions ?? [],
        dataScopes,
      }),
      getContext: () => ({
        locale: 'zh-CN',
        theme: { mode: theme.mode, primaryColor: runtimeConfig.primaryColor },
      }),
      transport: hostTransport,
      navigate: (target, options) => navigate(target, { replace: options.replace }),
      eventHub: shellEventHub,
    })
  }, [activePlan, dataScopes, me, navigate, releaseSet, theme.mode])

  useEffect(() => {
    shellEventHub.publish('shell.theme-changed', {
      mode: theme.mode,
      primaryColor: runtimeConfig.primaryColor,
    })
  }, [theme.mode])

  const handleStatus = useCallback((status: GovernedMicroAppStatus) => {
    setRuntimeStatus(status)
    if (
      status.state === 'error' &&
      releaseSet &&
      activePlan?.releaseId === releaseSet.primary.releaseId
    ) {
      const rollback = selectMicroAppRollback(releaseSet, activePlan.releaseId)
      if (rollback) setSelectedReleaseId(rollback.releaseId)
    }
  }, [activePlan, releaseSet])

  if (!runtimeConfig.microFrontendsEnabled) {
    return <Result status="404" title="微前端功能未启用" subTitle="请由平台管理员通过受控运行配置启用。" />
  }
  if (meQuery.isLoading || releaseQuery.isLoading) {
    return <div style={{ padding: 48, textAlign: 'center' }}><Spin size="large" /></div>
  }
  if (releaseQuery.isError || !releaseSet) {
    return (
      <Result
        status="error"
        title="微应用清单验证失败"
        subTitle="清单加载、信任密钥或签名校验未通过，系统已拒绝启动。"
      />
    )
  }
  if (!me || !activePlan || !breaker || !hostSdk) {
    return <Result status="403" title="无法建立受控宿主上下文" />
  }
  if (!dataScopesAllowed) {
    return <Result status="403" title="数据范围不足" subTitle="当前会话不满足微应用声明的数据范围。" />
  }

  const enabled = isMicroAppReleaseEnabled(releaseSet, featureFlags)
  const subjectId = selectMicroAppRolloutSubject(releaseSet, {
    userId: me.user_id,
    organizationId: me.organization_id,
  })
  const usingRollback = activePlan.releaseId === releaseSet.rollback?.releaseId

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Space align="center" wrap>
        <Typography.Title level={3} style={{ margin: 0 }}>
          {releaseSet.metadata.displayName}
        </Typography.Title>
        <Tag color={usingRollback ? 'orange' : 'green'}>
          {usingRollback ? '已回滚' : '主发布'} {activePlan.version}
        </Tag>
        <Tag>{activePlan.runtime === 'wujie' ? '可信无界运行时' : '独立域 iframe'}</Tag>
      </Space>
      {usingRollback ? (
        <Alert
          showIcon
          type="warning"
          message="主发布启动失败，已切换至签名清单指定的回滚版本"
        />
      ) : null}
      <GovernedMicroApp
        className="forge-microapp-container"
        circuitBreaker={breaker}
        enabled={enabled}
        hostSdk={hostSdk}
        permissions={me.permissions ?? []}
        plan={activePlan}
        subjectId={subjectId}
        loading={<div style={{ padding: 48, textAlign: 'center' }}><Spin size="large" /></div>}
        onStatusChange={handleStatus}
        renderFallback={(status) => (
          <Result
            status="warning"
            title="微应用当前不可用"
            subTitle={`运行状态：${status.state}${runtimeStatus.state === 'error' ? '，已执行降级策略' : ''}`}
          />
        )}
      />
    </Space>
  )
}
