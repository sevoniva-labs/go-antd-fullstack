export interface Principal {
  principal_type: 'USER' | 'TOKEN'
  user_id: string
  organization_id: string
  login_name: string
  display_name: string
  roles: string[]
  permissions?: string[]
  scopes?: string[]
  must_change_password: boolean
  authentication_level?: 'PASSWORD' | 'MFA'
  mfa_verified_at?: string
}

export interface User {
  id: string
  organization_id: string
  login_name: string
  display_name: string
  status: string
  must_change_password: boolean
  locked_until?: string
  created_at: string
  updated_at: string
  roles: string[]
  permissions?: string[]
}

export interface Organization {
  id: string
  org_key: string
  name: string
  status: string
  description: string
  max_users: number
  max_active_sessions: number
  created_at: string
  updated_at: string
}

export interface Department {
  id: string
  organization_id: string
  parent_id?: string
  department_key: string
  name: string
  status: 'ACTIVE' | 'DISABLED'
  sort_order: number
  created_at: string
  updated_at: string
}

export interface Position {
  id: string
  organization_id: string
  department_id: string
  position_key: string
  name: string
  description: string
  status: 'ACTIVE' | 'DISABLED'
  sort_order: number
  created_at: string
  updated_at: string
}

export interface UserGroup {
  id: string
  organization_id: string
  group_key: string
  name: string
  description: string
  status: 'ACTIVE' | 'DISABLED'
  roles: string[]
  member_ids: string[]
  member_count: number
  created_at: string
  updated_at: string
}

export interface UserAssignment {
  id: string
  organization_id: string
  user_id: string
  department_id: string
  position_id?: string
  primary: boolean
  valid_from: string
  valid_until?: string
  created_at: string
}

export interface Permission {
	key: string
	name: string
	description: string
	resource: string
	action: string
}

export type RoleDataScope = 'ORGANIZATION' | 'DEPARTMENT' | 'DEPARTMENT_TREE' | 'SELF' | 'CUSTOM'

export interface Role {
	key: string
	name: string
	description: string
	data_scope: RoleDataScope
	permissions: string[]
	data_scope_department_ids: string[]
}

export interface ApprovalTask {
  id: string
  assignee_id: string
  status: string
  decision: string
  comment: string
  transferred_from: string
  decided_at?: string
}

export interface ApprovalRequest {
  id: string
  organization_id: string
  request_type: string
  action: string
  resource: string
  resource_id: string
  summary: string
  payload_json: string
  request_digest: string
  applicant_id: string
  mode: 'ANY' | 'ALL' | 'QUORUM'
  required_approvals: number
  status: 'PENDING' | 'APPROVED' | 'REJECTED' | 'WITHDRAWN' | 'EXPIRED'
  expires_at: string
  created_at: string
  updated_at: string
  tasks: ApprovalTask[]
}

export interface TemporaryRoleGrant {
  id: string
  organization_id: string
  user_id: string
  role_key: string
  requested_by: string
  approval_id: string
  reason: string
  status: 'SCHEDULED' | 'ACTIVE' | 'EXPIRED' | 'REVOKED'
  valid_from: string
  valid_until: string
  revoked_at?: string
  revoked_by?: string
  revoke_reason?: string
  created_at: string
}

export interface SessionInfo {
  id: string
  user_id: string
  login_name: string
  display_name: string
  expires_at: string
  created_at: string
  last_seen_at: string
  client_ip: string
  user_agent: string
  current: boolean
}

export interface ApiToken {
  id: string
  name: string
  prefix: string
  scopes: string[]
  expires_at?: string
  last_used_at?: string
  created_at: string
}

export interface SystemInfo {
  application: string
  environment: string
  version: string
  providers: Record<string, string>
  compliance_profile?: string
}

export interface ReadinessCheck {
  name: string
  provider: string
  status: 'UP' | 'DOWN'
  error?: string
  duration_ms: number
}

export interface Readiness {
  status: 'UP' | 'DOWN'
  checks: ReadinessCheck[]
}

export interface SecurityPolicy {
  password_min_length: number
  password_require_upper: boolean
  password_require_lower: boolean
  password_require_digit: boolean
  password_require_symbol: boolean
  password_history: number
  password_max_age_days: number
  login_max_failures: number
  login_lock_duration_seconds: number
  session_ttl_seconds: number
  max_active_sessions: number
}

export interface AuditEvent {
  id: string
  occurred_at: string
  request_id: string
  organization_id?: string
  actor_id?: string
  actor_name?: string
  action: string
  resource_type?: string
  resource_id?: string
  result: string
  client_ip?: string
  details?: Record<string, unknown>
}
