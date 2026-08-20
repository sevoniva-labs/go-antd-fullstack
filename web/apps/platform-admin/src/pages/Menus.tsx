import { EditOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys, type Menu } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

const statusColor: Record<string, string> = { ACTIVE: 'success', DISABLED: 'default' }
type MenuUpdateValues = { parent_key: string; name: string; route: string; icon: string; permission_key: string; sort_order: number; status: Menu['status']; approval_id: string }

export function MenusPage() {
  const me = useMe().data
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: queryKeys.menus, queryFn: api.menus })
  const update = useMutation({
    mutationFn: ({ menu, values }: { menu: Menu; values: MenuUpdateValues }) => api.updateMenu(menu.key, values, values.approval_id),
    onSuccess: async () => { message.success('菜单变更已执行'); await queryClient.invalidateQueries({ queryKey: queryKeys.menus }) },
  })
  const columns: ProColumns<Menu>[] = [
    { title: '菜单标识', dataIndex: 'key', render: (_, row) => <Typography.Text code copyable>{row.key}</Typography.Text> },
    { title: '名称', dataIndex: 'name', width: 160 },
    { title: '父菜单', dataIndex: 'parent_key', render: (_, row) => row.parent_key || '根菜单' },
    { title: '路由', dataIndex: 'route', render: (_, row) => <Typography.Text code>{row.route || '-'}</Typography.Text> },
    { title: '权限标识', dataIndex: 'permission_key', render: (_, row) => row.permission_key ? <Typography.Text code>{row.permission_key}</Typography.Text> : '无' },
    { title: '排序', dataIndex: 'sort_order', width: 80 },
    { title: '状态', dataIndex: 'status', width: 100, render: (_, row) => <Tag color={statusColor[row.status] ?? 'warning'}>{row.status}</Tag> },
    {
      title: '操作', valueType: 'option', width: 100,
      render: (_, row) => can(me, 'system.menu.manage') ? <ModalForm<MenuUpdateValues>
        title={`变更菜单 · ${row.name}`}
        trigger={<Button type="link" icon={<EditOutlined />}>变更</Button>}
        initialValues={{ parent_key: row.parent_key, name: row.name, route: row.route, icon: row.icon, permission_key: row.permission_key, sort_order: row.sort_order, status: row.status }}
        onFinish={async (values) => { await update.mutateAsync({ menu: row, values }); return true }}
      >
        <Typography.Paragraph type="secondary">菜单变更必须先在审批中心创建并通过 `MENU_CHANGE` 申请，执行时服务端会再次校验摘要、权限和近期 MFA。</Typography.Paragraph>
        <ProFormText name="approval_id" label="审批执行票据" rules={[{ required: true, message: '请输入已通过审批的执行票据 ID' }]} />
        <ProFormText name="name" label="菜单名称" rules={[{ required: true, max: 200 }]} />
        <ProFormText name="parent_key" label="父菜单标识" />
        <ProFormText name="route" label="路由" />
        <ProFormText name="icon" label="图标标识" />
        <ProFormText name="permission_key" label="权限标识" />
        <ProFormDigit name="sort_order" label="排序" min={0} fieldProps={{ precision: 0 }} />
        <ProFormSelect name="status" label="状态" options={[{ value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' }]} rules={[{ required: true }]} />
      </ModalForm> : null,
    },
  ]
  if (query.isError) return <AppPageContainer title="菜单目录"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>

  const items = query.data?.menus ?? query.data?.items ?? []
  return (
    <AppPageContainer title="菜单目录" subTitle="菜单目录由后端权限模型控制；此页面只展示当前组织可见菜单，菜单变更必须通过受控审批与审计流程。">
      <AppProTable<Menu> rowKey="id" columns={columns} dataSource={items} loading={query.isLoading} search={false} />
    </AppPageContainer>
  )
}
