import { ApiError } from '../api/client'
import type { SwipeResult } from '../api/types'
import { api } from './api'

type SwipeBody = { hiring_card_id: string; candidate_id?: string; direction: 'left' | 'right' }

// sendSwipe sends one swipe. A network failure is retried with the same
// key, so the server never counts it twice. A 409 (already swiped) means an
// earlier try got through, so it is not an error.
export async function sendSwipe(swipe: SwipeBody, tries = 3): Promise<SwipeResult | null> {
  const key = crypto.randomUUID()
  for (let attempt = 1; ; attempt++) {
    try {
      return await api.swipe(swipe, key)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409) {
          return null
        }
        throw err // the server answered; trying again would not help
      }
      if (attempt >= tries) {
        throw err
      }
      await new Promise((resolve) => setTimeout(resolve, 1000 * attempt))
    }
  }
}
