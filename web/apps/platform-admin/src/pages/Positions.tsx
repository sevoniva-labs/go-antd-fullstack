import { EditOutlined, PlusOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea, ProFormTreeSelect } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { Department, Position } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState, StatusTag } from '@forge/design-system'
import { departmentTreeSelect } from '../departmentTree'

type PositionValues = {
  position_key?: string
  name: string
  description: string
  department_id: string
  status: 'ACTIVE' | 'DISABLED'
  sort_order: number
}

function PositionFields({ departments, includeKey }: { departments: Department[]; includeKey?: boolean }) {
  return (
    <>
      {includeKey && <ProFormText name="position_key" label="岗位标识" extra="创建后不可修改；岗位不自动授予系统角色。" rules={[{ required: true, pattern: /^[A-Za-z0-9._-]+$/, max: 100 }]} />}
      <ProFormText name="name" label="岗位名称" rules={[{ required: true, max: 200 }]} />
      <ProFormTextArea name="description" label="岗位说明" fieldProps={{ maxLength: 500, showCount: true }} />
      <ProFormTreeSelect
        name="department_id"
        label="所属部门"
        fieldProps={{ treeDefaultExpandAll: true, treeData: departmentTreeSelect(departments) }}
        rules={[{ required: true }]}
      />
      <ProFormSelect name="status" label="状态" options={[{ value: 'ACTIVE', label: 'ACTIVE' }, { value: 'DISABLED', label: 'DISABLED' }]} rules={[{ required: true }]} />
      <ProFormDigit name="sort_order" label="排序值" min={0} max={1_000_000} fieldProps={{ precision: 0 }} rules={[{ required: true }]} />
    </>
  )
}

export function PositionsPage() {
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const canManage = can(me, 'system.position.manage')
  const positions = useQuery({ queryKey: queryKeys.positions, queryFn: api.positions })
  const departments = useQuery({ queryKey: queryKeys.departments, queryFn: api.departments })
  const items = positions.data?.items ?? []
  const departmentItems = departments.data?.items ?? []
  const departmentById = new Map(departmentItems.map((item) => [item.id, item]))
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.positions })

  const create = useMutation({
    mutationFn: (values: PositionValues) => api.createPosition({
      position_key: values.position_key ?? '', name: values.name, description: values.description ?? '',
      department_id: values.department_id, status: values.status, sort_order: values.sort_order,
    }),
    onSuccess: async () => { message.success('岗位已创建'); await refresh() },
  })
  const update = useMutation({
    mutationFn: ({ id, values }: { id: string; values: PositionValues }) => api.updatePosition(id, values),
    onSuccess: async () => { message.success('岗位已更新'); await refresh() },
  })

  if (positions.isError || departments.isError) {
    return <AppPageContainer title="岗位管理"><ErrorState error={positions.error || departments.error} onRetry={() => { void positions.refetch(); void departments.refetch() }} /></AppPageContainer>
  }

  const columns: ProColumns<Position>[] = [
    { title: '岗位名称', dataIndex: 'name', width: 180, render: (_, row) => <Typography.Text strong>{row.name}</Typography.Text> },
    { title: '岗位标识', dataIndex: 'position_key', copyable: true, width: 180 },
    {
      title: '所属部门', dataIndex: 'department_id', width: 220,
      render: (_, row) => {
        const department = departmentById.get(row.department_id)
        return department ? <Space>{department.name}{department.status !== 'ACTIVE' && <Tag color="warning">部门已停用</Tag>}</Space> : <Tag color="error">部门不可见</Tag>
      },
    },
    { title: '状态', dataIndex: 'status', width: 110, render: (_, row) => <StatusTag value={row.status} /> },
    { title: '说明', dataIndex: 'description', ellipsis: true, search: false },
    { title: '排序', dataIndex: 'sort_order', width: 90, search: false },
    {
      title: '操作', valueType: 'option', width: 90, fixed: 'right',
      render: (_, row) => canManage ? (
        <ModalForm<PositionValues>
          title={`编辑岗位 · ${row.name}`}
          trigger={<Button type="link" icon={<EditOutlined />}>编辑</Button>}
          initialValues={{ name: row.name, description: row.description, department_id: row.department_id, status: row.status, sort_order: row.sort_order }}
          onFinish={async (values) => { await update.mutateAsync({ id: row.id, values }); return true }}
        ><PositionFields departments={departmentItems} /></ModalForm>
      ) : null,
    },
  ]

  return (
    <AppPageContainer title="岗位管理" subTitle="岗位描述组织任职关系，不等于系统角色；人员任职与职责分离策略将在后端独立生效。">
      <AppProTable<Position>
        rowKey="id"
        columns={columns}
        dataSource={items}
        loading={positions.isLoading || departments.isLoading}
        search={false}
        toolBarRender={() => canManage ? [
          <ModalForm<PositionValues>
            key="create"
            title="创建岗位"
            trigger={<Button type="primary" icon={<PlusOutlined />}>创建岗位</Button>}
            initialValues={{ status: 'ACTIVE', sort_order: 0, description: '' }}
            onFinish={async (values) => { await create.mutateAsync(values); return true }}
          ><PositionFields departments={departmentItems} includeKey /></ModalForm>,
        ] : []}
      />
    </AppPageContainer>
  )
}
