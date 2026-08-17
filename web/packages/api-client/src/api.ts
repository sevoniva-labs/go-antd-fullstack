import { apiDownload, apiFetch, type DownloadResult } from './client'
import type {
  ApiToken,
  AuditEvent,
  Department,
  Position,
  UserGroup,
  Organization,
  Permission,
  Principal,
  Readiness,
  SecurityPolicy,
  Role,
  SessionInfo,
  SystemInfo,
  User,
} from './types'

export const api = {
  login: (payload: { organization?: string; login_name: string; password: string }) =>
    apiFetch<Principal>('/auth/login', { method: 'POST', body: JSON.stringify(payload) }),
  logout: () => apiFetch<void>('/auth/logout', { method: 'POST' }),
  changePassword: (payload: { current_password: string; new_password: string }) =>
    apiFetch<void>('/auth/password', { method: 'PATCH', body: JSON.stringify(payload) }),
  me: () => apiFetch<Principal>('/me'),

  apiTokens: () => apiFetch<{ items: ApiToken[] }>('/api-tokens'),
  createApiToken: (payload: { name: string; scopes?: string[]; expires_days?: number }) =>
    apiFetch<{ token: ApiToken; secret: string; warning: string }>('/api-tokens', { method: 'POST', body: JSON.stringify(payload) }),
  revokeApiToken: (id: string) => apiFetch<void>(`/api-tokens/${id}`, { method: 'DELETE' }),

  systemInfo: () => apiFetch<SystemInfo>('/system/info'),
  readiness: () => apiFetch<Readiness>('/system/ready'),

  organization: () => apiFetch<Organization>('/admin/organization'),
  updateOrganization: (payload: {
    name: string
    description: string
    status: 'ACTIVE' | 'DISABLED'
    max_users: number
    max_active_sessions: number
  }) => apiFetch<Organization>('/admin/organization', { method: 'PATCH', body: JSON.stringify(payload) }),
  departments: () => apiFetch<{ items: Department[] }>('/admin/departments'),
  createDepartment: (payload: {
    department_key: string
    name: string
    parent_id?: string
    status: 'ACTIVE' | 'DISABLED'
    sort_order: number
  }) => apiFetch<Department>('/admin/departments', { method: 'POST', body: JSON.stringify(payload) }),
  updateDepartment: (departmentId: string, payload: {
    name: string
    parent_id?: string
    status: 'ACTIVE' | 'DISABLED'
    sort_order: number
  }) => apiFetch<Department>(`/admin/departments/${encodeURIComponent(departmentId)}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  positions: () => apiFetch<{ items: Position[] }>('/admin/positions'),
  createPosition: (payload: {
    position_key: string
    name: string
    description: string
    department_id: string
    status: 'ACTIVE' | 'DISABLED'
    sort_order: number
  }) => apiFetch<Position>('/admin/positions', { method: 'POST', body: JSON.stringify(payload) }),
  updatePosition: (positionId: string, payload: {
    name: string
    description: string
    department_id: string
    status: 'ACTIVE' | 'DISABLED'
    sort_order: number
  }) => apiFetch<Position>(`/admin/positions/${encodeURIComponent(positionId)}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  userGroups: () => apiFetch<{ items: UserGroup[] }>('/admin/user-groups'),
  createUserGroup: (payload: { group_key: string; name: string; description: string; status: 'ACTIVE' | 'DISABLED' }) =>
    apiFetch<UserGroup>('/admin/user-groups', { method: 'POST', body: JSON.stringify(payload) }),
  updateUserGroup: (groupId: string, payload: { name: string; description: string; status: 'ACTIVE' | 'DISABLED' }) =>
    apiFetch<UserGroup>(`/admin/user-groups/${encodeURIComponent(groupId)}`, { method: 'PATCH', body: JSON.stringify(payload) }),
  updateUserGroupMembers: (groupId: string, userIds: string[]) =>
    apiFetch<void>(`/admin/user-groups/${encodeURIComponent(groupId)}/members`, { method: 'PUT', body: JSON.stringify({ user_ids: userIds }) }),
  updateUserGroupRoles: (groupId: string, roles: string[]) =>
    apiFetch<void>(`/admin/user-groups/${encodeURIComponent(groupId)}/roles`, { method: 'PUT', body: JSON.stringify({ roles }) }),
  securityConfig: () => apiFetch<SecurityPolicy>('/admin/security-config'),
  updateSecurityConfig: (payload: SecurityPolicy) =>
    apiFetch<SecurityPolicy>('/admin/security-config', { method: 'PUT', body: JSON.stringify(payload) }),
  users: () => apiFetch<{ items: User[] }>('/admin/users'),
  createUser: (payload: { login_name: string; display_name: string; password: string; roles: string[] }) =>
    apiFetch<User>('/admin/users', { method: 'POST', body: JSON.stringify(payload) }),
  updateUserRoles: (userId: string, roles: string[]) =>
    apiFetch<void>(`/admin/users/${userId}/roles`, { method: 'PATCH', body: JSON.stringify({ roles }) }),
  updateUserStatus: (userId: string, status: 'ACTIVE' | 'DISABLED') =>
    apiFetch<void>(`/admin/users/${userId}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }),
  unlockUser: (userId: string) =>
    apiFetch<void>(`/admin/users/${userId}/unlock`, { method: 'POST' }),
  resetUserPassword: (userId: string, password: string) =>
    apiFetch<void>(`/admin/users/${userId}/reset-password`, { method: 'POST', body: JSON.stringify({ password }) }),

  roles: () => apiFetch<{ items: Role[] }>('/admin/roles'),
  permissions: () => apiFetch<{ items: Permission[] }>('/admin/permissions'),
  updateRolePermissions: (roleKey: string, permissions: string[]) =>
    apiFetch<void>(`/admin/roles/${encodeURIComponent(roleKey)}/permissions`, { method: 'PUT', body: JSON.stringify({ permissions }) }),

  sessions: () => apiFetch<{ items: SessionInfo[] }>('/admin/sessions'),
  revokeSession: (sessionId: string) => apiFetch<void>(`/admin/sessions/${sessionId}`, { method: 'DELETE' }),

  auditLogs: () => apiFetch<{ items: AuditEvent[] }>('/admin/audit-logs'),
  exportAuditLogs: (params?: { format?: 'json' | 'csv'; limit?: number }): Promise<DownloadResult> => {
    const q = new URLSearchParams()
    const format = params?.format ?? 'json'
    q.set('format', format)
    if (params?.limit !== undefined) q.set('limit', String(params.limit))
    const query = q.toString()
    return apiDownload(`/admin/audit-logs/export${query ? `?${query}` : ''}`)
  },
}
