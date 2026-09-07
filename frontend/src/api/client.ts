const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8081'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

interface Envelope<T> {
  success: boolean
  data: T | null
  error: string | null
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })

  let envelope: Envelope<T>
  try {
    envelope = (await res.json()) as Envelope<T>
  } catch {
    throw new ApiError(res.status, `unexpected non-JSON response (status ${res.status})`)
  }

  if (!envelope.success) {
    throw new ApiError(res.status, envelope.error ?? 'unknown error')
  }
  if (envelope.data === null) {
    throw new ApiError(res.status, 'server returned success with no data')
  }
  return envelope.data
}

export function getJSON<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'GET' })
}

export function postJSON<T>(path: string, body: unknown): Promise<T> {
  return request<T>(path, { method: 'POST', body: JSON.stringify(body) })
}

export function putJSON<T>(path: string, body: unknown): Promise<T> {
  return request<T>(path, { method: 'PUT', body: JSON.stringify(body) })
}

export function deleteJSON<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' })
}
