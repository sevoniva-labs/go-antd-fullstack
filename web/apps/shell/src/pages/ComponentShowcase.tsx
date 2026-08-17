import { CodeOutlined, DeleteOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, Button, Card, Col, Descriptions, Input, Row, Space, Tag, Typography } from 'antd'
import { useState } from 'react'
import {
  AppPageContainer,
  AppProTable,
  AppUpload,
  BoolTag,
  ConfirmAction,
  CopyText,
  DateTimeText,
  DetailDrawer,
  EmptyState,
  MetricCard,
  PermissionButton,
  SearchToolbar,
  SecretText,
  SensitiveText,
  StatusTag,
} from '../components'

interface SampleRow extends Record<string, unknown> {
  id: string
  name: string
  status: string
  enabled: boolean
  owner: string
  updated_at: string
}

const rows: SampleRow[] = [
  { id: 'svc-001', name: 'gateway-api', status: 'UP', enabled: true, owner: '平台架构组', updated_at: new Date().toISOString() },
  { id: 'svc-002', name: 'batch-worker', status: 'RUNNING', enabled: true, owner: '研发效能组', updated_at: new Date(Date.now() - 3600_000).toISOString() },
  { id: 'svc-003', name: 'legacy-adapter', status: 'DISABLED', enabled: false, owner: '集成组', updated_at: new Date(Date.now() - 86_400_000).toISOString() },
]

export function ComponentShowcasePage() {
  const [detail, setDetail] = useState<SampleRow | null>(null)
  const columns: ProColumns<SampleRow>[] = [
    { title: '服务名称', dataIndex: 'name', render: (_, row) => <Button type="link" onClick={() => setDetail(row)}>{row.name}</Button> },
    { title: '状态', dataIndex: 'status', render: (_, row) => <StatusTag value={row.status} /> },
    { title: '启用', dataIndex: 'enabled', render: (_, row) => <BoolTag value={row.enabled} /> },
    { title: '责任团队', dataIndex: 'owner' },
    { title: '更新时间', dataIndex: 'updated_at', render: (_, row) => <DateTimeText value={row.updated_at} /> },
  ]

  return (
    <AppPageContainer title="组件示例" subTitle="开发环境公共组件基线。生产环境默认隐藏该入口。">
      <Alert type="info" showIcon message="新业务页面优先复用这里的组件与交互模式，不要自行复制一套 Table / 状态颜色 / 敏感信息显示逻辑。" style={{ marginBottom: 16 }} />

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}><MetricCard title="请求成功率" value={99.98} suffix="%" /></Col>
        <Col xs={24} sm={12} xl={6}><MetricCard title="在线服务" value={18} /></Col>
        <Col xs={24} sm={12} xl={6}><MetricCard title="待处理告警" value={3} /></Col>
        <Col xs={24} sm={12} xl={6}><MetricCard title="今日变更" value={12} /></Col>
      </Row>

      <Card title="状态与敏感信息" style={{ marginTop: 16 }}>
        <Space wrap size={[12, 12]}>
          <StatusTag value="UP" />
          <StatusTag value="RUNNING" />
          <StatusTag value="WARNING" />
          <StatusTag value="FAILED" />
          <BoolTag value />
          <SensitiveText value="13800138000" />
          <SecretText value="fgt_live_xxxxxxxxxxxxxxxxx" />
          <CopyText value="req_01JXXXXXXXXXXXXXXXX" />
        </Space>
      </Card>

      <SearchToolbar
        filters={<Space wrap><Input.Search placeholder="关键词" style={{ width: 260 }} /><Tag>环境：DEV</Tag></Space>}
        actions={<Space><PermissionButton permission="system.user.create" type="primary" icon={<PlusOutlined />}>权限按钮</PermissionButton><ConfirmAction title="确认执行示例操作？" onConfirm={() => undefined}><Button danger icon={<DeleteOutlined />}>危险操作</Button></ConfirmAction></Space>}
      />

      <AppProTable<SampleRow> rowKey="id" search={false} columns={columns} dataSource={rows} />

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} md={12}><Card title="空状态"><EmptyState description="暂无符合条件的数据" /></Card></Col>
        <Col xs={24} md={12}>
          <Card title="文件选择基线">
            <Space direction="vertical">
              <AppUpload maxSizeMiB={20} beforeUpload={() => false}>
                <Button icon={<UploadOutlined />}>选择文件</Button>
              </AppUpload>
              <Typography.Text type="secondary">客户端限制只改善体验；服务端仍必须校验大小、类型、文件名、内容与恶意文件。</Typography.Text>
            </Space>
          </Card>
        </Col>
      </Row>

      <DetailDrawer title={detail ? `详情 · ${detail.name}` : '详情'} open={Boolean(detail)} onClose={() => setDetail(null)}>
        {detail && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID"><CopyText value={detail.id} /></Descriptions.Item>
            <Descriptions.Item label="名称">{detail.name}</Descriptions.Item>
            <Descriptions.Item label="状态"><StatusTag value={detail.status} /></Descriptions.Item>
            <Descriptions.Item label="责任团队">{detail.owner}</Descriptions.Item>
            <Descriptions.Item label="更新时间"><DateTimeText value={detail.updated_at} /></Descriptions.Item>
          </Descriptions>
        )}
      </DetailDrawer>

      <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
        <CodeOutlined /> 组件入口统一从 <Typography.Text code>src/components</Typography.Text> 导入，避免业务页面直接复制通用实现。
      </Typography.Paragraph>
    </AppPageContainer>
  )
}
