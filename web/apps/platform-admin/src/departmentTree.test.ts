import { describe, expect, it } from 'vitest'
import type { Department } from '@forge/api-client'
import { buildDepartmentTree, departmentDescendantIds, departmentTreeSelect } from './departmentTree'

const departments: Department[] = [
  { id: 'child', organization_id: 'org-1', parent_id: 'root', department_key: 'child', name: 'Child', status: 'ACTIVE', sort_order: 20, created_at: '', updated_at: '' },
  { id: 'root', organization_id: 'org-1', department_key: 'root', name: 'Root', status: 'ACTIVE', sort_order: 10, created_at: '', updated_at: '' },
  { id: 'disabled', organization_id: 'org-1', department_key: 'disabled', name: 'Disabled', status: 'DISABLED', sort_order: 30, created_at: '', updated_at: '' },
]

describe('department tree', () => {
  it('builds a stable hierarchy independent of input order', () => {
    const tree = buildDepartmentTree(departments)
    expect(tree.map((item) => item.id)).toEqual(['root', 'disabled'])
    expect(tree[0]?.children?.map((item) => item.id)).toEqual(['child'])
  })

  it('excludes self and descendants from reparent choices', () => {
    const excluded = departmentDescendantIds(departments, 'root')
    expect([...excluded].sort()).toEqual(['child', 'root'])
    expect(departmentTreeSelect(departments, excluded).map((item) => item.value)).toEqual(['disabled'])
  })

  it('marks disabled departments unavailable as parents', () => {
    expect(departmentTreeSelect(departments).find((item) => item.value === 'disabled')?.disabled).toBe(true)
  })
})
