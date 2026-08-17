import { CloudServerOutlined, DatabaseOutlined, DeploymentUnitOutlined, HddOutlined } from '@ant-design/icons'
import { StatisticCard } from '@ant-design/pro-components'
import { Alert, Card, Col, Descriptions, Row, Skeleton, Tag } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import { AppPageContainer } from '@forge/design-system'
import { StatusTag } from '@forge/design-system'
import { ErrorState } from '@forge/design-system'

export function SystemStatusPage() {
  const info = useQuery({ queryKey: queryKeys.systemInfo, queryFn: api.systemInfo, refetchInterval: 60_000 })
  const ready = useQuery({ queryKey: queryKeys.readiness, queryFn: api.readiness, refetchInterval: 30_000 })
  if (info.isLoading || ready.isLoading) return <AppPageContainer title="系统状态"><Skeleton /></AppPageContainer>
  if (info.isError || ready.isError) {
    return <AppPageContainer title="系统状态"><ErrorState error={info.error || ready.error} onRetry={() => { void info.refetch(); void ready.refetch() }} /></AppPageContainer>
  }

  return (
    <AppPageContainer title="系统状态" subTitle="应用运行时与依赖健康视图；生产环境后端会隐藏依赖错误详情。">
      {ready.data?.status === 'DOWN' && <Alert type="error" showIcon message="部分基础设施不可用，请结合 Prometheus / Trace / 集中日志进一步定位。" style={{ marginBottom: 16 }} />}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}><StatisticCard statistic={{ title: 'Readiness', value: ready.data?.status || 'UNKNOWN', icon: <DeploymentUnitOutlined /> }} /></Col>
        <Col xs={24} sm={12} xl={6}><StatisticCard statistic={{ title: 'Database', value: info.data?.providers.database || '-', icon: <DatabaseOutlined /> }} /></Col>
        <Col xs={24} sm={12} xl={6}><StatisticCard statistic={{ title: 'Cache', value: info.data?.providers.cache || '-', icon: <CloudServerOutlined /> }} /></Col>
        <Col xs={24} sm={12} xl={6}><StatisticCard statistic={{ title: 'Storage', value: info.data?.providers.storage || '-', icon: <HddOutlined /> }} /></Col>
      </Row>
      <Card title="基础设施健康" style={{ marginTop: 16 }}>
        <Descriptions column={{ xs: 1, md: 2, xl: 3 }} bordered>
          {(ready.data?.checks || []).map((check) => (
            <Descriptions.Item key={check.name} label={check.name}>
              <StatusTag value={check.status} /> {check.provider && <Tag>{check.provider}</Tag>} {check.duration_ms} ms
            </Descriptions.Item>
          ))}
        </Descriptions>
      </Card>
      <Card title="运行信息" style={{ marginTop: 16 }}>
        <Descriptions column={{ xs: 1, md: 2 }} bordered>
          <Descriptions.Item label="应用">{info.data?.application}</Descriptions.Item>
          <Descriptions.Item label="版本">{info.data?.version}</Descriptions.Item>
          <Descriptions.Item label="环境">{info.data?.environment}</Descriptions.Item>
          <Descriptions.Item label="合规 Profile">{info.data?.compliance_profile || '-'}</Descriptions.Item>
        </Descriptions>
      </Card>
    </AppPageContainer>
  )
}
