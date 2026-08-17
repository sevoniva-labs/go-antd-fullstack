import { Tabs } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { runtimeConfig } from '../app/config/runtime'
import { routeAllowed, routeByPath } from '../app/router/routes'
import { useMe } from '@forge/auth-sdk'

interface TabItem { path: string; name: string }
const storageKey = 'forge.page-tabs'

function readTabs(): TabItem[] {
  try {
    const value = JSON.parse(sessionStorage.getItem(storageKey) || '[]') as TabItem[]
    return Array.isArray(value) ? value : []
  } catch {
    return []
  }
}

export function PageTabs() {
  const location = useLocation()
  const navigate = useNavigate()
  const me = useMe().data
  const [tabs, setTabs] = useState<TabItem[]>(readTabs)

  useEffect(() => {
    if (!me) return
    setTabs((current) => {
      const allowed = current.filter((item) => {
        const route = routeByPath(item.path)
        return Boolean(route && routeAllowed(route, me))
      })
      sessionStorage.setItem(storageKey, JSON.stringify(allowed))
      return allowed
    })
  }, [me])

  useEffect(() => {
    const route = routeByPath(location.pathname)
    if (!route || !routeAllowed(route, me)) return
    setTabs((current) => {
      const next = current.some((item) => item.path === location.pathname)
        ? current
        : [...current, { path: location.pathname, name: route.name }]
      const bounded = next.slice(-10)
      sessionStorage.setItem(storageKey, JSON.stringify(bounded))
      return bounded
    })
  }, [location.pathname, me])

  const items = useMemo(() => tabs.map((item) => ({ key: item.path, label: item.name, closable: item.path !== '/dashboard' })), [tabs])
  if (!runtimeConfig.pageTabs || items.length === 0) return null

  return (
    <div className="page-tabs">
      <Tabs
        hideAdd
        type="editable-card"
        size="small"
        activeKey={location.pathname}
        items={items}
        onChange={(path) => navigate(path)}
        onEdit={(target, action) => {
          if (action !== 'remove' || typeof target !== 'string') return
          setTabs((current) => {
            const index = current.findIndex((item) => item.path === target)
            const next = current.filter((item) => item.path !== target)
            sessionStorage.setItem(storageKey, JSON.stringify(next))
            if (target === location.pathname) {
              navigate(next[Math.max(0, index - 1)]?.path || '/dashboard')
            }
            return next
          })
        }}
      />
    </div>
  )
}
