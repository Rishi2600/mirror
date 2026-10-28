import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { api } from './api'
import { sendSwipe } from './sendSwipe'

vi.mock('./api', () => ({ api: { swipe: vi.fn() } }))

const like = { hiring_card_id: 'card-1', direction: 'right' as const }

afterEach(() => {
  vi.mocked(api.swipe).mockReset()
  vi.useRealTimers()
})

describe('sendSwipe', () => {
  it('retries a network failure with the same key', async () => {
    vi.useFakeTimers()
    vi.mocked(api.swipe)
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce({ matched: true, match_id: 'm1' })

    const result = sendSwipe(like)
    await vi.advanceTimersByTimeAsync(1000)

    expect(await result).toEqual({ matched: true, match_id: 'm1' })
    const keys = vi.mocked(api.swipe).mock.calls.map((call) => call[1])
    expect(keys).toHaveLength(2)
    expect(keys[0]).toBe(keys[1])
  })

  it('treats "already swiped" as done', async () => {
    vi.mocked(api.swipe).mockRejectedValue(new ApiError(409, 'already swiped'))
    expect(await sendSwipe(like)).toBeNull()
    expect(api.swipe).toHaveBeenCalledTimes(1)
  })

  it('does not retry other server answers', async () => {
    vi.mocked(api.swipe).mockRejectedValue(new ApiError(403, 'not allowed'))
    await expect(sendSwipe(like)).rejects.toThrow('not allowed')
    expect(api.swipe).toHaveBeenCalledTimes(1)
  })
})
