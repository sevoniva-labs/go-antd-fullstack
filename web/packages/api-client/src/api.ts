import { apiDownload, apiFetch, type DownloadResult } from './client'
import type {
  ApiToken,
  AuditEvent,
  ApprovalRequest,
  Department,
  Position,
  UserGroup,
  UserAssignment,
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
  login: (payload: { organization?: string; login_name: string; password: string; mfa_code?: string; recovery_code?: string }) =>
    apiFetch<Principal>('/auth/login', { method: 'POST', body: JSON.stringify(payload) }),
  logout: () => apiFetch<void>('/auth/logout', { method: 'POST' }),
  changePassword: (payload: { current_password: string; new_password: string }) =>
    apiFetch<void>('/auth/password', { method: 'PATCH', body: JSON.stringify(payload) }),
  stepUpAuthentication: (payload: { current_password: string; mfa_code?: string; recovery_code?: string }) =>
    apiFetch<{ verified_at: string }>('/auth/step-up', { method: 'POST', body: JSON.stringify(payload) }),
  me: () => apiFetch<Principal>('/me'),
  mfaStatus: () => apiFetch<{ enabled: boolean }>('/mfa'),
  beginMfaEnrollment: (currentPassword: string) =>
    apiFetch<{ secret: string; provisioning_uri: string }>('/mfa/totp/enrollment', { method: 'POST', body: JSON.stringify({ current_password: currentPassword }) }),
  confirmMfaEnrollment: (code: string) =>
    apiFetch<{ recovery_codes: string[] }>('/mfa/totp/enrollment/confirmation', { method: 'POST', body: JSON.stringify({ code }) }),
  disableMfa: (payload: { current_password: string; code?: string; recovery_code?: string }) =>
    apiFetch<void>('/mfa/totp/disable', { method: 'POST', body: JSON.stringify(payload) }),

  apiTokens: () => apiFetch<{ items: ApiToken[] }>('/api-tokens'),
  createApiToken: (payload: { name: string; scopes?: string[]; expires_days?: number }) =>
    apiFetch<{ token: ApiToken; secret: string; warning: string }>('/api-tokens', { method: 'POST', body: JSON.stringify(payload) }),
  revokeApiToken: (id: string) => apiFetch<void>(`/api-tokens/${id}`, { method: 'DELETE' }),

  systemInfo: () => apiFetch<SystemInfo>('/system/info'),
  readiness: () => apiFetch<Readiness>('/system/ready'),

  approvals: () => apiFetch<{ items: ApprovalRequest[]; approvals: ApprovalRequest[] }>('/approvals'),
  createApproval: (payload: {
    request_type: string; action: string; resource: string; resource_id?: string; summary: string; payload_json: string;
    mode: ApprovalRequest['mode']; required_approvals: number; approver_ids: string[]; expires_in_seconds: number;
  }) => apiFetch<ApprovalRequest>('/approvals', { method: 'POST', body: JSON.stringify(payload) }),
  decideApproval: (approvalId: string, decision: 'APPROVE' | 'REJECT', comment: string) =>
    apiFetch<ApprovalRequest>(`/approvals/${encodeURIComponent(approvalId)}/decisions`, { method: 'POST', body: JSON.stringify({ decision, comment }) }),
  transferApproval: (approvalId: string, newAssigneeId: string, comment: string) =>
    apiFetch<ApprovalRequest>(`/approvals/${encodeURIComponent(approvalId)}/transfer`, { method: 'POST', body: JSON.stringify({ new_assignee_id: newAssigneeId, comment }) }),
  withdrawApproval: (approvalId: string, comment: string) =>
    apiFetch<ApprovalRequest>(`/approvals/${encodeURIComponent(approvalId)}/withdraw`, { method: 'POST', body: JSON.stringify({ comment }) }),

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
  updateSecurityConfig: (payload: SecurityPolicy, approvalId: string) =>
    apiFetch<SecurityPolicy>(`/admin/security-config?approval_id=${encodeURIComponent(approvalId)}`, { method: 'PUT', body: JSON.stringify(payload) }),
  users: () => apiFetch<{ items: User[] }>('/admin/users'),
  createUser: (payload: { login_name: string; display_name: string; password: string; roles: string[] }) =>
    apiFetch<User>('/admin/users', { method: 'POST', body: JSON.stringify(payload) }),
  updateUserRoles: (userId: string, roles: string[], approvalId: string) =>
    apiFetch<void>(`/admin/users/${userId}/roles`, { method: 'PATCH', body: JSON.stringify({ roles, approval_id: approvalId }) }),
  updateUserStatus: (userId: string, status: 'ACTIVE' | 'DISABLED') =>
    apiFetch<void>(`/admin/users/${userId}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }),
  unlockUser: (userId: string) =>
    apiFetch<void>(`/admin/users/${userId}/unlock`, { method: 'POST' }),
  resetUserPassword: (userId: string, password: string, approvalId: string) =>
    apiFetch<void>(`/admin/users/${userId}/reset-password`, { method: 'POST', body: JSON.stringify({ password, approval_id: approvalId }) }),
  userAssignments: (userId: string) =>
    apiFetch<{ items: UserAssignment[] }>(`/admin/users/${encodeURIComponent(userId)}/assignments`),
  replaceUserAssignments: (userId: string, assignments: Array<{
    department_id: string
    position_id?: string
    primary: boolean
    valid_from?: string
    valid_until?: string
  }>) => apiFetch<void>(`/admin/users/${encodeURIComponent(userId)}/assignments`, { method: 'PUT', body: JSON.stringify({ assignments }) }),

  roles: () => apiFetch<{ items: Role[] }>('/admin/roles'),
  permissions: () => apiFetch<{ items: Permission[] }>('/admin/permissions'),
  updateRolePermissions: (roleKey: string, permissions: string[], approvalId: string) =>
    apiFetch<void>(`/admin/roles/${encodeURIComponent(roleKey)}/permissions`, { method: 'PUT', body: JSON.stringify({ permissions, approval_id: approvalId }) }),
  updateRoleDataScope: (roleKey: string, dataScope: Role['data_scope'], departmentIds: string[], approvalId: string) =>
    apiFetch<void>(`/admin/roles/${encodeURIComponent(roleKey)}/data-scope`, {
      method: 'PUT', body: JSON.stringify({ data_scope: dataScope, department_ids: departmentIds, approval_id: approvalId }),
    }),

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
