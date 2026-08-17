import type { ReactElement } from 'react'
import { useEffect, useState } from 'react'
import { ModalForm, ProFormList, ProFormSelect, ProFormSwitch, ProFormText, ProFormTreeSelect } from '@ant-design/pro-components'
import { Alert, App, Form, Space, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { User, UserAssignment } from '@forge/api-client'
import { departmentTreeSelect } from '../departmentTree'

type AssignmentValue = {
  department_id: string
  position_id?: string
  primary?: boolean
  valid_from?: string
  valid_until?: string
}

type AssignmentForm = { assignments: AssignmentValue[] }

function localDateTime(value?: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function optionalISO(value?: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function assignmentValues(items: UserAssignment[]): AssignmentValue[] {
  return items.map((item) => ({
    department_id: item.department_id,
    position_id: item.position_id,
    primary: item.primary,
    valid_from: localDateTime(item.valid_from),
    valid_until: localDateTime(item.valid_until),
  }))
}

export function UserAssignmentsModal({ user, trigger }: { user: User; trigger: ReactElement }) {
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm<AssignmentForm>()
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const assignments = useQuery({ queryKey: queryKeys.userAssignments(user.id), queryFn: () => api.userAssignments(user.id), enabled: open })
  const departments = useQuery({ queryKey: queryKeys.departments, queryFn: api.departments, enabled: open })
  const positions = useQuery({ queryKey: queryKeys.positions, queryFn: api.positions, enabled: open })

  useEffect(() => {
    if (open && assignments.data) form.setFieldsValue({ assignments: assignmentValues(assignments.data.items) })
  }, [assignments.data, form, open])

  const replace = useMutation({
    mutationFn: (values: AssignmentForm) => api.replaceUserAssignments(user.id, (values.assignments ?? []).map((item) => ({
      department_id: item.department_id,
      position_id: item.position_id || undefined,
      primary: Boolean(item.primary),
      valid_from: optionalISO(item.valid_from),
      valid_until: optionalISO(item.valid_until),
    }))),
    onSuccess: async () => {
      message.success('用户任职已更新')
      await queryClient.invalidateQueries({ queryKey: queryKeys.userAssignments(user.id) })
    },
  })

  const departmentItems = departments.data?.items ?? []
  const departmentById = new Map(departmentItems.map((item) => [item.id, item]))
  const positionOptions = (positions.data?.items ?? []).map((position) => ({
    label: `${position.name} (${departmentById.get(position.department_id)?.name ?? '未知部门'})`,
    value: position.id,
    disabled: position.status !== 'ACTIVE',
  }))
  const error = assignments.error || departments.error || positions.error

  return (
    <ModalForm<AssignmentForm>
      form={form}
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) form.resetFields()
      }}
      title={`任职管理 · ${user.display_name || user.login_name}`}
      trigger={trigger}
      width={760}
      submitter={{ submitButtonProps: { disabled: Boolean(error) || assignments.isLoading || departments.isLoading || positions.isLoading } }}
      onFinish={async (values) => { await replace.mutateAsync(values); return true }}
    >
      <Alert
        type="info"
        showIcon
        message="任职关系不直接授予系统角色"
        description="存在任职时必须且只能有一个主岗；岗位必须属于所选部门。有效期用于调岗、兼岗和到期回收，最终校验由后端完成。"
        style={{ marginBottom: 16 }}
      />
      {error && <Alert type="error" showIcon message="任职基础数据加载失败" description={error instanceof Error ? error.message : '请稍后重试'} style={{ marginBottom: 16 }} />}
      <ProFormList
        name="assignments"
        label="任职记录"
        creatorButtonProps={{ creatorButtonText: '添加任职' }}
        copyIconProps={false}
        itemRender={({ listDom, action }, { index }) => (
          <Space align="start" style={{ display: 'flex', width: '100%' }}>
            <Typography.Text type="secondary" style={{ paddingTop: 8, minWidth: 52 }}>{index === 0 ? '任职' : `兼岗 ${index}`}</Typography.Text>
            <div style={{ flex: 1 }}>{listDom}</div>
            {action}
          </Space>
        )}
      >
        <ProFormTreeSelect
          name="department_id"
          label="部门"
          fieldProps={{ treeDefaultExpandAll: true, treeData: departmentTreeSelect(departmentItems) }}
          rules={[{ required: true }]}
          width="md"
        />
        <ProFormSelect name="position_id" label="岗位（可选）" options={positionOptions} fieldProps={{ showSearch: true, optionFilterProp: 'label', allowClear: true }} width="md" />
        <ProFormSwitch name="primary" label="主岗" />
        <ProFormText name="valid_from" label="生效时间" fieldProps={{ type: 'datetime-local' }} width="md" />
        <ProFormText name="valid_until" label="失效时间" fieldProps={{ type: 'datetime-local' }} width="md" />
      </ProFormList>
    </ModalForm>
  )
}
