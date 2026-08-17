import { ExportOutlined, EyeOutlined } from '@ant-design/icons'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Descriptions, InputNumber, Radio, Space, Tag, Typography } from 'antd'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import type { AuditEvent } from '@forge/api-client'
import { can } from '@forge/auth-sdk'
import { useMe } from '@forge/auth-sdk'
import { CopyText } from '@forge/design-system'
import { SensitiveText } from '@forge/design-system'
import { StatusTag } from '@forge/design-system'
import { AppPageContainer } from '@forge/design-system'
import { DetailDrawer } from '@forge/design-system'
import { ErrorState } from '@forge/design-system'
import { AppProTable } from '@forge/design-system'

export function AuditLogsPage() {
  const [detail, setDetail] = useState<AuditEvent | null>(null)
  const { message } = App.useApp()
  const me = useMe().data
  const canExport = can(me, 'system.audit.export')
  const query = useQuery({ queryKey: queryKeys.auditLogs, queryFn: api.auditLogs, refetchInterval: 30_000 })
  const [limit, setLimit] = useState<number>(2000)
  const [format, setFormat] = useState<'json' | 'csv'>('json')
  const [exporting, setExporting] = useState(false)
  const columns: ProColumns<AuditEvent>[] = [
    { title: '时间', dataIndex: 'occurred_at', valueType: 'dateTime', width: 180 },
    { title: '结果', dataIndex: 'result', width: 100, render: (_, row) => <StatusTag value={row.result} /> },
    { title: '操作', dataIndex: 'action', render: (_, row) => <Tag color="blue">{row.action}</Tag> },
    { title: '操作人', dataIndex: 'actor_name', width: 140 },
    { title: '资源类型', dataIndex: 'resource_type', width: 120 },
    { title: '资源 ID', dataIndex: 'resource_id', render: (_, row) => <CopyText value={row.resource_id} /> },
    { title: '客户端 IP', dataIndex: 'client_ip', width: 160, render: (_, row) => <SensitiveText value={row.client_ip} /> },
    { title: 'Request ID', dataIndex: 'request_id', width: 220, render: (_, row) => <CopyText value={row.request_id} /> },
    { title: '详情', valueType: 'option', fixed: 'right', render: (_, row) => <Button type="link" icon={<EyeOutlined />} onClick={() => setDetail(row)}>查看</Button> },
  ]
  if (query.isError) return <AppPageContainer title="审计日志"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>

  const exportAuditLogs = async () => {
    if (!canExport) return
    try {
      setExporting(true)
      const safeLimit = Math.min(5000, Math.max(1, limit))
      const result = await api.exportAuditLogs({ format, limit: safeLimit })
      const defaultFilename = `audit-logs-${new Date().toISOString().replace(/[-:T]/g, '').slice(0, 15)}.${format}`
      const link = document.createElement('a')
      const blobUrl = URL.createObjectURL(result.blob)
      link.href = blobUrl
      link.download = result.filename || defaultFilename
      link.rel = 'noopener'
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(blobUrl)
      message.success(`审计日志导出成功：${link.download}`)
    } catch (error) {
      message.error((error as Error).message || '导出审计日志失败')
    } finally {
      setExporting(false)
    }
  }

  return (
    <AppPageContainer title="审计日志" subTitle="记录登录、账号、角色、Token、会话等关键安全与管理操作；运行日志由集中日志平台采集。">
      <AppProTable<AuditEvent>
        rowKey="id"
        columns={columns}
        dataSource={query.data?.items || []}
        loading={query.isLoading}
        search={false}
        toolBarRender={() => [
          <Space key="audit-export" size={12} wrap>
            <InputNumber
              value={limit}
              min={1}
              max={5000}
              placeholder="导出条数"
              style={{ width: 140 }}
              onChange={(value) => setLimit(typeof value === 'number' ? value : 2000)}
            />
            <Radio.Group value={format} onChange={(e) => setFormat(e.target.value as 'json' | 'csv')}>
              <Radio value="json">JSON</Radio>
              <Radio value="csv">CSV</Radio>
            </Radio.Group>
            <Button
              type="primary"
              icon={<ExportOutlined />}
              loading={exporting}
              disabled={!canExport}
              onClick={() => void exportAuditLogs()}
            >
              导出
            </Button>
          </Space>,
        ]}
        options={{ reload: () => query.refetch() }}
      />
      <DetailDrawer title="审计详情" open={Boolean(detail)} onClose={() => setDetail(null)}>
        {detail && (
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="事件 ID"><CopyText value={detail.id} /></Descriptions.Item>
            <Descriptions.Item label="Request ID"><CopyText value={detail.request_id} /></Descriptions.Item>
            <Descriptions.Item label="操作"><Tag color="blue">{detail.action}</Tag></Descriptions.Item>
            <Descriptions.Item label="结果"><StatusTag value={detail.result} /></Descriptions.Item>
            <Descriptions.Item label="操作人">{detail.actor_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="资源">{detail.resource_type || '-'} / {detail.resource_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="客户端 IP"><SensitiveText value={detail.client_ip} /></Descriptions.Item>
            <Descriptions.Item label="详情">
              <Typography.Text code style={{ whiteSpace: 'pre-wrap' }}>{JSON.stringify(detail.details || {}, null, 2)}</Typography.Text>
            </Descriptions.Item>
          </Descriptions>
        )}
      </DetailDrawer>
    </AppPageContainer>
  )
}
