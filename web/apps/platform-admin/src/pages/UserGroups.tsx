import { EditOutlined, PlusOutlined, SafetyCertificateOutlined, TeamOutlined } from '@ant-design/icons'
import { ModalForm, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { UserGroup } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState, StatusTag } from '@forge/design-system'

type GroupValues = {
  group_key?: string
  name: string
  description: string
  status: 'ACTIVE' | 'DISABLED'
}

function GroupFields({ includeKey }: { includeKey?: boolean }) {
  return (
    <>
      {includeKey && <ProFormText name="group_key" label="用户组标识" extra="创建后不可修改。" rules={[{ required: true, pattern: /^[A-Za-z0-9._-]+$/, max: 100 }]} />}
      <ProFormText name="name" label="用户组名称" rules={[{ required: true, max: 200 }]} />
      <ProFormTextArea name="description" label="说明" fieldProps={{ maxLength: 500, showCount: true }} />
      <ProFormSelect name="status" label="状态" options={[{ value: 'ACTIVE', label: 'ACTIVE' }, { value: 'DISABLED', label: 'DISABLED' }]} rules={[{ required: true }]} />
    </>
  )
}

export function UserGroupsPage() {
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const canManage = can(me, 'system.user_group.manage')
  const groups = useQuery({ queryKey: queryKeys.userGroups, queryFn: api.userGroups })
  const users = useQuery({ queryKey: queryKeys.users, queryFn: api.users })
  const roles = useQuery({ queryKey: queryKeys.roles, queryFn: api.roles })
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.userGroups })
  const userOptions = (users.data?.items ?? []).map((user) => ({ label: `${user.display_name || user.login_name} (${user.login_name})`, value: user.id, disabled: user.status !== 'ACTIVE' }))
  const roleOptions = (roles.data?.items ?? []).filter((role) => role.key !== 'system_admin').map((role) => ({ label: `${role.name} (${role.key})`, value: role.key }))

  const create = useMutation({
    mutationFn: (values: GroupValues) => api.createUserGroup({ group_key: values.group_key ?? '', name: values.name, description: values.description ?? '', status: values.status }),
    onSuccess: async () => { message.success('用户组已创建'); await refresh() },
  })
  const update = useMutation({
    mutationFn: ({ id, values }: { id: string; values: GroupValues }) => api.updateUserGroup(id, values),
    onSuccess: async () => { message.success('用户组已更新'); await refresh() },
  })
  const updateMembers = useMutation({
    mutationFn: ({ id, memberIds }: { id: string; memberIds: string[] }) => api.updateUserGroupMembers(id, memberIds),
    onSuccess: async () => { message.success('用户组成员已更新，权限立即重新计算'); await refresh() },
  })
  const updateRoles = useMutation({
    mutationFn: ({ id, roleKeys }: { id: string; roleKeys: string[] }) => api.updateUserGroupRoles(id, roleKeys),
    onSuccess: async () => { message.success('用户组角色已更新，权限立即重新计算'); await refresh() },
  })

  if (groups.isError || users.isError || roles.isError) {
    return <AppPageContainer title="用户组管理"><ErrorState error={groups.error || users.error || roles.error} onRetry={() => { void groups.refetch(); void users.refetch(); void roles.refetch() }} /></AppPageContainer>
  }

  const columns: ProColumns<UserGroup>[] = [
    { title: '用户组名称', dataIndex: 'name', width: 180, render: (_, row) => <Typography.Text strong>{row.name}</Typography.Text> },
    { title: '用户组标识', dataIndex: 'group_key', copyable: true, width: 180 },
    { title: '状态', dataIndex: 'status', width: 110, render: (_, row) => <StatusTag value={row.status} /> },
    { title: '成员', dataIndex: 'member_count', width: 90, search: false, render: (_, row) => <Tag icon={<TeamOutlined />}>{row.member_count}</Tag> },
    { title: '角色', dataIndex: 'roles', search: false, render: (_, row) => <Space wrap size={[4, 4]}>{row.roles.length > 0 ? row.roles.map((role) => <Tag key={role} color="blue">{role}</Tag>) : <Typography.Text type="secondary">无授权角色</Typography.Text>}</Space> },
    { title: '说明', dataIndex: 'description', ellipsis: true, search: false },
    {
      title: '操作', valueType: 'option', width: 250, fixed: 'right',
      render: (_, row) => canManage ? (
        <Space size={0}>
          <ModalForm<GroupValues>
            title={`编辑用户组 · ${row.name}`}
            trigger={<Button type="link" icon={<EditOutlined />}>编辑</Button>}
            initialValues={{ name: row.name, description: row.description, status: row.status }}
            onFinish={async (values) => { await update.mutateAsync({ id: row.id, values }); return true }}
          ><GroupFields /></ModalForm>
          <ModalForm<{ member_ids: string[] }>
            title={`配置成员 · ${row.name}`}
            trigger={<Button type="link" icon={<TeamOutlined />}>成员</Button>}
            initialValues={{ member_ids: row.member_ids }}
            onFinish={async (values) => { await updateMembers.mutateAsync({ id: row.id, memberIds: values.member_ids ?? [] }); return true }}
          ><ProFormSelect name="member_ids" label="成员" fieldProps={{ mode: 'multiple', showSearch: true, optionFilterProp: 'label' }} options={userOptions} /></ModalForm>
          <ModalForm<{ roles: string[] }>
            title={`配置组角色 · ${row.name}`}
            trigger={<Button type="link" icon={<SafetyCertificateOutlined />}>角色</Button>}
            initialValues={{ roles: row.roles }}
            onFinish={async (values) => { await updateRoles.mutateAsync({ id: row.id, roleKeys: values.roles ?? [] }); return true }}
          >
            <Typography.Paragraph type="secondary">组角色会实时合并到所有活动成员。system_admin 仅允许实名账号直接授予，不能作为组角色。</Typography.Paragraph>
            <ProFormSelect name="roles" label="组角色" fieldProps={{ mode: 'multiple' }} options={roleOptions} />
          </ModalForm>
        </Space>
      ) : null,
    },
  ]

  return (
    <AppPageContainer title="用户组管理" subTitle="批量组织业务用户并统一授予普通角色；成员、组角色和启停均受后端授权上限与事务审计控制。">
      <AppProTable<UserGroup>
        rowKey="id"
        columns={columns}
        dataSource={groups.data?.items ?? []}
        loading={groups.isLoading || users.isLoading || roles.isLoading}
        search={false}
        toolBarRender={() => canManage ? [
          <ModalForm<GroupValues>
            key="create"
            title="创建用户组"
            trigger={<Button type="primary" icon={<PlusOutlined />}>创建用户组</Button>}
            initialValues={{ status: 'ACTIVE', description: '' }}
            onFinish={async (values) => { await create.mutateAsync(values); return true }}
          ><GroupFields includeKey /></ModalForm>,
        ] : []}
      />
    </AppPageContainer>
  )
}
