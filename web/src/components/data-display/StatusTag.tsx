import { Tag } from 'antd'

const tones: Record<string, string> = {
  ACTIVE: 'success',
  ENABLED: 'success',
  SUCCESS: 'success',
  UP: 'success',
  RUNNING: 'processing',
  PROCESSING: 'processing',
  PENDING: 'warning',
  WARN: 'warning',
  WARNING: 'warning',
  DISABLED: 'default',
  ARCHIVED: 'default',
  DOWN: 'error',
  FAILED: 'error',
  ERROR: 'error',
  LOCKED: 'error',
}

export function StatusTag({ value }: { value?: string }) {
  const text = value || 'UNKNOWN'
  return <Tag color={tones[text.toUpperCase()] || 'blue'}>{text}</Tag>
}
