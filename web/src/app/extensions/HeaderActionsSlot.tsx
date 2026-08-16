import type { ReactNode } from 'react'

// Product extension point for notification center, tenant switcher, helpdesk,
// approval inbox or other header actions. Keep the foundation vendor-neutral.
export function HeaderActionsSlot(): ReactNode {
  return null
}
