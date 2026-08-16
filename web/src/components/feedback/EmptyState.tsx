import { Empty, type EmptyProps } from 'antd'

export function EmptyState({ description = '暂无数据', ...props }: EmptyProps) {
  return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={description} {...props} />
}
