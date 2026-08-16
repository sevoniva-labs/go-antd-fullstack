import { Button, Result } from 'antd'
import React, { type ErrorInfo, type ReactNode } from 'react'
import { reportBrowserError } from '../../app/telemetry/browser'

interface State { hasError: boolean }

export class AppErrorBoundary extends React.Component<{ children: ReactNode }, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, _info: ErrorInfo) {
    reportBrowserError('react-boundary', error)
  }

  render() {
    if (!this.state.hasError) return this.props.children
    return (
      <Result
        status="500"
        title="页面发生异常"
        subTitle="请刷新页面；如果问题持续存在，请将 Request ID / Trace ID 提供给运维人员。"
        extra={<Button type="primary" onClick={() => window.location.reload()}>刷新页面</Button>}
      />
    )
  }
}
