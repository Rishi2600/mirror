import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useVisibleTime } from './useVisibleTime'

let visibility = 'visible'

function setVisibility(value: string) {
  visibility = value
  act(() => {
    document.dispatchEvent(new Event('visibilitychange'))
  })
}

beforeEach(() => {
  visibility = 'visible'
  vi.useFakeTimers()
  vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility as DocumentVisibilityState)
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('useVisibleTime', () => {
  it('counts only the time the page is visible', () => {
    const { result } = renderHook(() => useVisibleTime('card-a'))
    vi.advanceTimersByTime(2000)
    expect(result.current()).toBe(2000)

    setVisibility('hidden')
    vi.advanceTimersByTime(60000)
    expect(result.current()).toBe(2000)

    setVisibility('visible')
    vi.advanceTimersByTime(500)
    expect(result.current()).toBe(2500)
  })

  it('starts again for the next card', () => {
    const { result, rerender } = renderHook(({ key }) => useVisibleTime(key), { initialProps: { key: 'card-a' } })
    vi.advanceTimersByTime(3000)
    rerender({ key: 'card-b' })
    vi.advanceTimersByTime(400)
    expect(result.current()).toBe(400)
  })
})
