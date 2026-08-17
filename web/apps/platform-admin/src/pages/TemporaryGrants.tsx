import { CheckOutlined, PlusOutlined, StopOutlined } from '@ant-design/icons'
import { ModalForm, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { ApprovalRequest, TemporaryRoleGrant } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

type GrantPayload = { user_id: string; role_key: string; reason: string; valid_from: string; valid_until: string }
type RequestValues = GrantPayload & { approver_ids: string[] }

const statusColor: Record<TemporaryRoleGrant['status'], string> = {
  SCHEDULED: 'processing', ACTIVE: 'success', EXPIRED: 'default', REVOKED: 'error',
}

function normalizedTime(value: string) {
  return new Date(value).toISOString().replace('.000Z', 'Z')
}

function parseGrantPayload(approval: ApprovalRequest): GrantPayload | null {
  try {
    const payload = JSON.parse(approval.payload_json) as Partial<GrantPayload>
    if (!payload.user_id || !payload.role_key || !payload.reason || !payload.valid_from || !payload.valid_until) return null
    return payload as GrantPayload
  } catch {
    return null
  }
}

export function TemporaryGrantsPage() {
  const me = useMe().data
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const grants = useQuery({ queryKey: queryKeys.temporaryRoleGrants, queryFn: api.temporaryRoleGrants })
  const approvals = useQuery({ queryKey: queryKeys.approvals, queryFn: api.approvals })
  const users = useQuery({ queryKey: queryKeys.users, queryFn: api.users })
  const roles = useQuery({ queryKey: queryKeys.roles, queryFn: api.roles })
  const canManage = can(me, 'system.temporary_grant.manage')
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.temporaryRoleGrants }),
      queryClient.invalidateQueries({ queryKey: queryKeys.approvals }),
    ])
  }
  const requestGrant = useMutation({
    mutationFn: (values: RequestValues) => {
      const payload: GrantPayload = {
        user_id: values.user_id, role_key: values.role_key, reason: values.reason.trim(),
        valid_from: normalizedTime(values.valid_from), valid_until: normalizedTime(values.valid_until),
      }
      return api.createApproval({
        request_type: 'TEMPORARY_ROLE_GRANT', action: 'temporary_role_grant.create', resource: 'user', resource_id: payload.user_id,
        summary: `临时授予角色 ${payload.role_key}，有效至 ${payload.valid_until}。原因：${payload.reason}`,
        payload_json: JSON.stringify(payload), mode: 'ANY', required_approvals: 1, approver_ids: values.approver_ids, expires_in_seconds: 24 * 3600,
      })
    },
    onSuccess: async (approval) => { message.success(`审批已发起：${approval.id}`); await refresh() },
  })
  const executeGrant = useMutation({
    mutationFn: ({ approval, payload }: { approval: ApprovalRequest; payload: GrantPayload }) => api.createTemporaryRoleGrant({ ...payload, approval_id: approval.id }),
    onSuccess: async () => { message.success('临时授权已生效'); await refresh() },
  })
  const revokeGrant = useMutation({
    mutationFn: ({ grantId, reason }: { grantId: string; reason: string }) => api.revokeTemporaryRoleGrant(grantId, reason),
    onSuccess: async () => { message.success('临时授权已撤销'); await refresh() },
  })

  if (grants.isError || approvals.isError || users.isError || roles.isError) {
    return <AppPageContainer title="临时授权"><ErrorState error={grants.error || approvals.error || users.error || roles.error} onRetry={() => void refresh()} /></AppPageContainer>
  }

  const consumedApprovalIDs = new Set((grants.data?.items ?? []).map((grant) => grant.approval_id))
  const executable = (approvals.data?.items ?? []).filter((approval) =>
    approval.request_type === 'TEMPORARY_ROLE_GRANT' && approval.status === 'APPROVED' && approval.applicant_id === me?.user_id && !consumedApprovalIDs.has(approval.id) && parseGrantPayload(approval),
  )
  const userOptions = (users.data?.items ?? []).filter((user) => user.status === 'ACTIVE' && user.id !== me?.user_id).map((user) => ({ value: user.id, label: `${user.display_name || user.login_name} (${user.login_name})` }))
  const roleOptions = (roles.data?.items ?? []).filter((role) => role.key !== 'system_admin').map((role) => ({ value: role.key, label: `${role.name} (${role.key})` }))
  const userNames = new Map((users.data?.items ?? []).map((user) => [user.id, user.display_name || user.login_name]))
  const columns: ProColumns<TemporaryRoleGrant>[] = [
    { title: '状态', dataIndex: 'status', width: 110, render: (_, row) => <Tag color={statusColor[row.status]}>{row.status}</Tag> },
    { title: '用户', dataIndex: 'user_id', width: 180, render: (_, row) => userNames.get(row.user_id) ?? row.user_id },
    { title: '临时角色', dataIndex: 'role_key', width: 170, render: (_, row) => <Typography.Text code>{row.role_key}</Typography.Text> },
    { title: '生效时间', dataIndex: 'valid_from', valueType: 'dateTime', width: 180 },
    { title: '失效时间', dataIndex: 'valid_until', valueType: 'dateTime', width: 180 },
    { title: '原因', dataIndex: 'reason', ellipsis: true },
    { title: '审批票据', dataIndex: 'approval_id', copyable: true, width: 230 },
    {
      title: '操作', valueType: 'option', fixed: 'right', width: 100,
      render: (_, row) => canManage && (row.status === 'ACTIVE' || row.status === 'SCHEDULED') ? <ModalForm<{ reason: string }>
        title="撤销临时授权" trigger={<Button type="link" danger icon={<StopOutlined />}>撤销</Button>}
        onFinish={async (values) => { await revokeGrant.mutateAsync({ grantId: row.id, reason: values.reason }); return true }}
      ><ProFormTextArea name="reason" label="撤销原因" rules={[{ required: true, min: 4, max: 500 }]} /></ModalForm> : null,
    },
  ]

  return <AppPageContainer title="临时授权" subTitle="审批后按时生效、到期自动回收；特权角色最长 8 小时，普通角色最长 30 天。">
    {executable.length > 0 && <Card title="待执行的已通过审批" style={{ marginBottom: 16 }}>
      <Space direction="vertical" style={{ width: '100%' }}>
        {executable.map((approval) => {
          const payload = parseGrantPayload(approval)!
          return <Alert key={approval.id} type="success" showIcon message={approval.summary} description={<Space wrap>
            <Typography.Text code copyable>{approval.id}</Typography.Text>
            <Button type="primary" size="small" icon={<CheckOutlined />} loading={executeGrant.isPending} onClick={() => executeGrant.mutate({ approval, payload })}>执行授权</Button>
          </Space>} />
        })}
      </Space>
    </Card>}
    <AppProTable<TemporaryRoleGrant>
      rowKey="id" columns={columns} dataSource={grants.data?.items ?? []} loading={grants.isLoading} search={false}
      toolBarRender={() => canManage ? [<ModalForm<RequestValues>
        key="request" title="发起临时授权审批" trigger={<Button type="primary" icon={<PlusOutlined />}>申请临时授权</Button>}
        onFinish={async (values) => { await requestGrant.mutateAsync(values); return true }}
      >
        <ProFormSelect name="user_id" label="目标用户" options={userOptions} showSearch rules={[{ required: true }]} />
        <ProFormSelect name="role_key" label="临时角色" options={roleOptions} rules={[{ required: true }]} />
        <ProFormText name="valid_from" label="生效时间" fieldProps={{ type: 'datetime-local' }} rules={[{ required: true }]} />
        <ProFormText name="valid_until" label="失效时间" fieldProps={{ type: 'datetime-local' }} rules={[{ required: true }]} />
        <ProFormTextArea name="reason" label="业务原因" rules={[{ required: true, min: 8, max: 500 }]} />
        <ProFormSelect name="approver_ids" label="审批人用户 ID" fieldProps={{ mode: 'tags' }} rules={[{ required: true }]} />
      </ModalForm>] : []}
    />
  </AppPageContainer>
}
