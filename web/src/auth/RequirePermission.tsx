import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useMe } from './useMe'
import { can } from './access'

export function RequirePermission({ permission, children }: { permission?: string; children: ReactNode }) {
  const me = useMe().data
  if (!permission || can(me, permission)) return children
  return <Navigate to="/403" replace />
}
