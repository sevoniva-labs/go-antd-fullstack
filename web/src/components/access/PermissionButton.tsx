import { Button, type ButtonProps } from 'antd'
import { Access } from './Access'

export function PermissionButton({ permission, ...props }: ButtonProps & { permission: string }) {
  return <Access permission={permission}><Button {...props} /></Access>
}
