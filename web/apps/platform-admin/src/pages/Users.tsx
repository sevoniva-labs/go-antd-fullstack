import {
  CheckCircleOutlined,
  BranchesOutlined,
  EditOutlined,
  LockOutlined,
  PlusOutlined,
  StopOutlined,
  UnlockOutlined,
} from '@ant-design/icons'
import { ModalForm, ProFormSelect, ProFormText } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Space, Tag } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import type { User } from '@forge/api-client'
import { can } from '@forge/auth-sdk'
import { useMe } from '@forge/auth-sdk'
import { BoolTag } from '@forge/design-system'
import { StatusTag } from '@forge/design-system'
import { AppPageContainer } from '@forge/design-system'
import { ErrorState } from '@forge/design-system'
import { ConfirmAction } from '@forge/design-system'
import { AppProTable } from '@forge/design-system'
import { UserAssignmentsModal } from './UserAssignmentsModal'

export function UsersPage() {
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const users = useQuery({ queryKey: queryKeys.users, queryFn: api.users })
  const roles = useQuery({ queryKey: queryKeys.roles, queryFn: api.roles })
  const roleOptions = (roles.data?.items || []).map((role) => ({ label: role.name, value: role.key }))

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.users })
  const create = useMutation({
    mutationFn: api.createUser,
    onSuccess: async () => {
      message.success('用户已创建')
      await refresh()
    },
  })
  const updateRoles = useMutation({
    mutationFn: ({ userId, roleKeys, approvalId }: { userId: string; roleKeys: string[]; approvalId: string }) => api.updateUserRoles(userId, roleKeys, approvalId),
    onSuccess: async () => {
      message.success('用户角色已更新')
      await refresh()
    },
  })
  const updateStatus = useMutation({
    mutationFn: ({ userId, status }: { userId: string; status: 'ACTIVE' | 'DISABLED' }) => api.updateUserStatus(userId, status),
    onSuccess: async (_, variables) => {
      message.success(variables.status === 'ACTIVE' ? '账号已启用' : '账号已停用，现有会话已撤销')
      await refresh()
    },
  })
  const unlock = useMutation({
    mutationFn: api.unlockUser,
    onSuccess: async () => {
      message.success('账号已解锁')
      await refresh()
    },
  })
  const resetPassword = useMutation({
    mutationFn: ({ userId, password, approvalId }: { userId: string; password: string; approvalId: string }) => api.resetUserPassword(userId, password, approvalId),
    onSuccess: async () => {
      message.success('密码已重置，用户下次登录必须修改新密码')
      await refresh()
    },
  })

  if (users.isError || roles.isError) {
    return <AppPageContainer title="用户管理"><ErrorState error={users.error || roles.error} onRetry={() => { void users.refetch(); void roles.refetch() }} /></AppPageContainer>
  }

  const columns: ProColumns<User>[] = [
    { title: '登录名', dataIndex: 'login_name', copyable: true, width: 160 },
    { title: '显示名称', dataIndex: 'display_name', width: 160 },
    {
      title: '状态',
      dataIndex: 'status',
      render: (_, row) => (
        <Space size={4}>
          <StatusTag value={row.status} />
          {row.locked_until && new Date(row.locked_until).getTime() > Date.now() && <Tag color="error">LOCKED</Tag>}
        </Space>
      ),
      width: 150,
    },
    {
      title: '角色',
      dataIndex: 'roles',
      search: false,
      render: (_, row) => <Space wrap size={[4, 4]}>{row.roles.map((role) => <Tag key={role}>{role}</Tag>)}</Space>,
    },
    { title: '强制改密', dataIndex: 'must_change_password', search: false, render: (_, row) => <BoolTag value={row.must_change_password} />, width: 110 },
    { title: '创建时间', dataIndex: 'created_at', valueType: 'dateTime', search: false, width: 180 },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      render: (_, row) => {
        const canUpdate = can(me, 'system.user.update')
        const canManageRoles = can(me, 'system.user.role.manage')
        const canManageAssignments = can(me, 'system.user.assignment.manage')
        if (!canUpdate && !canManageRoles && !canManageAssignments) return null
        const isSelf = row.id === me?.user_id
        const locked = Boolean(row.locked_until && new Date(row.locked_until).getTime() > Date.now())
        return (
          <Space size={0} wrap>
            {canManageAssignments && <UserAssignmentsModal user={row} trigger={<Button type="link" icon={<BranchesOutlined />}>任职</Button>} />}
            {!isSelf && canManageRoles && <ModalForm
              title={`调整角色 · ${row.display_name || row.login_name}`}
              trigger={<Button type="link" icon={<EditOutlined />}>角色</Button>}
              initialValues={{ roles: row.roles }}
              onFinish={async (values) => {
                await updateRoles.mutateAsync({ userId: row.id, roleKeys: values.roles || ['user'], approvalId: values.approval_id })
                return true
              }}
            >
              <ProFormSelect name="roles" label="角色" fieldProps={{ mode: 'multiple' }} options={roleOptions} rules={[{ required: true }]} />
              <ProFormText
                name="approval_id"
                label="审批执行票据"
                tooltip="请先在审批中心创建 USER_ROLE_CHANGE 申请；操作 user.roles.update，资源 user，资源 ID 为当前用户 ID，载荷为按角色标识排序后的 roles 数组。"
                rules={[{ required: true, message: '请输入已通过审批的执行票据 ID' }]}
              />
            </ModalForm>}

            {!isSelf && canUpdate && (
              <ModalForm
                title={`重置密码 · ${row.display_name || row.login_name}`}
                trigger={<Button type="link" icon={<LockOutlined />}>重置密码</Button>}
                onFinish={async (values) => {
                  await resetPassword.mutateAsync({ userId: row.id, password: values.password, approvalId: values.approval_id })
                  return true
                }}
              >
                <ProFormText
                  name="approval_id"
                  label="审批执行票据"
                  tooltip="请先创建 USER_PASSWORD_RESET 申请；操作 user.password.reset，资源 user，资源 ID 为当前用户 ID，载荷固定为 force_change=true。审批载荷不得填写密码。"
                  rules={[{ required: true, message: '请输入已通过审批的执行票据 ID' }]}
                />
                <ProFormText.Password
                  name="password"
                  label="新初始密码"
                  extra="重置后会撤销该账号的现有会话，并要求下次登录强制改密。"
                  rules={[{ required: true, min: 12 }]}
                />
                <ProFormText.Password
                  name="confirm_password"
                  label="确认新初始密码"
                  dependencies={['password']}
                  rules={[
                    { required: true },
                    ({ getFieldValue }) => ({
                      validator: (_, value) => !value || value === getFieldValue('password')
                        ? Promise.resolve()
                        : Promise.reject(new Error('两次密码不一致')),
                    }),
                  ]}
                />
              </ModalForm>
            )}

            {locked && !isSelf && canUpdate && (
              <ConfirmAction title="确认解锁该账号？" onConfirm={() => unlock.mutateAsync(row.id)}>
                <Button type="link" icon={<UnlockOutlined />}>解锁</Button>
              </ConfirmAction>
            )}

            {!isSelf && canUpdate && row.status === 'ACTIVE' && (
              <ConfirmAction
                title="确认停用该账号？"
                description="停用后将立即撤销该账号现有会话。"
                danger
                onConfirm={() => updateStatus.mutateAsync({ userId: row.id, status: 'DISABLED' })}
              >
                <Button danger type="link" icon={<StopOutlined />}>停用</Button>
              </ConfirmAction>
            )}

            {!isSelf && canUpdate && row.status !== 'ACTIVE' && (
              <ConfirmAction title="确认启用该账号？" onConfirm={() => updateStatus.mutateAsync({ userId: row.id, status: 'ACTIVE' })}>
                <Button type="link" icon={<CheckCircleOutlined />}>启用</Button>
              </ConfirmAction>
            )}
          </Space>
        )
      },
    },
  ]

  return (
    <AppPageContainer title="用户管理" subTitle="账号生命周期、角色分配、锁定处置、密码重置与首次登录强制改密的统一入口。">
      <AppProTable<User>
        rowKey="id"
        columns={columns}
        dataSource={users.data?.items || []}
        loading={users.isLoading || roles.isLoading}
        search={false}
        toolBarRender={() => can(me, 'system.user.create') ? [
          <ModalForm
            key="create"
            title="创建用户"
            trigger={<Button type="primary" icon={<PlusOutlined />}>创建用户</Button>}
            onFinish={async (values) => {
              await create.mutateAsync({
                login_name: values.login_name,
                display_name: values.display_name,
                password: values.password,
                roles: can(me, 'system.user.role.manage') ? (values.roles || ['user']) : ['user'],
              })
              return true
            }}
          >
            <ProFormText name="login_name" label="登录名" rules={[{ required: true }]} />
            <ProFormText name="display_name" label="显示名称" rules={[{ required: true }]} />
            <ProFormText.Password
              name="password"
              label="初始密码"
              extra="初始密码仅用于首次登录；用户登录后必须修改。"
              rules={[{ required: true, min: 12 }]}
            />
{can(me, 'system.user.role.manage') && <ProFormSelect name="roles" label="角色" fieldProps={{ mode: 'multiple' }} initialValue={['user']} options={roleOptions} rules={[{ required: true }]} />}
          </ModalForm>,
        ] : []}
      />
    </AppPageContainer>
  )
}
