import { describe, expect, it } from 'vitest'
import type { Principal } from '@forge/api-client'
import { buildMenuRoutes } from './routes'

const principal: Principal = {
  principal_type: 'USER',
  user_id: 'user-1',
  organization_id: 'org-1',
  login_name: 'reviewer',
  display_name: 'Reviewer',
  roles: ['user'],
  permissions: [],
  must_change_password: false,
}

function visiblePaths(routes: Array<Record<string, unknown>>): string[] {
  return routes.flatMap((route) => {
    const nested = Array.isArray(route.routes) ? visiblePaths(route.routes as Array<Record<string, unknown>>) : []
    return typeof route.path === 'string' ? [route.path, ...nested] : nested
  })
}

describe('Shell menu authorization', () => {
  it('keeps public routes while hiding unauthorized platform entries', () => {
    const paths = visiblePaths(buildMenuRoutes(principal))
    expect(paths).toContain('/dashboard')
    expect(paths).not.toContain('/admin/users')
  })

  it('reveals a platform entry only after the permission is present', () => {
    const paths = visiblePaths(buildMenuRoutes({ ...principal, permissions: ['system.user.read'] }))
    expect(paths).toContain('/admin/users')
  })
})
