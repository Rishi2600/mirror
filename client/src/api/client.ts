// No DOM or Vite APIs in this folder, so React Native can reuse it.
import type {
  CardPreview,
  CardSettings,
  CV,
  Health,
  HiringCard,
  HiringFields,
  HiringPreview,
  ChatMessage,
  Match,
  MessagesPage,
  Preferences,
  Profile,
  Selectivity,
  SwipeResult,
  TimelineCandidate,
  TimelineJob,
  Unread,
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
  async function request<T>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}): Promise<T> {
    const isForm = body instanceof FormData
    const res = await fetch(baseUrl + path, {
      method,
      credentials: 'include',
      headers: body === undefined || isForm ? headers : { 'Content-Type': 'application/json', ...headers },
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
    verifyCode: (email: string, code: string, time_zone: string) =>
      request<User>('POST', '/v1/auth/otp/verify', { email, code, time_zone }),
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

    getTimeline: () => request<TimelineJob[]>('GET', '/v1/timeline'),

    // key makes a retried swipe return the first answer instead of a new swipe.
    swipe: (swipe: { hiring_card_id: string; candidate_id?: string; direction: 'left' | 'right'; dwell_ms?: number }, key: string) =>
      request<SwipeResult>('POST', '/v1/swipes', swipe, { 'Idempotency-Key': key }),
    getSelectivity: () => request<Selectivity>('GET', '/v1/selectivity'),
    listMatches: () => request<Match[]>('GET', '/v1/matches'),
    getMatch: (id: string) => request<Match>('GET', `/v1/matches/${id}`),
    unmatch: (id: string) => request<void>('DELETE', `/v1/matches/${id}`),

    // before is the oldest message already loaded; empty for the newest page.
    listMessages: (matchId: string, before = '') =>
      request<MessagesPage>('GET', `/v1/matches/${matchId}/messages${before ? `?before=${before}` : ''}`),
    // client_id makes a retried send save the message once.
    sendMessage: (matchId: string, body: string, client_id: string, file?: Blob, fileName?: string) => {
      if (!file) {
        return request<ChatMessage>('POST', `/v1/matches/${matchId}/messages`, { body, client_id })
      }
      const form = new FormData()
      form.append('body', body)
      form.append('client_id', client_id)
      form.append('file', file, fileName)
      return request<ChatMessage>('POST', `/v1/matches/${matchId}/messages`, form)
    },
    markRead: (matchId: string) => request<void>('POST', `/v1/matches/${matchId}/read`),
    deleteMessage: (id: string) => request<void>('DELETE', `/v1/messages/${id}`),
    getUnread: () => request<Unread>('GET', '/v1/unread'),
    getHiringCardTimeline: (id: string) => request<TimelineCandidate[]>('GET', `/v1/hiring-cards/${id}/timeline`),
  }
}
