import { ProTable, type ProTableProps } from '@ant-design/pro-components'

export function AppProTable<T extends Record<string, any>, U extends Record<string, any> = Record<string, any>>(
  props: ProTableProps<T, U>,
) {
  return (
    <ProTable<T, U>
      cardBordered
      scroll={{ x: 'max-content' }}
      pagination={{ defaultPageSize: 20, showSizeChanger: true, showQuickJumper: true, ...props.pagination }}
      options={{ density: true, fullScreen: true, reload: true, setting: true, ...props.options }}
      {...props}
    />
  )
}
