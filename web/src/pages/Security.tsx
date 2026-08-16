import { CheckCircleOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { Alert, Card, Col, Descriptions, Row, Space, Tag, Typography } from 'antd'
import { AppPageContainer } from '../components/layout/AppPageContainer'

const groups = [
  {
    title: '身份与访问控制',
    items: [
      ['认证', 'Cookie Session / Bearer API Token / OIDC 扩展位'],
      ['密码', 'Argon2id、复杂度、历史密码、有效期、首次登录强制改密'],
      ['授权', 'RBAC Permission，system_admin / security_admin / auditor 分立'],
      ['会话', '共享数据库会话、在线会话、强制下线、改密撤销其他会话'],
    ],
  },
  {
    title: '应用安全',
    items: [
      ['浏览器安全', 'CSRF / CSP / HSTS / X-Frame-Options / CORS Allowlist'],
      ['接口保护', 'Body Limit / Rate Limit / 统一错误码 / Request ID / Trace ID'],
      ['数据保护', '敏感日志禁止、脱敏组件、Secret Provider、TLS/mTLS Provider'],
      ['密码算法', 'Standard: SHA-256 + AES-GCM；GM Profile: SM3 + SM4-GCM；SM2/HSM/KMS 扩展位'],
    ],
  },
  {
    title: '审计与供应链',
    items: [
      ['审计', '关键安全管理操作写入数据库审计日志并保持组织边界'],
      ['可观测', 'slog / Prometheus / OpenTelemetry / readiness / liveness'],
      ['供应链', 'SAST / SCA / Secret Scan / SBOM / Image Scan / Provenance / Signature'],
      ['合规边界', '框架提供应用层工程基线，不等同于自动通过等保、密评或银行内部审查'],
    ],
  },
]

export function SecurityPage() {
  return (
    <AppPageContainer title="安全基线" subTitle="面向企业与金融场景的应用层安全能力清单。">
      <Alert
        type="info"
        showIcon
        message="合规是系统工程"
        description="等级保护、密码应用、数据安全、网络与主机安全、运维制度仍需结合实际部署环境、数据级别和内部制度完成评估。"
        style={{ marginBottom: 16 }}
      />
      <Row gutter={[16, 16]}>
        {groups.map((group) => (
          <Col xs={24} xl={8} key={group.title}>
            <Card title={<Space><SafetyCertificateOutlined />{group.title}</Space>} className="baseline-card">
              <Descriptions column={1} size="small">
                {group.items.map(([label, value]) => (
                  <Descriptions.Item key={label} label={label}>
                    <Typography.Text>{value}</Typography.Text>
                  </Descriptions.Item>
                ))}
              </Descriptions>
              <Tag color="success" icon={<CheckCircleOutlined />}>Foundation Baseline</Tag>
            </Card>
          </Col>
        ))}
      </Row>
    </AppPageContainer>
  )
}
