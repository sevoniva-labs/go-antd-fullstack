export interface BrowserErrorEvent {
  source: 'react-boundary' | 'unhandled-error' | 'unhandled-rejection'
  message: string
  stack?: string
  path: string
  occurredAt: string
}

const sensitiveAssignment = /\b(authorization|cookie|password|passwd|secret|token|api[_-]?key)\b\s*[:=]\s*([^\s,;]+)/gi
const bearerCredential = /\bbearer\s+[a-z0-9._~+/=-]+/gi

function safeText(value: string | undefined, maximum: number) {
  if (!value) return undefined
  return value
    .replace(bearerCredential, 'Bearer [REDACTED]')
    .replace(sensitiveAssignment, '$1=[REDACTED]')
    .slice(0, maximum)
}

// The foundation intentionally does not ship a vendor browser APM SDK.
// Product projects can bridge this stable DOM event to OpenTelemetry/Sentry/other approved tooling.
export function reportBrowserError(source: BrowserErrorEvent['source'], error: unknown) {
  const value = error instanceof Error ? error : new Error(String(error))
  const detail: BrowserErrorEvent = {
    source,
    message: safeText(value.message, 2048) ?? 'frontend error',
    stack: safeText(value.stack, 8192),
    path: window.location.pathname,
    occurredAt: new Date().toISOString(),
  }
  window.dispatchEvent(new CustomEvent<BrowserErrorEvent>('forge:frontend-error', { detail }))
}
