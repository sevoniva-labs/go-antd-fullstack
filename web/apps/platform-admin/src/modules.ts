export type PlatformAdminModuleKey =
  | 'users'
  | 'roles'
  | 'permissions'
  | 'organization'
  | 'departments'
  | 'security'
  | 'sessions'
  | 'audit-logs'
  | 'system-status'

export interface PlatformAdminModule {
  key: PlatformAdminModuleKey
  path: string
  name: string
  group: string
  permission?: string
}

export const platformAdminModules: readonly PlatformAdminModule[] = [
  { key: 'users', path: '/admin/users', name: '用户管理', group: '系统管理', permission: 'system.user.read' },
  { key: 'roles', path: '/admin/roles', name: '角色管理', group: '系统管理', permission: 'system.role.read' },
  { key: 'permissions', path: '/admin/permissions', name: '权限清单', group: '系统管理', permission: 'system.role.read' },
  { key: 'organization', path: '/admin/organization', name: '组织信息', group: '系统管理', permission: 'system.organization.read' },
  { key: 'departments', path: '/admin/departments', name: '部门管理', group: '系统管理', permission: 'system.department.read' },
  { key: 'security', path: '/security', name: '安全基线', group: '安全中心', permission: 'system.config.read' },
  { key: 'sessions', path: '/admin/sessions', name: '在线会话', group: '安全中心', permission: 'system.session.read' },
  { key: 'audit-logs', path: '/admin/audit-logs', name: '审计日志', group: '安全中心', permission: 'system.audit.read' },
  { key: 'system-status', path: '/ops/system', name: '系统状态', group: '运维中心' },
]
