import { EyeInvisibleOutlined, EyeOutlined } from '@ant-design/icons'
import { Button, Space, Typography } from 'antd'
import { useState } from 'react'

export function SecretText({ value, visibleChars = 4 }: { value?: string; visibleChars?: number }) {
  const [visible, setVisible] = useState(false)
  if (!value) return <>-</>
  const masked = value.length <= visibleChars ? '••••••••' : `${value.slice(0, visibleChars)}••••••••`
  return (
    <Space size={4}>
      <Typography.Text code copyable={visible ? { text: value } : false}>{visible ? value : masked}</Typography.Text>
      <Button
        type="text"
        size="small"
        aria-label={visible ? '隐藏密钥' : '显示密钥'}
        icon={visible ? <EyeInvisibleOutlined /> : <EyeOutlined />}
        onClick={() => setVisible((current) => !current)}
      />
    </Space>
  )
}
