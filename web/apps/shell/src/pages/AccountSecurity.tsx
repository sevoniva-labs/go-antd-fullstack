import { LockOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { ModalForm, ProForm, ProFormText } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Descriptions, List, Modal, QRCode, Space, Tag, Typography } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '@forge/api-client'
import { queryKeys } from '@forge/api-client'
import { useMe } from '@forge/auth-sdk'
import { AppPageContainer } from '@forge/design-system'

export function AccountSecurityPage() {
  const { message } = App.useApp()
  const me = useMe().data
  const queryClient = useQueryClient()
  const [enrollment, setEnrollment] = useState<{ secret: string; provisioning_uri: string } | null>(null)
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const mfa = useQuery({ queryKey: queryKeys.mfa, queryFn: api.mfaStatus })

  const refreshMfa = () => queryClient.invalidateQueries({ queryKey: queryKeys.mfa })

  return (
    <AppPageContainer title="账号安全" subTitle="管理个人密码和多因素认证。敏感变更会撤销其他会话并写入审计日志。">
      {me?.must_change_password && <Alert type="warning" showIcon message="当前账号必须修改初始密码后才能继续使用受保护功能。" style={{ marginBottom: 16 }} />}
      <Card title="账号信息" style={{ marginBottom: 16 }}>
        <Descriptions column={{ xs: 1, md: 2 }}>
          <Descriptions.Item label="登录名">{me?.login_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="显示名称">{me?.display_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="角色">{me?.roles.map((role) => <Tag key={role}>{role}</Tag>)}</Descriptions.Item>
          <Descriptions.Item label="主体类型">{me?.principal_type}</Descriptions.Item>
          <Descriptions.Item label="会话认证强度"><Tag color={me?.authentication_level === 'MFA' ? 'success' : 'warning'}>{me?.authentication_level || 'PASSWORD'}</Tag></Descriptions.Item>
          <Descriptions.Item label="最近 MFA 验证">{me?.mfa_verified_at ? new Date(me.mfa_verified_at).toLocaleString('zh-CN') : '尚未验证'}</Descriptions.Item>
        </Descriptions>
      </Card>
      <Card
        title="多因素认证"
        style={{ maxWidth: 760, marginBottom: 16 }}
        extra={mfa.data?.enabled ? <Tag color="success">已启用</Tag> : <Tag>未启用</Tag>}
      >
        <Typography.Paragraph type="secondary">
          TOTP 秘钥在服务端使用部署配置的标准或国密算法加密。启用后，每次本地账号登录都必须提供动态验证码或一次性恢复码。
        </Typography.Paragraph>
        {mfa.data?.enabled && <Alert
          type={me?.authentication_level === 'MFA' ? 'success' : 'warning'}
          showIcon
          message={me?.authentication_level === 'MFA' ? '当前会话已通过多因素认证' : '特权写操作需要近期多因素认证'}
          description="授权、组织治理和其他高风险操作要求最近十分钟内完成 MFA；验证超时后需重新执行二次认证。"
          style={{ marginBottom: 16 }}
        />}
        {mfa.data?.enabled && <ModalForm<{ current_password: string; mfa_code?: string; recovery_code?: string }>
          title="会话二次认证"
          trigger={<Button icon={<SafetyCertificateOutlined />}>立即二次认证</Button>}
          submitter={{ searchConfig: { submitText: '验证当前会话' } }}
          onFinish={async (values) => {
            await api.stepUpAuthentication(values)
            await queryClient.invalidateQueries({ queryKey: queryKeys.me })
            message.success('当前会话已提升为近期 MFA 认证')
            return true
          }}
        >
          <ProFormText.Password name="current_password" label="当前密码" rules={[{ required: true }]} />
          <ProFormText name="mfa_code" label="动态验证码" fieldProps={{ inputMode: 'numeric', autoComplete: 'one-time-code' }} />
          <ProFormText.Password name="recovery_code" label="恢复码（可选）" />
        </ModalForm>}
        {!mfa.data?.enabled ? <ModalForm<{ current_password: string }>
          title="启用多因素认证"
          trigger={<Button type="primary" icon={<SafetyCertificateOutlined />}>启用 MFA</Button>}
          submitter={{ searchConfig: { submitText: '生成绑定信息' } }}
          onFinish={async (values) => {
            setEnrollment(await api.beginMfaEnrollment(values.current_password))
            return true
          }}
        >
          <Alert type="info" showIcon message="需要验证当前密码" description="绑定信息十分钟内有效，未确认前不会启用 MFA。" style={{ marginBottom: 16 }} />
          <ProFormText.Password name="current_password" label="当前密码" rules={[{ required: true }]} />
        </ModalForm> : <ModalForm<{ current_password: string; code?: string; recovery_code?: string }>
          title="停用多因素认证"
          trigger={<Button danger icon={<LockOutlined />}>停用 MFA</Button>}
          submitter={{ searchConfig: { submitText: '确认停用' } }}
          onFinish={async (values) => {
            await api.disableMfa(values)
            await refreshMfa()
            message.success('多因素认证已停用，其他会话已撤销')
            return true
          }}
        >
          <Alert type="warning" showIcon message="停用会降低账号保护强度" description="请输入当前密码，并提供动态验证码或恢复码。" style={{ marginBottom: 16 }} />
          <ProFormText.Password name="current_password" label="当前密码" rules={[{ required: true }]} />
          <ProFormText name="code" label="动态验证码" fieldProps={{ inputMode: 'numeric', autoComplete: 'one-time-code' }} />
          <ProFormText.Password name="recovery_code" label="恢复码（可选）" />
        </ModalForm>}
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
      <ModalForm<{ code: string }>
        open={Boolean(enrollment)}
        onOpenChange={(open) => { if (!open) setEnrollment(null) }}
        title="扫描并确认认证器"
        submitter={{ searchConfig: { submitText: '确认启用' } }}
        onFinish={async (values) => {
          const result = await api.confirmMfaEnrollment(values.code)
          setEnrollment(null)
          setRecoveryCodes(result.recovery_codes)
          await refreshMfa()
          message.success('多因素认证已启用')
          return true
        }}
      >
        {enrollment && <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <QRCode value={enrollment.provisioning_uri} />
          <Typography.Text copyable={{ text: enrollment.secret }}>无法扫码时输入秘钥：{enrollment.secret}</Typography.Text>
          <ProFormText name="code" label="动态验证码" fieldProps={{ inputMode: 'numeric', autoComplete: 'one-time-code' }} rules={[{ required: true }]} />
        </Space>}
      </ModalForm>
      <Modal
        open={recoveryCodes.length > 0}
        title="妥善保存一次性恢复码"
        closable={false}
        maskClosable={false}
        okText="我已安全保存"
        cancelButtonProps={{ style: { display: 'none' } }}
        onOk={() => setRecoveryCodes([])}
      >
        <Alert type="warning" showIcon message="这些恢复码仅展示一次，每枚只能使用一次。请存入组织批准的密码管理工具。" style={{ marginBottom: 16 }} />
        <List bordered dataSource={recoveryCodes} renderItem={(code) => <List.Item><Typography.Text code copyable>{code}</Typography.Text></List.Item>} />
      </Modal>
    </AppPageContainer>
  )
}
