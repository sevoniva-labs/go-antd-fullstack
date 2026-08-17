import type { Principal } from '@forge/api-client'

export function can(me: Principal | undefined, permission: string): boolean {
  if (!me) return false
  return me.roles.includes('system_admin') || (me.permissions || []).includes(permission)
}
