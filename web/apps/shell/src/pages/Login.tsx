import {
  LockOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { LoginForm, ProFormText } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Divider, Space, Typography } from 'antd'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '@forge/api-client'
import { ApiError } from '@forge/api-client'
import { runtimeConfig } from '../app/config/runtime'
import { BrandMark } from '../layout/BrandMark'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const [mfaRequired, setMfaRequired] = useState(false)

  return (
    <main className="login-shell">
      <section className="login-visual">
        <div className="login-brand">
          <BrandMark />
          <Typography.Title level={2}>{runtimeConfig.appName}</Typography.Title>
        </div>
        <Typography.Title className="login-hero-title">统一业务管理平台</Typography.Title>
        <Typography.Paragraph className="login-hero-copy">
          提供统一认证、权限管理、操作审计与系统治理能力。
        </Typography.Paragraph>
        <Space direction="vertical" size={12} className="login-capabilities">
          <span><SafetyCertificateOutlined /> 认证与权限管理</span>
          <span><SafetyCertificateOutlined /> 操作审计与安全控制</span>
          <span><SafetyCertificateOutlined /> 容器化部署与国产化适配</span>
        </Space>
      </section>

      <section className="login-form-side">
        <Card className="login-card" bordered={false}>
          <div className="login-card-heading">
            <Typography.Title level={3}>登录</Typography.Title>
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
                    mfa_code: values.mfa_code,
                    recovery_code: values.recovery_code,
                  })
                  await queryClient.invalidateQueries({ queryKey: ['me'] })
                  const from = (location.state as { from?: string } | null)?.from || '/dashboard'
                  navigate(from, { replace: true })
                  return true
                } catch (error) {
                  if (error instanceof ApiError) {
                    if (error.errorCode === 'MFA_REQUIRED') {
                      setMfaRequired(true)
                      message.info('请输入动态验证码或恢复码')
                      return false
                    }
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
              {mfaRequired && <Alert type="info" showIcon message="需要多因素认证" description="输入认证器中的动态验证码；无法使用认证器时，可改用一枚未使用的恢复码。" style={{ marginBottom: 20 }} />}
              {mfaRequired && <ProFormText name="mfa_code" label="动态验证码" fieldProps={{ prefix: <SafetyCertificateOutlined />, inputMode: 'numeric', autoComplete: 'one-time-code' }} placeholder="6 位动态验证码" />}
              {mfaRequired && <ProFormText.Password name="recovery_code" label="恢复码（可选）" fieldProps={{ autoComplete: 'one-time-code' }} placeholder="与动态验证码二选一" />}
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

        </Card>
      </section>
    </main>
  )
}
