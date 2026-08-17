export interface BrowserErrorEvent {
  source: 'react-boundary' | 'unhandled-error' | 'unhandled-rejection'
  message: string
  stack?: string
  path: string
  occurredAt: string
}

// The foundation intentionally does not ship a vendor browser APM SDK.
// Product projects can bridge this stable DOM event to OpenTelemetry/Sentry/other approved tooling.
export function reportBrowserError(source: BrowserErrorEvent['source'], error: unknown) {
  const value = error instanceof Error ? error : new Error(String(error))
  const detail: BrowserErrorEvent = {
    source,
    message: value.message,
    stack: value.stack,
    path: window.location.pathname,
    occurredAt: new Date().toISOString(),
  }
  window.dispatchEvent(new CustomEvent<BrowserErrorEvent>('forge:frontend-error', { detail }))
}
