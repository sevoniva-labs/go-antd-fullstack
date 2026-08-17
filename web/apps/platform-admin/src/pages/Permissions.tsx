import type { ProColumns } from '@ant-design/pro-components'
import type { Permission } from '@forge/api-client'
import { useQuery } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import { AppPageContainer } from '@forge/design-system'
import { ErrorState } from '@forge/design-system'
import { AppProTable } from '@forge/design-system'
import { CopyText } from '@forge/design-system'

export function PermissionsPage() {
  const query = useQuery({ queryKey: queryKeys.permissions, queryFn: api.permissions })
  const columns: ProColumns<Permission>[] = [
    { title: '权限标识', dataIndex: 'key', render: (_, row) => <CopyText value={row.key} /> },
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
