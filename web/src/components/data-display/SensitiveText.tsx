import { EyeInvisibleOutlined, EyeOutlined } from '@ant-design/icons'
import { Button, Space, Typography } from 'antd'
import { useState } from 'react'

function mask(value: string) {
  if (value.length <= 4) return '*'.repeat(value.length)
  if (value.length <= 8) return `${value.slice(0, 2)}****${value.slice(-2)}`
  return `${value.slice(0, 3)}****${value.slice(-4)}`
}

export function SensitiveText({ value = '' }: { value?: string }) {
  const [visible, setVisible] = useState(false)
  if (!value) return <>-</>
  return (
    <Space size={4}>
      <Typography.Text>{visible ? value : mask(value)}</Typography.Text>
      <Button
        type="text"
        size="small"
        aria-label={visible ? '隐藏敏感信息' : '显示敏感信息'}
        icon={visible ? <EyeInvisibleOutlined /> : <EyeOutlined />}
        onClick={() => setVisible((current) => !current)}
      />
    </Space>
  )
}
