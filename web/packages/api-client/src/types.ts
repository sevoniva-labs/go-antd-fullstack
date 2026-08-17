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

export interface Permission {
  id: string
  permission_key: string
  name: string
  created_at: string
}

export interface Role {
  id: string
  role_key: string
  name: string
  permissions: Permission[]
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
