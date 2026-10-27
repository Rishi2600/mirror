import { createApiClient } from '../api/client'

// The app's API client, pointed at VITE_API_URL (empty means same origin).
export const api = createApiClient(import.meta.env.VITE_API_URL ?? '')
