import { apiFetch } from './client'
import type {
  ApiToken,
  AuditEvent,
  Organization,
  Permission,
  Principal,
  Readiness,
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
}
