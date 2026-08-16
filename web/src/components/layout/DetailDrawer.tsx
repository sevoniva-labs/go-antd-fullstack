import { Drawer, type DrawerProps } from 'antd'
import type { ReactNode } from 'react'

export function DetailDrawer({ title, open, onClose, children, width = 720, ...props }: {
  title: ReactNode
  open: boolean
  onClose: () => void
  children: ReactNode
  width?: number | string
} & Omit<DrawerProps, 'title' | 'open' | 'onClose' | 'children' | 'width'>) {
  return <Drawer title={title} open={open} onClose={onClose} width={width} destroyOnHidden {...props}>{children}</Drawer>
}
