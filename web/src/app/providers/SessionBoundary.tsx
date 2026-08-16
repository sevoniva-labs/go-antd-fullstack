import { useQueryClient } from '@tanstack/react-query'
import { useEffect, type ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

export function SessionBoundary({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()

  useEffect(() => {
    const unauthorized = () => {
      queryClient.clear()
      if (location.pathname !== '/login') {
        navigate('/login', { replace: true, state: { from: location.pathname } })
      }
    }
    window.addEventListener('forge:unauthorized', unauthorized)
    return () => window.removeEventListener('forge:unauthorized', unauthorized)
  }, [location.pathname, navigate, queryClient])

  return children
}
