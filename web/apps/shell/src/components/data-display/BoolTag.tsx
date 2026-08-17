import { Tag } from 'antd'
export function BoolTag({ value, trueText = '是', falseText = '否' }: { value?: boolean; trueText?: string; falseText?: string }) {
  return <Tag color={value ? 'success' : 'default'}>{value ? trueText : falseText}</Tag>
}
