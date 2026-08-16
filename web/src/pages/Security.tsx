import { EditOutlined } from '@ant-design/icons'
import { ModalForm, ProFormCheckbox, ProFormDigit } from '@ant-design/pro-components'
import { Alert, App, Button, Card, Col, Descriptions, Row, Space, Spin, Tag, Typography } from 'antd'
import { useMutation, useQuery } from '@tanstack/react-query'
import type { SecurityPolicy } from '../api/types'
import { api } from '../api/api'
import { queryKeys } from '../api/queryKeys'
import { can } from '../auth/access'
import { useMe } from '../auth/useMe'
import { AppPageContainer } from '../components/layout/AppPageContainer'
import { ErrorState } from '../components/feedback/ErrorState'

function policyRows(policy: SecurityPolicy) {
  return [
    ['密码最小长度', `${policy.password_min_length} 位`],
    ['口令历史长度', `${policy.password_history} 条`],
    ['口令有效期', `${policy.password_max_age_days} 天`],
    ['登录失败阈值', `${policy.login_max_failures} 次`],
    ['锁定时长', `${policy.login_lock_duration_seconds} 秒`],
    ['会话 TTL', `${policy.session_ttl_seconds} 秒`],
    ['并发会话', policy.max_active_sessions === 0 ? '不限制' : `${policy.max_active_sessions} 个`],
  ]
}

export function SecurityPage() {
  const { message } = App.useApp()
  const me = useMe().data
  const canRead = can(me, 'system.config.read')
  const canManage = can(me, 'system.security.manage')

  const policy = useQuery({ queryKey: queryKeys.securityConfig, queryFn: api.securityConfig, enabled: canRead })
  const update = useMutation({
    mutationFn: api.updateSecurityConfig,
    onSuccess: async () => {
      message.success('安全策略已保存')
      await policy.refetch()
    },
  })

  if (!canRead) {
    return (
      <AppPageContainer title="安全基线" subTitle="当前账号无组织安全配置读取权限。">
        <Alert
          type="warning"
          showIcon
          message="请联系安全管理员"
          description="system.config.read 或 system.security.manage 权限才能查看或修改组织安全策略。"
          style={{ marginBottom: 16 }}
        />
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={16}>
            <Card title="安全提示">
              <Typography.Paragraph>
                银行级场景建议结合统一身份管理、MFA、密评、WAF、DLP、SOC 等制度能力，不应仅依赖应用层参数。
              </Typography.Paragraph>
            </Card>
          </Col>
        </Row>
      </AppPageContainer>
    )
  }

  if (policy.isLoading) return <AppPageContainer title="安全基线"><Spin tip="加载安全策略..." /></AppPageContainer>
  if (policy.isError) return <AppPageContainer title="安全基线"><ErrorState error={policy.error} onRetry={() => void policy.refetch()} /></AppPageContainer>
  if (!policy.data) return <AppPageContainer title="安全基线" />

  return (
    <AppPageContainer title="安全基线" subTitle="组织级安全策略（密码、会话、失败锁定）可在线读取与配置。">
      <Alert
        type="info"
        showIcon
        message="安全策略生效范围"
        description="当前策略为组织级参数，修改后影响本组织新登录、改密、会话与审计行为。"
        style={{ marginBottom: 16 }}
      />
      <Row gutter={[16, 16]}>
        <Col xs={24} xl={16}>
          <Card
            title="组织安全策略"
            extra={
              canManage ? (
                <ModalForm
                  title="更新组织安全策略"
                  trigger={<Button type="primary" icon={<EditOutlined />}>编辑策略</Button>}
                  initialValues={policy.data}
                  onFinish={async (values: Record<string, any>) => {
                    await update.mutateAsync({
                      password_min_length: Number(values.password_min_length ?? policy.data.password_min_length),
                      password_require_upper: Boolean(values.password_require_upper),
                      password_require_lower: Boolean(values.password_require_lower),
                      password_require_digit: Boolean(values.password_require_digit),
                      password_require_symbol: Boolean(values.password_require_symbol),
                      password_history: Number(values.password_history ?? policy.data.password_history),
                      password_max_age_days: Number(values.password_max_age_days ?? policy.data.password_max_age_days),
                      login_max_failures: Number(values.login_max_failures ?? policy.data.login_max_failures),
                      login_lock_duration_seconds: Number(values.login_lock_duration_seconds ?? policy.data.login_lock_duration_seconds),
                      session_ttl_seconds: Number(values.session_ttl_seconds ?? policy.data.session_ttl_seconds),
                      max_active_sessions: Number(values.max_active_sessions ?? policy.data.max_active_sessions),
                    })
                    return true
                  }}
                >
                  <ProFormDigit name="password_min_length" label="最小长度" min={1} max={64} rules={[{ required: true }]} />
                  <ProFormCheckbox name="password_require_upper" label="必须包含大写" />
                  <ProFormCheckbox name="password_require_lower" label="必须包含小写" />
                  <ProFormCheckbox name="password_require_digit" label="必须包含数字" />
                  <ProFormCheckbox name="password_require_symbol" label="必须包含符号" />
                  <ProFormDigit name="password_history" label="历史密码保留数" min={0} rules={[{ required: true }]} />
                  <ProFormDigit name="password_max_age_days" label="口令有效期（天）" min={0} rules={[{ required: true }]} />
                  <ProFormDigit name="login_max_failures" label="登录失败锁定阈值" min={0} rules={[{ required: true }]} />
                  <ProFormDigit name="login_lock_duration_seconds" label="锁定时长（秒）" min={0} rules={[{ required: true }]} />
                  <ProFormDigit name="session_ttl_seconds" label="会话 TTL（秒）" min={1} rules={[{ required: true }]} />
                  <ProFormDigit name="max_active_sessions" label="并发会话数（0=不限制）" min={0} rules={[{ required: true }]} />
                </ModalForm>
              ) : null
            }
          >
            <Descriptions column={{ xs: 1, md: 2 }}>
              <Descriptions.Item label="最小长度">{policy.data.password_min_length}</Descriptions.Item>
              <Descriptions.Item label="密码历史">{policy.data.password_history}</Descriptions.Item>
              <Descriptions.Item label="口令有效期">{policy.data.password_max_age_days} 天</Descriptions.Item>
              <Descriptions.Item label="登录失败阈值">{policy.data.login_max_failures} 次</Descriptions.Item>
              <Descriptions.Item label="锁定时长">{policy.data.login_lock_duration_seconds} 秒</Descriptions.Item>
              <Descriptions.Item label="会话 TTL">{policy.data.session_ttl_seconds} 秒</Descriptions.Item>
              <Descriptions.Item label="并发会话">{policy.data.max_active_sessions === 0 ? '不限制' : policy.data.max_active_sessions}</Descriptions.Item>
              <Descriptions.Item label="密码规则">
                <Space size={[6, 6]} wrap>
                  {policy.data.password_require_upper ? <Tag>大写</Tag> : null}
                  {policy.data.password_require_lower ? <Tag>小写</Tag> : null}
                  {policy.data.password_require_digit ? <Tag>数字</Tag> : null}
                  {policy.data.password_require_symbol ? <Tag>符号</Tag> : null}
                </Space>
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
        <Col xs={24} xl={8}>
          <Card title="策略说明">
            <Typography.Paragraph>
              更新密码复杂度会影响已有会话与变更路径，建议与组织密码更换周期、运维窗口一起发布。
            </Typography.Paragraph>
            {policyRows(policy.data).map(([label, value]) => (
              <div key={label}>
                <strong>{label}：</strong> {value}
              </div>
            ))}
          </Card>
        </Col>
      </Row>
    </AppPageContainer>
  )
}
