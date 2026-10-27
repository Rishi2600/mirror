// The typed API client. This folder must not depend on the DOM or on Vite,
// so a React Native app can copy the pattern.
import createClient from 'openapi-fetch'
import type { paths } from './schema.gen'

export type ApiClient = ReturnType<typeof createApiClient>

// createApiClient returns a client for the API at baseUrl. An empty baseUrl
// means the same origin the app is served from. Credentials are always sent,
// so the session cookie reaches the API.
export function createApiClient(baseUrl: string) {
  return createClient<paths>({ baseUrl, credentials: 'include' })
}
