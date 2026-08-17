import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest'
import { setupServer } from 'msw/node'
import { getJSON } from './http'
import { createValidatedMicroAppManifest } from './manifest'

describe('micro-app test factory', () => {
  it('produces a manifest accepted by the production validator', () => {
    const manifest = createValidatedMicroAppManifest({}, {
      shellOrigin: 'https://forge.bank.example',
      production: true,
      trustedWujieOrigins: ['https://apps.bank.example'],
    })
    expect(manifest.name).toBe('example-remote')
  })
})

const server = setupServer(getJSON('https://forge.test/api/v1/status', { status: 'UP' }))

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('MSW helpers', () => {
  it('returns the Forge API envelope', async () => {
    const response = await fetch('https://forge.test/api/v1/status')
    const body = await response.json()
    expect(response.status).toBe(200)
    expect(body).toMatchObject({
      code: '000000',
      data: { status: 'UP' },
      request_id: 'test-request-id',
    })
  })
})
