import { expect, it, vi } from 'vitest'
import { reportBrowserError, type BrowserErrorEvent } from './browser'

it('redacts credentials and excludes the URL query from browser error events', () => {
  window.history.replaceState({}, '', '/admin/users?token=must-not-leak')
  const listener = vi.fn<(event: Event) => void>()
  window.addEventListener('forge:frontend-error', listener, { once: true })

  reportBrowserError('unhandled-error', new Error('request failed Authorization=Bearer.secret password=hunter2'))

  expect(listener).toHaveBeenCalledOnce()
  const event = listener.mock.calls[0]?.[0] as CustomEvent<BrowserErrorEvent>
  expect(event.detail.path).toBe('/admin/users')
  expect(event.detail.message).not.toContain('hunter2')
  expect(event.detail.message).not.toContain('Bearer.secret')
  expect(event.detail.message).toContain('[REDACTED]')
})
