import type { ErrorBody } from './types'

/**
 * Token storage key. Shared by the API client (it reads the token for the
 * `Authorization` header) and by `AuthContext` (it writes/removes the token).
 */
export const TOKEN_STORAGE_KEY = 'werkstatt.token'
export const EMPLOYEE_STORAGE_KEY = 'werkstatt.employee'

const configuredBaseUrl = import.meta.env.VITE_API_BASE_URL
const RAW_BASE_URL =
  configuredBaseUrl && !String(configuredBaseUrl).includes('${')
    ? String(configuredBaseUrl)
    : 'http://localhost:8080'
const API_BASE_URL = RAW_BASE_URL.replace(/\/+$/, '')

/** Uniform error thrown by `apiFetch` for every non-2xx response and for network failures. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export function getStoredToken(): string | null {
  try {
    return window.localStorage.getItem(TOKEN_STORAGE_KEY)
  } catch {
    return null
  }
}

/**
 * Builds the absolute URL for an API call. Callers pass a path relative to the
 * API root (e.g. `/auth/login` or `/workshop/orders`); the `/api` prefix is
 * added automatically unless the caller already stated it.
 */
export function resolveApiUrl(path: string): string {
  let suffix = path.startsWith('/') ? path : `/${path}`
  if (!suffix.startsWith('/api/') && suffix !== '/api') {
    suffix = `/api${suffix}`
  }
  return `${API_BASE_URL}${suffix}`
}

function isWorkshopPath(path: string): boolean {
  const suffix = path.startsWith('/') ? path : `/${path}`
  return suffix.includes('/workshop') || suffix.startsWith('/workshop')
}

async function readError(response: Response): Promise<ErrorBody> {
  try {
    const body = (await response.json()) as Partial<ErrorBody>
    if (body && typeof body.code === 'string' && typeof body.message === 'string') {
      return { code: body.code, message: body.message }
    }
  } catch {
    /* fall through to the generic body below */
  }
  return { code: `http_${response.status}`, message: response.statusText || 'Unbekannter Fehler' }
}

/**
 * Single entry point for every API call. Attaches the bearer token for the
 * protected `/workshop/*` routes, parses the uniform error body
 * `{ code, message }` and throws an `ApiError` on failure.
 */
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json')
  }
  if (init?.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getStoredToken()
  if (token && isWorkshopPath(path) && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  let response: Response
  try {
    response = await fetch(resolveApiUrl(path), { ...init, headers })
  } catch {
    throw new ApiError(0, 'network_error', 'Die Verbindung zum Server ist fehlgeschlagen.')
  }

  if (response.status === 204) {
    return undefined as T
  }

  if (!response.ok) {
    const error = await readError(response)
    throw new ApiError(response.status, error.code, error.message)
  }

  if (response.status === 205 || response.headers.get('content-length') === '0') {
    return undefined as T
  }

  return (await response.json()) as T
}
