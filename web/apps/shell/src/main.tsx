import React from 'react'
import ReactDOM from 'react-dom/client'
import AppRoutes from './App'
import { AppProviders } from './app/providers/AppProviders'
import { AppErrorBoundary } from './components/feedback/AppErrorBoundary'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppProviders>
      <AppErrorBoundary>
        <AppRoutes />
      </AppErrorBoundary>
    </AppProviders>
  </React.StrictMode>,
)
