import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys, type ConfigChange } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

type CreateForm = { namespace: string; group: string; data_id: string; version: number; expected_previous_version: number; value_digest: string; value_ref: string; sensitive: boolean | string }
type ApprovalForm = { approval_id: string }

const transitionLabels: Record<string, string> = { approve: '审批通过', publish: '发布', rollbackRequest: '申请回滚', rollback: '执行回滚' }

export function ConfigChangesPage() {
  const me = useMe().data
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const changes = useQuery({ queryKey: queryKeys.configChanges, queryFn: api.configChanges })
  const manage = can(me, 'system.config.manage')
  const create = useMutation({
    mutationFn: ({ values }: { values: CreateForm }) => api.createConfigChange({ ...values, sensitive: values.sensitive === true || values.sensitive === 'true' }),
    onSuccess: async () => { message.success('配置变更已登记，等待审批'); await queryClient.invalidateQueries({ queryKey: queryKeys.configChanges }) },
  })
  const transition = useMutation({
    mutationFn: ({ action, id, approvalId }: { action: keyof typeof transitionLabels; id: string; approvalId: string }) => {
      if (action === 'approve') return api.approveConfigChange(id, approvalId)
      if (action === 'publish') return api.publishConfigChange(id, approvalId)
      if (action === 'rollbackRequest') return api.requestConfigRollback(id, approvalId)
      return api.rollbackConfigChange(id, approvalId)
    },
    onSuccess: async (_, variables) => { message.success(`配置变更${transitionLabels[variables.action]}成功`); await queryClient.invalidateQueries({ queryKey: queryKeys.configChanges }) },
  })
  if (changes.isError) return <AppPageContainer title="配置变更"><ErrorState error={changes.error} onRetry={() => void changes.refetch()} /></AppPageContainer>

  const actionForm = (row: ConfigChange, action: keyof typeof transitionLabels) => <ModalForm<ApprovalForm> key={`${row.id}-${action}`} title={`${transitionLabels[action]}：${row.data_id}`} trigger={<Button type="link" size="small">{transitionLabels[action]}</Button>} onFinish={async (values) => { await transition.mutateAsync({ action, id: row.id, approvalId: values.approval_id }); return true }}><ProFormText name="approval_id" label="审批执行票据" rules={[{ required: true }]} /></ModalForm>
  const columns: ProColumns<ConfigChange>[] = [
    { title: '配置标识', dataIndex: 'data_id', render: (_, row) => <Typography.Text code>{`${row.namespace}/${row.group}/${row.data_id}`}</Typography.Text> },
    { title: '版本', dataIndex: 'version' },
    { title: '摘要', dataIndex: 'value_digest', copyable: true, ellipsis: true },
    { title: '引用', dataIndex: 'value_ref', ellipsis: true },
    { title: '状态', dataIndex: 'state', render: (_, row) => <Tag color={row.state === 'PUBLISHED' ? 'green' : row.state === 'REJECTED' ? 'red' : 'orange'}>{row.state}</Tag> },
    { title: '敏感', dataIndex: 'sensitive', render: (_, row) => row.sensitive ? <Tag color="red">仅摘要</Tag> : '否' },
    { title: '更新时间', dataIndex: 'updated_at', valueType: 'dateTime' },
    { title: '操作', valueType: 'option', render: (_, row) => manage ? <Space size={0}>{row.state === 'PENDING_APPROVAL' && actionForm(row, 'approve')}{row.state === 'APPROVED' && actionForm(row, 'publish')}{row.state === 'PUBLISHED' && actionForm(row, 'rollbackRequest')}{row.state === 'ROLLBACK_PENDING' && actionForm(row, 'rollback')}</Space> : null },
  ]
  return <AppPageContainer title="配置变更" subTitle="只保留版本和内容摘要；配置发布、审批和回滚由后端事务、审批票据和审计强制执行。">
    <Alert type="warning" showIcon message="敏感配置不进入历史明文" description="value_ref 必须指向受控配置保险箱或不可变制品，value_digest 用于发布前校验；页面不会读取配置内容。" style={{ marginBottom: 16 }} />
    <Card title="配置历史" extra={manage ? <ModalForm<CreateForm> title="登记配置变更" trigger={<Button type="primary" icon={<PlusOutlined />}>登记变更</Button>} onFinish={async (values) => { await create.mutateAsync({ values }); return true }} initialValues={{ group: 'DEFAULT_GROUP', expected_previous_version: 0, sensitive: 'true' }}>
      <ProFormText name="namespace" label="命名空间" rules={[{ required: true }]} /><ProFormText name="group" label="配置组" rules={[{ required: true }]} /><ProFormText name="data_id" label="Data ID" rules={[{ required: true }]} /><ProFormDigit name="version" label="版本" min={1} fieldProps={{ precision: 0 }} rules={[{ required: true }]} /><ProFormDigit name="expected_previous_version" label="期望前版本" min={0} fieldProps={{ precision: 0 }} rules={[{ required: true }]} /><ProFormText name="value_digest" label="内容 SHA-256" rules={[{ required: true, len: 64, pattern: /^[0-9a-fA-F]+$/ }]} /><ProFormText name="value_ref" label="受控内容引用" rules={[{ required: true, max: 500 }]} /><ProFormSelect name="sensitive" label="敏感配置" options={[{ value: 'true', label: '是，仅保留摘要' }, { value: 'false', label: '否' }]} />
    </ModalForm> : null}><AppProTable<ConfigChange> rowKey="id" columns={columns} dataSource={changes.data?.changes ?? changes.data?.items ?? []} loading={changes.isLoading} search={false} options={false} /></Card>
  </AppPageContainer>
}
