import { Card, Flex } from 'antd'
import type { ReactNode } from 'react'

export function SearchToolbar({ filters, actions }: { filters?: ReactNode; actions?: ReactNode }) {
  return (
    <Card size="small" className="search-toolbar">
      <Flex gap={12} wrap="wrap" justify="space-between" align="center">
        <div className="search-toolbar-filters">{filters}</div>
        <div className="search-toolbar-actions">{actions}</div>
      </Flex>
    </Card>
  )
}
