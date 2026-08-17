import { SearchOutlined } from '@ant-design/icons'
import { AutoComplete, Button, Modal, Typography } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { appRoutes, routeAllowed } from '../app/router/routes'
import { useMe } from '@forge/auth-sdk'

export function GlobalSearch() {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const me = useMe().data
  const options = useMemo(() => appRoutes
    .filter((route) => route.menu && routeAllowed(route, me))
    .map((route) => ({ value: route.path, label: `${route.group ? `${route.group} / ` : ''}${route.name}` })), [me])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setOpen(true)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  return (
    <>
      <Button type="text" icon={<SearchOutlined />} onClick={() => setOpen(true)}>
        <span className="global-search-label">搜索</span>
        <Typography.Text keyboard className="global-search-shortcut">⌘K</Typography.Text>
      </Button>
      <Modal title="全局导航" open={open} footer={null} onCancel={() => setOpen(false)} destroyOnHidden>
        <AutoComplete
          autoFocus
          style={{ width: '100%' }}
          options={options}
          placeholder="输入菜单名称快速跳转"
          filterOption={(input, option) => String(option?.label || '').toLowerCase().includes(input.toLowerCase())}
          onSelect={(path) => {
            setOpen(false)
            navigate(path)
          }}
        />
      </Modal>
    </>
  )
}
