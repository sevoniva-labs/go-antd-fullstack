import type { ReactNode } from 'react'
import { useMe } from '@forge/auth-sdk'
import { can } from '@forge/auth-sdk'

export function Access({ permission, fallback = null, children }: { permission: string; fallback?: ReactNode; children: ReactNode }) {
  return can(useMe().data, permission) ? children : fallback
}
