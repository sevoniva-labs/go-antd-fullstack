import { ProForm, ProFormText } from '@ant-design/pro-components'
import { Alert, App, Card, Descriptions, Tag } from 'antd'
import { useQueryClient } from '@tanstack/react-query'
import { api } from '../api/api'
import { queryKeys } from '../api/queryKeys'
import { useMe } from '../auth/useMe'
import { AppPageContainer } from '../components/layout/AppPageContainer'

export function AccountSecurityPage() {
  const { message } = App.useApp()
  const me = useMe().data
  const queryClient = useQueryClient()

  return (
    <AppPageContainer title="账号安全" subTitle="个人账号信息与密码策略。修改密码后，除当前会话外的其他会话将被撤销。">
      {me?.must_change_password && <Alert type="warning" showIcon message="当前账号必须修改初始密码后才能继续使用受保护功能。" style={{ marginBottom: 16 }} />}
      <Card title="账号信息" style={{ marginBottom: 16 }}>
        <Descriptions column={{ xs: 1, md: 2 }}>
          <Descriptions.Item label="登录名">{me?.login_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="显示名称">{me?.display_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="角色">{me?.roles.map((role) => <Tag key={role}>{role}</Tag>)}</Descriptions.Item>
          <Descriptions.Item label="主体类型">{me?.principal_type}</Descriptions.Item>
        </Descriptions>
      </Card>
      <Card title="修改密码" style={{ maxWidth: 760 }}>
        <ProForm
          submitter={{ searchConfig: { submitText: '修改密码' }, resetButtonProps: { style: { display: 'none' } } }}
          onFinish={async (values) => {
            await api.changePassword({ current_password: values.current_password, new_password: values.new_password })
            await queryClient.invalidateQueries({ queryKey: queryKeys.me })
            message.success('密码已修改，其他会话已失效')
            return true
          }}
        >
          <ProFormText.Password name="current_password" label="当前密码" rules={[{ required: true }]} />
          <ProFormText.Password
            name="new_password"
            label="新密码"
            extra="默认至少 12 位；实际复杂度以服务端安全配置为准。"
            rules={[{ required: true, min: 12 }]}
          />
          <ProFormText.Password
            name="confirm_password"
            label="确认新密码"
            dependencies={['new_password']}
            rules={[
              { required: true },
              ({ getFieldValue }) => ({
                validator: (_, value) => !value || value === getFieldValue('new_password')
                  ? Promise.resolve()
                  : Promise.reject(new Error('两次密码不一致')),
              }),
            ]}
          />
        </ProForm>
      </Card>
    </AppPageContainer>
  )
}
