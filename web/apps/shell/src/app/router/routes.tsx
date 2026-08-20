import {
  ApartmentOutlined,
  AppstoreOutlined,
  AuditOutlined,
  CheckSquareOutlined,
  DashboardOutlined,
  FileProtectOutlined,
  KeyOutlined,
  IdcardOutlined,
  LockOutlined,
  MenuOutlined,
  PartitionOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TeamOutlined,
  UserSwitchOutlined,
  UsergroupAddOutlined,
  CodeOutlined,
} from '@ant-design/icons'
import type { ComponentType, LazyExoticComponent, ReactNode } from 'react'
import { lazy } from 'react'
import { matchPath } from 'react-router-dom'
import type { Menu, Principal } from '@forge/api-client'
import { can } from '@forge/auth-sdk'
import { platformAdminModules, type PlatformAdminModuleKey } from '@forge/platform-admin/modules'
import { runtimeConfig } from '../config/runtime'

const DashboardPage = lazy(() => import('../../pages/Dashboard').then((m) => ({ default: m.DashboardPage })))
const UsersPage = lazy(() => import('@forge/platform-admin/users').then((m) => ({ default: m.UsersPage })))
const RolesPage = lazy(() => import('@forge/platform-admin/roles').then((m) => ({ default: m.RolesPage })))
const PermissionsPage = lazy(() => import('@forge/platform-admin/permissions').then((m) => ({ default: m.PermissionsPage })))
const MenusPage = lazy(() => import('@forge/platform-admin/menus').then((m) => ({ default: m.MenusPage })))
const OrganizationPage = lazy(() => import('@forge/platform-admin/organization').then((m) => ({ default: m.OrganizationPage })))
const DepartmentsPage = lazy(() => import('@forge/platform-admin/departments').then((m) => ({ default: m.DepartmentsPage })))
const PositionsPage = lazy(() => import('@forge/platform-admin/positions').then((m) => ({ default: m.PositionsPage })))
const UserGroupsPage = lazy(() => import('@forge/platform-admin/user-groups').then((m) => ({ default: m.UserGroupsPage })))
const SessionsPage = lazy(() => import('@forge/platform-admin/sessions').then((m) => ({ default: m.SessionsPage })))
const AuditLogsPage = lazy(() => import('@forge/platform-admin/audit-logs').then((m) => ({ default: m.AuditLogsPage })))
const ApprovalsPage = lazy(() => import('@forge/platform-admin/approvals').then((m) => ({ default: m.ApprovalsPage })))
const TemporaryGrantsPage = lazy(() => import('@forge/platform-admin/temporary-grants').then((m) => ({ default: m.TemporaryGrantsPage })))
const EmergencyAccessPage = lazy(() => import('@forge/platform-admin/temporary-grants').then((m) => ({ default: m.EmergencyAccessPage })))
const AccessReviewsPage = lazy(() => import('@forge/platform-admin/access-reviews').then((m) => ({ default: m.AccessReviewsPage })))
const DataGovernancePage = lazy(() => import('@forge/platform-admin/data-governance').then((m) => ({ default: m.DataGovernancePage })))
const ConfigChangesPage = lazy(() => import('@forge/platform-admin/data-governance').then((m) => ({ default: m.ConfigChangesPage })))
const SecurityPage = lazy(() => import('@forge/platform-admin/security').then((m) => ({ default: m.SecurityPage })))
const SystemStatusPage = lazy(() => import('@forge/platform-admin/system-status').then((m) => ({ default: m.SystemStatusPage })))
const AccountSecurityPage = lazy(() => import('../../pages/AccountSecurity').then((m) => ({ default: m.AccountSecurityPage })))
const ApiTokensPage = lazy(() => import('../../pages/ApiTokens').then((m) => ({ default: m.ApiTokensPage })))
const ComponentShowcasePage = lazy(() => import('../../pages/ComponentShowcase').then((m) => ({ default: m.ComponentShowcasePage })))
const MicroAppPage = lazy(() => import('../../pages/MicroApp').then((m) => ({ default: m.MicroAppPage })))

export interface AppRoute {
  path: string
  name: string
  icon?: ReactNode
  permission?: string
  group?: string
  menu?: boolean
  component: LazyExoticComponent<ComponentType>
}

const platformAdminComponents: Record<PlatformAdminModuleKey, LazyExoticComponent<ComponentType>> = {
  users: UsersPage,
  roles: RolesPage,
  permissions: PermissionsPage,
  menus: MenusPage,
  organization: OrganizationPage,
  departments: DepartmentsPage,
  positions: PositionsPage,
  'user-groups': UserGroupsPage,
  security: SecurityPage,
  sessions: SessionsPage,
  'audit-logs': AuditLogsPage,
  approvals: ApprovalsPage,
  'temporary-grants': TemporaryGrantsPage,
  'emergency-access': EmergencyAccessPage,
  'access-reviews': AccessReviewsPage,
  'data-governance': DataGovernancePage,
  'config-changes': ConfigChangesPage,
  'system-status': SystemStatusPage,
}

const platformAdminIcons: Record<PlatformAdminModuleKey, ReactNode> = {
  users: <TeamOutlined />,
  roles: <UserSwitchOutlined />,
  permissions: <FileProtectOutlined />,
  menus: <MenuOutlined />,
  organization: <ApartmentOutlined />,
  departments: <PartitionOutlined />,
  positions: <IdcardOutlined />,
  'user-groups': <UsergroupAddOutlined />,
  security: <SafetyCertificateOutlined />,
  sessions: <LockOutlined />,
  'audit-logs': <AuditOutlined />,
  approvals: <CheckSquareOutlined />,
  'temporary-grants': <SafetyCertificateOutlined />,
  'emergency-access': <SafetyCertificateOutlined />,
  'access-reviews': <SafetyCertificateOutlined />,
  'data-governance': <SafetyCertificateOutlined />,
  'config-changes': <SafetyCertificateOutlined />,
  'system-status': <SettingOutlined />,
}

const platformRoutes: AppRoute[] = platformAdminModules.map((module) => ({
  ...module,
  icon: platformAdminIcons[module.key],
  menu: true,
  component: platformAdminComponents[module.key],
}))

export const appRoutes: AppRoute[] = [
  { path: '/dashboard', name: '工作台', icon: <DashboardOutlined />, menu: true, component: DashboardPage },

  ...platformRoutes,

  { path: '/account/security', name: '账号安全', icon: <LockOutlined />, group: '个人中心', menu: true, component: AccountSecurityPage },
  { path: '/account/api-tokens', name: 'API Token', icon: <KeyOutlined />, group: '个人中心', menu: true, component: ApiTokensPage },

  ...(runtimeConfig.microFrontendsEnabled ? [{ path: '/apps/example-remote', name: '示例微应用', icon: <AppstoreOutlined />, permission: 'example.remote.read', group: '业务应用', menu: true, component: MicroAppPage } satisfies AppRoute] : []),

  ...(runtimeConfig.componentPlayground ? [{ path: '/dev/components', name: '组件示例', icon: <CodeOutlined />, group: '开发工具', menu: true, component: ComponentShowcasePage } satisfies AppRoute] : []),
]

export function routeAllowed(route: AppRoute, me?: Principal) {
  return !route.permission || can(me, route.permission)
}

export function routeByPath(path: string) {
  return appRoutes.find((route) => Boolean(matchPath({ path: route.path, end: true }, path)))
}


export function buildMenuRoutes(me?: Principal, catalog?: Menu[]) {
  const visible = appRoutes.filter((route) => route.menu && routeAllowed(route, me))
  const useCatalog = catalog !== undefined
  const catalogByRoute = new Map((catalog ?? []).filter((item) => item.status === 'ACTIVE' && item.route).map((item) => [item.route, item]))
  const catalogNames = new Map((catalog ?? []).map((item) => [item.key, item.name]))
  const result: Array<Record<string, any>> = []
  const groupIndex = new Map<string, { path: string; name: string; routes: Array<Record<string, any>> }>()

  for (const route of visible) {
    const catalogItem = catalogByRoute.get(route.path)
    if (useCatalog && route.path.startsWith('/admin/') && !catalogItem) continue
    const item = { path: route.path, name: catalogItem?.name ?? route.name, icon: route.icon }
    const groupName = catalogItem?.parent_key ? catalogNames.get(catalogItem.parent_key) : route.group
    if (!groupName) {
      result.push(item)
      continue
    }
    let group = groupIndex.get(groupName)
    if (!group) {
      group = { path: `/group/${groupIndex.size}`, name: groupName, routes: [] }
      groupIndex.set(groupName, group)
      result.push(group)
    }
    group.routes.push(item)
  }
  return result
}
