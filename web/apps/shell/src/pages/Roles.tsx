import { EditOutlined } from '@ant-design/icons'
import { ModalForm, ProFormCheckbox } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import type { Role } from '@forge/api-client'
import { useMe } from '../auth/useMe'
import { can } from '../auth/access'
import { AppPageContainer } from '../components/layout/AppPageContainer'
import { ErrorState } from '../components/feedback/ErrorState'
import { AppProTable } from '../components/table/AppProTable'

export function RolesPage() {
  const qc = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const roles = useQuery({ queryKey: queryKeys.roles, queryFn: api.roles })
  const permissions = useQuery({ queryKey: queryKeys.permissions, queryFn: api.permissions })
  const update = useMutation({
    mutationFn: ({ roleKey, permissionKeys }: { roleKey: string; permissionKeys: string[] }) => api.updateRolePermissions(roleKey, permissionKeys),
    onSuccess: async () => {
      message.success('角色权限已更新')
      await qc.invalidateQueries({ queryKey: queryKeys.roles })
    },
  })

  if (roles.isError || permissions.isError) {
    return <AppPageContainer title="角色管理"><ErrorState error={roles.error || permissions.error} onRetry={() => { void roles.refetch(); void permissions.refetch() }} /></AppPageContainer>
  }

  const columns: ProColumns<Role>[] = [
    { title: '角色标识', dataIndex: 'role_key', copyable: true, width: 180 },
    { title: '角色名称', dataIndex: 'name', width: 160 },
    {
      title: '权限',
      dataIndex: 'permissions',
      search: false,
      render: (_, row) => (
        <Space wrap size={[4, 4]}>
          {row.role_key === 'system_admin'
            ? <Tag color="red">隐式全部权限</Tag>
            : row.permissions.map((item) => <Tag key={item.permission_key}>{item.permission_key}</Tag>)}
        </Space>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, row) => {
        if (row.role_key === 'system_admin' || !can(me, 'system.role.manage')) return null
        return (
          <ModalForm
            title={`配置角色权限 · ${row.name}`}
            trigger={<Button type="link" icon={<EditOutlined />}>配置权限</Button>}
            initialValues={{ permissions: row.permissions.map((item) => item.permission_key) }}
            onFinish={async (values) => {
              await update.mutateAsync({ roleKey: row.role_key, permissionKeys: values.permissions || [] })
              return true
            }}
          >
            <Typography.Paragraph type="secondary">
              权限是代码定义的稳定能力标识；业务模块新增权限时应同步进入后端权限清单，而不是仅在页面中创建字符串。
            </Typography.Paragraph>
            <ProFormCheckbox.Group
              name="permissions"
              label="权限"
              options={(permissions.data?.items || []).map((item) => ({ label: `${item.name} (${item.permission_key})`, value: item.permission_key }))}
            />
          </ModalForm>
        )
      },
    },
  ]

  return (
    <AppPageContainer title="角色管理" subTitle="内置三员分立与普通用户角色；system_admin 为隐式超级权限，不允许在页面修改。">
      <AppProTable<Role> rowKey="id" columns={columns} dataSource={roles.data?.items || []} loading={roles.isLoading || permissions.isLoading} search={false} />
    </AppPageContainer>
  )
}
