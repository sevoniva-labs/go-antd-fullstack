import { CheckCircleOutlined, EditOutlined, StopOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import { App, Button, Card, Descriptions, Spin, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import { can } from '@forge/auth-sdk'
import { useMe } from '@forge/auth-sdk'
import { AppPageContainer } from '../components/layout/AppPageContainer'
import { ErrorState } from '../components/feedback/ErrorState'
import { CopyText } from '../components/data-display/CopyText'

export function OrganizationPage() {
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const me = useMe().data
  const canManage = can(me, 'system.organization.manage')
  const query = useQuery({ queryKey: queryKeys.organization, queryFn: api.organization })

  const update = useMutation({
    mutationFn: api.updateOrganization,
    onSuccess: async () => {
      message.success('组织信息已更新')
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization })
    },
  })

  if (query.isError) return <AppPageContainer title="组织信息"><ErrorState error={query.error} onRetry={() => void query.refetch()} /></AppPageContainer>
  if (query.isLoading) return <AppPageContainer title="组织信息"><Spin tip="加载组织信息..." /></AppPageContainer>
  if (!query.data) return <AppPageContainer title="组织信息" />

  const status = query.data.status?.toUpperCase() === 'ACTIVE'

  return (
    <AppPageContainer title="组织信息" subTitle="组织级参数（状态、配额、会话并发）可直接编辑，影响该组织的登录与账号策略边界。">
      <Card
        title="组织档案"
        extra={
          canManage ? (
            <ModalForm
              title="编辑组织信息"
              trigger={<Button type="primary" icon={<EditOutlined />}>编辑组织</Button>}
              initialValues={query.data}
              onFinish={async (values) => {
                await update.mutateAsync({
                  name: values.name || query.data.name,
                  description: values.description || '',
                  status: values.status || query.data.status,
                  max_users: Number(values.max_users ?? query.data.max_users),
                  max_active_sessions: Number(values.max_active_sessions ?? query.data.max_active_sessions),
                })
                return true
              }}
            >
              <ProFormText name="name" label="组织名称" rules={[{ required: true }]}
                placeholder="组织正式名称" />
              <ProFormTextArea name="description" label="组织描述" placeholder="可填写组织边界与用途" autoSize={{ minRows: 2, maxRows: 4 }} />
              <ProFormSelect
                name="status"
                label="组织状态"
                options={[{ value: 'ACTIVE', label: 'ACTIVE' }, { value: 'DISABLED', label: 'DISABLED' }]}
                rules={[{ required: true }]}
              />
              <ProFormDigit
                name="max_users"
                label="最大用户数"
                min={0}
                tooltip="0 表示不限制"
                fieldProps={{ precision: 0 }}
                rules={[{ required: true, min: 0 }]}
              />
              <ProFormDigit
                name="max_active_sessions"
                label="最大并发会话数"
                min={0}
                tooltip="0 表示不限制"
                fieldProps={{ precision: 0 }}
                rules={[{ required: true, min: 0 }]}
              />
            </ModalForm>
          ) : null
        }
      >
        <Descriptions column={{ xs: 1, md: 2 }} bordered>
          <Descriptions.Item label="组织 ID"><CopyText value={query.data.id} /></Descriptions.Item>
          <Descriptions.Item label="组织标识">{query.data.org_key}</Descriptions.Item>
          <Descriptions.Item label="组织名称">{query.data.name}</Descriptions.Item>
          <Descriptions.Item label="组织状态">
            <Tag color={status ? 'success' : 'error'}>{query.data.status}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="组织描述">{query.data.description || '-'}</Descriptions.Item>
          <Descriptions.Item label="用户配额">
            {query.data.max_users === 0 ? <Typography.Text type="secondary">不限制</Typography.Text> : query.data.max_users}
          </Descriptions.Item>
          <Descriptions.Item label="会话并发">
            {query.data.max_active_sessions === 0 ? (
              <Typography.Text type="secondary">不限制</Typography.Text>
            ) : (
              <Tag color="blue">{query.data.max_active_sessions}</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">{new Date(query.data.created_at).toLocaleString()}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{new Date(query.data.updated_at).toLocaleString()}</Descriptions.Item>
        </Descriptions>
      </Card>
      <Card title="组织状态说明" style={{ marginTop: 16 }}>
        <Typography.Paragraph>
          <Tag color={status ? 'success' : 'error'} icon={status ? <CheckCircleOutlined /> : <StopOutlined />}>
            {status ? '启用' : '停用'}
          </Tag>{' '}
          {!status && (
            <><StopOutlined /> 当前组织已停用，登录与管理操作将按组织状态拦截。</>
          )}
          {status && <><CheckCircleOutlined /> 当前组织状态正常，组织内账号可通过口令策略与组织级配置进行管理。</>}
        </Typography.Paragraph>
      </Card>
    </AppPageContainer>
  )
}
