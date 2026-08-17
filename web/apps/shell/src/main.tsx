import React from 'react'
import ReactDOM from 'react-dom/client'
import { configureApiClient } from '@forge/api-client'
import AppRoutes from './App'
import { runtimeConfig } from './app/config/runtime'
import { AppProviders } from './app/providers/AppProviders'
import { AppErrorBoundary } from '@forge/design-system/error-boundary'
import './index.css'

configureApiClient({ baseUrl: runtimeConfig.apiBaseUrl })

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppProviders>
      <AppErrorBoundary>
        <AppRoutes />
      </AppErrorBoundary>
    </AppProviders>
  </React.StrictMode>,
)
