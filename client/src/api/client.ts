// No DOM or Vite APIs in this folder, so React Native can reuse it.
import type {
  CardPreview,
  CardSettings,
  CV,
  Health,
  HiringCard,
  HiringFields,
  HiringPreview,
  Preferences,
  Profile,
  User,
} from './types'

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
  // body is sent as JSON, or as a multipart form when it is FormData.
  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const isForm = body instanceof FormData
    const res = await fetch(baseUrl + path, {
      method,
      credentials: 'include',
      headers: body === undefined || isForm ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : isForm ? body : JSON.stringify(body),
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

    // Resolves to null when the candidate has not uploaded a CV yet.
    getProfile: () =>
      request<Profile>('GET', '/v1/profile').catch((err) => {
        if (err instanceof ApiError && err.status === 404) {
          return null
        }
        throw err
      }),

    uploadCV: (file: Blob) => {
      const form = new FormData()
      form.append('cv', file)
      return request<Profile>('POST', '/v1/profile/cv', form)
    },

    updateProfile: (cv: CV, preferences: Preferences, card_settings: CardSettings) =>
      request<Profile>('PUT', '/v1/profile', { cv, preferences, card_settings }),

    uploadPhoto: (file: Blob) => {
      const form = new FormData()
      form.append('photo', file)
      return request<Profile>('POST', '/v1/profile/photo', form)
    },

    getCard: () => request<CardPreview>('GET', '/v1/card'),
    finishCard: () => request<Profile>('POST', '/v1/card/finish'),

    updateMe: (first_name: string) => request<User>('PUT', '/v1/me', { first_name }),

    listHiringCards: () => request<HiringCard[]>('GET', '/v1/hiring-cards'),
    createHiringCard: (description: string) => request<HiringCard>('POST', '/v1/hiring-cards', { description }),
    getHiringCard: (id: string) => request<HiringCard>('GET', `/v1/hiring-cards/${id}`),
    updateHiringCard: (id: string, fields: HiringFields) => request<HiringCard>('PUT', `/v1/hiring-cards/${id}`, fields),
    setHiringCardStatus: (id: string, status: 'live' | 'closed') =>
      request<HiringCard>('PUT', `/v1/hiring-cards/${id}/status`, { status }),
    uploadHiringCardImage: (id: string, file: Blob) => {
      const form = new FormData()
      form.append('image', file)
      return request<HiringCard>('POST', `/v1/hiring-cards/${id}/image`, form)
    },
    previewHiringCard: (id: string) => request<HiringPreview>('GET', `/v1/hiring-cards/${id}/preview`),
  }
}
