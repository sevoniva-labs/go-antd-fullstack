import type { ReactNode } from 'react'
import { useMe } from '../../auth/useMe'
import { can } from '../../auth/access'

export function Access({ permission, fallback = null, children }: { permission: string; fallback?: ReactNode; children: ReactNode }) {
  return can(useMe().data, permission) ? children : fallback
}
