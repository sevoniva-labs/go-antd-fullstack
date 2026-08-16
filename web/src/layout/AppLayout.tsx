import {
  CompressOutlined,
  ExpandOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  LogoutOutlined,
  MoonOutlined,
  QuestionCircleOutlined,
  SunOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { ProLayout } from '@ant-design/pro-components'
import { App, Avatar, Button, Dropdown, Space, Tag, Tooltip } from 'antd'
import { Suspense, useEffect, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api } from '../api/api'
import { environmentTone, runtimeConfig } from '../app/config/runtime'
import { buildMenuRoutes, routeByPath } from '../app/router/routes'
import { useThemeMode } from '../app/providers/ThemeModeProvider'
import { useMe } from '../auth/useMe'
import { BrandMark } from './BrandMark'
import { GlobalSearch } from './GlobalSearch'
import { PageTabs } from './PageTabs'
import { PageLoading } from '../components/feedback/PageLoading'
import { HeaderActionsSlot } from '../app/extensions/HeaderActionsSlot'

export function AppLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const { mode, compact, toggleMode, setCompact } = useThemeMode()
  const [fullscreen, setFullscreen] = useState(Boolean(document.fullscreenElement))
  const routes = buildMenuRoutes(me)

  useEffect(() => {
    const sync = () => setFullscreen(Boolean(document.fullscreenElement))
    document.addEventListener('fullscreenchange', sync)
    return () => document.removeEventListener('fullscreenchange', sync)
  }, [])

  useEffect(() => {
    const route = routeByPath(location.pathname)
    document.title = route ? `${route.name} · ${runtimeConfig.appName}` : runtimeConfig.appName
  }, [location.pathname])

  async function toggleFullscreen() {
    if (!document.fullscreenElement) {
      await document.documentElement.requestFullscreen()
      setFullscreen(true)
    } else {
      await document.exitFullscreen()
      setFullscreen(false)
    }
  }

  return (
    <ProLayout
      className="forge-layout"
      title={runtimeConfig.appName}
      logo={<BrandMark />}
      layout={runtimeConfig.layoutMode}
      navTheme="light"
      fixedHeader
      fixSiderbar
      siderWidth={224}
      contentWidth="Fluid"
      route={{ routes }}
      location={{ pathname: location.pathname }}
      menu={{ type: 'group', autoClose: false }}
      menuItemRender={(item, dom) => (
        <div onClick={() => item.path && !item.path.startsWith('/group/') && navigate(item.path)}>{dom}</div>
      )}
      actionsRender={() => [
        runtimeConfig.showEnvironmentBadge ? <Tag key="env" color={environmentTone()} className="environment-badge">{runtimeConfig.environment}</Tag> : null,
        <GlobalSearch key="search" />,
        <HeaderActionsSlot key="product-actions" />,
        runtimeConfig.helpUrl ? (
          <Tooltip key="help" title="帮助文档">
            <Button type="text" icon={<QuestionCircleOutlined />} onClick={() => window.open(runtimeConfig.helpUrl, '_blank', 'noopener,noreferrer')} />
          </Tooltip>
        ) : null,
        <Tooltip key="density" title={compact ? '标准密度' : '紧凑密度'}>
          <Button type="text" icon={compact ? <ExpandOutlined /> : <CompressOutlined />} onClick={() => setCompact(!compact)} />
        </Tooltip>,
        <Tooltip key="fullscreen" title={fullscreen ? '退出全屏' : '全屏'}>
          <Button type="text" icon={fullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />} onClick={toggleFullscreen} />
        </Tooltip>,
        <Tooltip key="theme" title={mode === 'light' ? '深色模式' : '浅色模式'}>
          <Button type="text" icon={mode === 'light' ? <MoonOutlined /> : <SunOutlined />} onClick={toggleMode} />
        </Tooltip>,
      ].filter(Boolean)}
      footerRender={() => runtimeConfig.footerText ? <div className="app-footer">{runtimeConfig.footerText}</div> : null}
      avatarProps={{
        src: undefined,
        render: () => (
          <Dropdown
            menu={{
              items: [
                { key: 'account', icon: <UserOutlined />, label: '账号安全', onClick: () => navigate('/account/security') },
                { key: 'logout', type: 'divider' },
                {
                  key: 'logout-action',
                  icon: <LogoutOutlined />,
                  label: '退出登录',
                  onClick: async () => {
                    try {
                      await api.logout()
                    } finally {
                      queryClient.clear()
                      message.success('已退出登录')
                      navigate('/login', { replace: true })
                    }
                  },
                },
              ],
            }}
          >
            <Space className="user-trigger">
              <Avatar>{(me?.display_name || me?.login_name || 'U').slice(0, 1).toUpperCase()}</Avatar>
              <span className="user-name">{me?.display_name || me?.login_name}</span>
            </Space>
          </Dropdown>
        ),
      }}
    >
      <PageTabs />
      <Suspense fallback={<PageLoading />}>
        <Outlet />
      </Suspense>
    </ProLayout>
  )
}
