import { Button, Result } from 'antd'
export function SystemErrorPage() {
  return <Result status="500" title="系统异常" subTitle="请求未能正常完成，请稍后重试。" extra={<Button type="primary" onClick={() => window.location.reload()}>刷新页面</Button>} />
}
