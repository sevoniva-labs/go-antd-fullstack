import { describe, expect, it } from 'vitest'
import { platformAdminModules } from './modules'

describe('platformAdminModules', () => {
  it('uses unique keys and paths', () => {
    expect(new Set(platformAdminModules.map((module) => module.key)).size).toBe(platformAdminModules.length)
    expect(new Set(platformAdminModules.map((module) => module.path)).size).toBe(platformAdminModules.length)
  })

  it('requires an explicit permission for every administrative route', () => {
    const unguarded = platformAdminModules.filter((module) => module.path.startsWith('/admin/') && !module.permission)
    expect(unguarded).toEqual([])
  })
})
