import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { UNDO_MS, useSwipeQueue } from './useSwipeQueue'

beforeEach(() => {
  vi.useFakeTimers()
})
afterEach(() => {
  vi.useRealTimers()
})

describe('useSwipeQueue', () => {
  it('sends a swipe only after the undo window', () => {
    const onSend = vi.fn()
    const { result } = renderHook(() => useSwipeQueue<string>(onSend))

    act(() => result.current.swipe('card-1', 'right'))
    expect(result.current.waiting).toEqual({ item: 'card-1', direction: 'right' })
    act(() => vi.advanceTimersByTime(UNDO_MS - 1))
    expect(onSend).not.toHaveBeenCalled()

    act(() => vi.advanceTimersByTime(1))
    expect(onSend).toHaveBeenCalledWith({ item: 'card-1', direction: 'right' })
    expect(result.current.waiting).toBeNull()
  })

  it('undo takes the swipe back, and it is never sent', () => {
    const onSend = vi.fn()
    const { result } = renderHook(() => useSwipeQueue<string>(onSend))

    act(() => result.current.swipe('card-1', 'left'))
    let undone = null
    act(() => {
      undone = result.current.undo()
    })
    act(() => vi.advanceTimersByTime(UNDO_MS * 2))

    expect(undone).toEqual({ item: 'card-1', direction: 'left' })
    expect(onSend).not.toHaveBeenCalled()
  })

  it('a new swipe sends the waiting one at once; only the last can be undone', () => {
    const onSend = vi.fn()
    const { result } = renderHook(() => useSwipeQueue<string>(onSend))

    act(() => result.current.swipe('card-1', 'right'))
    act(() => result.current.swipe('card-2', 'left'))
    expect(onSend).toHaveBeenCalledTimes(1)
    expect(onSend).toHaveBeenCalledWith({ item: 'card-1', direction: 'right' })

    act(() => {
      result.current.undo()
    })
    act(() => {
      expect(result.current.undo()).toBeNull()
    })
  })

  it('leaving the page sends the waiting swipe', () => {
    const onSend = vi.fn()
    const { result, unmount } = renderHook(() => useSwipeQueue<string>(onSend))

    act(() => result.current.swipe('card-1', 'right'))
    unmount()
    expect(onSend).toHaveBeenCalledWith({ item: 'card-1', direction: 'right' })
  })
})
