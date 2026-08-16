import { runtimeConfig } from '../app/config/runtime'

export interface ApiEnvelope<T = unknown> {
  code: string
  message: string
  data: T
  error?: string
  request_id?: string
  trace_id?: string
  timestamp: string
  field_errors?: Array<{ path: string; code: string; message?: string }>
}

export class ApiError extends Error {
  status: number
  code: string
  errorCode?: string
  requestId?: string
  traceId?: string
  fieldErrors?: Array<{ path: string; code: string; message?: string }>

  constructor(status: number, body?: ApiEnvelope<unknown>) {
    super(body?.message ?? `HTTP ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.code = body?.code ?? '900099'
    this.errorCode = body?.error
    this.requestId = body?.request_id
    this.traceId = body?.trace_id
    this.fieldErrors = body?.field_errors
  }
}

function csrfToken(): string {
  const item = document.cookie
    .split(';')
    .map((x) => x.trim())
    .find((x) => x.startsWith('forge_csrf='))
  return item ? decodeURIComponent(item.slice('forge_csrf='.length)) : ''
}

function endpoint(path: string) {
  const base = runtimeConfig.apiBaseUrl.replace(/\/+$/, '')
  return `${base}${path.startsWith('/') ? path : `/${path}`}`
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) {
    const csrf = csrfToken()
    if (csrf) headers.set('X-CSRF-Token', csrf)
  }

  const response = await fetch(endpoint(path), {
    ...init,
    method,
    headers,
    credentials: 'include',
  })

  const text = await response.text()
  let envelope: ApiEnvelope<T> | undefined
  if (text) {
    try {
      envelope = JSON.parse(text) as ApiEnvelope<T>
    } catch {
      envelope = undefined
    }
  }

  if (!response.ok) {
    const error = new ApiError(response.status, envelope as ApiEnvelope<unknown> | undefined)
    if (response.status === 401 && path !== '/auth/login') {
      window.dispatchEvent(new CustomEvent('forge:unauthorized'))
    }
    throw error
  }
  if (!envelope) return undefined as T
  if (envelope.code !== '000000') throw new ApiError(response.status, envelope as ApiEnvelope<unknown>)
  return envelope.data
}
