import { describe, expect, it } from 'vitest'
import { can } from './access'
import type { Principal } from '../api/types'

const base: Principal = {
  principal_type: 'USER',
  user_id: 'u1',
  organization_id: 'o1',
  login_name: 'demo',
  display_name: 'Demo',
  roles: ['user'],
  permissions: ['system.user.read'],
  must_change_password: false,
}

describe('can', () => {
  it('accepts explicit permission', () => {
    expect(can(base, 'system.user.read')).toBe(true)
  })

  it('rejects missing permission', () => {
    expect(can(base, 'system.audit.read')).toBe(false)
  })

  it('treats system_admin as superuser', () => {
    expect(can({ ...base, roles: ['system_admin'], permissions: [] }, 'anything')).toBe(true)
  })
})
