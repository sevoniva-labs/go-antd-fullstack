import { Spin } from 'antd'
import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { ApiError } from '@forge/api-client'
import { useMe } from './useMe'

export function RequireAuth({ children }: { children: ReactNode }) {
  const me = useMe()
  const location = useLocation()
  if (me.isLoading) return <div className="center-screen"><Spin size="large" /></div>
  if (me.error instanceof ApiError && me.error.status === 401) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  if (me.isError) throw me.error
  if (me.data?.must_change_password && location.pathname !== '/account/security') {
    return <Navigate to="/account/security" replace />
  }
  return children
}
