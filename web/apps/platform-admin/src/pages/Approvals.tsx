import { CheckOutlined, PlusOutlined, RetweetOutlined, StopOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, App, Button, Descriptions, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { ApprovalRequest } from '@forge/api-client'
import { useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

const statusColor: Record<string, string> = { PENDING: 'processing', APPROVED: 'success', REJECTED: 'error', WITHDRAWN: 'default', EXPIRED: 'warning' }

type CreateValues = {
  request_type: string; action: string; resource: string; resource_id?: string; summary: string; payload_json: string;
  mode: ApprovalRequest['mode']; required_approvals: number; approver_ids: string[]; expires_in_hours: number;
}

export function ApprovalsPage() {
  const me = useMe().data
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: queryKeys.approvals, queryFn: api.approvals })
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
  const create = useMutation({ mutationFn: (values: CreateValues) => api.createApproval({ request_type: values.request_type, action: values.action, resource: values.resource, resource_id: values.resource_id, summary: values.summary, payload_json: values.payload_json, mode: values.mode, required_approvals: values.required_approvals, approver_ids: values.approver_ids, expires_in_seconds: values.expires_in_hours * 3600 }), onSuccess: async () => { message.success('审批申请已提交'); await refresh() } })
  const decide = useMutation({ mutationFn: ({ id, decision, comment }: { id: string; decision: 'APPROVE' | 'REJECT'; comment: string }) => api.decideApproval(id, decision, comment), onSuccess: async () => { message.success('审批决定已提交'); await refresh() } })
  const transfer = useMutation({ mutationFn: ({ id, assignee, comment }: { id: string; assignee: string; comment: string }) => api.transferApproval(id, assignee, comment), onSuccess: async () => { message.success('审批任务已转办'); await refresh() } })
  const withdraw = useMutation({ mutationFn: ({ id, comment }: { id: string; comment: string }) => api.withdrawApproval(id, comment), onSuccess: async () => { message.success('审批申请已撤回'); await refresh() } })

  if (query.isError) return <AppPageContainer title="审批中心"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>

  const columns: ProColumns<ApprovalRequest>[] = [
    { title: '状态', dataIndex: 'status', width: 110, render: (_, row) => <Tag color={statusColor[row.status]}>{row.status}</Tag> },
    { title: '申请摘要', dataIndex: 'summary', width: 280, ellipsis: true },
    { title: '类型', dataIndex: 'request_type', width: 150 },
    { title: '动作 / 资源', search: false, width: 220, render: (_, row) => <Typography.Text code>{row.action} · {row.resource}{row.resource_id ? `/${row.resource_id}` : ''}</Typography.Text> },
    { title: '审批模式', search: false, width: 130, render: (_, row) => `${row.mode} · ${row.required_approvals}` },
    { title: '到期时间', dataIndex: 'expires_at', valueType: 'dateTime', width: 180, search: false },
    {
      title: '操作', valueType: 'option', width: 270, fixed: 'right',
      render: (_, row) => {
        if (row.status !== 'PENDING') return null
        const assigned = row.tasks.some((task) => task.assignee_id === me?.user_id && task.status === 'PENDING')
        return <Space size={0} wrap>
          {assigned && <ModalForm<{ comment?: string }> title="同意审批" trigger={<Button type="link" icon={<CheckOutlined />}>同意</Button>} onFinish={async (values) => { await decide.mutateAsync({ id: row.id, decision: 'APPROVE', comment: values.comment ?? '' }); return true }}><ProFormTextArea name="comment" label="审批意见" fieldProps={{ maxLength: 1000, showCount: true }} /></ModalForm>}
          {assigned && <ModalForm<{ comment: string }> title="拒绝审批" trigger={<Button type="link" danger icon={<StopOutlined />}>拒绝</Button>} onFinish={async (values) => { await decide.mutateAsync({ id: row.id, decision: 'REJECT', comment: values.comment }); return true }}><ProFormTextArea name="comment" label="拒绝原因" rules={[{ required: true }]} fieldProps={{ maxLength: 1000, showCount: true }} /></ModalForm>}
          {assigned && <ModalForm<{ new_assignee_id: string; comment?: string }> title="转办审批" trigger={<Button type="link" icon={<RetweetOutlined />}>转办</Button>} onFinish={async (values) => { await transfer.mutateAsync({ id: row.id, assignee: values.new_assignee_id, comment: values.comment ?? '' }); return true }}><ProFormText name="new_assignee_id" label="新审批人用户 ID" rules={[{ required: true }]} /><ProFormTextArea name="comment" label="转办说明" /></ModalForm>}
          {row.applicant_id === me?.user_id && <ModalForm<{ comment?: string }> title="撤回审批" trigger={<Button type="link">撤回</Button>} onFinish={async (values) => { await withdraw.mutateAsync({ id: row.id, comment: values.comment ?? '' }); return true }}><ProFormTextArea name="comment" label="撤回说明" /></ModalForm>}
        </Space>
      },
    },
  ]

  return <AppPageContainer title="审批中心" subTitle="审批摘要由服务端规范化计算；申请人不能审批自己的申请，所有状态变化与审计日志同事务提交。">
    <Alert type="info" showIcon message="高风险操作需要近期 MFA" description="创建、同意、拒绝和转办前，请在账号安全中完成会话二次认证。审批通过不代表命令自动执行，执行时仍会重新校验权限、摘要和资源版本。" style={{ marginBottom: 16 }} />
    <AppProTable<ApprovalRequest>
      rowKey="id" columns={columns} dataSource={query.data?.items ?? []} loading={query.isLoading} search={false}
      expandable={{ expandedRowRender: (row) => <Descriptions size="small" column={1}>
        <Descriptions.Item label="请求摘要"><Typography.Text code copyable>{row.request_digest}</Typography.Text></Descriptions.Item>
        <Descriptions.Item label="申请人">{row.applicant_id}</Descriptions.Item>
        <Descriptions.Item label="审批任务"><Space wrap>{row.tasks.map((task) => <Tag key={task.id}>{task.assignee_id} · {task.status}{task.decision ? ` · ${task.decision}` : ''}</Tag>)}</Space></Descriptions.Item>
      </Descriptions> }}
      toolBarRender={() => [<ModalForm<CreateValues>
        key="create" title="发起审批" trigger={<Button type="primary" icon={<PlusOutlined />}>发起审批</Button>}
        initialValues={{ mode: 'ANY', required_approvals: 1, expires_in_hours: 24, payload_json: '{}' }}
        onFinish={async (values) => { await create.mutateAsync(values); return true }}
      >
        <ProFormText name="request_type" label="申请类型" rules={[{ required: true, max: 100 }]} />
        <ProFormText name="action" label="目标动作" rules={[{ required: true, max: 160 }]} />
        <ProFormText name="resource" label="资源类型" rules={[{ required: true, max: 160 }]} />
        <ProFormText name="resource_id" label="资源 ID" />
        <ProFormTextArea name="summary" label="申请摘要" rules={[{ required: true, max: 500 }]} />
        <ProFormTextArea name="payload_json" label="请求载荷 JSON" extra="载荷不持久化，只用于服务端计算不可变请求摘要。" rules={[{ required: true }]} />
        <ProFormSelect name="mode" label="审批模式" options={[{ value: 'ANY', label: '或签' }, { value: 'ALL', label: '会签' }, { value: 'QUORUM', label: '指定门槛' }]} rules={[{ required: true }]} />
        <ProFormDigit name="required_approvals" label="通过门槛" min={1} fieldProps={{ precision: 0 }} rules={[{ required: true }]} />
        <ProFormSelect name="approver_ids" label="审批人用户 ID" fieldProps={{ mode: 'tags' }} rules={[{ required: true }]} />
        <ProFormDigit name="expires_in_hours" label="有效期（小时）" min={1} max={720} fieldProps={{ precision: 0 }} rules={[{ required: true }]} />
      </ModalForm>]}
    />
  </AppPageContainer>
}
