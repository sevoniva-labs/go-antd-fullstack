import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiFetch, configureApiClient } from './client'

afterEach(() => {
  vi.unstubAllGlobals()
  configureApiClient({ baseUrl: '/api/v1' })
})

describe('configureApiClient', () => {
  it('rejects credential exfiltration through a cross-origin runtime config', () => {
    expect(() => configureApiClient({ baseUrl: 'https://evil.example/api' })).toThrow(/Shell origin/)
    expect(() => configureApiClient({ baseUrl: '/api?redirect=evil' })).toThrow(/query/)
  })

  it('uses a normalized same-origin API path', async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({
      code: '000000', message: 'success', data: { ok: true }, timestamp: '2026-08-17T00:00:00Z',
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)
    configureApiClient({ baseUrl: '/gateway/api/' })
    await expect(apiFetch<{ ok: boolean }>('/health')).resolves.toEqual({ ok: true })
    expect(fetchMock).toHaveBeenCalledWith('/gateway/api/health', expect.objectContaining({ credentials: 'include' }))
  })
})
