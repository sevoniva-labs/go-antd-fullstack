import type { ProColumns } from '@ant-design/pro-components'
import type { Permission } from '../api/types'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/api'
import { queryKeys } from '../api/queryKeys'
import { AppPageContainer } from '../components/layout/AppPageContainer'
import { ErrorState } from '../components/feedback/ErrorState'
import { AppProTable } from '../components/table/AppProTable'
import { CopyText } from '../components/data-display/CopyText'

export function PermissionsPage() {
  const query = useQuery({ queryKey: queryKeys.permissions, queryFn: api.permissions })
  const columns: ProColumns<Permission>[] = [
    { title: '权限标识', dataIndex: 'permission_key', render: (_, row) => <CopyText value={row.permission_key} /> },
    { title: '名称', dataIndex: 'name' },
    { title: '创建时间', dataIndex: 'created_at', valueType: 'dateTime', search: false },
  ]
  if (query.isError) return <AppPageContainer title="权限清单"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>

  return (
    <AppPageContainer title="权限清单" subTitle="权限由代码与 API 契约定义，前端只负责展示和授权组合，避免产生无法被后端识别的“幽灵权限”。">
      <AppProTable<Permission> rowKey="id" columns={columns} dataSource={query.data?.items || []} loading={query.isLoading} search={false} />
    </AppPageContainer>
  )
}
