import { ApartmentOutlined } from '@ant-design/icons'
import { Card, Descriptions, Result, Skeleton, Tag } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/api'
import { queryKeys } from '../api/queryKeys'
import { AppPageContainer } from '../components/layout/AppPageContainer'
import { ErrorState } from '../components/feedback/ErrorState'
import { CopyText } from '../components/data-display/CopyText'

export function OrganizationPage() {
  const query = useQuery({ queryKey: queryKeys.organization, queryFn: api.organization })
  if (query.isError) return <AppPageContainer title="组织信息"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>
  return (
    <AppPageContainer title="组织信息" subTitle="当前登录主体所属组织；多租户业务建议在此基础上继续扩展组织级配额、数据域和授权边界。">
      <Card>
        {query.isLoading ? <Skeleton /> : query.data ? (
          <Descriptions column={{ xs: 1, md: 2 }} bordered>
            <Descriptions.Item label="组织名称"><ApartmentOutlined /> {query.data.name}</Descriptions.Item>
            <Descriptions.Item label="组织标识"><Tag color="blue">{query.data.org_key}</Tag></Descriptions.Item>
            <Descriptions.Item label="组织 ID"><CopyText value={query.data.id} /></Descriptions.Item>
            <Descriptions.Item label="创建时间">{new Date(query.data.created_at).toLocaleString()}</Descriptions.Item>
          </Descriptions>
        ) : <Result status="warning" title="未读取到组织信息" />}
      </Card>
    </AppPageContainer>
  )
}
