// No DOM or Vite APIs in this folder, so React Native can reuse it.
import createClient from 'openapi-fetch'
import type { paths } from './schema.gen'

export type ApiClient = ReturnType<typeof createApiClient>

// An empty baseUrl means same origin. Credentials carry the session cookie.
export function createApiClient(baseUrl: string) {
  return createClient<paths>({ baseUrl, credentials: 'include' })
}
