import { Card, Statistic, type StatisticProps } from 'antd'
import type { ReactNode } from 'react'

export function MetricCard({ title, value, prefix, suffix, loading = false }: {
  title: ReactNode
  value?: StatisticProps['value']
  prefix?: ReactNode
  suffix?: ReactNode
  loading?: boolean
}) {
  return (
    <Card loading={loading} className="metric-card">
      <Statistic title={title} value={value ?? '-'} prefix={prefix} suffix={suffix} />
    </Card>
  )
}
