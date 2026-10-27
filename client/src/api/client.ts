// No DOM or Vite APIs in this folder, so React Native can reuse it.
import type { Health, User } from './types'

export type ApiClient = ReturnType<typeof createApiClient>

// ApiError carries the HTTP status and the server's error message.
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// An empty baseUrl means same origin. Credentials carry the session cookie.
export function createApiClient(baseUrl: string) {
  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await fetch(baseUrl + path, {
      method,
      credentials: 'include',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => null)
      throw new ApiError(res.status, data?.error ?? `API answered ${res.status}`)
    }
    if (res.status === 204) {
      return undefined as T
    }
    return res.json() as Promise<T>
  }

  return {
    getHealth: () => request<Health>('GET', '/v1/healthz'),

    // Resolves to null when nobody is signed in.
    getMe: () =>
      request<User>('GET', '/v1/me').catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          return null
        }
        throw err
      }),

    requestCode: (email: string) => request<void>('POST', '/v1/auth/otp/request', { email }),
    verifyCode: (email: string, code: string) =>
      request<User>('POST', '/v1/auth/otp/verify', { email, code }),
    setRole: (role: 'candidate' | 'employer') => request<User>('POST', '/v1/me/role', { role }),
    logout: () => request<void>('POST', '/v1/auth/logout'),
  }
}
