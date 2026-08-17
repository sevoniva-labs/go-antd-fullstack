import { EditOutlined } from '@ant-design/icons'
import { ModalForm, ProFormCheckbox, ProFormDependency, ProFormSelect, ProFormTreeSelect } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import type { Role, RoleDataScope } from '@forge/api-client'
import { useMe } from '@forge/auth-sdk'
import { can } from '@forge/auth-sdk'
import { AppPageContainer } from '@forge/design-system'
import { ErrorState } from '@forge/design-system'
import { AppProTable } from '@forge/design-system'
import { departmentTreeSelect } from '../departmentTree'

const dataScopeLabels: Record<RoleDataScope, string> = {
  ORGANIZATION: '本机构全部数据',
  DEPARTMENT: '本人所属部门',
  DEPARTMENT_TREE: '本人所属部门及下级',
  SELF: '仅本人数据',
  CUSTOM: '自定义部门',
}

type DataScopeValues = { data_scope: RoleDataScope; department_ids?: string[] }

export function RolesPage() {
  const qc = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const roles = useQuery({ queryKey: queryKeys.roles, queryFn: api.roles })
  const permissions = useQuery({ queryKey: queryKeys.permissions, queryFn: api.permissions })
  const canReadDepartments = can(me, 'system.department.read')
  const departments = useQuery({ queryKey: queryKeys.departments, queryFn: api.departments, enabled: canReadDepartments })
  const canManageDataScope = me?.principal_type !== 'TOKEN' && Boolean(me?.roles.includes('system_admin'))
  const update = useMutation({
    mutationFn: ({ roleKey, permissionKeys }: { roleKey: string; permissionKeys: string[] }) => api.updateRolePermissions(roleKey, permissionKeys),
    onSuccess: async () => {
      message.success('角色权限已更新')
      await qc.invalidateQueries({ queryKey: queryKeys.roles })
    },
  })
  const updateDataScope = useMutation({
    mutationFn: ({ roleKey, values }: { roleKey: string; values: DataScopeValues }) => api.updateRoleDataScope(
      roleKey, values.data_scope, values.data_scope === 'CUSTOM' ? values.department_ids ?? [] : [],
    ),
    onSuccess: async () => {
      message.success('角色数据范围已更新')
      await qc.invalidateQueries({ queryKey: queryKeys.roles })
    },
  })

  if (roles.isError || permissions.isError) {
    return <AppPageContainer title="角色管理"><ErrorState error={roles.error || permissions.error} onRetry={() => { void roles.refetch(); void permissions.refetch() }} /></AppPageContainer>
  }

  const columns: ProColumns<Role>[] = [
    { title: '角色标识', dataIndex: 'key', copyable: true, width: 180 },
    { title: '角色名称', dataIndex: 'name', width: 160 },
    {
      title: '数据范围', dataIndex: 'data_scope', width: 210,
      render: (_, row) => (
        <Space direction="vertical" size={2}>
          <Tag color={row.data_scope === 'ORGANIZATION' ? 'red' : row.data_scope === 'CUSTOM' ? 'gold' : 'blue'}>
            {dataScopeLabels[row.data_scope] ?? row.data_scope}
          </Tag>
          {row.data_scope === 'CUSTOM' && <Typography.Text type="secondary">{row.data_scope_department_ids.length} 个部门</Typography.Text>}
        </Space>
      ),
    },
    {
      title: '权限',
      dataIndex: 'permissions',
      search: false,
      render: (_, row) => (
        <Space wrap size={[4, 4]}>
          {row.key === 'system_admin'
            ? <Tag color="red">隐式全部权限</Tag>
            : row.permissions.map((item) => <Tag key={item}>{item}</Tag>)}
        </Space>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, row) => {
        if (row.key === 'system_admin') return null
        return (
          <Space size={0} wrap>
            {can(me, 'system.role.manage') && <ModalForm
              title={`配置角色权限 · ${row.name}`}
              trigger={<Button type="link" icon={<EditOutlined />}>配置权限</Button>}
              initialValues={{ permissions: row.permissions }}
              onFinish={async (values) => {
                await update.mutateAsync({ roleKey: row.key, permissionKeys: values.permissions || [] })
                return true
              }}
            >
              <Typography.Paragraph type="secondary">
                权限是代码定义的稳定能力标识；业务模块新增权限时应同步进入后端权限清单，而不是仅在页面中创建字符串。
              </Typography.Paragraph>
              <ProFormCheckbox.Group
                name="permissions"
                label="权限"
                options={(permissions.data?.items || []).map((item) => ({ label: `${item.name} (${item.key})`, value: item.key }))}
              />
            </ModalForm>}
            {canManageDataScope && <ModalForm<DataScopeValues>
              title={`配置数据范围 · ${row.name}`}
              trigger={<Button type="link" icon={<EditOutlined />}>数据范围</Button>}
              initialValues={{ data_scope: row.data_scope, department_ids: row.data_scope_department_ids }}
              onFinish={async (values) => { await updateDataScope.mutateAsync({ roleKey: row.key, values }); return true }}
            >
              <Typography.Paragraph type="secondary">
                数据范围由服务端强制应用于用户目录和任职关系；自定义范围只允许选择活动部门。
              </Typography.Paragraph>
              <ProFormSelect
                name="data_scope" label="范围类型" rules={[{ required: true }]}
                options={Object.entries(dataScopeLabels).map(([value, label]) => ({ value, label }))}
              />
              <ProFormDependency name={['data_scope']}>
                {({ data_scope }) => data_scope === 'CUSTOM' ? <ProFormTreeSelect
                  name="department_ids" label="授权部门" rules={[{ required: true, message: '至少选择一个活动部门' }]}
                  fieldProps={{ treeData: departmentTreeSelect((departments.data?.items ?? []).filter((item) => item.status === 'ACTIVE')), treeDefaultExpandAll: true, treeCheckable: true }}
                /> : null}
              </ProFormDependency>
            </ModalForm>}
          </Space>
        )
      },
    },
  ]

  return (
    <AppPageContainer title="角色管理" subTitle="内置三员分立与普通用户角色；system_admin 为隐式超级权限，不允许在页面修改。">
      <AppProTable<Role> rowKey="key" columns={columns} dataSource={roles.data?.items || []} loading={roles.isLoading || permissions.isLoading} search={false} />
    </AppPageContainer>
  )
}
