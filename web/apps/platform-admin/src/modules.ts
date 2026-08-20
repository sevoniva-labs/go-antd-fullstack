export type PlatformAdminModuleKey =
  | 'users'
  | 'roles'
  | 'permissions'
  | 'menus'
  | 'organization'
  | 'departments'
  | 'positions'
  | 'user-groups'
  | 'security'
  | 'sessions'
  | 'audit-logs'
  | 'system-status'
  | 'approvals'
  | 'temporary-grants'
  | 'emergency-access'
  | 'access-reviews'
  | 'data-governance'
  | 'config-changes'

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
  { key: 'menus', path: '/admin/menus', name: '菜单目录', group: '系统管理', permission: 'system.menu.read' },
  { key: 'organization', path: '/admin/organization', name: '组织信息', group: '系统管理', permission: 'system.organization.read' },
  { key: 'departments', path: '/admin/departments', name: '部门管理', group: '系统管理', permission: 'system.department.read' },
  { key: 'positions', path: '/admin/positions', name: '岗位管理', group: '系统管理', permission: 'system.position.read' },
  { key: 'user-groups', path: '/admin/user-groups', name: '用户组管理', group: '系统管理', permission: 'system.user_group.read' },
  { key: 'security', path: '/security', name: '安全基线', group: '安全中心', permission: 'system.config.read' },
  { key: 'sessions', path: '/admin/sessions', name: '在线会话', group: '安全中心', permission: 'system.session.read' },
  { key: 'audit-logs', path: '/admin/audit-logs', name: '审计日志', group: '安全中心', permission: 'system.audit.read' },
  { key: 'approvals', path: '/governance/approvals', name: '审批中心', group: '治理中心', permission: 'approval.request.read' },
  { key: 'temporary-grants', path: '/governance/temporary-grants', name: '临时授权', group: '治理中心', permission: 'system.temporary_grant.read' },
  { key: 'emergency-access', path: '/security/emergency-access', name: '应急授权', group: '安全中心', permission: 'system.emergency_access.read' },
  { key: 'access-reviews', path: '/governance/access-reviews', name: '访问复核', group: '治理中心', permission: 'system.access_review.read' },
  { key: 'data-governance', path: '/admin/data-governance', name: '数据治理', group: '治理中心', permission: 'system.data_policy.read' },
  { key: 'config-changes', path: '/admin/config-changes', name: '配置变更', group: '治理中心', permission: 'system.config.read' },
  { key: 'system-status', path: '/ops/system', name: '系统状态', group: '运维中心', permission: 'system.status.read' },
]
