import dayjs from 'dayjs'
import { Typography } from 'antd'

export function DateTimeText({ value, format = 'YYYY-MM-DD HH:mm:ss' }: { value?: string | number | Date; format?: string }) {
  if (!value) return <>-</>
  const time = dayjs(value)
  if (!time.isValid()) return <Typography.Text type="secondary">-</Typography.Text>
  return <Typography.Text title={time.toISOString()}>{time.format(format)}</Typography.Text>
}
