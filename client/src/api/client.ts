// No DOM or Vite APIs in this folder, so React Native can reuse it.
import type { Health } from './types'

export type ApiClient = ReturnType<typeof createApiClient>

// An empty baseUrl means same origin. Credentials carry the session cookie.
export function createApiClient(baseUrl: string) {
  async function get<T>(path: string): Promise<T> {
    const res = await fetch(baseUrl + path, { credentials: 'include' })
    if (!res.ok) {
      throw new Error(`API answered ${res.status}`)
    }
    return res.json() as Promise<T>
  }

  return {
    getHealth: () => get<Health>('/v1/healthz'),
  }
}
