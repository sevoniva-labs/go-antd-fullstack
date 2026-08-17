import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText } from '@ant-design/pro-components'
import type { ProColumns } from '@ant-design/pro-components'
import { Alert, App, Button, Modal, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import type { ApiToken } from '@forge/api-client'
import { useMe } from '@forge/auth-sdk'
import { AppPageContainer } from '../components/layout/AppPageContainer'
import { ErrorState } from '../components/feedback/ErrorState'
import { AppProTable } from '../components/table/AppProTable'
import { ConfirmAction } from '../components/security/ConfirmAction'

export function ApiTokensPage() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const me = useMe().data
  const list = useQuery({ queryKey: queryKeys.apiTokens, queryFn: api.apiTokens })
  const revoke = useMutation({
    mutationFn: api.revokeApiToken,
    onSuccess: async () => {
      message.success('Token 已撤销')
      await qc.invalidateQueries({ queryKey: queryKeys.apiTokens })
    },
  })

  const scopeOptions = me?.roles.includes('system_admin')
    ? [{ label: '全部权限 (*)', value: '*' }]
    : (me?.permissions || []).map((item) => ({ label: item, value: item }))

  const columns: ProColumns<ApiToken>[] = [
    { title: '名称', dataIndex: 'name' },
    { title: '前缀', dataIndex: 'prefix', copyable: true },
    { title: 'Scopes', dataIndex: 'scopes', render: (_, row) => <Space wrap>{row.scopes.map((x) => <Tag key={x}>{x}</Tag>)}</Space> },
    { title: '过期时间', dataIndex: 'expires_at', valueType: 'dateTime' },
    { title: '最近使用', dataIndex: 'last_used_at', valueType: 'dateTime' },
    {
      title: '操作',
      valueType: 'option',
      render: (_, row) => (
        <ConfirmAction title="确认撤销此 Token？" danger onConfirm={() => revoke.mutateAsync(row.id)}>
          <Button danger type="link" icon={<DeleteOutlined />}>撤销</Button>
        </ConfirmAction>
      ),
    },
  ]

  if (list.isError) return <AppPageContainer title="API Token"><ErrorState error={list.error} onRetry={() => void list.refetch()} /></AppPageContainer>

  return (
    <AppPageContainer title="API Token" subTitle="用于 CI/CD、服务间调用和自动化。Token 原文只在创建成功时返回一次。">
      <Alert
        type="info"
        showIcon
        message="机器身份与浏览器身份分离"
        description="API Token 使用 Bearer 认证，不复用浏览器 Session/CSRF；最终权限同时受 Token scope 与账号 RBAC 约束。"
        style={{ marginBottom: 16 }}
      />
      <AppProTable<ApiToken>
        rowKey="id"
        search={false}
        columns={columns}
        dataSource={list.data?.items || []}
        loading={list.isLoading}
        toolBarRender={() => [
          <ModalForm
            key="new"
            title="创建 API Token"
            trigger={<Button type="primary" icon={<PlusOutlined />}>创建 Token</Button>}
            onFinish={async (values) => {
              const result = await api.createApiToken({
                name: values.name,
                scopes: values.scopes,
                expires_days: values.expires_days || 90,
              })
              await qc.invalidateQueries({ queryKey: queryKeys.apiTokens })
              Modal.warning({
                title: '请立即保存 Token',
                width: 680,
                content: (
                  <>
                    <Typography.Paragraph>该 Secret 不会再次显示，请保存到企业 Secret 管理系统。</Typography.Paragraph>
                    <Typography.Text copyable code>{result.secret}</Typography.Text>
                  </>
                ),
              })
              return true
            }}
          >
            <ProFormText name="name" label="名称" rules={[{ required: true }]} />
            <ProFormSelect name="scopes" label="Scopes" fieldProps={{ mode: 'multiple' }} options={scopeOptions} />
            <ProFormDigit name="expires_days" label="有效天数" initialValue={90} min={1} max={365} />
          </ModalForm>,
        ]}
      />
    </AppPageContainer>
  )
}
