import type { ProColumns } from '@ant-design/pro-components'
import { Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { api, queryKeys, type Menu } from '@forge/api-client'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

const statusColor: Record<string, string> = { ACTIVE: 'success', DISABLED: 'default' }

export function MenusPage() {
  const query = useQuery({ queryKey: queryKeys.menus, queryFn: api.menus })
  const columns: ProColumns<Menu>[] = [
    { title: '菜单标识', dataIndex: 'key', render: (_, row) => <Typography.Text code copyable>{row.key}</Typography.Text> },
    { title: '名称', dataIndex: 'name', width: 160 },
    { title: '父菜单', dataIndex: 'parent_key', render: (_, row) => row.parent_key || '根菜单' },
    { title: '路由', dataIndex: 'route', render: (_, row) => <Typography.Text code>{row.route || '-'}</Typography.Text> },
    { title: '权限标识', dataIndex: 'permission_key', render: (_, row) => row.permission_key ? <Typography.Text code>{row.permission_key}</Typography.Text> : '无' },
    { title: '排序', dataIndex: 'sort_order', width: 80 },
    { title: '状态', dataIndex: 'status', width: 100, render: (_, row) => <Tag color={statusColor[row.status] ?? 'warning'}>{row.status}</Tag> },
  ]
  if (query.isError) return <AppPageContainer title="菜单目录"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>

  const items = query.data?.menus ?? query.data?.items ?? []
  return (
    <AppPageContainer title="菜单目录" subTitle="菜单目录由后端权限模型控制；此页面只展示当前组织可见菜单，菜单变更必须通过受控审批与审计流程。">
      <AppProTable<Menu> rowKey="id" columns={columns} dataSource={items} loading={query.isLoading} search={false} />
    </AppPageContainer>
  )
}
