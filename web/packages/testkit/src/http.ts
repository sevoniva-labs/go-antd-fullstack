import { HttpResponse, http, type RequestHandler } from 'msw'

export interface TestApiEnvelope<T> {
  code: string
  message: string
  data: T
  request_id: string
  trace_id: string
  timestamp: string
}

export function apiEnvelope<T>(data: T, code = '000000', message = 'success'): TestApiEnvelope<T> {
  return {
    code,
    message,
    data,
    request_id: 'test-request-id',
    trace_id: 'test-trace-id',
    timestamp: '2026-08-17T00:00:00Z',
  }
}

export function getJSON<T>(url: string, data: T, status = 200): RequestHandler {
  return http.get(url, () => HttpResponse.json(apiEnvelope(data), { status }))
}

export function postJSON<T>(url: string, data: T, status = 200): RequestHandler {
  return http.post(url, () => HttpResponse.json(apiEnvelope(data), { status }))
}

export function unauthorized(url: string): RequestHandler {
  return http.all(url, () => HttpResponse.json(
    apiEnvelope(null, '200001', 'authentication required'),
    { status: 401 },
  ))
}
