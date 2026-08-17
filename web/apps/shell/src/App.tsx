import { Navigate, Route, Routes } from 'react-router-dom'
import { appRoutes } from './app/router/routes'
import { RequireAuth } from '@forge/auth-sdk'
import { RequirePermission } from '@forge/auth-sdk'
import { AppLayout } from './layout/AppLayout'
import { LoginPage } from './pages/Login'
import { ForbiddenPage } from './pages/Forbidden'
import { NotFoundPage } from './pages/NotFound'
import { SystemErrorPage } from './pages/SystemError'

export default function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth><AppLayout /></RequireAuth>}>
        <Route path="/403" element={<ForbiddenPage />} />
        <Route path="/500" element={<SystemErrorPage />} />
        {appRoutes.map((route) => {
          const Component = route.component
          return (
            <Route
              key={route.path}
              path={route.path}
              element={<RequirePermission permission={route.permission}><Component /></RequirePermission>}
            />
          )
        })}
        <Route path="*" element={<NotFoundPage />} />
      </Route>
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}
