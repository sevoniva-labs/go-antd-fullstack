import { CheckOutlined, PlusOutlined, StopOutlined } from '@ant-design/icons'
import { ModalForm, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { ApprovalRequest, EmergencyAccessGrant, User } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

type EmergencyPayload = {
  target_user_id?: string
  scope?: string
  reason: string
  privilege_keys: string[]
  expires_at: string
}

type RequestValues = EmergencyPayload & { approver_ids: string[] }

const statusColor: Record<EmergencyAccessGrant['status'], string> = {
  SCHEDULED: 'processing', ACTIVE: 'error', EXPIRED: 'default', REVOKED: 'success',
}

function normalizedTime(value: string) {
  return new Date(value).toISOString().replace('.000Z', 'Z')
}

function normalizedKeys(values: string[] | undefined) {
  return [...new Set((values ?? []).map((value) => value.trim()).filter(Boolean))]
}

function parseEmergencyPayload(approval: ApprovalRequest): EmergencyPayload | null {
  try {
    const payload = JSON.parse(approval.payload_json) as Partial<EmergencyPayload>
    const privilegeKeys = normalizedKeys(payload.privilege_keys)
    if (!payload.reason || !payload.expires_at || privilegeKeys.length === 0 || (!payload.target_user_id && !payload.scope)) return null
    return {
      target_user_id: payload.target_user_id?.trim() || undefined,
      scope: payload.scope?.trim() || undefined,
      reason: payload.reason,
      privilege_keys: privilegeKeys,
      expires_at: payload.expires_at,
    }
  } catch {
    return null
  }
}

function userLabel(user: User) {
  return `${user.display_name || user.login_name} (${user.login_name})`
}

export function EmergencyAccessPage() {
  const me = useMe().data
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const grants = useQuery({ queryKey: queryKeys.emergencyAccess, queryFn: api.emergencyAccess })
  const approvals = useQuery({ queryKey: queryKeys.approvals, queryFn: api.approvals })
  const users = useQuery({ queryKey: queryKeys.users, queryFn: api.users })
  const canManage = can(me, 'system.emergency_access.manage')
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.emergencyAccess }),
      queryClient.invalidateQueries({ queryKey: queryKeys.approvals }),
    ])
  }
  const request = useMutation({
    mutationFn: (values: RequestValues) => {
      const payload: EmergencyPayload = {
        target_user_id: values.target_user_id || undefined,
        scope: values.scope?.trim() || undefined,
        reason: values.reason.trim(),
        privilege_keys: normalizedKeys(values.privilege_keys),
        expires_at: normalizedTime(values.expires_at),
      }
      return api.createApproval({
        request_type: 'EMERGENCY_ACCESS', action: 'emergency_access.create', resource: 'emergency_access',
        resource_id: payload.target_user_id || payload.scope, summary: `应急授权至 ${payload.expires_at}。原因：${payload.reason}`,
        payload_json: JSON.stringify(payload), mode: 'ANY', required_approvals: 1, approver_ids: values.approver_ids, expires_in_seconds: 3600,
      })
    },
    onSuccess: async (approval) => { message.success(`应急授权审批已发起：${approval.id}`); await refresh() },
  })
  const execute = useMutation({
    mutationFn: ({ approval, payload }: { approval: ApprovalRequest; payload: EmergencyPayload }) => api.createEmergencyAccess({ ...payload, approval_id: approval.id }),
    onSuccess: async () => { message.success('应急授权已生效'); await refresh() },
  })
  const revoke = useMutation({
    mutationFn: ({ grantId, reason }: { grantId: string; reason: string }) => api.revokeEmergencyAccess(grantId, reason),
    onSuccess: async () => { message.success('应急授权已撤销'); await queryClient.invalidateQueries({ queryKey: queryKeys.emergencyAccess }) },
  })

  if (grants.isError || approvals.isError || users.isError) {
    return <AppPageContainer title="应急授权"><ErrorState error={grants.error || approvals.error || users.error} onRetry={() => void refresh()} /></AppPageContainer>
  }

  const grantRows = grants.data?.grants ?? []
  const consumedApprovalIDs = new Set(grantRows.map((grant) => grant.approval_id))
  const executable = (approvals.data?.items ?? []).filter((approval) =>
    approval.request_type === 'EMERGENCY_ACCESS' && approval.status === 'APPROVED' && approval.applicant_id === me?.user_id && !consumedApprovalIDs.has(approval.id) && parseEmergencyPayload(approval),
  )
  const userOptions = (users.data?.items ?? []).filter((user) => user.status === 'ACTIVE' && user.id !== me?.user_id).map((user) => ({ value: user.id, label: userLabel(user) }))
  const userNames = new Map((users.data?.items ?? []).map((user) => [user.id, userLabel(user)]))
  const columns: ProColumns<EmergencyAccessGrant>[] = [
    { title: '状态', dataIndex: 'status', width: 100, render: (_, row) => <Tag color={statusColor[row.status]}>{row.status}</Tag> },
    { title: '申请人', dataIndex: 'requester_id', width: 180, render: (_, row) => userNames.get(row.requester_id) ?? row.requester_id },
    { title: '目标用户 / 范围', dataIndex: 'target_user_id', width: 220, render: (_, row) => row.target_user_id ? userNames.get(row.target_user_id) ?? row.target_user_id : <Typography.Text code>{row.scope}</Typography.Text> },
    { title: '权限范围', dataIndex: 'privilege_keys', width: 260, render: (_, row) => <Space wrap>{row.privilege_keys.map((key) => <Tag key={key}>{key}</Tag>)}</Space> },
    { title: '失效时间', dataIndex: 'expires_at', valueType: 'dateTime', width: 190 },
    { title: '原因', dataIndex: 'reason', ellipsis: true },
    { title: '审批票据', dataIndex: 'approval_id', copyable: true, width: 230 },
    {
      title: '操作', valueType: 'option', fixed: 'right', width: 100,
      render: (_, row) => canManage && row.status === 'ACTIVE' ? <ModalForm<{ reason: string }>
        title="撤销应急授权" trigger={<Button type="link" danger icon={<StopOutlined />}>撤销</Button>}
        onFinish={async (values) => { await revoke.mutateAsync({ grantId: row.id, reason: values.reason }); return true }}
      ><ProFormTextArea name="reason" label="撤销原因" rules={[{ required: true, min: 8, max: 500 }]} /></ModalForm> : null,
    },
  ]

  return <AppPageContainer title="应急授权" subTitle="Break-glass 授权必须经过近期 MFA、审批执行票据和最小权限约束，最长 60 分钟并全程审计。">
    <Alert type="warning" showIcon message="高风险能力，默认拒绝" description="不能授权 system_admin 或通配权限；目标用户为空时必须填写明确范围。审批通过不代表自动生效，执行时服务端会再次校验 MFA、审批摘要、组织和时效。" style={{ marginBottom: 16 }} />
    {executable.length > 0 && <Card title="待执行的已通过审批" style={{ marginBottom: 16 }}>
      <Space direction="vertical" style={{ width: '100%' }}>
        {executable.map((approval) => {
          const payload = parseEmergencyPayload(approval)!
          return <Alert key={approval.id} type="error" showIcon message={approval.summary} description={<Space wrap>
            <Typography.Text code copyable>{approval.id}</Typography.Text>
            <Button type="primary" size="small" icon={<CheckOutlined />} loading={execute.isPending} onClick={() => execute.mutate({ approval, payload })}>执行授权</Button>
          </Space>} />
        })}
      </Space>
    </Card>}
    <AppProTable<EmergencyAccessGrant>
      rowKey="id" columns={columns} dataSource={grantRows} loading={grants.isLoading} search={false}
      toolBarRender={() => canManage ? [<ModalForm<RequestValues>
        key="request" title="发起应急授权审批" trigger={<Button danger type="primary" icon={<PlusOutlined />}>申请应急授权</Button>}
        onFinish={async (values) => {
          if (!values.target_user_id && !values.scope?.trim()) { message.error('目标用户和授权范围至少填写一项'); return false }
          if (values.target_user_id === me?.user_id) { message.error('不能将应急授权指向当前用户'); return false }
          const keys = normalizedKeys(values.privilege_keys)
          if (keys.length === 0 || keys.some((key) => key === '*' || key === 'system_admin')) { message.error('必须填写具体权限，禁止通配或 system_admin'); return false }
          await request.mutateAsync({ ...values, privilege_keys: keys }); return true
        }}
      >
        <ProFormSelect name="target_user_id" label="目标用户（可选）" options={userOptions} showSearch />
        <ProFormText name="scope" label="授权范围（目标用户为空时必填）" placeholder="例如：payment.review 或 order:read" />
        <ProFormSelect name="privilege_keys" label="具体权限（1-20 项）" fieldProps={{ mode: 'tags' }} rules={[{ required: true, min: 1, max: 20 }]} />
        <ProFormText name="expires_at" label="失效时间（最长 60 分钟）" fieldProps={{ type: 'datetime-local' }} rules={[{ required: true }]} />
        <ProFormTextArea name="reason" label="业务原因" rules={[{ required: true, min: 8, max: 500 }]} />
        <ProFormSelect name="approver_ids" label="审批人用户 ID" fieldProps={{ mode: 'tags' }} rules={[{ required: true }]} />
      </ModalForm>] : []}
    />
  </AppPageContainer>
}
