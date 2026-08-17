import { LogoutOutlined } from '@ant-design/icons'
import type { ProColumns } from '@ant-design/pro-components'
import { App, Button, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import type { SessionInfo } from '@forge/api-client'
import { AppPageContainer } from '@forge/design-system'
import { ErrorState } from '@forge/design-system'
import { AppProTable } from '@forge/design-system'
import { ConfirmAction } from '@forge/design-system'
import { SensitiveText } from '@forge/design-system'

export function SessionsPage() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const query = useQuery({ queryKey: queryKeys.sessions, queryFn: api.sessions, refetchInterval: 30_000 })
  const revoke = useMutation({
    mutationFn: api.revokeSession,
    onSuccess: async () => {
      message.success('会话已强制下线')
      await qc.invalidateQueries({ queryKey: queryKeys.sessions })
    },
  })

  const columns: ProColumns<SessionInfo>[] = [
    { title: '用户', dataIndex: 'display_name', render: (_, row) => <Space>{row.display_name || row.login_name}{row.current && <Tag color="processing">当前会话</Tag>}</Space> },
    { title: '登录名', dataIndex: 'login_name' },
    { title: '客户端 IP', dataIndex: 'client_ip', render: (_, row) => <SensitiveText value={row.client_ip} /> },
    { title: '创建时间', dataIndex: 'created_at', valueType: 'dateTime' },
    { title: '最近活动', dataIndex: 'last_seen_at', valueType: 'dateTime' },
    { title: '过期时间', dataIndex: 'expires_at', valueType: 'dateTime' },
    { title: 'User Agent', dataIndex: 'user_agent', ellipsis: true, render: (_, row) => <Typography.Text ellipsis={{ tooltip: row.user_agent }}>{row.user_agent || '-'}</Typography.Text> },
    {
      title: '操作',
      valueType: 'option',
      render: (_, row) => row.current ? <Tag>不可下线当前会话</Tag> : (
        <ConfirmAction title="确认强制下线该会话？" description="目标浏览器的后续请求将立即失效。" danger onConfirm={() => revoke.mutateAsync(row.id)}>
          <Button danger type="link" icon={<LogoutOutlined />}>强制下线</Button>
        </ConfirmAction>
      ),
    },
  ]

  if (query.isError) return <AppPageContainer title="在线会话"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>

  return (
    <AppPageContainer title="在线会话" subTitle="用于安全运营与账号处置；多副本部署下会话状态仍以共享数据库为准。">
      <AppProTable<SessionInfo> rowKey="id" columns={columns} dataSource={query.data?.items || []} loading={query.isLoading} search={false} />
    </AppPageContainer>
  )
}
