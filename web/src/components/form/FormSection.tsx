import { Divider, Typography } from 'antd'
import type { ReactNode } from 'react'

export function FormSection({ title, description, children }: { title: ReactNode; description?: ReactNode; children: ReactNode }) {
  return (
    <section className="form-section">
      <div className="form-section-heading">
        <Typography.Text strong>{title}</Typography.Text>
        {description && <Typography.Text type="secondary">{description}</Typography.Text>}
      </div>
      <Divider />
      {children}
    </section>
  )
}
