import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, queryKeys, type DataDeletionEvidence, type DataFieldPolicy } from '@forge/api-client'
import { can, useMe } from '@forge/auth-sdk'
import { AppPageContainer, AppProTable, ErrorState } from '@forge/design-system'

const classificationOptions = [
  { value: 'public', label: '公开' }, { value: 'internal', label: '内部' }, { value: 'confidential', label: '机密' },
  { value: 'restricted', label: '受限' }, { value: 'personal_information', label: '个人信息' }, { value: 'important_data', label: '重要数据' },
]
const maskOptions = [
  { value: 'none', label: '不脱敏' }, { value: 'stars', label: '通用星号' }, { value: 'mobile', label: '手机号' },
  { value: 'id_card', label: '证件号' }, { value: 'bank_card', label: '银行卡' }, { value: 'email', label: '邮箱' }, { value: 'name', label: '姓名' },
]

type PolicyForm = { field_key: string; classification: string; owner: string; purpose: string; residency: string; retention_days: number; tags: string; mask_strategy: string; export_approval: boolean | string; watermark: boolean | string; approval_id: string }
type EvidenceForm = { resource_type: string; resource_digest: string; field_keys: string; reason: string; records_deleted: number; deleted_at: string; approval_id: string }

export function DataGovernancePage() {
  const me = useMe().data
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const policies = useQuery({ queryKey: queryKeys.dataPolicies, queryFn: api.dataPolicies })
  const evidence = useQuery({ queryKey: queryKeys.dataDeletionEvidence, queryFn: api.dataDeletionEvidence })
  const managePolicies = can(me, 'system.data_policy.manage')
  const manageRetention = can(me, 'system.data.retention.manage')
  const upsert = useMutation({
    mutationFn: ({ values }: { values: PolicyForm }) => api.upsertDataPolicy({ field_key: values.field_key, classification: values.classification, owner: values.owner, purpose: values.purpose, residency: values.residency, retention_days: values.retention_days, tags: values.tags.split(',').map((tag) => tag.trim()).filter(Boolean), mask_strategy: values.mask_strategy, export_approval: values.export_approval === true || values.export_approval === 'true', watermark: values.watermark === true || values.watermark === 'true' }, values.approval_id),
    onSuccess: async () => { message.success('数据策略已审批执行'); await queryClient.invalidateQueries({ queryKey: queryKeys.dataPolicies }) },
  })
  const record = useMutation({
    mutationFn: ({ values }: { values: EvidenceForm }) => api.recordDataDeletionEvidence({ resource_type: values.resource_type, resource_digest: values.resource_digest, field_keys: values.field_keys.split(',').map((key) => key.trim()).filter(Boolean), reason: values.reason, records_deleted: values.records_deleted, deleted_at: new Date(values.deleted_at).toISOString() }, values.approval_id),
    onSuccess: async () => { message.success('删除证明已记录'); await queryClient.invalidateQueries({ queryKey: queryKeys.dataDeletionEvidence }) },
  })
  if (policies.isError || evidence.isError) return <AppPageContainer title="数据治理"><ErrorState error={policies.error || evidence.error} onRetry={() => { void policies.refetch(); void evidence.refetch() }} /></AppPageContainer>

  const policyColumns: ProColumns<DataFieldPolicy>[] = [
    { title: '字段标识', dataIndex: 'field_key', render: (_, row) => <Typography.Text code copyable>{row.field_key}</Typography.Text> },
    { title: '分类', dataIndex: 'classification', render: (_, row) => <Tag color={row.classification === 'restricted' || row.classification === 'personal_information' ? 'red' : 'blue'}>{row.classification}</Tag> },
    { title: '责任人', dataIndex: 'owner' }, { title: '驻留', dataIndex: 'residency' }, { title: '保留天数', dataIndex: 'retention_days' },
    { title: '脱敏', dataIndex: 'mask_strategy' }, { title: '导出审批', dataIndex: 'export_approval', render: (_, row) => row.export_approval ? <Tag color="orange">必需</Tag> : '否' },
  ]
  const evidenceColumns: ProColumns<DataDeletionEvidence>[] = [
    { title: '资源类型', dataIndex: 'resource_type' }, { title: '资源摘要', dataIndex: 'resource_digest', copyable: true, ellipsis: true },
    { title: '字段数', dataIndex: 'field_keys', render: (_, row) => row.field_keys.length }, { title: '删除记录数', dataIndex: 'records_deleted' },
    { title: '删除时间', dataIndex: 'deleted_at', valueType: 'dateTime' }, { title: '证明哈希', dataIndex: 'evidence_hash', copyable: true, ellipsis: true },
  ]
  return <AppPageContainer title="数据治理" subTitle="数据策略、导出审批与删除证明由后端按组织边界、审批票据和审计事务强制执行。">
    <Alert type="warning" showIcon message="高敏感字段默认强制脱敏、导出审批和水印" description="删除证明只记录已完成的业务删除结果，不代替业务保留任务或实际删除执行。字段未登记时，服务端拒绝导出与证明登记。" style={{ marginBottom: 16 }} />
    <Card title="字段策略目录" style={{ marginBottom: 16 }}>
      <AppProTable<DataFieldPolicy> rowKey="id" columns={policyColumns} dataSource={policies.data?.policies ?? policies.data?.items ?? []} loading={policies.isLoading} search={false}
        toolBarRender={() => managePolicies ? [<ModalForm<PolicyForm> key="upsert" title="登记/变更数据策略" trigger={<Button type="primary" icon={<PlusOutlined />}>登记策略</Button>} onFinish={async (values) => { await upsert.mutateAsync({ values }); return true }} initialValues={{ classification: 'internal', mask_strategy: 'none', retention_days: 365, export_approval: 'false', watermark: 'false' }}>
          <ProFormText name="approval_id" label="审批执行票据" rules={[{ required: true }]} /><ProFormText name="field_key" label="字段标识" rules={[{ required: true, max: 200 }]} /><ProFormSelect name="classification" label="分类分级" options={classificationOptions} rules={[{ required: true }]} /><ProFormText name="owner" label="责任人" rules={[{ required: true }]} /><ProFormText name="purpose" label="处理目的" rules={[{ required: true }]} /><ProFormText name="residency" label="数据驻留" rules={[{ required: true }]} /><ProFormDigit name="retention_days" label="保留天数" min={1} fieldProps={{ precision: 0 }} rules={[{ required: true }]} /><ProFormText name="tags" label="标签" placeholder="用逗号分隔" /><ProFormSelect name="mask_strategy" label="脱敏策略" options={maskOptions} rules={[{ required: true }]} /><ProFormSelect name="export_approval" label="导出审批" options={[{ value: 'true', label: '必需' }, { value: 'false', label: '按策略' }]} /><ProFormSelect name="watermark" label="导出水印" options={[{ value: 'true', label: '必需' }, { value: 'false', label: '按策略' }]} />
        </ModalForm>] : []}
      />
    </Card>
    <Card title="删除证明" extra={manageRetention ? <ModalForm<EvidenceForm> title="记录删除证明" trigger={<Button type="primary" icon={<PlusOutlined />}>记录证明</Button>} onFinish={async (values) => { await record.mutateAsync({ values }); return true }}>
      <ProFormText name="approval_id" label="审批执行票据" rules={[{ required: true }]} /><ProFormText name="resource_type" label="资源类型" rules={[{ required: true }]} /><ProFormText name="resource_digest" label="资源 SHA-256 摘要" rules={[{ required: true, len: 64, pattern: /^[0-9a-fA-F]+$/ }]} /><ProFormText name="field_keys" label="字段标识" placeholder="用逗号分隔" rules={[{ required: true }]} /><ProFormTextArea name="reason" label="删除原因" rules={[{ required: true, min: 4, max: 500 }]} /><ProFormDigit name="records_deleted" label="删除记录数" min={0} fieldProps={{ precision: 0 }} rules={[{ required: true }]} /><ProFormText name="deleted_at" label="删除时间" placeholder="例如 2026-08-20T18:00:00+08:00" rules={[{ required: true }]} />
    </ModalForm> : null}><AppProTable<DataDeletionEvidence> rowKey="id" columns={evidenceColumns} dataSource={evidence.data?.evidence ?? evidence.data?.items ?? []} loading={evidence.isLoading} search={false} options={false} /></Card>
  </AppPageContainer>
}

export { ConfigChangesPage } from './ConfigChanges'
