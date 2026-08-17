import type { Department } from '@forge/api-client'

export type DepartmentNode = Department & { children?: DepartmentNode[] }

export interface DepartmentTreeOption {
  title: string
  value: string
  disabled: boolean
  children?: DepartmentTreeOption[]
}

function sortNodes(nodes: DepartmentNode[]): void {
  nodes.sort((left, right) => left.sort_order - right.sort_order || left.department_key.localeCompare(right.department_key))
  for (const node of nodes) {
    if (node.children) sortNodes(node.children)
  }
}

export function buildDepartmentTree(items: readonly Department[]): DepartmentNode[] {
  const byId = new Map<string, DepartmentNode>(
    items.map((item): [string, DepartmentNode] => [item.id, { ...item, children: [] }]),
  )
  const roots: DepartmentNode[] = []
  for (const item of byId.values()) {
    const parent = item.parent_id ? byId.get(item.parent_id) : undefined
    if (parent) parent.children?.push(item)
    else roots.push(item)
  }
  sortNodes(roots)
  return roots
}

export function departmentDescendantIds(items: readonly Department[], departmentId: string): Set<string> {
  const children = new Map<string, string[]>()
  for (const item of items) {
    if (!item.parent_id) continue
    const current = children.get(item.parent_id) ?? []
    current.push(item.id)
    children.set(item.parent_id, current)
  }
  const result = new Set<string>([departmentId])
  const pending = [departmentId]
  while (pending.length > 0) {
    const current = pending.pop()
    if (!current) continue
    for (const child of children.get(current) ?? []) {
      if (result.has(child)) continue
      result.add(child)
      pending.push(child)
    }
  }
  return result
}

export function departmentTreeSelect(items: readonly Department[], excluded = new Set<string>()): DepartmentTreeOption[] {
  return buildDepartmentTree(items)
    .filter((item) => !excluded.has(item.id))
    .map(function toOption(item): DepartmentTreeOption {
      const children = (item.children ?? []).filter((child) => !excluded.has(child.id)).map(toOption)
      return {
        title: `${item.name} (${item.department_key})`,
        value: item.id,
        disabled: item.status !== 'ACTIVE',
        children: children.length > 0 ? children : undefined,
      }
    })
}
