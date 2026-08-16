import {
  LockOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { LoginForm, ProFormText } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Divider, Space, Typography } from 'antd'
import { useQueryClient } from '@tanstack/react-query'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api/api'
import { ApiError } from '../api/client'
import { runtimeConfig } from '../app/config/runtime'
import { BrandMark } from '../layout/BrandMark'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const { message } = App.useApp()

  return (
    <main className="login-shell">
      <section className="login-visual">
        <div className="login-brand">
          <BrandMark />
          <Typography.Title level={2}>{runtimeConfig.appName}</Typography.Title>
        </div>
        <Typography.Title className="login-hero-title">面向企业生产环境的统一应用基座</Typography.Title>
        <Typography.Paragraph className="login-hero-copy">
          React + Ant Design + Go，默认纳入认证授权、审计、可观测、配置治理、中间件 Provider、容器化与信创扩展边界。
        </Typography.Paragraph>
        <Space direction="vertical" size={12} className="login-capabilities">
          <span><SafetyCertificateOutlined /> 安全基线与三员分立</span>
          <span><SafetyCertificateOutlined /> 统一 API / 日志 / Trace / Metrics</span>
          <span><SafetyCertificateOutlined /> Docker / Kubernetes / 信创 Profile</span>
        </Space>
        <div className="login-env">Environment · {runtimeConfig.environment}</div>
      </section>

      <section className="login-form-side">
        <Card className="login-card" bordered={false}>
          <div className="login-card-heading">
            <Typography.Title level={3}>欢迎登录</Typography.Title>
            <Typography.Text type="secondary">{runtimeConfig.description}</Typography.Text>
          </div>

          {runtimeConfig.localLoginEnabled ? (
            <LoginForm
              submitter={{ searchConfig: { submitText: '登录' }, submitButtonProps: { size: 'large', block: true } }}
              onFinish={async (values) => {
                try {
                  await api.login({
                    organization: values.organization || runtimeConfig.defaultOrganization,
                    login_name: values.login_name,
                    password: values.password,
                  })
                  await queryClient.invalidateQueries({ queryKey: ['me'] })
                  const from = (location.state as { from?: string } | null)?.from || '/dashboard'
                  navigate(from, { replace: true })
                  return true
                } catch (error) {
                  if (error instanceof ApiError) {
                    message.error(`${error.message}${error.requestId ? ` · Request ID ${error.requestId}` : ''}`)
                  } else {
                    message.error('登录失败')
                  }
                  return false
                }
              }}
            >
              {runtimeConfig.showOrganizationField && (
                <ProFormText name="organization" initialValue={runtimeConfig.defaultOrganization} label="组织" placeholder={runtimeConfig.defaultOrganization} />
              )}
              <ProFormText name="login_name" fieldProps={{ prefix: <UserOutlined /> }} placeholder="用户名" rules={[{ required: true, message: '请输入用户名' }]} />
              <ProFormText.Password name="password" fieldProps={{ prefix: <LockOutlined /> }} placeholder="密码" rules={[{ required: true, message: '请输入密码' }]} />
            </LoginForm>
          ) : (
            <Alert type="info" showIcon message="本地账号密码登录已关闭" />
          )}

          {runtimeConfig.oidcLoginUrl && (
            <>
              <Divider plain>企业统一认证</Divider>
              <Button size="large" block onClick={() => { window.location.href = runtimeConfig.oidcLoginUrl }}>
                使用 SSO 登录
              </Button>
            </>
          )}

          <Typography.Paragraph type="secondary" className="login-security-tip">
            初始管理员通过部署 Secret 注入。生产环境禁止在镜像、前端配置或 YAML 中保存默认口令。
          </Typography.Paragraph>
        </Card>
      </section>
    </main>
  )
}
