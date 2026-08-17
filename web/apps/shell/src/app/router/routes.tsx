import {
  ApartmentOutlined,
  AuditOutlined,
  DashboardOutlined,
  FileProtectOutlined,
  KeyOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TeamOutlined,
  UserSwitchOutlined,
  CodeOutlined,
} from '@ant-design/icons'
import type { ComponentType, LazyExoticComponent, ReactNode } from 'react'
import { lazy } from 'react'
import { matchPath } from 'react-router-dom'
import type { Principal } from '@forge/api-client'
import { can } from '@forge/auth-sdk'
import { runtimeConfig } from '../config/runtime'

const DashboardPage = lazy(() => import('../../pages/Dashboard').then((m) => ({ default: m.DashboardPage })))
const UsersPage = lazy(() => import('../../pages/Users').then((m) => ({ default: m.UsersPage })))
const RolesPage = lazy(() => import('../../pages/Roles').then((m) => ({ default: m.RolesPage })))
const PermissionsPage = lazy(() => import('../../pages/Permissions').then((m) => ({ default: m.PermissionsPage })))
const OrganizationPage = lazy(() => import('../../pages/Organization').then((m) => ({ default: m.OrganizationPage })))
const SessionsPage = lazy(() => import('../../pages/Sessions').then((m) => ({ default: m.SessionsPage })))
const AuditLogsPage = lazy(() => import('../../pages/AuditLogs').then((m) => ({ default: m.AuditLogsPage })))
const SecurityPage = lazy(() => import('../../pages/Security').then((m) => ({ default: m.SecurityPage })))
const SystemStatusPage = lazy(() => import('../../pages/SystemStatus').then((m) => ({ default: m.SystemStatusPage })))
const AccountSecurityPage = lazy(() => import('../../pages/AccountSecurity').then((m) => ({ default: m.AccountSecurityPage })))
const ApiTokensPage = lazy(() => import('../../pages/ApiTokens').then((m) => ({ default: m.ApiTokensPage })))
const ComponentShowcasePage = lazy(() => import('../../pages/ComponentShowcase').then((m) => ({ default: m.ComponentShowcasePage })))

export interface AppRoute {
  path: string
  name: string
  icon?: ReactNode
  permission?: string
  group?: string
  menu?: boolean
  component: LazyExoticComponent<ComponentType>
}

export const appRoutes: AppRoute[] = [
  { path: '/dashboard', name: '工作台', icon: <DashboardOutlined />, menu: true, component: DashboardPage },

  { path: '/admin/users', name: '用户管理', icon: <TeamOutlined />, permission: 'system.user.read', group: '系统管理', menu: true, component: UsersPage },
  { path: '/admin/roles', name: '角色管理', icon: <UserSwitchOutlined />, permission: 'system.role.read', group: '系统管理', menu: true, component: RolesPage },
  { path: '/admin/permissions', name: '权限清单', icon: <FileProtectOutlined />, permission: 'system.role.read', group: '系统管理', menu: true, component: PermissionsPage },
  { path: '/admin/organization', name: '组织信息', icon: <ApartmentOutlined />, permission: 'system.organization.read', group: '系统管理', menu: true, component: OrganizationPage },

  { path: '/security', name: '安全基线', icon: <SafetyCertificateOutlined />, group: '安全中心', menu: true, component: SecurityPage },
  { path: '/admin/sessions', name: '在线会话', icon: <LockOutlined />, permission: 'system.session.read', group: '安全中心', menu: true, component: SessionsPage },
  { path: '/admin/audit-logs', name: '审计日志', icon: <AuditOutlined />, permission: 'system.audit.read', group: '安全中心', menu: true, component: AuditLogsPage },

  { path: '/ops/system', name: '系统状态', icon: <SettingOutlined />, group: '运维中心', menu: true, component: SystemStatusPage },

  { path: '/account/security', name: '账号安全', icon: <LockOutlined />, group: '个人中心', menu: true, component: AccountSecurityPage },
  { path: '/account/api-tokens', name: 'API Token', icon: <KeyOutlined />, group: '个人中心', menu: true, component: ApiTokensPage },

  ...(runtimeConfig.componentPlayground ? [{ path: '/dev/components', name: '组件示例', icon: <CodeOutlined />, group: '开发工具', menu: true, component: ComponentShowcasePage } satisfies AppRoute] : []),
]

export function routeAllowed(route: AppRoute, me?: Principal) {
  return !route.permission || can(me, route.permission)
}

export function routeByPath(path: string) {
  return appRoutes.find((route) => Boolean(matchPath({ path: route.path, end: true }, path)))
}


export function buildMenuRoutes(me?: Principal) {
  const visible = appRoutes.filter((route) => route.menu && routeAllowed(route, me))
  const result: Array<Record<string, any>> = []
  const groupIndex = new Map<string, { path: string; name: string; routes: Array<Record<string, any>> }>()

  for (const route of visible) {
    const item = { path: route.path, name: route.name, icon: route.icon }
    if (!route.group) {
      result.push(item)
      continue
    }
    let group = groupIndex.get(route.group)
    if (!group) {
      group = { path: `/group/${groupIndex.size}`, name: route.group, routes: [] }
      groupIndex.set(route.group, group)
      result.push(group)
    }
    group.routes.push(item)
  }
  return result
}
