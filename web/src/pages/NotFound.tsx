import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'
export function NotFoundPage() {
  const navigate = useNavigate()
  return <Result status="404" title="404" subTitle="访问的页面不存在或已经被移除。" extra={<Button type="primary" onClick={() => navigate('/dashboard')}>返回工作台</Button>} />
}
