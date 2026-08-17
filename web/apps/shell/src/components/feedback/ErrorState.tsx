import { Button, Result, Space, Typography } from 'antd'
import { ApiError } from '@forge/api-client'

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const apiError = error instanceof ApiError ? error : undefined
  return (
    <Result
      status="error"
      title="页面加载失败"
      subTitle={apiError?.message || '系统暂时无法完成请求，请稍后重试。'}
      extra={
        <Space direction="vertical">
          {onRetry && <Button type="primary" onClick={onRetry}>重新加载</Button>}
          {apiError?.requestId && <Typography.Text type="secondary" copyable>Request ID: {apiError.requestId}</Typography.Text>}
          {apiError?.traceId && <Typography.Text type="secondary" copyable>Trace ID: {apiError.traceId}</Typography.Text>}
        </Space>
      }
    />
  )
}
