import { Button, Popconfirm, type ButtonProps } from 'antd'
import type { ReactNode } from 'react'

export function ConfirmAction({
  title,
  description,
  children,
  onConfirm,
  danger,
  buttonProps,
}: {
  title: ReactNode
  description?: ReactNode
  children?: ReactNode
  onConfirm: () => void | Promise<void>
  danger?: boolean
  buttonProps?: ButtonProps
}) {
  const trigger = children ?? <Button danger={danger} {...buttonProps} />
  return <Popconfirm title={title} description={description} onConfirm={onConfirm} okButtonProps={{ danger }}>{trigger}</Popconfirm>
}
