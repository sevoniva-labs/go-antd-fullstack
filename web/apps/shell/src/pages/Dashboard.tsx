import {
  AuditOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { StatisticCard } from '@ant-design/pro-components'
import { Alert, Card, Col, List, Row, Skeleton, Space, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import { AppPageContainer } from '@forge/design-system'
import { StatusTag } from '@forge/design-system'
import { ErrorState } from '@forge/design-system'
import { useMe } from '@forge/auth-sdk'
import { can } from '@forge/auth-sdk'

export function DashboardPage() {
  const navigate = useNavigate()
  const me = useMe().data
  const info = useQuery({ queryKey: queryKeys.systemInfo, queryFn: api.systemInfo, refetchInterval: 60_000 })
  const ready = useQuery({ queryKey: queryKeys.readiness, queryFn: api.readiness, refetchInterval: 30_000 })

  if (info.isLoading || ready.isLoading) return <AppPageContainer title="工作台"><Skeleton /></AppPageContainer>
  if (info.isError || ready.isError) {
    return <AppPageContainer title="工作台"><ErrorState error={info.error || ready.error} onRetry={() => { void info.refetch(); void ready.refetch() }} /></AppPageContainer>
  }

  const quick = [
    { title: '用户管理', desc: '账号与角色管理', path: '/admin/users', icon: <TeamOutlined />, show: can(me, 'system.user.read') },
    { title: '审计日志', desc: '查看关键管理和安全操作', path: '/admin/audit-logs', icon: <AuditOutlined />, show: can(me, 'system.audit.read') },
    { title: '安全基线', desc: '安全策略与合规控制', path: '/security', icon: <SafetyCertificateOutlined />, show: true },
    { title: '系统状态', desc: '服务状态与健康检查', path: '/ops/system', icon: <CloudServerOutlined />, show: true },
  ].filter((item) => item.show)

  return (
    <AppPageContainer title="工作台" subTitle="系统运行概览">
      {ready.data?.status === 'DOWN' && (
        <Alert
          type="error"
          showIcon
          message="部分基础设施不可用"
          description="请进入系统状态页，并结合 Prometheus、Trace 和集中日志进行定位。"
          style={{ marginBottom: 16 }}
        />
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}>
          <StatisticCard statistic={{ title: '运行状态', value: ready.data?.status || 'UNKNOWN', icon: <SafetyCertificateOutlined /> }} />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <StatisticCard statistic={{ title: '数据库', value: info.data?.providers.database || '-', icon: <DatabaseOutlined /> }} />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <StatisticCard statistic={{ title: '缓存', value: info.data?.providers.cache || '-', icon: <CloudServerOutlined /> }} />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <StatisticCard statistic={{ title: '应用版本', value: info.data?.version || '-', icon: <SafetyCertificateOutlined /> }} />
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} xl={15}>
          <Card title="基础设施健康">
            <List
              dataSource={ready.data?.checks || []}
              renderItem={(item) => (
                <List.Item
                  extra={<Space><StatusTag value={item.status} /><Typography.Text type="secondary">{item.duration_ms} ms</Typography.Text></Space>}
                >
                  <List.Item.Meta
                    title={item.name}
                    description={item.provider ? `服务：${item.provider}` : '服务信息在生产环境可能被隐藏'}
                  />
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} xl={9}>
          <Card title="快速入口">
            <List
              dataSource={quick}
              renderItem={(item) => (
                <List.Item className="quick-entry" onClick={() => navigate(item.path)}>
                  <List.Item.Meta avatar={item.icon} title={item.title} description={item.desc} />
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>

      <Card title="当前运行配置" style={{ marginTop: 16 }}>
        <Space wrap>
          <Tag color="blue">{info.data?.environment || '-'}</Tag>
          <Tag>{info.data?.compliance_profile || 'default'}</Tag>
          <Tag>数据库 · {info.data?.providers.database || '-'}</Tag>
          <Tag>缓存 · {info.data?.providers.cache || '-'}</Tag>
          <Tag>消息 · {info.data?.providers.messaging || '-'}</Tag>
          <Tag>检索 · {info.data?.providers.search || '-'}</Tag>
          <Tag>对象存储 · {info.data?.providers.storage || '-'}</Tag>
        </Space>
      </Card>
    </AppPageContainer>
  )
}
