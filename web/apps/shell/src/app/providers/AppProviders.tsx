import { App as AntdApp } from 'antd'
import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useEffect, useState, type ReactNode } from 'react'
import { BrowserRouter } from 'react-router-dom'
import { ApiError } from '@forge/api-client'
import { ThemeModeProvider } from './ThemeModeProvider'
import { SessionBoundary } from './SessionBoundary'
import { reportBrowserError } from '../telemetry/browser'

function shouldRetry(failureCount: number, error: unknown) {
  if (error instanceof ApiError && error.status < 500) return false
  return failureCount < 1
}

export function AppProviders({ children }: { children: ReactNode }) {
  const [queryClient] = useState(() => new QueryClient({
    queryCache: new QueryCache(),
    mutationCache: new MutationCache(),
    defaultOptions: {
      queries: {
        retry: shouldRetry,
        refetchOnWindowFocus: false,
        staleTime: 15_000,
      },
      mutations: { retry: false },
    },
  }))

  useEffect(() => {
    const onError = (event: ErrorEvent) => reportBrowserError('unhandled-error', event.error || event.message)
    const onRejection = (event: PromiseRejectionEvent) => reportBrowserError('unhandled-rejection', event.reason)
    window.addEventListener('error', onError)
    window.addEventListener('unhandledrejection', onRejection)
    return () => {
      window.removeEventListener('error', onError)
      window.removeEventListener('unhandledrejection', onRejection)
    }
  }, [])

  return (
    <ThemeModeProvider>
      <AntdApp>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter><SessionBoundary>{children}</SessionBoundary></BrowserRouter>
        </QueryClientProvider>
      </AntdApp>
    </ThemeModeProvider>
  )
}
