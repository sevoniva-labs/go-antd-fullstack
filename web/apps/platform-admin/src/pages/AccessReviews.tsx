import { CheckOutlined, PlusOutlined, StopOutlined } from '@ant-design/icons'
import { ModalForm, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, App, Button, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys } from '@forge/api-client'
import type { AccessReview, AccessReviewItem } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

const statusColor: Record<string, string> = { OPEN: 'processing', COMPLETED: 'success', EXPIRED: 'warning' }
const decisionColor: Record<string, string> = { PENDING: 'processing', APPROVE: 'success', REVOKE: 'error', EXCEPTION: 'warning' }

function ReviewItems({ review, canManage }: { review: AccessReview; canManage: boolean }) {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['access-review-items', review.id], queryFn: () => api.accessReviewItems(review.id) })
  const decide = useMutation({
    mutationFn: ({ item, decision, reason }: { item: AccessReviewItem; decision: AccessReviewItem['decision']; reason: string }) => api.decideAccessReviewItem(review.id, item.id, decision, reason),
    onSuccess: async () => { message.success('复核决定已记录'); await queryClient.invalidateQueries({ queryKey: queryKeys.accessReviews }); await queryClient.invalidateQueries({ queryKey: ['access-review-items', review.id] }) },
  })
  const columns: ProColumns<AccessReviewItem>[] = [
    { title: '用户', dataIndex: 'login_name', width: 180 },
    { title: '有效角色', dataIndex: 'role_key', render: (_, row) => <Typography.Text code>{row.role_key}</Typography.Text> },
    { title: '决定', dataIndex: 'decision', width: 110, render: (_, row) => <Tag color={decisionColor[row.decision]}>{row.decision}</Tag> },
    { title: '说明', dataIndex: 'reason', ellipsis: true },
    { title: '决定时间', dataIndex: 'decided_at', valueType: 'dateTime', width: 180 },
    {
      title: '操作', valueType: 'option', width: 220,
      render: (_, row) => canManage && row.decision === 'PENDING' && review.status === 'OPEN' ? <Space size={0}>
        <Button type="link" icon={<CheckOutlined />} onClick={() => decide.mutate({ item: row, decision: 'APPROVE', reason: '' })}>保留</Button>
        <ModalForm<{ reason: string }> title="撤销该权限" trigger={<Button type="link" danger icon={<StopOutlined />}>撤销</Button>} onFinish={async (values) => { await decide.mutateAsync({ item: row, decision: 'REVOKE', reason: values.reason }); return true }}><ProFormTextArea name="reason" label="撤销原因" rules={[{ required: true, min: 8, max: 500 }]} /></ModalForm>
        <ModalForm<{ reason: string }> title="标记例外" trigger={<Button type="link">例外</Button>} onFinish={async (values) => { await decide.mutateAsync({ item: row, decision: 'EXCEPTION', reason: values.reason }); return true }}><ProFormTextArea name="reason" label="例外说明" rules={[{ required: true, min: 8, max: 500 }]} /></ModalForm>
      </Space> : null,
    },
  ]
  if (query.isError) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />
  return <AppProTable<AccessReviewItem> rowKey="id" columns={columns} dataSource={query.data?.items ?? []} loading={query.isLoading} search={false} options={false} />
}

export function AccessReviewsPage() {
  const me = useMe().data
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const reviews = useQuery({ queryKey: queryKeys.accessReviews, queryFn: api.accessReviews })
  const users = useQuery({ queryKey: queryKeys.users, queryFn: api.users })
  const canManage = can(me, 'system.access_review.manage')
  const create = useMutation({ mutationFn: (values: { reviewer_id: string; due_at: string }) => api.createAccessReview({ reviewer_id: values.reviewer_id, due_at: new Date(values.due_at).toISOString() }), onSuccess: async () => { message.success('访问复核已创建'); await queryClient.invalidateQueries({ queryKey: queryKeys.accessReviews }) } })
  if (reviews.isError || users.isError) return <AppPageContainer title="访问复核"><ErrorState error={reviews.error || users.error} onRetry={() => void reviews.refetch()} /></AppPageContainer>
  const columns: ProColumns<AccessReview>[] = [
    { title: '状态', dataIndex: 'status', width: 120, render: (_, row) => <Tag color={statusColor[row.status]}>{row.status}</Tag> },
    { title: '复核人', dataIndex: 'reviewer_name', width: 180 },
    { title: '截止时间', dataIndex: 'due_at', valueType: 'dateTime', width: 190 },
    { title: '创建时间', dataIndex: 'created_at', valueType: 'dateTime', width: 190 },
    { title: '复核单号', dataIndex: 'id', copyable: true, ellipsis: true },
  ]
  const reviewerOptions = (users.data?.items ?? []).filter((user) => user.status === 'ACTIVE' && user.id !== me?.user_id).map((user) => ({ value: user.id, label: `${user.display_name || user.login_name} (${user.login_name})` }))
  return <AppPageContainer title="访问复核" subTitle="按快照逐项复核直接角色、用户组角色和有效临时授权；撤销决定只形成审计证据，权限变更仍需审批链。">
    <Alert type="info" showIcon message="复核结果不可替代权限审批" description="请先完成近期 MFA。对需要撤权的条目，复核决定会留痕，实际权限调整必须通过角色变更审批。" style={{ marginBottom: 16 }} />
    <AppProTable<AccessReview> rowKey="id" columns={columns} dataSource={reviews.data?.items ?? []} loading={reviews.isLoading} search={false}
      expandable={{ expandedRowRender: (review) => <ReviewItems review={review} canManage={canManage} /> }}
      toolBarRender={() => canManage ? [<ModalForm<{ reviewer_id: string; due_at: string }> key="create" title="创建访问复核" trigger={<Button type="primary" icon={<PlusOutlined />}>创建复核</Button>} onFinish={async (values) => { await create.mutateAsync(values); return true }}>
        <ProFormSelect name="reviewer_id" label="复核人" options={reviewerOptions} rules={[{ required: true }]} />
        <ProFormText name="due_at" label="截止时间" placeholder="例如 2026-09-20T18:00:00+08:00" rules={[{ required: true }]} />
      </ModalForm>] : []}
    />
  </AppPageContainer>
}
