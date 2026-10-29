import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChatMessage, LiveEvent } from '../../api/types'
import { api } from '../../lib/api'
import { LiveContext, type Listener } from '../../lib/live'
import { Chat } from './Chat'

vi.mock('../../lib/api', () => ({
  api: { listMessages: vi.fn(), sendMessage: vi.fn(), markRead: vi.fn(), deleteMessage: vi.fn() },
}))

const msg = (id: string, sender: string, body: string, at: string): ChatMessage => ({
  id, match_id: 'm1', sender_id: sender, client_id: id, body, created_at: at, deleted: false,
})

let listeners: Listener[] = []
const live = {
  subscribe: (fn: Listener) => {
    listeners.push(fn)
    return () => {
      listeners = listeners.filter((l) => l !== fn)
    }
  },
  sendTyping: vi.fn(),
}

beforeEach(() => {
  listeners = []
  vi.mocked(api.markRead).mockResolvedValue(undefined)
  vi.mocked(api.listMessages).mockResolvedValue({
    messages: [
      msg('a', 'other', 'Hello Jordan', '2026-10-29T10:00:00.1Z'),
      msg('b', 'me', 'Hi Ana, thanks!', '2026-10-29T10:01:00.12Z'),
    ],
    has_more: false,
    other_read_at: '2026-10-29T10:02:00Z',
  })
})
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function renderChat() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <LiveContext.Provider value={live}>
        <Chat matchId="m1" myId="me" otherName="Ana" />
      </LiveContext.Provider>
    </QueryClientProvider>,
  )
}

function type(text: string) {
  const box = screen.getByRole('textbox', { name: 'Message' })
  fireEvent.change(box, { target: { value: text } })
  fireEvent.keyDown(box, { key: 'Enter' })
}

describe('Chat', () => {
  it('shows the chat, marks it read and shows Seen under my last message', async () => {
    renderChat()
    expect(await screen.findByText('Hello Jordan')).toBeTruthy()
    const bubbles = screen.getAllByTestId('message')
    expect(bubbles.map((b) => b.textContent?.includes('Seen'))).toEqual([false, true])
    expect(bubbles[0].textContent).not.toContain('Delete')
    expect(api.markRead).toHaveBeenCalledWith('m1')
  })

  it('sends with Enter, and retries a failed send with the same client id', async () => {
    vi.mocked(api.sendMessage)
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce(msg('c', 'me', 'Monday works', '2026-10-29T10:05:00Z'))
    renderChat()
    await screen.findByText('Hello Jordan')

    type('Monday works')
    expect(await screen.findByText('Not sent. Check your connection.')).toBeTruthy()
    expect((screen.getByRole('textbox', { name: 'Message' }) as HTMLTextAreaElement).value).toBe('')

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(screen.queryByText('Not sent. Check your connection.')).toBeNull())
    const ids = vi.mocked(api.sendMessage).mock.calls.map((call) => call[2])
    expect(ids).toHaveLength(2)
    expect(ids[0]).toBe(ids[1])
    expect(screen.getAllByTestId('message')).toHaveLength(3)
  })

  it('deletes my message only after confirming', async () => {
    vi.mocked(api.deleteMessage).mockResolvedValue(undefined)
    renderChat()
    await screen.findByText('Hi Ana, thanks!')

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'No' }))
    expect(api.deleteMessage).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'Yes' }))
    expect(await screen.findByText('Message deleted')).toBeTruthy()
    expect(api.deleteMessage).toHaveBeenCalledWith('b')
  })

  it('shows when the other person is typing, and sends my typing', async () => {
    renderChat()
    await screen.findByText('Hello Jordan')
    act(() => listeners.forEach((fn) => fn({ type: 'typing', match_id: 'm1' } as LiveEvent)))
    expect(screen.getByText('Ana is typing...')).toBeTruthy()

    fireEvent.change(screen.getByRole('textbox', { name: 'Message' }), { target: { value: 'H' } })
    expect(live.sendTyping).toHaveBeenCalledWith('m1')
  })

  it('refuses a file over 10 MB before sending', async () => {
    renderChat()
    await screen.findByText('Hello Jordan')
    const big = new File([new Uint8Array(10 * 1024 * 1024 + 1)], 'huge.pdf', { type: 'application/pdf' })
    fireEvent.change(screen.getByLabelText('Attach a file'), { target: { files: [big] } })
    expect(screen.getByRole('alert').textContent).toBe('Files must be 10 MB or smaller.')
    expect(screen.queryByText(/huge\.pdf/)).toBeNull()
  })
})
