import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { StrictMode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useLive } from '../lib/live'
import { LiveProvider } from './LiveProvider'

// FakeSocket stands in for the browser's WebSocket; the test opens and
// closes it by hand.
class FakeSocket {
  static OPEN = 1
  static all: FakeSocket[] = []
  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  constructor(public url: string) {
    FakeSocket.all.push(this)
  }
  send(data: string) {
    this.sent.push(data)
  }
  close() {} // the real close event comes later, sent by the test
  open() {
    this.readyState = FakeSocket.OPEN
    this.onopen?.()
  }
  closed() {
    this.readyState = 3
    this.onclose?.()
  }
}

function TypingButton() {
  const live = useLive()
  return (
    <button type="button" onClick={() => live.sendTyping('m1')}>
      type
    </button>
  )
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

function renderProvider(queryClient = new QueryClient()) {
  render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <LiveProvider>
          <TypingButton />
        </LiveProvider>
      </QueryClientProvider>
    </StrictMode>,
  )
}

describe('LiveProvider', () => {
  it('keeps sending on the new socket when an old one closes late', () => {
    renderProvider()
    // StrictMode mounts twice: the first socket is replaced by a second.
    const [first, second] = FakeSocket.all
    second.open()
    act(() => first.closed())

    fireEvent.click(screen.getByRole('button', { name: 'type' }))
    expect(second.sent).toEqual(['{"type":"typing","match_id":"m1"}'])
  })

  it('reconnects after a drop and reloads what may have changed', () => {
    vi.useFakeTimers()
    const queryClient = new QueryClient()
    const reload = vi.spyOn(queryClient, 'invalidateQueries')
    renderProvider(queryClient)
    const socket = FakeSocket.all.at(-1)!
    socket.open()
    expect(reload).not.toHaveBeenCalled()

    act(() => socket.closed())
    act(() => vi.advanceTimersByTime(1000))
    const again = FakeSocket.all.at(-1)!
    expect(again).not.toBe(socket)
    again.open()
    expect(reload.mock.calls.map((call) => call[0]?.queryKey)).toEqual([['matches'], ['unread'], ['match'], ['messages']])
  })
})
