import { Typography } from 'antd'
export function CopyText({ value, ellipsis = true }: { value?: string; ellipsis?: boolean }) {
  if (!value) return <>-</>
  return <Typography.Text copyable ellipsis={ellipsis ? { tooltip: value } : false}>{value}</Typography.Text>
}
