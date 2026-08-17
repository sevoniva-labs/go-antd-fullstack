import { EditOutlined, PlusOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTreeSelect } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { Department } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState, StatusTag } from '@forge/design-system'
import { buildDepartmentTree, departmentDescendantIds, departmentTreeSelect } from '../departmentTree'

type DepartmentValues = {
  department_key?: string
  name: string
  parent_id?: string
  status: 'ACTIVE' | 'DISABLED'
  sort_order: number
}

function DepartmentFields({ items, excluded, includeKey }: { items: Department[]; excluded?: Set<string>; includeKey?: boolean }) {
  return (
    <>
      {includeKey && <ProFormText name="department_key" label="部门标识" extra="创建后不可修改；仅允许字母、数字、点、下划线和连字符。" rules={[{ required: true, pattern: /^[A-Za-z0-9._-]+$/, max: 100 }]} />}
      <ProFormText name="name" label="部门名称" rules={[{ required: true, max: 200 }]} />
      <ProFormTreeSelect
        name="parent_id"
        label="上级部门"
        allowClear
        placeholder="留空表示根部门"
        fieldProps={{ treeDefaultExpandAll: true, treeData: departmentTreeSelect(items, excluded) }}
      />
      <ProFormSelect name="status" label="状态" options={[{ value: 'ACTIVE', label: 'ACTIVE' }, { value: 'DISABLED', label: 'DISABLED' }]} rules={[{ required: true }]} />
      <ProFormDigit name="sort_order" label="排序值" min={0} max={1_000_000} fieldProps={{ precision: 0 }} rules={[{ required: true }]} />
    </>
  )
}

export function DepartmentsPage() {
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const canManage = can(me, 'system.department.manage')
  const query = useQuery({ queryKey: queryKeys.departments, queryFn: api.departments })
  const items = query.data?.items ?? []
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.departments })

  const create = useMutation({
    mutationFn: (values: DepartmentValues) => api.createDepartment({
      department_key: values.department_key ?? '', name: values.name, parent_id: values.parent_id,
      status: values.status, sort_order: values.sort_order,
    }),
    onSuccess: async () => { message.success('部门已创建'); await refresh() },
  })
  const update = useMutation({
    mutationFn: ({ id, values }: { id: string; values: DepartmentValues }) => api.updateDepartment(id, values),
    onSuccess: async () => { message.success('部门已更新'); await refresh() },
  })

  if (query.isError) {
    return <AppPageContainer title="部门管理"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>
  }

  const columns: ProColumns<Department>[] = [
    { title: '部门名称', dataIndex: 'name', width: 240, render: (_, row) => <Space><Typography.Text strong={!row.parent_id}>{row.name}</Typography.Text>{!row.parent_id && <Tag>根部门</Tag>}</Space> },
    { title: '部门标识', dataIndex: 'department_key', copyable: true, width: 180 },
    { title: '状态', dataIndex: 'status', width: 110, render: (_, row) => <StatusTag value={row.status} /> },
    { title: '排序', dataIndex: 'sort_order', width: 90, search: false },
    { title: '更新时间', dataIndex: 'updated_at', valueType: 'dateTime', width: 180, search: false },
    {
      title: '操作', valueType: 'option', width: 190, fixed: 'right',
      render: (_, row) => canManage ? (
        <Space size={0}>
          {row.status === 'ACTIVE' && <ModalForm<DepartmentValues>
            title={`创建子部门 · ${row.name}`}
            trigger={<Button type="link" icon={<PlusOutlined />}>子部门</Button>}
            initialValues={{ parent_id: row.id, status: 'ACTIVE', sort_order: 0 }}
            onFinish={async (values) => { await create.mutateAsync(values); return true }}
          ><DepartmentFields items={items} includeKey /></ModalForm>}
          <ModalForm<DepartmentValues>
            title={`编辑部门 · ${row.name}`}
            trigger={<Button type="link" icon={<EditOutlined />}>编辑</Button>}
            initialValues={{ name: row.name, parent_id: row.parent_id, status: row.status, sort_order: row.sort_order }}
            onFinish={async (values) => { await update.mutateAsync({ id: row.id, values }); return true }}
          ><DepartmentFields items={items} excluded={departmentDescendantIds(items, row.id)} /></ModalForm>
        </Space>
      ) : null,
    },
  ]

  return (
    <AppPageContainer title="部门管理" subTitle="组织内部门树是岗位归属、数据范围和审批路由的基础；停用父部门前必须先停用活动子部门。">
      <AppProTable<Department>
        rowKey="id"
        columns={columns}
        dataSource={buildDepartmentTree(items)}
        loading={query.isLoading}
        search={false}
        expandable={{ defaultExpandAllRows: true }}
        toolBarRender={() => canManage ? [
          <ModalForm<DepartmentValues>
            key="create"
            title="创建根部门"
            trigger={<Button type="primary" icon={<PlusOutlined />}>创建部门</Button>}
            initialValues={{ status: 'ACTIVE', sort_order: 0 }}
            onFinish={async (values) => { await create.mutateAsync(values); return true }}
          ><DepartmentFields items={items} includeKey /></ModalForm>,
        ] : []}
      />
    </AppPageContainer>
  )
}
